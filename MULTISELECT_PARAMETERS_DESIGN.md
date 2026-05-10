# Multi-Select Parameter Design

## 1. Overview

This document describes the design for supporting multiple values per parameter in the reporting_app. Currently, every parameter is a single string value (`map[string]string` everywhere). This design enables parameters like `organization_id=1&organization_id=2` to be used in SQL `IN` clauses and other multi-value contexts.

### 1.1 Goals

- Allow report authors to declare parameters that accept multiple values
- Maintain backward compatibility: single-value parameters continue working unchanged
- Preserve security model: HMAC signatures, immutable/mutable classification, nonce tracking
- Provide a clear, consistent API for thick client JavaScript
- No new SQL injection vectors — parameter expansion happens only after safe placeholder injection

### 1.2 Non-Goals

- No change to the HMAC algorithm itself (still HMAC-SHA256)
- No change to the nonce or expiry model
- No change to database connection pooling or query logging infrastructure
- No change to the thick client's role as the sole data bridge

---

## 2. Core Type System Changes

### 2.1 `internal/core/types.go`

#### 2.1.1 `ParamSet` — Change map type from `string` to `[]string`

**Current:**
```go
type ParamSet struct {
    Immutable map[string]string
    Mutable   map[string]string
}
```

**New:**
```go
type ParamSet struct {
    Immutable map[string][]string
    Mutable   map[string][]string
}
```

**Consequences:**
- Every caller of `ParamSet` must be updated
- Empty parameter: change from `""` (empty string) to `[]string{""}` (slice with one empty string) — this preserves the "optional parameter provided but empty" semantics (used for SQL NULL)
- A truly absent parameter is `nil` or not present in the map
- JSON marshaling/unmarshaling behavior changes: `map[string]string` → `map[string][]string` produces different JSON. Single value `{"status": "active"}` becomes `{"status": ["active"]}`

#### 2.1.2 `Report` struct — No change needed

The `ImmutableParams []string` and `MutableParams []string` fields are just lists of parameter *names*. They do not hold values, so they remain unchanged.

#### 2.1.3 `Report.ExtractImmutable` — Signature change

**Current:**
```go
func (r *Report) ExtractImmutable(params map[string]string) map[string]string
```

**New:**
```go
func (r *Report) ExtractImmutable(params map[string][]string) map[string][]string
```

**Consequences:**
- Return type changes from `map[string]string` to `map[string][]string`
- All callers pass `map[string][]string` now
- The function iterates `r.ImmutableParams` and collects all values for each name from the input map

#### 2.1.4 `Report.ValidateParams` — Signature change

**Current:**
```go
func (r *Report) ValidateParams(params map[string]string) error
```

**New:**
```go
func (r *Report) ValidateParams(params map[string][]string) error
```

**Consequences:**
- Validation logic: check that each key exists in the declared param lists (unchanged)
- No new validation needed per-value — the values are opaque strings, validated only by the SQL layer
- If a parameter is declared as multi-value, we may want to add a `max_values` constraint (see Section 8)

#### 2.1.5 `Report.ContainsParam` and `Report.IsImmutable` — No change needed

These iterate over parameter name lists, not values. Unchanged.

#### 2.1.6 New: `Report.IsMultiValue` method

**Add:**
```go
// IsMultiValue checks if a parameter supports multiple values.
// Returns true if the parameter name is declared with multi_value: true
// in the report YAML (see Section 6).
func (r *Report) IsMultiValue(name string) bool {
    for _, n := range r.MultiValueParams {
        if n == name {
            return true
        }
    }
    return false
}
```

**Consequences:**
- Requires `MultiValueParams []string` field on `Report` struct
- This is an opt-in declaration: report authors explicitly mark which params support multi-values
- Default (absent from the list): single-value behavior

### 2.2 `internal/core/constants.go` — No change needed

All constants are about defaults, limits, and system parameter names. Unchanged.

---

## 3. Report YAML Schema Changes

### 3.1 New `multi_value_params` field in `report.yaml`

Add a new top-level field to the report YAML:

```yaml
id: example_dashboard
name: "Example Dashboard"
...

# Existing fields (unchanged)
immutable_params:
  - organization_id
  - user_id

mutable_params:
  - start_date
  - end_date
  - status

# NEW: Parameters that accept multiple values
multi_value_params:
  - organization_id   # immutable multi-value (in HMAC)
  - status            # mutable multi-value (user filter)

# Existing datasources (unchanged)
datasources:
  ...
```

**Consequences:**
- `multi_value_params` is a list of parameter names (strings)
- A parameter can appear in both `mutable_params` and `multi_value_params`
- A parameter can appear in both `immutable_params` and `multi_value_params` (multi-select on immutable filters like "organizations")
- If a parameter name appears in `multi_value_params` but NOT in `immutable_params` or `mutable_params`, it is silently ignored (loader emits a warning)
- The field is optional; omitting it means no parameters support multi-values

### 3.2 SQL Syntax for Multi-Value Expansion

#### 3.2.1 Default behavior: `{{param}}` expands based on value count

The placeholder `{{param}}` in SQL is expanded dynamically:

| Value count | SQL expansion | Args |
|-------------|---------------|------|
| 0 (absent/nil) | `1=1` (no-op) | `[]` |
| 1 (single value) | `col = ?` | `["value"]` |
| 2+ (multi value) | `col IN (?, ?, ...)` | `["v1", "v2", ...]` |

**Example expansion:**
```sql
-- Original SQL:
SELECT * FROM orders WHERE status = {{status}}

-- status absent (nil or empty slice):
SELECT * FROM orders WHERE 1=1

-- status = ["active"]:
SELECT * FROM orders WHERE status = ?
-- args: ["active"]

-- status = ["active", "pending"]:
SELECT * FROM orders WHERE status IN (?, ?)
-- args: ["active", "pending"]
```

