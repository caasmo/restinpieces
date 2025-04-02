import Restinpieces from "./sdk/restinpieces.js";

document.addEventListener("DOMContentLoaded", () => {
  const urlParams = new URLSearchParams(window.location.search);
  const token = urlParams.get("token");
  const messageDiv = document.getElementById("message");
  const formDiv = document.getElementById("form");
  const actionsDiv = document.getElementById("actions");
  const confirmButton = document.getElementById("confirmButton");

  const rp = new Restinpieces({
    baseURL: "http://localhost:8080"
  });

  if (!token) {
    showError("Missing email change token");
    return;
  }

  confirmButton.addEventListener("click", () => {
    const password = document.getElementById("password").value.trim();

    if (!password) {
      showError("Please enter your current password");
      return;
    }

    messageDiv.className = "confirm-message confirm-loading";
    messageDiv.textContent = "Processing email change...";
    messageDiv.classList.remove("confirm-hidden");

    rp.confirmEmailChange({ token, password })
      .then((response) => {
        if (!response?.message) {
          throw new Error("Invalid email change response");
        }
        showSuccess(response.message);
        actionsDiv.classList.remove("confirm-hidden");
      })
      .catch((error) => {
        console.error("Email change failed:", error);
        let errorMessage = "Failed to confirm email change";
        if (error.response) {
          errorMessage = error.response.message || 
                        error.response.data?.message || 
                        JSON.stringify(error.response, null, 2);
        } else if (error.message) {
          errorMessage = error.message;
        }
        showError(errorMessage);
      });
  });

  function showSuccess(message) {
    messageDiv.className = "confirm-message confirm-success";
    messageDiv.textContent = message;
    formDiv.classList.add("confirm-hidden");
  }

  function showError(message) {
    messageDiv.className = "confirm-message confirm-error";
    messageDiv.textContent = message;
    formDiv.classList.add("confirm-hidden");
    actionsDiv.classList.remove("confirm-hidden");
  }
});
