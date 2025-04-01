import Restinpieces from "./sdk/restinpieces.js";

document.addEventListener("DOMContentLoaded", () => {
  const messageDiv = document.getElementById("message");
  const formDiv = document.getElementById("form");
  const actionsDiv = document.getElementById("actions");
  const changeButton = document.getElementById("changeButton");

  const rp = new Restinpieces({
    baseURL: "http://localhost:8080"
  });

  // Show loading message
  messageDiv.className = "verify-message verify-loading";
  messageDiv.textContent = "Loading user information...";
  messageDiv.classList.remove("verify-hidden");

  // Load auth data
  const authData = rp.store.auth.load();
  if (!authData || !authData.record || !authData.record.email) {
    showError("You must be logged in to request email change");
    return;
  }

  // Show form
  messageDiv.classList.add("verify-hidden");
  formDiv.classList.remove("verify-hidden");

  // Handle email change request
  changeButton.addEventListener("click", () => {
    const newEmail = document.getElementById("newEmail").value.trim();
    const currentEmail = authData.record.email;

    if (!newEmail) {
      showError("Please enter a new email address");
      return;
    }

    if (newEmail === currentEmail) {
      showError("New email must be different from current email");
      return;
    }

    messageDiv.className = "verify-message verify-loading";
    messageDiv.textContent = "Sending email change request...";

    rp.requestEmailChange({ 
      email: currentEmail,
      new_email: newEmail 
    })
      .then((response) => {
        if (!response?.message) {
          throw new Error("Invalid email change response: missing message");
        }
        showSuccess(response.message);
        actionsDiv.classList.remove("verify-hidden");
      })
      .catch((error) => {
        console.error("Email change request failed:", error);
        let errorMessage = "Failed to request email change";
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