**Critical consequence for SQL authors:** The placeholder replaces the *entire* comparison expression (`col = {{param}}`), not just the value. If the SQL has `WHERE col = {{param}} AND other = 1`, the expansion works because `{{param}}` is replaced by either `?` or `IN (?, ?, ...)`. This is valid SQL in all cases.

#### 3.2.2 Explicit mode: `{{param:in}}` and `{{param:eq}}`

| Syntax | Expansion | Use case |
|--------|-----------|----------|
| `{{param}}` | Auto: `= ?` for 1 value, `IN (?,...)` for 2+ | Default, works for most cases |
| `{{param:in}}` | Always `IN (?,...)` | Always expect multi-value, even if only 1 value |
| `{{param:eq}}` | Always `= ?` | Always expect single-value |

**Consequences:**
- `{{param}}` (bare) is the default and most convenient: it auto-expands based on runtime value count
- When `{{param}}` expands to `IN (?,...)`, the surrounding SQL must be valid with an `IN` clause
- Report authors should use `{{param:eq}}` if they need to force single-value behavior even on a multi-value param
- Report authors should use `{{param:in}}` if they always want multi-value semantics

### 3.3 YAML Parsing Changes (`internal/loader/loader.go`)

#### 3.3.1 Add `MultiValueParams` field to raw struct

In `parseReportYAML`, add to the raw struct:

```go
var raw struct {
    ...
    MultiValueParams []string `yaml:"multi_value_params"`
    ...
}
```

**Consequences:**
- `yaml.v3` handles this as a simple string list — no special parsing needed
- If the YAML omits the field, `raw.MultiValueParams` is `nil` (Go zero value for `[]string`)

#### 3.3.2 Add `MultiValueParams` to `core.Report`

Add to the `Report` struct:

```go
type Report struct {
    ...
    ImmutableParams   []string
    MutableParams     []string
    MultiValueParams  []string   // NEW
    ...
}
```

**Consequences:**
- The loader sets `report.MultiValueParams = raw.MultiValueParams` after parsing
- No conversion or validation needed at load time — the list is opaque
- Validation of which params are actually multi-value happens at query time via `Report.IsMultiValue()`

#### 3.3.3 Validate multi_value_params references

Add validation in `ValidateReport`:

```go
func ValidateReport(report *core.Report) error {
    // ... existing validation ...
    
    allDeclared := make(map[string]bool)
    for _, n := range report.ImmutableParams {
        allDeclared[n] = true
    }
    for _, n := range report.MutableParams {
        allDeclared[n] = true
    }
    
    for _, n := range report.MultiValueParams {
        if !allDeclared[n] {
            return fmt.Errorf("multi_value_params references undeclared parameter '%s'", n)
        }
    }
    
    return nil
}
```

**Consequences:**
- A parameter in `multi_value_params` MUST also be in `immutable_params` or `mutable_params`
- Invalid YAML is caught at load time, not at query time
- This prevents typos from silently becoming no-ops

---

## 4. Security / HMAC Changes

### 4.1 `internal/security/hmac.go`

#### 4.1.1 `SignURL` — Signature change

**Current:**
```go
func SignURL(reportID string, expires int64, nonce string, immutableParams map[string]string, secret []byte) string
```

**New:**
```go
func SignURL(reportID string, expires int64, nonce string, immutableParams map[string][]string, secret []byte) string
```

**Consequences:**
- The canonical parameter string must now handle multiple values per key
- All callers pass `map[string][]string` now

#### 4.1.2 `canonicalParams` — Major change

**Current:**
```go
func canonicalParams(params map[string]string) string {
    // Collect and sort keys
    keys := make([]string, 0, len(params))
    for k := range params {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    
    // Build canonical string
    var buf strings.Builder
    for i, k := range keys {
        if i > 0 {
            buf.WriteString("&")
        }
        buf.WriteString(k)
        buf.WriteString("=")
        buf.WriteString(url.QueryEscape(params[k]))
    }
    return buf.String()
}
```

**New:**
```go
func canonicalParams(params map[string][]string) string {
    if len(params) == 0 {
        return ""
    }
    
    // Collect and sort keys
    keys := make([]string, 0, len(params))
    for k := range params {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    
    // Build canonical string
    var buf strings.Builder
    for i, k := range keys {
        if i > 0 {
            buf.WriteString("&")
        }
        buf.WriteString(k)
        buf.WriteString("=")
        // Sort values within each key for determinism
        values := make([]string, len(params[k]))
        copy(values, params[k])
        sort.Strings(values)
        for j, v := range values {
            if j > 0 {
                buf.WriteString(",")
            }
            buf.WriteString(url.QueryEscape(v))
        }
    }
    return buf.String()
}
```

**Consequences:**
- Canonical form changes from `key=value&key2=value2` to `key=val1,val2&key2=val3`
- Values within each key are sorted alphabetically for determinism (so `org=2,1` becomes `org=1,2`)
- Values are comma-separated within a key — this delimiter must not appear in URL-encoded values (url.QueryEscape handles this)
- **Breaking change:** HMAC signatures generated with the old format will NOT verify with the new format
- **Migration implication:** All existing signed URLs become invalid after deployment. The parent application must regenerate URLs after the reporting_app is updated
- If backward compatibility is needed during a transition period, support both formats in `canonicalParams` with a version prefix or heuristic (rare old-style = no commas in value positions)

#### 4.1.3 `VerifyURL` — Signature change

**Current:**
```go
func VerifyURL(reportID string, expires int64, nonce string, immutableParams map[string]string, sig string, secret []byte) bool
```

