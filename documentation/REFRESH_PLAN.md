# Runtime Refresh of Reports and Snippets: Plan

**Version:** 1.0  
**Date:** 2026-05-17  
**Status:** Planned — Not Implemented

---

## 1. Overview

Currently, the reporting application loads all reports and snippets at startup and caches them in memory. To pick up changes to `report.yaml` files or snippet YAML files, the entire application must be restarted. This creates friction during development and requires downtime for production updates.

This plan introduces **runtime refresh capability** — the ability to reload reports and snippets without stopping the application.

### 1.1 Goals

1. **Developer Experience** — Edit a report's `report.yaml` or a snippet file and have changes take effect immediately
2. **Zero-Downtime Updates** — Deploy new report definitions or snippet changes without service interruption  
3. **Security** — Maintain the existing security model; reload capability doesn't create new attack vectors
4. **Simplicity** — Minimal code changes, leverage existing patterns and interfaces
5. **Observability** — Clear logging of reload events and outcomes

### 1.2 Non-Goals

1. **Hot reload of HTML/JS/CSS** — Report HTML files are served directly from disk; browser cache controls handle this
2. **Database connection refresh** — Database configurations remain static (require restart to change)
3. **Dynamic configuration** — Environment variables and security settings remain startup-only
4. **UI for reloading** — This is an operational feature, not a user-facing one

---

## 2. Design Decisions

### 2.1 Dual Activation Mechanisms

We'll support **two complementary activation mechanisms**:

| Mechanism | Trigger | Use Case |
|-----------|---------|----------|
| **SIGHUP Signal** | `kill -HUP <pid>` | Operations team, developers in terminal |
| **Admin HTTP Endpoint** | `POST /admin/reload` | CI/CD pipelines, automation tools |

**Rationale:** Signals are POSIX-standard and secure (requires server access). HTTP endpoints are accessible remotely for automation. Both have valid use cases.

### 2.2 Opt-in Security

Both mechanisms will be **disabled by default** and require explicit opt-in via environment variables:

```bash
# Enable SIGHUP reload capability
ENABLE_SIGHUP_RELOAD=true  # default: false

# Enable admin HTTP endpoints (requires ADMIN_TOKEN)
ENABLE_ADMIN_API=true      # default: false  
ADMIN_TOKEN=supersecret123 # required if ENABLE_ADMIN_API=true
```

**Rationale:** Security-by-default. Production deployments that don't need runtime reload won't have the capability exposed.

### 2.3 Atomic Updates

The reload process will be **atomic**:

1. Load all new reports from disk
2. Load all new snippets from disk  
3. **Only if both succeed**, atomically update the handlers
4. If either fails, keep the previous state and log the error

**Rationale:** Prevents partial updates where reports reference snippets that aren't loaded.

### 2.4 Thread Safety

The reload operation will use a **read-write lock** pattern:

- **Read lock** — Acquired by each HTTP request while accessing reports/snippets
- **Write lock** — Acquired during reload, blocks new requests temporarily

**Rationale:** Prevents data races during reloads. HTTP requests may briefly wait but won't fail.

### 2.5 Logging & Observability

Comprehensive logging at each stage:

```
INFO  Received SIGHUP, beginning reload...
INFO  Loaded 15 reports from ./reports
INFO  Loaded 8 snippets from ./snippets  
INFO  Reload successful, now serving 15 reports, 8 snippets
INFO  Admin API: Reports and snippets reloaded by client 192.168.1.100
```

**Rationale:** Operators need visibility into reload operations and outcomes.

---

## 3. Implementation Plan

### 3.1 Phase 1: Thread-Safe Loader Wrapper

Create a thread-safe wrapper around the existing `loader.Loader`:

```go
// internal/loader/safe_loader.go
type SafeLoader struct {
    mu      sync.RWMutex
    loader  *Loader
    reportsDir string
}

func (s *SafeLoader) GetReport(id string) (*core.Report, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.loader.GetReport(id)
}

func (s *SafeLoader) Reload() error {
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.loader.Reload()
}
```

### 3.2 Phase 2: Snippet Manager with Atomic Updates

Create a snippet manager that supports atomic updates:

```go
// internal/loader/snippet_manager.go  
type SnippetManager struct {
    mu       sync.RWMutex
    snippets map[string]*Snippet
    dir      string
}

func (s *SnippetManager) Get(name string) (*Snippet, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    snippet, ok := s.snippets[name]
    return snippet, ok
}

func (s *SnippetManager) Reload() error {
    newSnippets, err := LoadSnippets(s.dir)
    if err != nil {
        return err
    }
    
    s.mu.Lock()
    defer s.mu.Unlock()
    s.snippets = newSnippets
    return nil
}
```

