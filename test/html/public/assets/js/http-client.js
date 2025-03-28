import { ClientResponseError } from "./client-response-error.js";

export class HttpClient {
  constructor(baseURL) {
    this.baseURL = baseURL;
  }

  /**
   * Makes an HTTP request with JSON handling and standardized error responses
   *
   * @param {string} path - The URL path to request
   * @param {string} [method="GET"] - The HTTP method to use
   * @param {Object} [queryParams={}] - Query parameters to include
   * @param {Object|null} [body=null] - Request body (will be JSON.stringified)
   * @param {Object} [headers={}] - Additional request headers
   * @param {AbortSignal|null} [signal=null] - Optional AbortSignal for cancellation
   * @returns {Promise<any>} - Resolves with parsed response JSON
   */
  requestJson(
    path,
    method = "GET",
    queryParams = {},
    body = null,
    headers = {},
    signal = null,
  ) {
    let url = this.buildUrl(this.baseURL, path);

    const serializedQueryParams = this.serializeQueryParams(queryParams);
    if (serializedQueryParams) {
      url += (url.includes("?") ? "&" : "?") + serializedQueryParams;
    }

    const requestHeaders = {
      "Content-Type": "application/json",
      ...headers, // Allow overriding
    };

    return fetch(url, {
      method,
      headers: requestHeaders,
      body: body ? JSON.stringify(body) : null,
      signal, // Pass the signal to fetch
    })
      .then((response) => {
        // Check for non-2xx status *before* parsing JSON
        if (!response.ok) {
          // Try to parse JSON error, but be resilient to non-JSON errors
          return response.text().then((text) => {
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
        // response.json() is javscript object or array, etc scalar
        return response.json().catch(() => {
          // Handle json in case of not json.
          throw new ClientResponseError({
            url: response.url,
            status: response.status,
            response: { message: "Invalid JSON response" },
          });
        });
      })
      .catch((error) => {
        // Ensure *all* errors are wrapped in ClientResponseError
        if (error instanceof ClientResponseError) {
          throw error; // Already a ClientResponseError, re-throw
        }
        // Check if it's an AbortError
        if (error.name === "AbortError") {
          throw new ClientResponseError({
            url: url, // Use the constructed URL
            isAbort: true,
            originalError: error,
            response: { message: "Request aborted" },
          });
        }
        // Wrap other errors (e.g., network errors)
        throw new ClientResponseError({
          url: url, // Use the constructed URL
          originalError: error,
          response: { message: error.message || "Network or unknown error" },
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
  buildUrl(baseUrl, path) {
    // Handle empty baseUrl - use current directory
    if (baseUrl === "") {
      const pathParts = window.location.pathname.split("/");
      pathParts.pop(); // Remove the last part (file or empty string)
      baseUrl = pathParts.join("/") + "/";
    }

    // Create full URL, handling relative URLs
    let url;
    if (!baseUrl.startsWith("http://") && !baseUrl.startsWith("https://")) {
      // For relative URLs, use the URL constructor with current location as base
      const base = baseUrl.startsWith("/")
        ? window.location.origin
        : window.location.href;
      url = new URL(baseUrl, base).href;
    } else {
      url = baseUrl;
    }

    // Add path if provided
    if (path) {
      url =
        url +
        (url.endsWith("/") ? "" : "/") +
        (path.startsWith("/") ? path.substring(1) : path);
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
