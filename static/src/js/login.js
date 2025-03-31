import Restinpieces from "./sdk/restinpieces.js";

class LoginHandler {
  constructor() {
    this.rp = new Restinpieces({
      baseURL: "http://localhost:8080",
    });
    this.form = document.getElementById("passwordLoginForm");
    this.errorDiv = document.getElementById("error");
    this.providerList = document.getElementById("list");

    this.initEventListeners();
    this.loadProviders();
  }

  initEventListeners() {
    this.form.addEventListener("submit", (e) => {
      e.preventDefault();
      this.handleLogin();
    });
  }

  loadProviders() {
    this.rp
      .listOauth2Providers()
      .then((response) => {
        this.providerList.innerHTML = "";

        if (!response?.data?.providers) {
          this.providerList.innerHTML = "<li>No OAuth2 providers.</li>";
          return;
        }

        response.data.providers.forEach((provider) => {
          const li = document.createElement("li");
          const a = document.createElement("a");
          a.textContent = `Login with ${provider.displayName}`;
          a.href = provider.authURL;
          a.addEventListener("click", () => {
            this.rp.store.provider.save(provider);
          });
          li.appendChild(a);
          this.providerList.appendChild(li);
        });

        if (response.data.providers.length === 0) {
          this.providerList.innerHTML = "<li>No OAuth2 providers.</li>";
        }
      })
      .catch((error) => {
        console.error("Error loading providers:", error);
        this.providerList.innerHTML = "<li>Error loading providers</li>";
      });
  }

  showMessage(text, isError = false) {
    this.errorDiv.textContent = text;
    this.errorDiv.className = isError ? "message error" : "message success";
  }

  createSuccessUI(userRecord) {
    const userInfo = document.createElement("div");
    userInfo.className = "user-info";
    userInfo.innerHTML = `
      <h3>Login Successful!</h3>
      <pre>${JSON.stringify(userRecord, null, 2)}</pre>
    `;

    const buttonContainer = document.createElement("div");
    buttonContainer.className = "button-container";

    const dashboardButton = document.createElement("button");
    dashboardButton.textContent = "Go to Dashboard";
    dashboardButton.onclick = () => (window.location.href = "/dashboard.html");
    buttonContainer.appendChild(dashboardButton);

    this.errorDiv.textContent = "";
    this.errorDiv.appendChild(userInfo);
    this.errorDiv.appendChild(buttonContainer);
  }

  async handleLogin() {
    const email = document.getElementById("email").value;
    const password = document.getElementById("password").value;

    this.showMessage("Logging in...");

    try {
      const response = await this.rp.authWithPassword({
        identity: email,
        password,
      });

      if (response?.data?.access_token) {
        this.rp.store.auth.save(response.data);
        this.createSuccessUI(response.data.record);
      } else {
        throw new Error("Invalid login response: missing access token");
      }
    } catch (error) {
      console.error("Login failed:", error);
      this.showMessage(error.response?.message || error.message, true);
    }
  }
}

document.addEventListener("DOMContentLoaded", () => new LoginHandler());
