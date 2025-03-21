class ClientResponseError extends Error {
    /**
     * Creates a standardized error object for HTTP client requests
     * @param {Object} errData - Error data object
     * @param {string} [errData.url] - The URL that caused the error
     * @param {number} [errData.status] - HTTP status code
     * @param {boolean} [errData.isAbort] - Whether the request was aborted
     * @param {Error} [errData.originalError] - Original error object
     * @param {Object} [errData.response] - Response data from the server
     */
    constructor(errData) {
        // Pass the message to parent Error constructor if available
        super(errData?.response?.message || "ClientResponseError");
        
        this.url = errData?.url || "";
        this.status = errData?.status || 0;
		// this is only meaninfull with a requestJson with AbortController
        this.isAbort = Boolean(errData?.isAbort);
        this.originalError = errData?.originalError;
        this.response = errData?.response || {};
        this.name = "ClientResponseError " + this.status;
        this.message = this.response?.message; // Prioritize the server's message

        if (!this.message) {
            if (this.isAbort) {
                this.message = "The request was autocancelled.";
            } else if (this.originalError?.cause?.message?.includes("ECONNREFUSED")) {
                this.message = "Failed to connect to the server";
            } else {
                this.message = "Something went wrong while processing your request.";
            }
        }
    }
}

function requestJson(
    baseUrl,
    path,
    method = "GET",
    queryParams = {},
    body = null,
    headers = {}
) {
    let url = buildUrl(baseUrl, path);

    const serializedQueryParams = serializeQueryParams(queryParams);
    if (serializedQueryParams) {
        url += (url.includes("?") ? "&" : "?") + serializedQueryParams;
    }

    const requestHeaders = {
        "Content-Type": "application/json",
        ...headers // Allow overriding
    };

    return fetch(url, {
        method,
        headers: requestHeaders,
        body: body ? JSON.stringify(body) : null,
    })
    .then(response => {
        // Check for non-2xx status *before* parsing JSON
        if (!response.ok) {
            // Try to parse JSON error, but be resilient to non-JSON errors
            return response.text().then(text => {
                let parsedError = {};
                try {
                    parsedError = JSON.parse(text);
                } catch (_) {
                    // If parsing fails, use the raw text as the message
                    parsedError = { message: text || "Unknown error" };
                }
                throw new ClientResponseError({
                    url: response.url,
                    status: response.status,
                    response: parsedError,
                });
            });
        }

        // Handle 204 No Content (and similar) gracefully.
        if (response.status === 204) {
            return {}; // Return empty object for no-content
        }
        return response.json().catch(() => {
            // Handle json in case of not json.
            throw new ClientResponseError({
                url: response.url,
                status: response.status,
                response: { message: "Invalid JSON response" },
            });
        });
    })
    .catch(error => {
        // Ensure *all* errors are wrapped in ClientResponseError
        if (error instanceof ClientResponseError) {
            throw error; // Already a ClientResponseError, re-throw
        }
        throw new ClientResponseError({
            url: error.url,
            status: error.status,
            originalError: error,
            response: { message: error.message }
        });
    });
}

function requestJsonAuth(
    baseUrl,
    path,
    method = "GET",
    queryParams = {},
    body = null,
    headers = {}
) {
    // Get access token from localStorage
    const token = localStorage.getItem('access_token');
    
    // Add Authorization header
    const authHeaders = {
        ...headers,
        'Authorization': `Bearer ${token}`
    };
    
    // Call original function with enhanced headers
    return requestJson(baseUrl, path, method, queryParams, body, authHeaders);
}

/**
 * Builds a URL by combining baseUrl and path for browser environments.
 * 
 * @param {string} baseUrl - The base URL (absolute or relative)
 * @param {string} [path] - Optional path to append
 * @return {string} The combined URL
 */
