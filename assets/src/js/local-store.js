export class LocalStore {
  // Key registry (remains static as it's class-level config)
  static #keys = {
    auth: "_rip_auth",
    provider: "_rip_provider",
    endpoints: "_rip_endpoints",
  };

  // Private generic methods (now instance methods)
  #get(key) {
    try {
      const value = localStorage.getItem(LocalStore.#keys[key]);
      return value ? JSON.parse(value) : null;
    } catch (error) {
      console.error(`Failed to retrieve ${key}:`, error);
      throw new Error(`Failed to retrieve ${key}: ` + error.message);
    }
  }

  #set(key, value) {
    try {
      localStorage.setItem(LocalStore.#keys[key], JSON.stringify(value));
    } catch (error) {
      console.error(`Failed to store ${key}:`, error);
      throw new Error(`Failed to store ${key}: ` + error.message);
    }
  }

  // Public methods for 'auth' (now instance methods)
  loadAuth() {
    return this.#get("auth");
  }

  saveAuth(value) {
    this.#set("auth", value);
  }

  isTokenValid() {
    try {
      // Use instance method `this.loadAuth()`
      const auth = this.loadAuth();
      if (!auth?.access_token) return false;
      const payload = JSON.parse(atob(auth.access_token.split(".")[1]));
      return payload.exp > Date.now() / 1000; // milliseconds since 1970
    } catch {
      return false;
    }
  }

  // Public methods for 'provider' (now instance methods)
  loadProvider() {
    return this.#get("provider");
  }

  saveProvider(value) {
    this.#set("provider", value);
  }

  // Public methods for 'endpoints' (now instance methods)
  loadEndpoints() {
    return this.#get("endpoints");
  }

  saveEndpoints(value) {
    this.#set("endpoints", value);
  }
}
