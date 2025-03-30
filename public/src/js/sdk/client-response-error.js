/**
 * Creates a standardized error object for HTTP client requests
 * @param {Object} errData - Error data object
 * @param {string} [errData.url] - The URL that caused the error
 * @param {number} [errData.status] - HTTP status code
 * @param {boolean} [errData.isAbort] - Whether the request was aborted
 * @param {Error} [errData.originalError] - Original error object
 * @param {Object} [errData.response] - Response data from the server
 */
export class ClientResponseError extends Error {
  constructor(errData) {
    // Pass the message to parent Error constructor if available
    super(errData?.response?.message || "ClientResponseError");

    this.url = errData?.url || "";
    this.status = errData?.status || 0;
    // this is only meaningful with a requestJson with AbortController
    this.isAbort = Boolean(errData?.isAbort);
    this.originalError = errData?.originalError;
    this.response = errData?.response || {};
    this.name = "ClientResponseError " + this.status;
    this.message = this.response?.message; // Prioritize the server's message
    this.code = this.response?.code || ""; // Prioritize the server's message

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
