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

function serializeQueryParams(params) {
    const result = [];

    for (const key in params) {
        const encodedKey = encodeURIComponent(key);
        const arrValue = Array.isArray(params[key]) ? params[key] : [params[key]];

        for (let v of arrValue) {
            v = prepareQueryParamValue(v);
            if (v === null) {
                continue;
            }
            result.push(encodedKey + "=" + v);
        }
    }

    return result.join("&");
}

function prepareQueryParamValue(value) {
    if (value === null || typeof value === "undefined") {
        return null;
    }

    if (value instanceof Date) {
        return encodeURIComponent(value.toISOString().replace("T", " "));
    }

    if (typeof value === "object") {
        return encodeURIComponent(JSON.stringify(value));
    }

    return encodeURIComponent(value);
}
