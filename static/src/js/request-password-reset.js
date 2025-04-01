import Restinpieces from "./sdk/restinpieces.js";

document.addEventListener("DOMContentLoaded", () => {
  const messageDiv = document.getElementById("message");
  const formDiv = document.getElementById("form");
  const actionsDiv = document.getElementById("actions");
  const resetButton = document.getElementById("resetButton");
  const emailInput = document.getElementById("email");

  const rp = new Restinpieces({
    baseURL: "http://localhost:8080"
  });

  // Show form
  messageDiv.classList.add("verify-hidden");
  formDiv.classList.remove("verify-hidden");

  // Handle password reset request
  resetButton.addEventListener("click", () => {
    const email = emailInput.value.trim();
    
    if (!email) {
      showError("Please enter your email address");
      return;
    }

    messageDiv.className = "verify-message verify-loading";
    messageDiv.textContent = "Sending password reset email...";

    rp.requestPasswordReset({ email })
      .then((response) => {
        if (!response?.message) {
          throw new Error(
            "Invalid password reset response: missing message"
          );
        }
        showSuccess(response.message);
        actionsDiv.classList.remove("verify-hidden");
      })
      .catch((error) => {
        console.error("Password reset request failed:", error);
        let errorMessage = "Failed to request password reset";
        if (error.response) {
          errorMessage =
            error.response.message ||
            error.response.data?.message ||
            JSON.stringify(error.response, null, 2);
        } else if (error.message) {
          errorMessage = error.message;
        }
        showError(errorMessage);
      });
  });

  function showSuccess(message) {
    messageDiv.className = "verify-message verify-success";
    messageDiv.textContent = message;
    formDiv.classList.add("verify-hidden");
  }

  function showError(message) {
    messageDiv.className = "verify-message verify-error";
    messageDiv.textContent = message;
    formDiv.classList.add("verify-hidden");
    actionsDiv.classList.remove("verify-hidden");
  }
});
