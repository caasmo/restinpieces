import Restinpieces from "./sdk/restinpieces.js";

class ConfirmPasswordReset {
  constructor() {
    this.rp = new Restinpieces({
      baseURL: "http://localhost:8080", 
    });
    this.form = document.getElementById("confirmResetForm");
    this.messageDiv = document.getElementById("message");
    this.form.addEventListener("submit", this.handleSubmit.bind(this));
  }

  validateForm(password, confirmPassword) {
    if (password !== confirmPassword) {
      throw new Error("Passwords do not match");
    }
    if (password.length < 8) {
      throw new Error("Password must be at least 8 characters");
    }
  }

  showMessage(text, isError = false) {
    this.messageDiv.textContent = text;
    this.messageDiv.className = isError ? "message error" : "message success";
  }

  createSuccessUI() {
    const successMessage = document.createElement("div");
    successMessage.className = "success-message";
    successMessage.innerHTML = `<h3>Password reset successful!</h3>`;

    const loginButton = document.createElement("button");
    loginButton.textContent = "Log In";
    loginButton.onclick = () => (window.location.href = "/login.html");

    this.form.remove();
    this.messageDiv.textContent = "";
    this.messageDiv.appendChild(successMessage);
    this.messageDiv.appendChild(loginButton);
  }

  handleSubmit(e) {
    e.preventDefault();

    const params = new URLSearchParams(window.location.search);
    const token = params.get("token");
    const password = document.getElementById("password").value;
    const confirmPassword = document.getElementById("confirmPassword").value;

    this.messageDiv.textContent = "";
    this.messageDiv.classList.remove("error", "success");

    try {
      this.validateForm(password, confirmPassword);

      if (!token) {
        throw new Error("Invalid password reset token");
      }

      this.showMessage("Resetting password...");

      this.rp.request("confirm_password_reset", {}, {
        token: token,
        password: password,
        password_confirm: confirmPassword
      })
      .then((response) => {
        if (response?.status === 200 && response?.message) {
          this.showMessage("Password reset successful!");
          this.createSuccessUI();
        } else {
          throw new Error(response?.message || "Invalid response from server");
        }
      })
      .catch((error) => {
        console.error("Password reset failed:", error);
        this.showMessage(
          error.response
            ? error.response.data?.message || "Password reset failed"
            : error.message,
          true
        );
      });
    } catch (error) {
      console.error("Validation failed:", error);
      this.showMessage(error.message, true);
    }
  }
}

document.addEventListener("DOMContentLoaded", () => new ConfirmPasswordReset());