**New:**
```go
func VerifyURL(reportID string, expires int64, nonce string, immutableParams map[string][]string, sig string, secret []byte) bool
```

**Consequences:**
- Calls `SignURL` internally, so uses the new canonical form
- All callers pass `map[string][]string` now

#### 4.1.4 `ExtractParams` — Change to return `map[string][]string`

**Current:**
```go
func ExtractParams(query url.Values) map[string]string {
    params := make(map[string]string)
    for key, values := range query {
        if key == core.ParamReportID || key == core.ParamExpires || 
           key == core.ParamNonce || key == core.ParamSig {
            continue
        }
        if len(values) > 0 {
            params[key] = values[0]
        } else {
            params[key] = ""
        }
    }
    return params
}
```

**New:**
```go
func ExtractParams(query url.Values) map[string][]string {
    params := make(map[string][]string)
    for key, values := range query {
        if key == core.ParamReportID || key == core.ParamExpires || 
           key == core.ParamNonce || key == core.ParamSig {
            continue
        }
        if len(values) > 0 {
            params[key] = values  // Keep ALL values
        } else {
            params[key] = []string{""}  // Single empty string for absent-but-present
        }
    }
    return params
}
```

**Consequences:**
- `r.URL.Query()` already returns `url.Values` (i.e., `map[string][]string`), so we now use the full slice
- A parameter with `key=value1&key=value2` produces `params[key] = []string{"value1", "value2"}`
- A parameter with `key=` (empty value) produces `params[key] = []string{""}`
- A parameter with just `key` (no `=`) produces `params[key] = []string{""}`
- Return type changes from `map[string]string` to `map[string][]string`
- All callers must be updated

#### 4.1.6 HMAC Migration Strategy

Because the canonical form changes, this is a **breaking change** to the HMAC protocol:

1. **Deploy strategy:** The reporting_app must be deployed first (new canonical form). The parent application must be deployed second (generates URLs with the new format).
2. **Transition period:** During overlap, old signed URLs from the parent app will fail verification. This is expected and acceptable.
3. **No rollback:** The old canonical form cannot be supported alongside the new one without adding a version flag, which adds complexity. The simplest approach is a coordinated deploy with a known downtime window.
4. **Testing:** Before deploy, generate test URLs with the new canonical form and verify they produce the expected signature.

---

## 5. Loader / SQL Injection Changes

### 5.1 `internal/loader/validator.go` — `InjectParams` rewrite

This is the **most critical** change. The current `InjectParams` function replaces each `{{param}}` with a single `?` and appends one arg. It must be rewritten to handle multi-value expansion.

**Current:**
```go
func InjectParams(sql string, params map[string]string) (string, []interface{}) {
    re := regexp.MustCompile(`\{\{(\w+)\}\}`)
    
    var result strings.Builder
    result.Grow(len(sql))
    var args []interface{}
    
    lastIndex := 0
    for _, match := range re.FindAllStringSubmatchIndex(sql, -1) {
        result.WriteString(sql[lastIndex:match[0]])
        paramName := sql[match[2]:match[3]]
        result.WriteString("?")
        
        if value, ok := params[paramName]; ok {
            if value == "" {
                args = append(args, nil)
            } else {
                args = append(args, value)
            }
        } else {
            args = append(args, nil)
        }
        
        lastIndex = match[1]
    }
    
    result.WriteString(sql[lastIndex:])
    return result.String(), args
}
```

**New:**
```go
func InjectParams(sql string, params map[string][]string, report *core.Report) (string, []interface{}) {
    re := regexp.MustCompile(`\{\{(\w+)(?::(\w+))?\}\}`)
    
    var result strings.Builder
    result.Grow(len(sql))
    var args []interface{}
    
    lastIndex := 0
    for _, match := range re.FindAllStringSubmatchIndex(sql, -1) {
        result.WriteString(sql[lastIndex:match[0]])
        
        // Extract parameter name and optional mode
        paramName := sql[match[2]:match[3]]
        mode := ""
        if match[4] != -1 {
            mode = sql[match[4]:match[5]]
        }
        
        // Determine values
        var values []string
        if vals, ok := params[paramName]; ok && vals != nil {
            values = vals
        }
        
        // Determine expansion based on mode and value count
        switch mode {
        case "in":
            // Always expand as IN clause
            placeholders := make([]string, len(values))
            for i := range values {
                placeholders[i] = "?"
                if len(values) == 0 {
                    args = append(args, nil)
                } else if values[i] == "" {
                    args = append(args, nil)
                } else {
                    args = append(args, values[i])
                }
            }
            if len(values) == 0 {
                result.WriteString("1=1")
            } else {
                result.WriteString("IN (" + strings.Join(placeholders, ",") + ")")
            }
        case "eq":
            // Always expand as single = ?
            if len(values) == 0 || values[0] == "" {
                result.WriteString("= ?")
                args = append(args, nil)
            } else {
                result.WriteString("= ?")
                args = append(args, values[0])
            }
        case "":
            // Auto mode: expand based on value count
            if len(values) == 0 {
                result.WriteString("1=1")
            } else if len(values) == 1 {
                result.WriteString("= ?")
                if values[0] == "" {
                    args = append(args, nil)
                } else {
                    args = append(args, values[0])
                }
            } else {
                placeholders := make([]string, len(values))
                for i := range values {
                    placeholders[i] = "?"
                    if values[i] == "" {
                        args = append(args, nil)
                    } else {
                        args = append(args, values[i])
                    }
                }
                result.WriteString("IN (" + strings.Join(placeholders, ",") + ")")
            }
        default:
            // Unknown mode — treat as auto
            // (handled by falling through to auto)
            if len(values) == 0 {
                result.WriteString("1=1")
            } else if len(values) == 1 {
                result.WriteString("= ?")
                if values[0] == "" {
                    args = append(args, nil)
                } else {
                    args = append(args, values[0])
                }
            } else {
                placeholders := make([]string, len(values))
                for i := range values {
                    placeholders[i] = "?"
                    if values[i] == "" {
                        args = append(args, nil)
                    } else {
                        args = append(args, values[i])
                    }
                }
                result.WriteString("IN (" + strings.Join(placeholders, ",") + ")")
            }
        }
        
        lastIndex = match[1]
    }
    
    result.WriteString(sql[lastIndex:])
    return result.String(), args
}
```

