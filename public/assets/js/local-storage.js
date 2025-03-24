export class LocalStorage {
    /**
     * Saves JWT token to localStorage
     *
     * @param {string} token - The JWT token
     */
    static saveAccessToken(token) {
      if (token === undefined) {
        throw new Error('Invalid token: token is required');
      }
      
      try {
        localStorage.setItem('access_token', token);
      } catch (error) {
        throw new Error(`Failed to save access token: ${error.message}`);
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
        // which can occur if storage is unavailable or quota is exceeded
        return "";
      }
    }

    /**
     * Saves user record to localStorage
     * Use {} to delete
     * @param {Object} record - The user record object
     */
    static saveUserRecord(record) {
      if (!record) {
        throw new Error('Invalid record: record is missing');
      }
      
      try {
        localStorage.setItem('user_record', JSON.stringify(record));
      } catch (error) {
        throw new Error(`Failed to save user record: ${error.message}`);
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
        // by Json parse
        return {};
      }
    }

    /**
     * Handles the email registration response by saving the token and user record
     * @param {Object} data - The JSON data returned from the requestJson method
     */
    static handleEmailRegistration(data) {
      // Verify we have the necessary data
      if (!data || !data.token || !data.record) {
        throw new Error('Invalid response data: token or record missing');
      }

      // Save JWT and user record using the specialized functions
      this.saveAccessToken(data.token);
      this.saveUserRecord(data.record);
    }
}