function buildUrl(baseUrl, path) {
  // Handle empty baseUrl - use current directory
  if (baseUrl === "") {
    const pathParts = window.location.pathname.split('/');
    pathParts.pop(); // Remove the last part (file or empty string)
    baseUrl = pathParts.join('/') + '/';
  }
  
  // Create full URL, handling relative URLs
  let url;
  if (!baseUrl.startsWith("http://") && !baseUrl.startsWith("https://")) {
    // For relative URLs, use the URL constructor with current location as base
    const base = baseUrl.startsWith("/") ? window.location.origin : window.location.href;
    url = new URL(baseUrl, base).href;
  } else {
    url = baseUrl;
  }
  
  // Add path if provided
  if (path) {
    url = url + (url.endsWith("/") ? "" : "/") + (path.startsWith("/") ? path.substring(1) : path);
  }
  
  return url;
}

/**
 * Serializes an object of parameters into a URL-encoded query string.
 * 
 * This function handles various data types:
 * - Strings, numbers, booleans: directly encoded
 * - Arrays: creates multiple entries with the same parameter name
 * - Date objects: converted to ISO strings with "T" replaced by space
 * - Objects: converted to JSON strings and encoded
 * - null/undefined values: skipped entirely
 * 
 * @param {Object} params - The object containing parameters to serialize
 * @returns {string} URL-encoded query string
 * 
 * @example
 * // Basic parameters
 * serializeQueryParams({ name: "John Doe", age: 30 })
 * // Returns: "name=John%20Doe&age=30"
 * 
 * @example
 * // Array parameters
 * serializeQueryParams({ colors: ["red", "green", "blue"] })
 * // Returns: "colors=red&colors=green&colors=blue"
 * 
 * @example
 * // Object parameters (converted to JSON)
 * serializeQueryParams({ filter: { minPrice: 10, maxPrice: 100 } })
 * // Returns: "filter=%7B%22minPrice%22%3A10%2C%22maxPrice%22%3A100%7D"
 * 
 * @example
 * // Date parameters
 * serializeQueryParams({ created: new Date("2025-03-21T12:00:00Z") })
 * // Returns: "created=2025-03-21%2012%3A00%3A00.000Z"
 * 
 * @example
 * // Handling null values
 * serializeQueryParams({ name: "Test", category: null })
 * // Returns: "name=Test" (category is skipped)
 * 
 * @example
 * // Mixed parameter types
 * serializeQueryParams({
 *   id: 1234,
 *   tags: ["new", "featured"],
 *   metadata: { version: "1.0" },
 *   updated: new Date("2025-03-21")
 * })
 * // Returns a complex query string with all parameters properly encoded
 */
function serializeQueryParams(params) {
    const result = [];

    for (const key in params) {
        const encodedKey = encodeURIComponent(key);
        const values = Array.isArray(params[key]) ? params[key] : [params[key]];

        for (let value of values) {
            // Skip null/undefined values
            if (value === null || value === undefined) {
                continue;
            }
            
            // Format based on value type
            if (value instanceof Date) {
                value = value.toISOString().replace("T", " ");
            } else if (typeof value === "object") {
                value = JSON.stringify(value);
            }
            
            result.push(`${encodedKey}=${encodeURIComponent(value)}`);
        }
    }

    return result.join("&");
}

/**
 * Saves JWT token to localStorage
 * @param {string} token - The JWT token
 */
function saveAccessToken(token) {
  if (!token) {
    throw new Error('Invalid token: token is missing');
  }
  localStorage.setItem('access_token', token);
}

/**
 * Saves user record to localStorage
 * @param {Object} record - The user record object
 */
function saveUserRecord(record) {
  if (!record) {
    throw new Error('Invalid record: record is missing');
  }
  localStorage.setItem('user_record', JSON.stringify(record));
}

/**
 * Handles the email registration response by saving the token and user record
 * @param {Object} data - The JSON data returned from the requestJson method
 */
function handleEmailRegistration(data) {
  // Verify we have the necessary data
  if (!data || !data.token || !data.record) {
    throw new Error('Invalid response data: token or record missing');
  }

  // Save JWT and user record using the specialized functions
  saveJwt(data.token);
  saveUserRecord(data.record);
}

