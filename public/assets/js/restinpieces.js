
class Restinpieces {
    constructor(baseURL = "/", authStore = new LocalAuthStore(), lang = "en-US") {
        this.baseURL = baseURL;
        this.lang = lang;
        //this.authStore = authStore;
        //this.recordServices = {}; // Cache for record services
        //this.enableAutoCancellation = true;
        //this.cancelControllers = {};
    }

    /**
     * Makes an HTTP request with JSON handling and standardized error responses
     * 
     * @param {string} path - The URL path to request
     * @param {string} [method="GET"] - The HTTP method to use
     * @param {Object} [queryParams={}] - Query parameters to include
     * @param {Object|null} [body=null] - Request body (will be JSON.stringified)
     * @param {Object} [headers={}] - Additional request headers
     * @returns {Promise<any>} - Resolves with parsed response JSON
     */
    requestJson(path, method = "GET", queryParams = {}, body = null, headers = {}) {
        let url = this.buildUrl(this.baseURL, path);

        const serializedQueryParams = this.serializeQueryParams(queryParams);
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
     * Makes an authenticated HTTP request by adding Authorization header
     * 
     * @param {string} path - The URL path to request
     * @param {string} [method="GET"] - The HTTP method to use
     * @param {Object} [queryParams={}] - Query parameters to include
     * @param {Object|null} [body=null] - Request body (will be JSON.stringified)
     * @param {Object} [headers={}] - Additional request headers
     * @returns {Promise<any>} - Resolves with parsed response JSON
     */
    requestJsonAuth(path, method = "GET", queryParams = {}, body = null, headers = {}) {
        const token = localStorage.getItem('access_token');
        const authHeaders = {
            ...headers,
            'Authorization': `Bearer ${token}`
        };
        return this.requestJson(path, method, queryParams, body, authHeaders);
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
     * @param {Object} params - The object containing parameters to serialize
     * @returns {string} URL-encoded query string
     */
    serializeQueryParams(params) {
        const result = [];
        for (const key in params) {
            const encodedKey = encodeURIComponent(key);
            const values = Array.isArray(params[key]) ? params[key] : [params[key]];
            for (let value of values) {
                if (value === null || value === undefined) {
                    continue;
                }
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
}
