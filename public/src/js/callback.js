import Restinpieces from "./sdk/restinpieces.js";

document.addEventListener("DOMContentLoaded", () => {
  const messageDiv = document.getElementById("message");
  const params = new URLSearchParams(window.location.search);
  const rp = new Restinpieces("http://localhost:8080");
  const provider = rp.store.provider.load();

  if (!provider) {
    showError("No provider information found");
    return;
  }

  const authRequest = {
    provider: provider.name,
    code: params.get("code"),
    code_verifier: provider.codeVerifier,
    redirect_uri: provider.redirectURL,
  };

  rp.authWithOauth2(authRequest)
    .then((response) => {
      if (response?.data?.access_token && response?.data?.record) {
        rp.store.auth.save(response.data);
        showSuccess(response.data.record);
      } else {
        throw new Error("No access token in response data");
      }
    })
    .catch((error) => {
      console.error("Authentication failed:", error);
      showError(error.response || { message: error.message });
    });

  function showSuccess(record) {
    messageDiv.textContent = "Authentication successful!";
    messageDiv.classList.add("success");

    const userInfo = document.createElement("div");
    userInfo.className = "oauth-user-info";
    userInfo.innerHTML = `
      <h3>Authentication Successful!</h3>
      <pre>${JSON.stringify(record, null, 2)}</pre>
    `;

    const buttonContainer = document.createElement("div");
    buttonContainer.className = "oauth-button-container";

    const dashboardButton = document.createElement("button");
    dashboardButton.textContent = "Go to Dashboard";
    dashboardButton.onclick = () =>
      (window.location.href = "/dashboard.html");
    buttonContainer.appendChild(dashboardButton);

    messageDiv.textContent = "";
    messageDiv.appendChild(userInfo);
    messageDiv.appendChild(buttonContainer);
  }

  function showError(error) {
    console.error("OAuth2 error:", error);
    messageDiv.textContent = error.message || "Authentication failed";
    messageDiv.classList.add("error");
  }
});