**Consequences:**
- Function signature changes: `InjectParams(sql string, params map[string][]string, report *core.Report)` — the `report` parameter is needed for `IsMultiValue` checks if we want to validate mode usage
- Regex changes from `\{\{(\w+)\}\}` to `\{\{(\w+)(?::(\w+))?\}\}` to capture optional mode suffix
- For `{{param}}` with 0 values: injects `1=1` (always-true no-op) — this means the surrounding `AND` or `WHERE` must be valid with `1=1`
- For `{{param}}` with 1 value: injects `= ?` (single placeholder)
- For `{{param}}` with 2+ values: injects `IN (?, ?, ...)` (multi placeholder)
- **Important SQL authoring rule:** The placeholder must replace the entire comparison. SQL like `WHERE col = {{param}}` works because `{{param}}` is replaced by `?` or `IN (?, ...)`. SQL like `WHERE col IN ({{param}})` would produce `WHERE col IN (? IN (?,?))` which is invalid — authors should NOT use `IN (` before a `{{param}}` placeholder.
- For `{{param:in}}` with 0 values: injects `1=1` (no-op)
- For `{{param:eq}}` with 1+ values: always uses `= ?` (takes first value only)
- Empty string values (`""`) are converted to `nil` (SQL NULL) — same as current behavior
- The `args` slice grows dynamically: one entry per value per placeholder
- **Performance:** For a parameter with 100 values, this produces 100 `?` placeholders. MySQL has a `max_allowed_packet` limit and a query parameter limit (~65535). For practical multi-select filters, 10-50 values is typical and safe.

---

## 6. Handler Changes

### 6.1 `internal/handler/embed.go` — EmbedHandler

#### 6.1.1 `ServeHTTP` — Parameter extraction change

**Current:**
```go
allParams := security.ExtractParams(r.URL.Query())
```

**New:** Same call, but `allParams` is now `map[string][]string` instead of `map[string]string`.

**Consequences:**
- `ValidateParams` call: `report.ValidateParams(allParams)` — the signature now takes `map[string][]string`
- `ExtractImmutable` call: `immutableParams := report.ExtractImmutable(allParams)` — returns `map[string][]string`
- `SignURL` call (in HMAC path): `security.VerifyURL(..., immutableParams, ...)` — uses `map[string][]string`
- `generateReportConfig` call: the `params` field in the JSON config changes (see 6.1.3)

#### 6.1.2 `generateReportConfig` — JSON serialization change

**Current:**
```go
func generateReportConfig(report *core.Report, params map[string]string, currentURL string) string {
    paramsJSON, _ := json.Marshal(params)
    ...
}
```

**New:**
```go
func generateReportConfig(report *core.Report, params map[string][]string, currentURL string) string {
    paramsJSON, _ := json.Marshal(params)
    ...
}
```

**Consequences:**
- The JSON produced for `window.ReportConfig.params` changes from `{"status": "active"}` to `{"status": ["active"]}`
- **This is a breaking change for thick client JavaScript** — the thick client must parse arrays, not strings
- For single-value params, the array has length 1
- For absent params, the array may be `[]string{""}` or `nil` depending on how the URL was constructed
- The thick client must handle both formats during a transition period, or the deploy must be coordinated

#### 6.1.3 `injectReportConfig` — No change needed

This function just wraps JSON in a script tag. Unchanged.

### 6.2 `internal/handler/refresh.go` — RefreshHandler

#### 6.2.1 `RefreshRequest` — Change params type

**Current:**
```go
type RefreshRequest struct {
    Params map[string]string `json:"params"`
}
```

**New:**
```go
type RefreshRequest struct {
    Params map[string][]string `json:"params"`
}
```

**Consequences:**
- The thick client must send arrays in the JSON body: `{"params": {"status": ["active", "pending"]}}`
- Single-value params are sent as arrays of length 1: `{"params": {"status": ["active"]}}`
- Absent params are not included in the JSON body (they come from the URL)
- **Backward compatibility:** The thick client may send `"status": "active"` (string) during transition. The JSON decoder will fail to unmarshal a string into `[]string`. To handle this, add a custom unmarshaler:

```go
func (r *RefreshRequest) UnmarshalJSON(data []byte) error {
    type Alias RefreshRequest
    aux := struct {
        Params map[string]interface{} `json:"params"`
        *Alias
    }{
        Alias: (*Alias)(r),
    }
    if err := json.Unmarshal(data, &aux); err != nil {
        return err
    }
    
    r.Params = make(map[string][]string)
    for k, v := range aux.Params {
        switch val := v.(type) {
        case string:
            r.Params[k] = []string{val}
        case []interface{}:
            result := make([]string, len(val))
            for i, item := range val {
                result[i] = fmt.Sprintf("%v", item)
            }
            r.Params[k] = result
        }
    }
    return nil
}
```

#### 6.2.2 `mergeParams` — Change to handle `[]string`

**Current:**
```go
func (h *RefreshHandler) mergeParams(report *core.Report, original, new map[string]string) map[string]string
```

**New:**
```go
func (h *RefreshHandler) mergeParams(report *core.Report, original, new map[string][]string) map[string][]string
```

