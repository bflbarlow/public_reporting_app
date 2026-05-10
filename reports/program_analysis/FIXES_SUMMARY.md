# Fixes for Thick Client Initialization Issues

## Problems Identified

1. **LocalStorage sandbox error**: Document is sandboxed without 'allow-same-origin' flag
2. **Thick client timing issue**: `window.ReportApp` might not be initialized when report code runs
3. **Infinite retry potential**: Old code could loop indefinitely

## Solutions Implemented

### 1. Fixed LocalStorage Sandbox Errors
- Wrapped all `localStorage` calls in try-catch blocks
- Graceful fallback when localStorage is unavailable
- Theme preference won't persist in sandboxed environments (but will still work)

### 2. Improved Thick Client Initialization
- **New `setupThickClientListener()` function**:
  - Waits for `window.ReportApp` to be defined (with 10 retry limit)
  - Checks for required methods (`refresh`, `getParam`)
  - Uses `window.ReportApp.on('ready', ...)` pattern from thick client guide
  - Handles case where `.on` method doesn't exist

### 3. Better Error Handling
- Shows user-friendly "waiting" messages instead of immediate fatal errors
- Limits retry attempts (10 tries, 500ms apart = 5 seconds total)
- Provides clear fatal error if thick client never loads
- Debug logging to console for troubleshooting

### 4. Code Structure Changes
- Separated UI initialization from thick client waiting
- UI can initialize immediately (skeleton screens, theme setup)
- Data loading waits for thick client readiness
- Removed old immediate-check pattern that caused failures

## Key Code Changes

### LocalStorage Fix
```javascript
try {
    localStorage.setItem('report-theme', isDark ? 'dark' : 'light');
} catch (e) {
    console.log('LocalStorage not available (sandboxed), theme preference not saved');
}
```

### Thick Client Listener Pattern
```javascript
function setupThickClientListener() {
    // Wait for window.ReportApp to be defined
    if (!window.ReportApp) {
        setTimeout(setupThickClientListener, 500);
        return;
    }
    
    // Check if already ready
    if (window.ReportApp.getParam && typeof window.ReportApp.refresh === 'function') {
        initializeReport();
        return;
    }
    
    // Wait for ready event
    window.ReportApp.on('ready', function() {
        initializeReport();
    });
}
```

## Testing Notes

### Expected Behavior
1. **Normal case**: Thick client loads within 1-2 seconds, report initializes normally
2. **Slow load case**: Report shows skeleton screens while waiting, then loads data
3. **Sandboxed case**: Theme preferences don't persist, but report still works
4. **Failure case**: After 5 seconds, shows clear error message

### Debugging Tips
- Check browser console for `window.ReportApp` status
- Look for network requests to `/refresh` endpoint
- Verify `window.ReportConfig` exists (injected by server)
- Check for any console errors from thick client script

## Backend Considerations

If thick client consistently fails to load:

1. **Check server logs** for HMAC validation errors
2. **Verify** thick client script is being injected
3. **Check** if report is being served with sandbox headers
4. **Test** with `ENABLE_PUBLIC_PATHS=true` for development

## User Instructions

If encountering "Thick client not available" error:

1. **Refresh the page** (with cache clear: Ctrl+F5 or Cmd+Shift+R)
2. **Check URL** - must be accessed via `/api/embed` endpoint
3. **Contact support** if error persists after refresh

---

**Note**: The fixes require the updated `dashboard.html` file. If server is caching old version, may need server restart or cache clear.