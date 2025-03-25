export class RestinpiecesLocalStore {
    // Private key registry (only accessible within the class)
    static #keys = {
        auth: '_rip_auth',
        provider: '_rip_provider',
        endpoints: '_rip_endpoints'
    };

    // Private generic methods
    static #get(key) {
        try {
            const value = localStorage.getItem(this.#keys[key]);
            return value ? JSON.parse(value) : null;
        } catch (error) {
            console.error(`Failed to retrieve ${key}:`, error);
            throw new Error(`Failed to retrieve ${key}: ` + error.message);
        }
    }

    static #set(key, value) {
        try {
            localStorage.setItem(this.#keys[key], JSON.stringify(value));
        } catch (error) {
            console.error(`Failed to store ${key}:`, error);
            throw new Error(`Failed to store ${key}: ` + error.message);
        }
    }

    // Public methods for 'auth'
    static loadAuth() {
        return this.#get('auth');
    }

    static saveAuth(value) {
        this.#set('auth', value);
    }

    static isTokenValid() {
        try {
            const auth = this.loadAuth();
            if (!auth?.access_token) return false;
            
            const payload = JSON.parse(atob(auth.access_token.split('.')[1]));
            return payload.exp > Date.now() / 1000;
        } catch {
            return false;
        }
    }

    // Public methods for 'provider'
    static loadProvider() {
        return this.#get('provider');
    }

    static saveProvider(value) {
        this.#set('provider', value);
    }

    // Public methods for 'endpoints'
    static loadEndpoints() {
        return this.#get('endpoints');
    }

    static saveEndpoints(value) {
        this.#set('endpoints', value);
    }
}