### 3.3 Phase 3: Main Application Integration

Update `main.go` to use the thread-safe components and add reload triggers:

```go
// main.go additions
func main() {
    // ... existing initialization
    
    // Create thread-safe components
    safeLoader := loader.NewSafeLoader(config.reportsDir)
    snippetManager := loader.NewSnippetManager(config.snippetsDir)
    
    // Update handlers to use thread-safe components
    embedHandler := handler.NewEmbedHandler(
        safeLoader,       // Instead of raw loader
        // ... other args
    )
    embedHandler.SetSnippetManager(snippetManager)  // Instead of SetSnippets
    
    // Setup reload triggers if enabled
    if os.Getenv("ENABLE_SIGHUP_RELOAD") == "true" {
        setupSIGHUPHandler(safeLoader, snippetManager)
    }
    
    if os.Getenv("ENABLE_ADMIN_API") == "true" {
        setupAdminEndpoints(safeLoader, snippetManager, mux)
    }
    
    // ... start server
}
```

### 3.4 Phase 4: SIGHUP Handler

Implement the signal handler:

```go
func setupSIGHUPHandler(loader *loader.SafeLoader, snippets *loader.SnippetManager) {
    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, syscall.SIGHUP)
    
    go func() {
        for range sigs {
            log.Println("🔄 SIGHUP received - reloading reports and snippets")
            
            // Reload reports first
            if err := loader.Reload(); err != nil {
                log.Printf("❌ Failed to reload reports: %v", err)
                continue
            }
            
            // Reload snippets second  
            if err := snippets.Reload(); err != nil {
                log.Printf("❌ Failed to reload snippets: %v", err)
                // Continue anyway - reports reloaded successfully
            }
            
            reportCount := len(loader.ListReports())
            snippetCount := snippets.Count()
            log.Printf("✅ Reload complete. Now serving %d reports, %d snippets", 
                reportCount, snippetCount)
        }
    }()
}
```

### 3.5 Phase 5: Admin HTTP Endpoints

Add protected admin endpoints:

```go
// internal/handler/admin.go
type AdminHandler struct {
    loader   *loader.SafeLoader
    snippets *loader.SnippetManager
    token    string
}

func (h *AdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Verify admin token
    if r.Header.Get("Authorization") != "Bearer "+h.token {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    
    switch r.URL.Path {
    case "/admin/reload":
        h.handleReload(w, r)
    case "/admin/status":
        h.handleStatus(w, r)
    }
}

func (h *AdminHandler) handleReload(w http.Response.Request) {
    // Similar logic to SIGHUP handler
    // Return JSON with success/failure and counts
}
```

---

## 4. API Design

### 4.1 SIGHUP Signal

No API changes needed. Standard POSIX signal:

```bash
# Find the PID
ps aux | grep reporting_app

# Send SIGHUP
kill -HUP 12345
```

### 4.2 Admin HTTP Endpoints

**Endpoint:** `POST /admin/reload`  
**Authentication:** Bearer token (from ADMIN_TOKEN env var)

**Request:**
```bash
curl -X POST http://localhost:8080/admin/reload \
  -H "Authorization: Bearer supersecret123"
```

**Response (success):**
```json
{
  "status": "success",
  "reports": 15,
  "snippets": 8,
  "timestamp": "2026-05-17T14:30:00Z"
}
```

**Response (partial failure):**
```json
{
  "status": "partial",
  "reports": 15,
  "snippets": 8,
  "warnings": ["Failed to reload snippets: open ./snippets: permission denied"],
  "timestamp": "2026-12-17T14:31:00Z"
}
```

**Response (failure):**
```json
{
  "status": "error",
  "error": "Failed to reload reports: invalid YAML in reports/broken/report.yaml",
  "timestamp": "2026-05-17T14:32:00Z"
}
```

**Endpoint:** `GET /admin/status`  
**Authentication:** Bearer token

**Response:**
```json
{
  "status": "ok",
  "reports": 15,
  "snippets": 8,
  "uptime": "2h15m",
  "memory_mb": 45.2
}
```

---

## 5. Security Considerations

### 5.1 Threat Model

| Threat | Mitigation |
|--------|------------|
| **Unauthorized reload** | Bearer token required for HTTP, POSIX signals require server access |
| **Token leakage** | ADMIN_TOKEN only in environment, not in code/logs |
| **Denial of service** | Rate limiting on admin endpoints, reloads queued/limited |
| **Information disclosure** | Admin endpoints disabled by default |

### 5.2 Configuration Security

- ✅ `ENABLE_SIGHUP_RELOAD=false` by default
- ✅ `ENABLE_ADMIN_API=false` by default  
- ✅ `ADMIN_TOKEN` required if admin API enabled
- ✅ Tokens validated on every admin request
- ✅ No default/fallback tokens