**Consequences:**
- Immutable parameter check: compare slices for equality (all values must match in order) or compare sorted sets of values
- For immutable params with multi-value support, the **entire set of values** must be unchanged, not just individual values. If `organization_id=[1,2]` in the URL and the refresh sends `organization_id=[1,3]`, this is a security violation because the set changed.
- Mutable params: the new values **replace** the old values entirely (not append). This is the simplest and most predictable behavior.
- If a mutable param is absent from the refresh request, its original values are preserved.

#### 6.2.3 `executeDatasources` — Pass `map[string][]string` to `InjectParams`

**Current:**
```go
queryResult, err := h.dbManager.ExecuteDatasource(db, ds, params, rowLimit, timeout, report.ID, name, report.Database)
```

**New:** Same call, but `params` is now `map[string][]string`. The `ExecuteDatasource` and `InjectParams` functions must be updated to accept and handle this type.

#### 6.2.4 `generateNextURL` — URL building change

**Current:**
```go
for key, value := range params {
    urlStr += fmt.Sprintf("&%s=%s", url.QueryEscape(key), url.QueryEscape(value))
}
```

**New:**
```go
for key, values := range params {
    for _, value := range values {
        urlStr += fmt.Sprintf("&%s=%s", url.QueryEscape(key), url.QueryEscape(value))
    }
}
```

**Consequences:**
- A multi-value param like `status=["active", "pending"]` produces `&status=active&status=pending` in the URL
- URL parameter order: keys are iterated in map order (non-deterministic in Go). For HMAC determinism, the URL order does NOT matter for the signature — the HMAC uses the canonical form, not the raw URL. However, the URL order affects the thick client's URL parsing.
- The thick client must parse repeated keys to reconstruct multi-value params
- For single-value params, the URL looks identical to before: `&status=active`

---

## 7. Thick Client JavaScript Changes

### 7.1 `static/thick_client.js` — DataStore

#### 7.1.1 `DataStore.params` — Change from `map<string, string>` to `map<string, string[]>`

**Current:**
```javascript
const DataStore = {
    params: {},
    
    getParam(key) {
        return this.params[key];  // returns string
    },
    
    setParam(key, value) {
        this.params[key] = value;  // value is string
    },
    
    getParams() {
        return { ...this.params };  // returns {key: string, ...}
    },
};
```

**New:**
```javascript
const DataStore = {
    params: {},
    
    getParam(key) {
        // Returns the first value, or undefined if absent
        const values = this.params[key];
        return values && values.length > 0 ? values[0] : undefined;
    },
    
    getParamValues(key) {
        // Returns all values for a key
        return this.params[key] ? [...this.params[key]] : [];
    },
    
    setParam(key, value) {
        // Sets a single value (replaces all values)
        this.params[key] = Array.isArray(value) ? value : [value];
    },
    
    setParamValues(key, values) {
        // Sets multiple values directly
        this.params[key] = Array.isArray(values) ? [...values] : [values];
    },
    
    addParamValue(key, value) {
        // Appends a value without removing existing ones
        if (!this.params[key]) {
            this.params[key] = [];
        }
        this.params[key].push(value);
    },
    
    removeParamValue(key, value) {
        // Removes a specific value
        if (this.params[key]) {
            this.params[key] = this.params[key].filter(v => v !== value);
            if (this.params[key].length === 0) {
                delete this.params[key];
            }
        }
    },
    
    clearParam(key) {
        delete this.params[key];
    },
    
    getParams() {
        // Returns a copy with values as arrays
        const result = {};
        for (const [key, values] of Object.entries(this.params)) {
            result[key] = [...values];
        }
        return result;
    },
};
```

**Consequences:**
- `getParam(key)` still returns a string (first value) for backward compatibility with report code that calls it
- `getParamValues(key)` is the new method for getting all values
- `setParam(key, value)` now accepts either a string or an array — if a string, it wraps it in an array
- `setParamValues(key, values)` explicitly sets an array of values
- `addParamValue` / `removeParamValue` enable incremental multi-select updates
- `getParams()` returns `{key: [...values]}` — report code that iterates params must handle arrays
- `isImmutable(key)` and `isMutable(key)` — no change, these check names not values

#### 7.1.2 `RefreshController.refresh` — Build request body

**Current:**
```javascript
const paramsToSend = { ...mutableCurrentParams, ...newParams };
// mutableCurrentParams is {key: string, ...}
```

**New:**
```javascript
const mutableCurrentParams = {};
for (const [key, values] of Object.entries(currentParams)) {
    if (DataStore.isMutable(key)) {
        mutableCurrentParams[key] = values;  // values is []string
    }
}

const paramsToSend = { ...mutableCurrentParams, ...newParams };
// newParams may contain strings or arrays — normalize:
for (const [key, val] of Object.entries(paramsToSend)) {
    if (!Array.isArray(val)) {
        paramsToSend[key] = [val];
    }
}
```

**Consequences:**
- The JSON body sent to `/refresh` changes from `{"params": {"status": "active"}}` to `{"params": {"status": ["active"]}}`
- The server's `RefreshRequest.UnmarshalJSON` custom unmarshaler handles both formats during transition
- Report code that calls `window.ReportApp.refresh({status: "active"})` still works — the thick client normalizes strings to arrays
- Report code that calls `window.ReportApp.refresh({status: ["active", "pending"]})` now works for multi-value params

#### 7.1.3 `RefreshController` — Parse response params

**Current:**
```javascript
// The next_url from the server is parsed back into params
// by ExtractParams on the next embed load
```

**New:** Same behavior — the thick client does not parse the URL back into params. The server embeds the params in `window.ReportConfig.params` as JSON arrays. The thick client reads from `window.ReportConfig.params` on page load.

