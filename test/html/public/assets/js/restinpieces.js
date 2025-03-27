import { ClientResponseError } from './client-response-error.js';
import { LocalStore } from './local-store.js';
import { HttpClient } from './http-client.js'; // Import the new class

class Restinpieces {
    // Default configuration
    static defaultConfig = {
        baseURL: "/",
        lang: "en-US",
        storage: null, // Will be instantiated if null
        endpointsPath: "GET /api/list-endpoints", 
    };

    constructor(config = {}) {
        // Merge user config with defaults
        const mergedConfig = { ...Restinpieces.defaultConfig, ...config };

        this.baseURL = mergedConfig.baseURL;
        this.lang = mergedConfig.lang;
        this.storage = mergedConfig.storage || new LocalStore(); // Instantiate storage
        this.httpClient = new HttpClient(this.baseURL); // Instantiate HttpClient
        this.endpointsPath = mergedConfig.endpointsPath;
        this.endpointsPromise = null; // Tracks ongoing fetch endpoint requests

        // TODO: Consider moving these to config or removing if unused
        //this.recordServices = {};
        //this.enableAutoCancellation = true;
        //this.cancelControllers = {};

        // Expose the injected storage instance with a cleaner interface
        // Note: This assumes the storage provider has these methods.
        // A more robust solution might involve checking method existence.
        this.store = {
            auth: {
                save: (data) => this.storage.saveAuth(data),
                load: () => this.storage.loadAuth(),
                isValid: () => this.storage.isTokenValid(),
            },
            provider: {
                save: (data) => this.storage.saveProvider(data),
                load: () => this.storage.loadProvider(),
            },
            endpoints: {
                save: (data) => this.storage.saveEndpoints(data),
                load: () => this.storage.loadEndpoints(),
            }
        };
    }

    // --- Endpoint Management ---

	fetchEndpoints() {
		const cachedEndpoints = this.store.endpoints.load();
		if (cachedEndpoints) {
			return Promise.resolve(cachedEndpoints);
		}

		if (!this.endpointsPromise) {
			// Use configured endpoints path
			const [method, endpointPath] = this.endpointsPath.split(' ');

            // Use the HttpClient instance for the request
			this.endpointsPromise = this.httpClient.requestJson(endpointPath, method) 
				.then(response => {
					if (!response?.data) {
						throw new ClientResponseError({
							response: { message: "Empty endpoints response" }
						});
					}

					this.store.endpoints.save(response.data);
					this.endpointsPromise = null; // Reset after completion
					return response.data;
				})
				.catch(error => {
					this.endpointsPromise = null; // Reset on error
					console.error("Failed to fetch endpoints:", error);
					throw error;
				});
		}

		return this.endpointsPromise;
	}

    // --- Core Request Methods ---

	request(endpointKey, queryParams = {}, body = null, headers = {}, signal = null) {
		return this.fetchEndpoints()
			.then(endpoints => {
				const methodAndPath = endpoints[endpointKey]; // e.g., "POST /api/users"
				const [method, path] = methodAndPath.split(' ');

				if (!path) {
					throw new Error(`Endpoint "${endpointKey}" not found or invalid format in endpoints list.`);
				}
                // Use the HttpClient instance for the actual request
				return this.httpClient.requestJson(path, method, queryParams, body, headers, signal); 
			})
			.catch(error => {
                // Add more context to the error if it's not already a ClientResponseError
                if (!(error instanceof ClientResponseError)) {
				    console.error(`Error preparing request to "${endpointKey}":`, error);
                }
				throw error;
			});
	}

    requestAuth(endpointKey, queryParams = {}, body = null, headers = {}, signal = null) {
        const authData = this.store.auth.load() || {};
        const token = authData.access_token || '';

        if (!token) {
            // Return a rejected promise directly. We don't know the final URL yet,
            // so we construct a placeholder or leave it empty.
            return Promise.reject(new ClientResponseError({
                // url: this.httpClient.buildUrl(this.baseURL, `unknown_path_for_${endpointKey}`), // Less ideal
                status: 401,
                response: { message: "No authentication token available." }
            }));
        }

        const authHeaders = {
            ...headers,
            'Authorization': `Bearer ${token}`
        };

        // Delegate the actual request to the general `request` method
        return this.request(endpointKey, queryParams, body, authHeaders, signal);
    }

    // --- Authentication Methods ---

	// TODO: Implement refresh logic if needed (e.g., using refresh token)
	RefreshAuth() {
		// This likely needs a specific endpoint and potentially different logic
		// depending on the refresh token strategy.
		// Example placeholder:
		// const authData = this.store.auth.load() || {};
		// const refreshToken = authData.refresh_token;
		// return this.request('refresh_token_endpoint', {}, { refresh_token: refreshToken });
		return this.requestAuth('refresh_auth'); // Assuming 'refresh_auth' uses the access token
	}

	ListOauth2Providers() {
		return this.request('list_oauth2_providers');
	}

	RegisterWithPassword(body = null, headers = {}, signal = null) {
		return this.request('register_with_password', {}, body, headers, signal);
	}

	RequestVerification(body = null, headers = {}, signal = null) {
		return this.request('request_verification', {}, body, headers, signal);
	}

	ConfirmVerification(body = null, headers = {}, signal = null) {
		return this.request('confirm_verification', {}, body, headers, signal);
	}

	authWithPassword(body = null, headers = {}, signal = null) {
		return this.request('auth_with_password', {}, body, headers, signal);
	}

	AuthWithOauth2(body = null, headers = {}, signal = null) {
		return this.request('auth_with_oauth2', {}, body, headers, signal);
	}
}

export default Restinpieces;

