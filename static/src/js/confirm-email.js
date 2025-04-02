import Restinpieces from "./sdk/restinpieces.js";

document.addEventListener("DOMContentLoaded", () => {
  const urlParams = new URLSearchParams(window.location.search);
  const token = urlParams.get("token");
  const messageDiv = document.getElementById("message");
  const actionsDiv = document.getElementById("actions");

  if (!token) {
    showError("Missing verification token");
    return;
  }

  const rp = new Restinpieces({
    baseURL: "http://localhost:8080",
  });
  rp.confirmEmailVerification({ token })
    .then((response) => {
      if (!response.message) {
        throw new Error("Invalid verification response: missing message");
      }
      showSuccess(response.message);
    })
    .catch((error) => {
      console.error("Verification failed:", error);
      let errorMessage = "Email verification failed";
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

  function showSuccess(message) {
    messageDiv.className = "confirm-message confirm-success";
    messageDiv.textContent = message;
    actionsDiv.classList.remove("confirm-hidden");
  }

  function showError(message) {
    messageDiv.className = "confirm-message confirm-error";
    messageDiv.textContent = message;
    actionsDiv.classList.remove("confirm-hidden");
  }
});
