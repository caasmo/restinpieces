import Restinpieces from "./sdk/restinpieces.js";

function getProviders() {
  const resultDiv = document.getElementById("result");
  const errorDiv = document.getElementById("error");

  resultDiv.textContent = "Loading...";
  errorDiv.textContent = "";

  const rp = new Restinpieces({
    baseURL: "http://localhost:8080",
  });

  rp.listOauth2Providers()
    .then((data) => {
      resultDiv.textContent = JSON.stringify(data, null, 2);
      document.getElementById("result-section").classList.remove("providers-result-section");
    })
    .catch((error) => {
      let errorText = "Error: " + error.message + "\n";
      errorText += "Status: " + error.status + "\n";
      if (error.response) {
        errorText +=
          "Response:\n" + JSON.stringify(error.response, null, 2);
      }
      if (error.url) {
        errorText += "\nURL: " + error.url;
      }
      errorDiv.textContent = errorText;
      resultDiv.textContent = "";
      console.error(error);
    });
}

document
  .getElementById("get-providers-btn")
  .addEventListener("click", getProviders);
