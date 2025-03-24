class RestinpiecesStorage {
    // Core storage methods
    Set(key, value) {
        try {
            localStorage.setItem(key, JSON.stringify(value));
            return true;
        } catch (error) {
            console.error('Storage set failed:', error);
            return false;
        }
    }

    Get(key) {
        try {
            const value = localStorage.getItem(key);
            return value ? JSON.parse(value) : null;
        } catch (error) {
            console.error('Storage get failed:', error);
            return null;
        }
    }

    Remove(key) {
        try {
            localStorage.removeItem(key);
            return true;
        } catch (error) {
            console.error('Storage remove failed:', error);
            return false;
        }
    }

    // Auth-specific methods
    SaveAuth(auth) {
        return this.Set('auth', auth);
    }

    LoadAuth() {
        return this.Get('auth') || {};
    }

    ClearAuth() {
        return this.Remove('auth');
    }
}
