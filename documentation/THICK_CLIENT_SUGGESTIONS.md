# Thick Client Suggestions

**Date:** 2026-05-09  
**Author:** Report Developer  
**Context:** Review of `customer_sql_reporting` report — initial load showed no data despite valid URL and console log confirming dashboard initialization.

---

## Issues Found and Fixes Applied

### 1. CRITICAL: Thick client script not loaded in report.html

**Problem:** The `report.html` had a comment saying "Thick Client is automatically injected by the server" but contained **no `<script>` tag** to load `/static/thick_client.js`. Without the thick client, `window.ReportApp` is never defined, so the dashboard silently does nothing.

**Fix Applied:** Added `<script src="/static/thick_client.js"></script>` to `report.html` in the `<head>` section.

**Impact:** This is the **primary cause** of the no-data issue. The thick client is the data bridge between the reporting app server and the report's JavaScript. Without it, `window.ReportApp.refresh()` is undefined and no data is ever fetched.

---

### 2. SQL NULL comparison bug in `referral_status_filter` (3 queries)

**Problem:** In SQL, `column = NULL` **always evaluates to NULL** (not true). When a specific referral status value is passed via the URL parameter, the filter silently matches zero rows because the `= NULL` comparison never succeeds.

**Fix Applied:** Changed all 3 occurrences in `report.yaml` from:

```sql
AND ({{referral_status_filter}} IS NULL OR r.referral_status = {{referral_status_filter}})
```

to:

```sql
AND ({{referral_status_filter}} IS NULL OR r.referral_status IS NULL OR r.referral_status = {{referral_status_filter}})
```

**Impact:** When `referral_status_filter` is set to a specific value (e.g., "Completed"), the query now correctly matches rows where the status equals that value **or** is NULL.

---

### 3. Improved error visibility in report.html

**Problem:** When the thick client is unavailable or data loading fails, the error messages were too generic to diagnose.

**Fix Applied:** Added detailed console logging and error messages:

- Logs `window.ReportApp` and `window.ReportConfig` values when thick client is missing
- Logs parsed `ReportConfig.params` for debugging
- Adds error stack traces in catch blocks
- Shows raw error strings in toast messages

**Impact:** Future issues will be diagnosable from the browser console without needing to inspect server logs.

---

## Technical Requests for reporting_app

### Request 1: Auto-inject thick client script in embed handler

**Problem:** The embed handler's `renderReport()` only injects `ReportConfig` but does **not** inject the thick client script. Report developers must manually add `<script src="/static/thick_client.js"></script>` to every `report.html`. This is a silent failure point.

**Proposed change in `internal/handler/embed.go`, `renderReport()`:**

```go
func (h *EmbedHandler) renderReport(w http.ResponseWriter, r *http.Request, report *core.Report, params map[string]string) {
    htmlPath := filepath.Join("reports", report.ID, "report.html")
    content, err := os.ReadFile(htmlPath)
    if err != nil {
        http.Error(w, "Report HTML not found", http.StatusInternalServerError)
        return
    }
    
    config := generateReportConfig(report, params, r.URL.String())
    htmlWithConfig := injectReportConfig(string(content), config)
    
    // Auto-inject thick client script if not already present
    if !strings.Contains(htmlWithConfig, "thick_client.js") {
        thickClientScript := "<script src=\"/static/thick_client.js\"></script>"
        bodyCloseIndex := strings.LastIndex(htmlWithConfig, "</body>")
        if bodyCloseIndex != -1 {
            htmlWithConfig = htmlWithConfig[:bodyCloseIndex] + thickClientScript + "\n" + htmlWithConfig[bodyCloseIndex:]
        }
    }
    
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.Write([]byte(htmlWithConfig))
}
```

**Reason:** Eliminates a class of silent failures where report developers forget to add the thick client script. The thick client is a **required dependency** for all embedded reports — making it automatic removes an entire failure mode.

---

### Request 2: Thick client should emit `report:data-ready` DOM event

**Problem:** Per the thick client design docs, the thick client should emit a `report:data-ready` event for decoupled notification. But the current implementation only has a custom `emit`/`on` system and doesn't dispatch a native DOM event.

**Proposed change in `static/thick_client.js`, `init()`:**

```javascript
function init() {
    DataStore.init();
    
    // Emit native DOM event for decoupled notification
    document.dispatchEvent(new CustomEvent('report:data-ready', {
        detail: { data: DataStore.params, reportId: DataStore.config.reportId }
    }));
    
    // Also emit via custom event system for backward compatibility
    if (global.ReportApp.emit) {
        global.ReportApp.emit('ready', { reportId: DataStore.config.reportId });
    }
    
    console.log('Thick client ready');
}
```

**Reason:** The design docs specify this event as part of the contract between thick client and reports. Reports may rely on it for initialization. Native DOM events allow multiple listeners and follow web platform standards.

---

### Request 3: Thick client should validate `ReportConfig` before exposing `ReportApp`

**Problem:** If `window.ReportConfig` is missing, the thick client still exposes `window.ReportApp` with an empty data store. This causes confusing silent failures — `window.ReportApp` exists but fails on the first method call.

**Proposed change in `static/thick_client.js`, `init()`:**

```javascript
function init() {
    if (!window.ReportConfig) {
        console.error('FATAL: ReportConfig not found. Thick client will not initialize.');
        // Do NOT expose window.ReportApp — it would be broken
        return;
    }
    DataStore.init();
    // ... rest of init ...
}
```

**Reason:** Prevents the thick client from silently exposing a broken `ReportApp` object. If `ReportConfig` is missing, it's a server-side bug — the report should fail fast and loudly rather than silently do nothing.

---

### Request 4: Consolidate script injection into `injectReportConfig`

**Alternative to Request 1:** Instead of auto-injecting in `renderReport()`, pass a flag to `injectReportConfig` to include the thick client script.

**Proposed change in `internal/handler/embed.go`, `injectReportConfig`:**

```go
func injectReportConfig(htmlContent string, configJSON string, includeThickClient bool) string {
    configScript := fmt.Sprintf("<script>window.ReportConfig = %s;</script>", configJSON)
    
    bodyCloseIndex := strings.LastIndex(htmlContent, "</body>")
    if bodyCloseIndex == -1 {
        if includeThickClient {
            return htmlContent + "\n<script src=\"/static/thick_client.js\"></script>\n" + configScript
        }
        return htmlContent + "\n" + configScript
    }
    
    result := htmlContent[:bodyCloseIndex] + configScript
    if includeThickClient {
        result += "\n<script src=\"/static/thick_client.js\"></script>"
    }
    return result + "\n" + htmlContent[bodyCloseIndex:]
}
```

**Reason:** Cleaner API, single source of truth for script injection. `renderReport()` would call `injectReportConfig(html, config, true)`.

---

## Summary of Changes Made in Reports Directory

| File | Change | Type |
|------|--------|------|
| `report.html` | Added `<script src="/static/thick_client.js"></script>` | Critical fix |
| `report.html` | Enhanced console logging and error messages | Improvement |
| `report.yaml` | Fixed NULL comparison in `referral_status_filter` (3 queries) | Bug fix |

---

## Recommendation Priority

1. **High:** Request 1 — Auto-inject thick client script (preventive)
2. **High:** Request 3 — Validate `ReportConfig` before exposing `ReportApp` (fail fast)
3. **Medium:** Request 2 — Emit `report:data-ready` DOM event (contract compliance)
4. **Medium:** Request 4 — Consolidate script injection (cleanup, alternative to #1)

---

*Document created during review of customer_sql_reporting report.*
