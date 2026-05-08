/**
 * Thick Client for Reporting App
 * Sole data bridge between reporting_app and reports
 */

(function(global) {
    'use strict';

    // Data store for report state
    const DataStore = {
        config: null,
        params: {},
        
        init() {
            // Load configuration from window.ReportConfig
            if (window.ReportConfig) {
                this.config = window.ReportConfig;
                this.params = this.config.params || {};
            } else {
                console.error('ReportConfig not found in window');
                this.config = {};
            }
            
            console.log('Thick client initialized for report:', this.config.reportId);
        },
        
        getParam(key) {
            return this.params[key];
        },
        
        setParam(key, value) {
            this.params[key] = value;
        },
        
        getParams() {
            return { ...this.params };
        },
        
        isImmutable(key) {
            return (this.config.immutableParams || []).includes(key);
        },
        
        isMutable(key) {
            return (this.config.mutableParams || []).includes(key);
        }
    };

    // Refresh controller
    const RefreshController = {
        async refresh(newParams = {}) {
            console.log('Refresh requested with params:', newParams);
            
            // Merge new params with current params
            const currentParams = DataStore.getParams();
            const paramsToSend = { ...currentParams, ...newParams };
            
            // Build refresh URL
            const currentUrl = DataStore.config.currentUrl;
            if (!currentUrl) {
                throw new Error('No current URL available for refresh');
            }
            
            try {
                // Make refresh request
                const response = await fetch('/refresh?' + currentUrl.split('?')[1], {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({ params: newParams })
                });
                
                if (!response.ok) {
                    const error = await response.json();
                    throw new Error(error.error || `Refresh failed with status ${response.status}`);
                }
                
                const result = await response.json();
                
                // Update DataStore with new URL for next refresh
                if (result.next_url) {
                    DataStore.config.currentUrl = result.next_url;
                }
                
                // Return data to caller
                return result.data || {};
                
            } catch (error) {
                console.error('Refresh failed:', error);
                throw error;
            }
        }
    };

    // Public API exposed to reports
    global.ReportApp = {
        // Data access
        refresh: (params) => RefreshController.refresh(params),
        
        // Parameter management
        getParam: (key) => DataStore.getParam(key),
        setParam: (key, value) => {
            if (DataStore.isImmutable(key)) {
                throw new Error(`Cannot change immutable parameter: ${key}`);
            }
            if (!DataStore.isMutable(key)) {
                throw new Error(`Unknown parameter: ${key}`);
            }
            DataStore.setParam(key, value);
            return true;
        },
        getParams: () => DataStore.getParams(),
        
        // Utility methods
        isImmutable: (key) => DataStore.isImmutable(key),
        isMutable: (key) => DataStore.isMutable(key),
        
        // Events
        on(event, callback) {
            // Simple event system
            if (!this._events) this._events = {};
            if (!this._events[event]) this._events[event] = [];
            this._events[event].push(callback);
        },
        
        emit(event, data) {
            if (this._events && this._events[event]) {
                this._events[event].forEach(callback => callback(data));
            }
        }
    };

    // Initialize when DOM is ready
    function init() {
        DataStore.init();
        
        // Emit ready event
        if (global.ReportApp.emit) {
            global.ReportApp.emit('ready', { reportId: DataStore.config.reportId });
        }
        
        console.log('Thick client ready');
    }

    // Start initialization
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

})(window);