**Consequences:**
- `window.ReportConfig.params` is now `{"status": ["active"]}` instead of `{"status": "active"}`
- Report code that reads `window.ReportConfig.params.status` gets an array, not a string
- **Breaking change for report developers:** Any report code that accesses `window.ReportConfig.params` directly (not through the thick client API) must be updated to handle arrays

### 7.2 `window.ReportApp` API surface — Additions

```javascript
// NEW methods on window.ReportApp:

// Get all values for a parameter
ReportApp.getParamValues(key) // returns string[]

// Set multiple values for a mutable parameter
ReportApp.setParamValues(key, values) // values is string[]

// Add a single value to a mutable parameter (appends)
ReportApp.addParamValue(key, value)

// Remove a single value from a mutable parameter
ReportApp.removeParamValue(key, value)

// Check if a parameter supports multi-values
ReportApp.isMultiValue(key) // returns boolean
```

**Consequences:**
- `ReportApp.getParam(key)` — unchanged behavior, returns first value (string)
- `ReportApp.setParam(key, value)` — value can now be string or string[]
- `ReportApp.refresh(newParams)` — values in `newParams` can be string or string[]
- `ReportApp.isMultiValue(key)` — checks `report.MultiValueParams` list
- All new methods are additive — no existing methods change behavior

---

## 8. `main.go` — URL Generation

### 8.1 `generateURL` flag-based URL generator

**Current:**
```go
paramsStr := flag.String("params", "", "Parameters as key=value,key=value")
// ...
for _, pair := range pairs {
    kv := strings.Split(pair, "=")
    params[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
}
```

**New:**
```go
paramsStr := flag.String("params", "", "Parameters as key=value,key=value or key=val1,val2")
// ...
for _, pair := range pairs {
    kv := strings.SplitN(pair, "=", 2)  // SplitN to preserve commas in value
    key := strings.TrimSpace(kv[0])
    valueStr := strings.TrimSpace(kv[1])
    
    // Check if value contains commas (multi-value)
    if strings.Contains(valueStr, ",") {
        rawValues := strings.Split(valueStr, ",")
        values := make([]string, len(rawValues))
        for i, v := range rawValues {
            values[i] = strings.TrimSpace(v)
        }
        params[key] = values
    } else {
        params[key] = []string{valueStr}
    }
}
```

**Consequences:**
- The `-params` flag format changes: `key=val1,val2` produces `params[key] = []string{"val1", "val2"}`
- If a value itself contains a comma, this is ambiguous — report authors should use URL encoding or the thick client for complex values
- The generated URL will have repeated keys: `&key=val1&key=val2`
- The HMAC canonical form uses comma-separated values: `key=val1,val2`
- **Testing:** The `-genurl` flag must be tested with multi-value params to verify the generated URL and signature are correct

### 8.2 `generateNextURL` — URL building

**Current:**
```go
for key, value := range params {
    urlStr += fmt.Sprintf("&%s=%s", url.QueryEscape(key), url.QueryEscape(value))
}
```

**New:** (Same as in refresh handler, repeated here for completeness)
```go
for key, values := range params {
    for _, value := range values {
        urlStr += fmt.Sprintf("&%s=%s", url.QueryEscape(key), url.QueryEscape(value))
    }
}
```

**Consequences:**
- URL keys are iterated in map order (non-deterministic) — the URL order varies between runs
- For HMAC determinism, this does NOT matter — the signature uses the canonical form, not the URL
- The thick client parses the URL on embed load and reconstructs params from repeated keys

---

## 9. Database Layer Changes

### 9.1 `internal/database/queries.go`

#### 9.1.1 `ExecuteDatasource` — Pass `map[string][]string`

**Current:**
```go
func ExecuteDatasource(db *sql.DB, ds core.Datasource, params map[string]string, ...) (*core.QueryResult, error)
```

**New:**
```go
func ExecuteDatasource(db *sql.DB, ds core.Datasource, params map[string][]string, ...) (*core.QueryResult, error)
```

**Consequences:**
- Calls `InjectParams(ds.SQL, params)` — the InjectParams function is updated (see Section 5)
- No changes to `ExecuteQuery` or `ExecuteQueryWithLogger` — they already accept `[]interface{}` for args
- The `args` slice from `InjectParams` grows dynamically with multi-value params
- **Row limit still applies:** The `rowLimit` limits output rows, not query parameters. Multi-value params do not affect row limiting.

#### 9.1.2 `ExecuteDatasourceWithLogger` — Pass `map[string][]string`

Same change as above. The logger receives the expanded SQL with all `?` placeholders replaced by actual values in the log output.

**Consequence for query logging:**
- Logs will show the fully expanded SQL with all `?` placeholders and all args
- For a param with 50 values, the log will show 50 `?` placeholders — this is verbose but correct
- Query log analysis tools must handle variable-length parameter lists

### 9.2 `internal/database/manager.go` — No change needed

The `Manager` struct and its methods (GetClient, ConnectionConfig, CloseAll) do not deal with parameters. Unchanged.

---

## 10. Constraints and Validation

### 10.1 Maximum values per parameter

**Recommendation:** Add a `max_values` constraint per parameter in the YAML:

```yaml
multi_value_params:
  organization_id:
    max: 50
  status:
    max: 20
```

**Consequences:**
- Prevents abuse: a user sending `&status=a&status=b&status=c...` with thousands of values could cause query performance issues or hit MySQL parameter limits
- Validation happens in `ValidateParams` or in the refresh handler before SQL execution
- Default (if `max` is omitted): 50 values (reasonable for most UI multi-select widgets)
- If exceeded: return HTTP 400 with error message

### 10.2 Maximum total parameters per URL

