import Restinpieces from "./sdk/restinpieces.js";

document.addEventListener("DOMContentLoaded", () => {
  // Initialize Restinpieces with config
  const rp = new Restinpieces({
    baseURL: "http://localhost:8080",
  });

  // Display initial user info and token from store
  const authData = rp.store.auth.load() || {};
  const userInfo = authData.record || {};
  const currentToken = authData.access_token || "";

  document.getElementById("user-info").textContent = JSON.stringify(
    userInfo,
    null,
    2,
  );
  document.getElementById("current-token").textContent = currentToken;

  function refreshToken() {
    const refreshError = document.getElementById("refresh-error");
    refreshError.textContent = "";

    // Get current auth data before refresh
    const currentAuth = rp.store.auth.load() || {};
    const currentToken = currentAuth.access_token || "No token found";

    // Save current token as previous token
    document.getElementById("previous-token").textContent = currentToken;

    rp.refreshAuth()
      .then((response) => {
        if (response?.data?.access_token) {
          // Save new auth data (preserving existing record)
          const newAuth = {
            ...currentAuth,
            access_token: response.data.access_token,
            expires_in: response.data.expires_in,
          };
          rp.store.auth.save(newAuth);

          // Display new token and update UI
          document.getElementById("new-token").textContent =
            response.data.access_token;
          document.getElementById("current-token").textContent =
            response.data.access_token;
          // Update previous token for next refresh
          document.getElementById("previous-token").textContent =
            currentToken;
          // Update user info with any new data from refresh
          const updatedUserInfo = {
            ...userInfo,
            ...(response.data.record || {}),
          };
          document.getElementById("user-info").textContent =
            JSON.stringify(updatedUserInfo, null, 2);
        } else {
          throw new Error("No access token in response");
        }
      })
      .catch((error) => {
        console.error("Token refresh failed:", error);
        refreshError.textContent = error.response
          ? JSON.stringify(error.response, null, 2)
          : error.message;
      });
  }

  // Attach click handlers to all buttons
  document
    .getElementById("refresh-token-btn")
    .addEventListener("click", refreshToken);

  document
    .getElementById("verify-email-btn")
    .addEventListener("click", () => {
      window.location.href = "/request-email-verification.html";
    });

  document
    .getElementById("change-email-btn")
    .addEventListener("click", () => {
      window.location.href = "/request-email-change.html";
    });

  document
    .getElementById("logout-btn")
    .addEventListener("click", () => {
      window.location.href = "/logout.html";
    });

  document
    .getElementById("request-password-reset-btn")
    .addEventListener("click", () => {
      window.location.href = "/request-password-reset.html";
    });
});
