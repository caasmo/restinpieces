import Restinpieces from "./sdk/restinpieces.js";

class RegisterForm {
  constructor() {
    this.rp = new Restinpieces({
      baseURL: "http://localhost:8080",
    });
    this.form = document.getElementById("registerForm");
    this.messageDiv = document.getElementById("message");
    this.form.addEventListener("submit", this.handleSubmit.bind(this));
  }

  validateForm(email, password, confirmPassword) {
    if (password !== confirmPassword) {
      throw new Error("Passwords do not match");
    }
    if (password.length < 8) {
      throw new Error("Password must be at least 8 characters");
    }
    return { email, password, confirmPassword };
  }

  showMessage(text, isError = false) {
    this.messageDiv.textContent = text;
    this.messageDiv.className = isError ? "message error" : "message success";
  }

  createSuccessUI(userRecord) {
    const userInfo = document.createElement("div");
    userInfo.className = "user-info";
    userInfo.innerHTML = `
      <h3>Registration Successful!</h3>
      <pre>${JSON.stringify(userRecord, null, 2)}</pre>
    `;

    const buttonContainer = document.createElement("div");
    buttonContainer.className = "button-container";

    if (!userRecord.verified) {
      const verifyButton = document.createElement("button");
      verifyButton.textContent = "Verify Email";
      verifyButton.onclick = () => (window.location.href = "/verify-email");
      buttonContainer.appendChild(verifyButton);
    }

    const dashboardButton = document.createElement("button");
    dashboardButton.textContent = "Go to Dashboard";
    dashboardButton.onclick = () => (window.location.href = "dashboard.html");
    buttonContainer.appendChild(dashboardButton);

    this.form.remove();
    this.messageDiv.textContent = "";
    this.messageDiv.appendChild(userInfo);
    this.messageDiv.appendChild(buttonContainer);
  }

  handleSubmit(e) {
    e.preventDefault();

    const email = document.getElementById("email").value;
    const password = document.getElementById("password").value;
    const confirmPassword = document.getElementById("confirmPassword").value;

    this.messageDiv.textContent = "";
    this.messageDiv.classList.remove("error", "success");

    try {
      const { email: validatedEmail, password: validatedPassword } =
        this.validateForm(email, password, confirmPassword);

      this.showMessage("Registering...");

      this.rp
        .registerWithPassword({
          identity: validatedEmail,
          password: validatedPassword,
          password_confirm: confirmPassword,
        })
        .then((response) => {
          if (response.data?.access_token && response.data?.record) {
            this.rp.store.auth.save(response.data);
            this.showMessage("Registration successful!");
            this.createSuccessUI(response.data.record);
          } else {
            throw new Error("Invalid registration response");
          }
        })
        .catch((error) => {
          console.error("Registration failed:", error);
          this.showMessage(
            error.response
              ? JSON.stringify(error.response, null, 2)
              : error.message,
            true,
          );
        });
    } catch (error) {
      console.error("Validation failed:", error);
      this.showMessage(error.message, true);
    }
  }
}

document.addEventListener("DOMContentLoaded", () => new RegisterForm());
