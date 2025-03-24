
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
            ...headers
        };

        return fetch(url, {
            method,
            headers: requestHeaders,
            body: body ? JSON.stringify(body) : null,
        })
        .then(response => {
            if (!response.ok) {
                return response.text().then(text => {
                    let parsedError = {};
                    try {
                        parsedError = JSON.parse(text);
                    } catch (_) {
                        parsedError = { message: text || "Unknown error" };
                    }
                    throw new ClientResponseError({
                        url: response.url,
                        status: response.status,
                        response: parsedError,
                    });
                });
            }

            if (response.status === 204) {
                return {};
            }
            return response.json().catch(() => {
                throw new ClientResponseError({
                    url: response.url,
                    status: response.status,
                    response: { message: "Invalid JSON response" },
                });
            });
        })
        .catch(error => {
            if (error instanceof ClientResponseError) {
                throw error;
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
    buildUrl(baseUrl, path) {
        if (baseUrl === "") {
            const pathParts = window.location.pathname.split('/');
            pathParts.pop();
            baseUrl = pathParts.join('/') + '/';
        }
        
        let url;
        if (!baseUrl.startsWith("http://") && !baseUrl.startsWith("https://")) {
            const base = baseUrl.startsWith("/") ? window.location.origin : window.location.href;
            url = new URL(baseUrl, base).href;
        } else {
            url = baseUrl;
        }
        
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
