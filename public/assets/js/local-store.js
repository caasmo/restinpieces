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
    SaveAuth(auth) {
        if (!auth || !auth.access_token || !auth.user_record) {
            console.error('Invalid auth object - must contain access_token and user_record');
            return false;
        }
        return this.Set('auth', { 
            access_token: auth.access_token,
            user_record: auth.user_record,
            timestamp: Date.now()
        });
    }

    LoadAuth() {
        const auth = this.Get('auth');
        return auth || {
            access_token: null,
            user_record: null
        };
    }

    ClearAuth() {
        return this.Remove('auth');
    }
}
