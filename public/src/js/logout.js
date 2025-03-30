import Restinpieces from "./sdk/restinpieces.js";

const rp = new Restinpieces({
  baseURL: "http://localhost:8080",
});
rp.store.auth.save(null);