**Current limit:** No explicit limit on total URL length. URLs can grow arbitrarily long with multi-value params.

**Recommendation:** Add a `max_url_length` configuration (default: 8192 bytes, matching common browser/CDN limits):

```go
// In config:
maxURLLength int // default 8192
```

**Consequences:**
- In `ServeHTTP` (embed handler), check `len(r.URL.String()) > maxURLLength` → return HTTP 414 (URI Too Long)
- In `generateNextURL`, check the constructed URL length → error if exceeded
- Report authors should be aware that multi-value params on immutable params expand the HMAC-signed URL

### 10.3 Validation of multi-value param usage

In `ValidateDatasource`, add a check that `{{param:in}}` or `{{param:eq}}` mode is only used on declared multi-value params:

```go
func ValidateDatasource(name string, ds core.Datasource, report *core.Report) error {
    // ... existing validation ...
    
    // Extract mode from placeholders
    re := regexp.MustCompile(`\{\{(\w+)(?::(\w+))?\}\}`)
    for _, match := range re.FindAllStringSubmatch(ds.SQL, -1) {
        mode := match[2]  // may be empty
        if mode != "" {
            paramName := match[1]
            isMulti := report.IsMultiValue(paramName)
            if mode == "eq" && isMulti {
                // Allow eq mode on multi-value param (takes first value)
            } else if mode == "in" && !isMulti {
                return fmt.Errorf("datasource %s: {{%s:in}} used on non-multi-value param '%s'", name, paramName, paramName)
            }
        }
    }
    
    return nil
}
```

**Consequences:**
- `{{param:eq}}` on a non-multi-value param: allowed (explicit override)
- `{{param:in}}` on a non-multi-value param: error (should not produce IN clause)
- `{{param}}` (bare) on a non-multi-value param: allowed (auto-expands to `= ?` for 1 value)
- This validation catches report author errors at load time

---

## 11. Migration and Deployment

### 11.1 Breaking Changes Summary

| Component | Change | Impact |
|-----------|--------|--------|
| HMAC canonical form | `key=val` → `key=val1,val2` | **All existing signed URLs become invalid** |
| `ParamSet` types | `map[string]string` → `map[string][]string` | Compile-time breakage; all callers must update |
| `InjectParams` signature | `map[string]string` → `map[string][]string` | Compile-time breakage |
| `RefreshRequest.Params` | `map[string]string` → `map[string][]string` | JSON format change in API |
| `window.ReportConfig.params` | `{k: "v"}` → `{k: ["v"]}` | Thick client and report code must update |
| Thick client `DataStore` | String values → array values | Thick client API changes |
| URL building | `&key=val` → `&key=val1&key=val2` | URL format change |

### 11.2 Deploy Order

1. **Deploy reporting_app with new code** (new canonical form, new types)
2. **Deploy parent application** (generates URLs with new format)
3. **Deploy thick client JS** (parses arrays)
4. **Update report definitions** (add `multi_value_params` where needed)

**Downtime window:** Between step 1 and step 2, any existing signed URLs from the old parent app will fail HMAC verification. This is expected. Plan for a brief period where old URLs are invalid.

### 11.3 Backward Compatibility Options

#### Option A: No backward compatibility (recommended)
- Deploy all components together or with minimal gap
- Simplest approach, no legacy code paths
- All existing signed URLs become invalid at deploy time

#### Option B: Transitional dual-mode
- In `canonicalParams`, detect old format (no commas in values) and support both
- In `ExtractParams`, detect if the URL has repeated keys or single keys
- In thick client, detect if `window.ReportConfig.params` values are strings or arrays
- **Con:** Adds complexity, testing burden, and potential for bugs. Only justified if you cannot coordinate the deploy.

### 11.4 Testing Checklist

- [ ] Single-value param still works (regression test)
- [ ] Multi-value param with 2 values produces correct SQL `IN (?, ?)`
- [ ] Multi-value param with 10 values produces correct SQL
- [ ] HMAC signature for multi-value param is deterministic (sorted values)
- [ ] HMAC signature rejects tampered multi-value param
- [ ] Refresh with multi-value mutable param works
- [ ] Refresh rejects changed multi-value immutable param
- [ ] Thick client `setParam(key, ["a","b"])` sends correct JSON
- [ ] Thick client `refresh({key: ["a","b"]})` works
- [ ] URL with repeated keys is parsed correctly by the server
- [ ] Report YAML with `multi_value_params` validates correctly
- [ ] Report YAML with undeclared `multi_value_params` name is rejected
- [ ] Query log shows expanded SQL with all placeholders
- [ ] Row limit still applies correctly with multi-value params
- [ ] `-genurl` flag produces correct multi-value URL

### 11.5 Rollback Plan

If issues are found after deploy:
1. Revert parent application to old code (generates old-format URLs)
2. **Do NOT revert reporting_app** — the new canonical form is more robust
3. This means old signed URLs from the reverted parent app will still fail verification on the new reporting_app
4. Full rollback requires reverting both with a coordinated deploy

---

## 12. Example: Complete Multi-Value Report

### 12.1 `reports/multi_select_demo/report.yaml`

```yaml
id: multi_select_demo
name: "Multi-Select Demo"
database: default
visibility: public
expires_after: 3600
max_rows: 10000

immutable_params:
  - organization_id
  - user_id

mutable_params:
  - status
  - region

multi_value_params:
  - organization_id
  - status

# Note: region is single-value only (not in multi_value_params)

datasources:
  orders_by_status:
    sql: |
      SELECT status, COUNT(*) as count, SUM(amount) as total
      FROM orders
      WHERE organization_id = {{organization_id}}
        AND status = {{status}}
      GROUP BY status
    row_limit: 100

  orders_by_region:
    sql: |
      SELECT region, COUNT(*) as count
      FROM orders
      WHERE organization_id = {{organization_id}}
        AND status = {{status}}
      GROUP BY region
    row_limit: 500
```

