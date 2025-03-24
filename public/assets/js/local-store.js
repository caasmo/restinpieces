class RestinpiecesStorage {
    constructor() {
        this.prefix = 'rp_';
    }

    // Core storage methods
    Set(key, value) {
        try {
            localStorage.setItem(`${this.prefix}${key}`, JSON.stringify(value));
            return true;
        } catch (error) {
            console.error('Storage set failed:', error);
            return false;
        }
    }

    Get(key) {
        try {
            const value = localStorage.getItem(`${this.prefix}${key}`);
            return value ? JSON.parse(value) : null;
        } catch (error) {
            console.error('Storage get failed:', error);
            return null;
        }
    }

    Remove(key) {
        try {
            localStorage.removeItem(`${this.prefix}${key}`);
            return true;
        } catch (error) {
            console.error('Storage remove failed:', error);
            return false;
        }
    }

    // Auth-specific methods
    SaveAuth(token, userData) {
        return this.Set('auth', { 
            token, 
            user: userData,
            timestamp: Date.now() 
        });
    }

    LoadAuth() {
        return this.Get('auth') || {};
    }

    ClearAuth() {
        return this.Remove('auth');
    }
}
