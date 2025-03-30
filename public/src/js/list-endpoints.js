import Restinpieces from "./sdk/restinpieces.js";

const rp = new Restinpieces({
  baseURL: "http://localhost:8080",
});

function renderEndpoints(endpoints, listElement, containerElement) {
  listElement.innerHTML = "";

  if (endpoints) {
    let html = "";
    for (const [key, value] of Object.entries(endpoints)) {
      const method = value.split(" ")[0].toLowerCase();
      const path = value
        .replace(/^GET|POST|PUT|DELETE|PATCH /, "")
        .trim();
      html += `
        <li class="endpoint-item">
          <span class="method ${method}">${method.toUpperCase()}</span>
          <span>${path}</span>
        </li>`;
    }
    listElement.innerHTML = html;
    containerElement.classList.remove("hidden");
  } else {
    containerElement.classList.add("hidden");
  }
}

function showSavedEndpoints() {
  const endpoints = rp.store.endpoints.load();
  renderEndpoints(
    endpoints,
    document.getElementById("saved-endpoints-list"),
    document.getElementById("saved-endpoints-container"),
  );
}

function showFetchedEndpoints(endpoints) {
  renderEndpoints(
    endpoints,
    document.getElementById("fetched-endpoints-list"),
    document.getElementById("fetched-endpoints-container"),
  );
}

function getEndpoints() {
  const errorDiv = document.getElementById("error");
  errorDiv.textContent = "Loading...";

  // Show loading state
  rp.store.endpoints.save(null);
  showSavedEndpoints();

  rp.fetchEndpoints()
    .then((endpoints) => {
      errorDiv.textContent = "";

      if (!endpoints) {
        errorDiv.textContent = "No endpoints data received";
        return;
      }

      // Show the fresh endpoints on the right
      showFetchedEndpoints(endpoints);

      // Also update saved endpoints
      rp.store.endpoints.save(endpoints);
      showSavedEndpoints();
    })
    .catch((error) => {
      errorDiv.textContent = `Error: ${error.message}`;
      console.error(error);
    });
}

function checkAuthStatus() {
  const authText = document.getElementById("auth-text");

  if (rp.store.auth.isValid()) {
    authText.textContent = "✅ Authenticated";
    showSavedEndpoints();
  } else {
    authText.textContent = "❌ Not authenticated";
  }
}

// Initial load
checkAuthStatus();
showSavedEndpoints();

// Event listener
document
  .getElementById("get-endpoints-btn")
  .addEventListener("click", getEndpoints);