### 5.3 Rate Limiting

Admin endpoints will implement basic rate limiting:

```go
// Simple in-memory rate limiting
var reloadLastAttempt time.Time
var reloadMinInterval = 5 * time.Second

func (h *AdminHandler) handleReload(w http.ResponseWriter, r *http.Request) {
    if time.Since(reloadLastAttempt) < reloadMinInterval {
        http.Error(w, "Rate limited", http.StatusTooManyRequests)
        return
    }
    reloadLastAttempt = time.Now()
    // ... handle reload
}
```

### 5.4 Audit Logging

All admin actions logged with:

- Timestamp
- Client IP address  
- Action performed
- Outcome (success/failure)
- Report/snippet counts after change

---

## 6. Error Handling

### 6.1 Graceful Degradation

- If reports reload fails **but snippets succeed**: Keep old reports, log error
- If snippets reload fails **but reports succeed**: Use old snippets, log warning  
- If both fail: Keep full old state, log error

### 6.2 Error Recovery

Partial directory corruption handling:

1. Scan directory, skip invalid files with warnings
2. Load all valid reports/snippets
3. Update with whatever succeeded
4. Continue serving with partial update

### 6.3 Monitoring

Key metrics to monitor:

- `reload_attempts_total` — counter
- `reload_success_total` — counter  
- `reload_duration_seconds` — histogram
- `reports_loaded` — gauge
- `snippets_loaded` — gauge

---

## 7. Rollout Strategy

### 7.1 Development Testing

1. Implement thread-safe components
2. Unit test concurrent access patterns
3. Integration test with SIGHUP
4. Manual testing in development environment

### 7.2 Staging Validation

1. Deploy with features **disabled** (default)
2. Enable one mechanism at a time
3. Test CI/CD integration with admin endpoints
4. Validate security controls

### 7.3 Production Deployment

1. Ship features **disabled by default**
2. Document how to enable if needed
3. Monitor for adoption
4. Gather feedback for improvements

### 7.4 Backward Compatibility

- Existing code continues to work unchanged
- New environment variables optional
- Default behavior identical to current
- No breaking changes to APIs

---

## 8. Alternatives Considered

### 8.1 File System Watching (fsnotify)

**Pros:** Automatic, no manual triggers needed  
**Cons:** Complex, CPU overhead, race conditions, platform-specific issues  
**Decision:** Rejected — manual triggers are sufficient and more predictable

### 8.2 Single Mechanism Only

**Pros:** Simpler implementation  
**Cons:** Doesn't cover all use cases (signals for ops, HTTP for automation)  
**Decision:** Rejected — both mechanisms have valid use cases

### 8.3 Reload via Existing HTTP API

**Pros:** No new endpoints  
**Cons:** Confuses security model, mixes concerns  
**Decision:** Rejected — dedicated admin API is cleaner

### 8.4 Configuration-Driven Reload

**Schedule reloads via cron configuration**  
**Pros:** Predictable, automated  
**Cons:** Less responsive, adds configuration complexity  
**Decision:** Deferred — can be added later if needed

---

## 9. Future Enhancements

### 9.1 Health Check Integration

Add reload capability to health checks:

```
GET /health
{
  "status": "healthy",
  "reports_loaded": 15,
  "snippets_loaded": 8,
  "last_reload": "2026-05-17T14:30:00Z"
}
```

### 9.2 Reload Webhook

Fire webhook notifications after successful reloads:

```yaml
# Configurable webhooks
RELOAD_WEBHOOK_URL=https://hooks.slack.com/...
RELOAD_WEBHOOK_EVENTS=success,error
```

### 9.3 Dry Run Mode

```bash
curl -X POST /admin/reload?dry_run=true
```
Returns what would change without actually reloading.

### 9.4 Per-Report/Snippet Reload

Reload individual reports or snippets instead of all:

```
POST /admin/reload/report/{report_id}
POST /admin/reload/snippet/{snippet_name}
```

---

## 10. Success Criteria

1. **✅ Reports** — Can be reloaded without restart
2. **✅ Snippets** — Can be reloaded without restart  
3. **✅ Thread Safety** — Concurrent requests don't crash or see inconsistent state
4. **✅ Security** — No new attack vectors introduced
5. **✅ Observability** — Clear logs and status endpoints
6. **✅ Backward Compatible** — Existing deployments unaffected
7. **✅ Performance** — No noticeable impact on request latency

---

**Implementation Priority:** Medium  
**Estimated Effort:** 1-2 developer weeks  
**Risk Level:** Low (optional feature, disabled by default)