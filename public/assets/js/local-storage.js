import { ClientResponseError } from './client-response-error.js';

export class LocalStorage {
    /**
     * Saves JWT token to localStorage
     *
     * @param {string} token - The JWT token
     */
    static saveAccessToken(token) {
      if (token === undefined) {
        throw new ClientResponseError({
            response: { message: 'Invalid token: token is required' }
        });
      }
      
      try {
        localStorage.setItem('access_token', token);
      } catch (error) {
        throw new ClientResponseError({
            originalError: error,
            response: { message: `Failed to save access token: ${error.message}` }
        });
      }
    }

    /**
     * Retrieves the access token from localStorage
     * @returns {string} The access token 
     */
    static loadAccessToken() {
      try {
        return localStorage.getItem('access_token') || "";
      } catch (error) {
        throw new ClientResponseError({
            originalError: error,
            response: { message: 'Failed to load access token' }
        });
      }
    }

    /**
     * Saves user record to localStorage
     * Use {} to delete
     * @param {Object} record - The user record object
     */
    static saveUserRecord(record) {
      if (!record) {
        throw new ClientResponseError({
            response: { message: 'Invalid record: record is missing' }
        });
      }
      
      try {
        localStorage.setItem('user_record', JSON.stringify(record));
      } catch (error) {
        throw new ClientResponseError({
            originalError: error,
            response: { message: `Failed to save user record: ${error.message}` }
        });
      }
    }

    /**
     * Retrieves the user record from localStorage
     * @returns {Object} The parsed user record object or null if not found
     */
    static loadUserRecord() {
      try {
        const record = localStorage.getItem('user_record');
        if (!record) {
          return {};
        }
        
        return JSON.parse(record);
      } catch (error) {
        throw new ClientResponseError({
            originalError: error,
            response: { message: 'Failed to load user record' }
        });
      }
    }

    /**
     * Handles the email registration response by saving the token and user record
     * @param {Object} data - The JSON data returned from the requestJson method
     */
    static handleEmailRegistration(data) {
      try {
        if (!data) {
          throw new Error('No response data received');
        }

        // Check for both possible token field names
        const token = data.access_token || data.token;
        if (!token) {
          throw new Error('Response missing access token');
        }

        if (!data.record) {
          throw new Error('Response missing user record');
        }

        // Save the token and user record
        this.saveAccessToken(token);
        this.saveUserRecord(data.record);

        // Also save the full auth data if needed
        localStorage.setItem('auth_data', JSON.stringify({
          expires_in: data.expires_in,
          token_type: data.token_type
        }));
      } catch (error) {
        console.error('Failed to handle registration:', error);
        console.debug('Registration response data:', data);
        throw new ClientResponseError({
          response: {
            message: error.message,
            code: 'registration_failed'
          },
          originalError: error
        });
      }
    }
}