### 12.2 Initial URL (signed by parent app)

```
/api/embed?
  report_id=multi_select_demo
  &expires=1735689200
  &nonce=abc123
  &sig=xyz
  &organization_id=1
  &organization_id=2
  &user_id=42
  &status=
  &region=
```

**HMAC canonical form:**
```
multi_select_demo:1735689200:abc123:organization_id=1,2:user_id=42
```

**Note:** `status` and `region` are NOT in the HMAC because they are mutable (and empty). Only immutable params (`organization_id`, `user_id`) are signed.

### 12.3 Refresh Request (thick client)

```javascript
// User selects multiple statuses
await window.ReportApp.refresh({
  status: ["active", "pending", "completed"]
});
```

**JSON body sent to `/refresh`:**
```json
{
  "params": {
    "status": ["active", "pending", "completed"]
  }
}
```

**Server-side SQL expansion for `orders_by_status`:**
```
SELECT status, COUNT(*) as count, SUM(amount) as total
FROM orders
WHERE organization_id = ?
  AND status IN (?, ?, ?)
GROUP BY status
```

**Args:** `["1", "active", "pending", "completed"]` (organization_id values are expanded too, but since it's used in `= {{organization_id}}` with 2 values, this is a problem — see Section 12.4)

### 12.4 Problem: Immutable multi-value with `=` operator

The SQL `WHERE organization_id = {{organization_id}}` with `organization_id=[1,2]` produces:
```
WHERE organization_id = IN (?, ?)
```

This is **invalid SQL**. The `=` operator cannot be used with multi-value params.

**Solution:** Report authors must use `{{organization_id:in}}` for immutable params that support multi-values:

```sql
WHERE organization_id IN {{organization_id}}
```

With `organization_id=[1,2]`:
```
WHERE organization_id IN (?, ?)
```

**This means:**
- For immutable params with `multi_value_params`, the SQL MUST use `IN` syntax (either explicit `{{param:in}}` or implicit `{{param}}` where the `IN` is already in the SQL)
- The thick client or parent app should validate this before generating URLs (warn if immutable multi-value param has >1 value but SQL uses `=`)
- This is a **report author responsibility**, not an automatic fix

### 12.5 Corrected SQL for the demo report

```yaml
datasources:
  orders_by_status:
    sql: |
      SELECT status, COUNT(*) as count, SUM(amount) as total
      FROM orders
      WHERE organization_id IN {{organization_id}}
        AND status = {{status}}
      GROUP BY status
    row_limit: 100
```

With `organization_id=[1,2]` and `status=["active","pending"]`:
```
WHERE organization_id IN (?, ?)
  AND status IN (?, ?)
```

Args: `["1", "2", "active", "pending"]`

---

## 13. Summary of All File Changes

| File | Change Type | Description |
|------|-------------|-------------|
| `internal/core/types.go` | Modify | `ParamSet` maps to `[]string`, add `MultiValueParams` field, add `IsMultiValue()` method |
| `internal/core/constants.go` | None | No changes |
| `internal/loader/loader.go` | Modify | Add `MultiValueParams` to raw struct, parse and set on Report |
| `internal/loader/validator.go` | Modify | Rewrite `InjectParams` for multi-value, add multi_value validation |
| `internal/security/hmac.go` | Modify | `SignURL`/`VerifyURL` take `map[string][]string`, rewrite `canonicalParams`, `ExtractParams` returns `map[string][]string` |
| `internal/handler/embed.go` | Modify | All param maps to `[]string`, `generateReportConfig` serializes arrays |
| `internal/handler/refresh.go` | Modify | `RefreshRequest.Params` to `[]string`, `mergeParams` handles arrays, `generateNextURL` emits repeated keys |
| `internal/database/queries.go` | Modify | `ExecuteDatasource` takes `map[string][]string` |
| `internal/database/manager.go` | None | No changes |
| `static/thick_client.js` | Modify | `DataStore` params to arrays, new methods `getParamValues`/`setParamValues`/`addParamValue`/`removeParamValue`, `RefreshController` normalizes to arrays |
| `main.go` | Modify | `generateURL` flag parses comma-separated values, `generateNextURL` emits repeated keys |
| `reports/*/report.yaml` | Report author | Add `multi_value_params` list |

### 14. Open Questions

1. **Delimiter choice:** We use comma `,` to separate values in the HMAC canonical form. Is there any case where a URL-encoded value could contain a comma? `url.QueryEscape` does NOT escape commas (they are valid in query strings). If a parameter value contains a literal comma, the canonical form becomes ambiguous. **Mitigation:** Use a different delimiter that is never valid in URL-encoded values, such as `|` (pipe) or `\x00` (null byte, which url.QueryEscape produces as `%00`).

2. **Order preservation:** Should multi-value params preserve insertion order or always be sorted? We chose sorted for HMAC determinism. But the thick client and report authors may expect a specific order. **Recommendation:** Sort in HMAC canonical form only; preserve order in URL and API.

3. **Default `max_values`:** What is a reasonable default? 50 values produces 50 `?` placeholders. MySQL's `max_allowed_packet` default is 4MB, which can handle thousands of parameters. But query performance degrades with large `IN` clauses. **Recommendation:** Default to 50, with a per-param override in YAML.

4. **Empty multi-value:** What does `status=` (empty value) with multi-value semantics mean? `[]string{""}` (one empty string) or `[]string{}` (zero values)? **Recommendation:** `status=` → `[]string{""}` (one empty string, expands to `= ?` with NULL). `status` absent from URL → `nil` (expands to `1=1`).
