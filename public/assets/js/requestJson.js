class ClientResponseError extends Error {
    constructor(errData) {
        super("ClientResponseError");
        Object.setPrototypeOf(this, ClientResponseError.prototype);

        this.url = errData?.url || "";
        this.status = errData?.status || 0;
        this.isAbort = !!errData?.isAbort;  // Retain isAbort for consistency
        this.originalError = errData?.originalError;
        this.response = errData?.response || errData?.data || {};
        this.name = "ClientResponseError " + this.status;
        this.message = this.response?.message; // Prioritize the server's message

        if (!this.message) {
            if (this.isAbort) {
                this.message = "The request was autocancelled."; // No longer specific to PocketBase
            } else if (this.originalError?.cause?.message?.includes("ECONNREFUSED ::1")) {
              this.message = "Failed to connect to the server"
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
        "Content-Type": "application/json", // Always JSON for this function
        ...headers // Allow overriding
    };
     // pocketbase specific, could be removed for other uses
    if (!getHeader(requestHeaders,"Accept-Language")){
      requestHeaders["Accept-Language"] = "en-US";
    }

    return fetch(url, {
        method,
        headers: requestHeaders,
        body: body ? JSON.stringify(body) : null, // Stringify the JSON body, handle null body
    })
    .then(response => {
        // Check for non-2xx status *before* parsing JSON
        if (!response.ok) {
            // Try to parse JSON error, but be resilient to non-JSON errors
            return response.text().then(text => { // First try to read as text
                let parsedError = {};
                try {
                    parsedError = JSON.parse(text); // Try to parse as JSON
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
          return {} // Return empty object for no-content
        }
        return response.json().catch(() => {
          // Handle json in case of not json.
          throw new ClientResponseError({
            url: response.url,
              status: response.status,
              response: {message: "Invalid JSON response"},
          })
        });
    })
    .catch(error => {
        // Ensure *all* errors are wrapped in ClientResponseError
        if (error instanceof ClientResponseError) {
            throw error; // Already a ClientResponseError, re-throw
        }
        throw new ClientResponseError(error); // Wrap other errors
    });
}


function buildUrl(baseUrl, path) {
    let url = baseUrl;

    if (
        typeof window !== "undefined" &&
        !!window.location &&
        !url.startsWith("https://") &&
        !url.startsWith("http://")
    ) {
        url = window.location.origin?.endsWith("/")
            ? window.location.origin.substring(0, window.location.origin.length - 1)
            : window.location.origin || "";

        if (!baseUrl.startsWith("/")) {
            url += window.location.pathname || "/";
            url += url.endsWith("/") ? "" : "/";
        }

        url += baseUrl;
    }

    if (path) {
        url += url.endsWith("/") ? "" : "/";
        url += path.startsWith("/") ? path.substring(1) : path;
    }

    return url;
}

function getHeader(headers, name){
  name = name.toLowerCase();

  for (let key in headers) {
      if (key.toLowerCase() == name) {
          return headers[key];
      }
  }

  return null;
}

function serializeQueryParams(params) {
    const result = [];

    for (const key in params) {
        const encodedKey = encodeURIComponent(key);
        const arrValue = Array.isArray(params[key]) ? params[key] : [params[key]];

        for (let v of arrValue) {
          v = prepareQueryParamValue(v)
            if(v === null){
              continue
            }
            result.push(encodedKey + "=" + v);
        }
    }

    return result.join("&");
}
function prepareQueryParamValue(value){
  if (value === null || typeof value === "undefined") {
      return null;
  }

  if (value instanceof Date) {
      return encodeURIComponent(value.toISOString().replace("T", " "));
  }

  if (typeof value === "object") {
    return encodeURIComponent(JSON.stringify(value));
  }

  return encodeURIComponent(value)
}

///// --- Example Usage ---
///
///// Example 1: GET request with query parameters
///sendJsonRequest(
///    "http://127.0.0.1:8090",
///    "/api/collections/example/records",
///    "GET",
///    { page: 1, perPage: 10 }
///)
///.then(data => console.log("GET Success:", data))
///.catch(error => console.error("GET Error:", error, error.status, error.response));
///
///// Example 2: POST request with JSON body
///const postData = {
///    title: "My JSON Post",
///    description: "This is a JSON payload."
///};
///
///sendJsonRequest(
///    "http://127.0.0.1:8090",
///    "/api/collections/example/records",
///    "POST",
///    {}, // queryParams
///    postData,
///    {}    // headers
///)
///.then(data => console.log("POST Success:", data))
///.catch(error => console.error("POST Error:", error, error.status, error.response));
///
/////Example 3: GET request with no results
///sendJsonRequest(
///    "http://127.0.0.1:8090",
///    "/api/collections/example/records",
///    "GET",
///    { filter: "id='nonexistent'" } // Assuming this filter returns no results
///)
///.then(data => console.log("GET Success (no results):", data)) // Expect empty array/object
///.catch(error => console.error("GET Error (no results):", error));
///
///// Example 4: Request to a non-existent endpoint (404)
///sendJsonRequest(
///    "http://127.0.0.1:8090",
///    "/api/nonexistent",
///    "GET"
///)
///.then(data => console.log("GET Success (should not happen):", data))
///.catch(error => console.error("GET Error (404):", error, error.status, error.response, error.url));
///
///// Example 5:  Server returns invalid JSON (simulate a server error)
///// You'd need to modify your server or use a mock server to test this properly
///sendJsonRequest(
///    "http://127.0.0.1:8090",
///    "/api/invalid-json", // Assume this endpoint returns invalid JSON
///    "GET"
///)
///.then(data => console.log("GET Success (invalid JSON, should not happen):", data))
///.catch(error => console.error("GET Error (Invalid JSON):", error, error.status, error.response));
///
///// Example 6: 204 No Content
///// PocketBase doesn't usually return 204 for GET requests, but it's good practice to handle it
///sendJsonRequest(
///  "http://127.0.0.1:8090",
///  "/api/some-endpoint", // Endpoint that returns 204
///  "DELETE"  // DELETE is a common method to return 204
///).then(data => {
///  console.log("DELETE Success (204):", data); // Expect an empty object: {}
///}).catch(error => {
///  console.error("DELETE Error (204):", error);
///});
