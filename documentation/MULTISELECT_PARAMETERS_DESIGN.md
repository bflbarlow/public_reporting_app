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

### 3.2 SQL Syntax for Parameter Expansion

#### 3.2.1 Basic placeholder expansion

The placeholder `{{param}}` in SQL is replaced with either a single `?` placeholder, multiple `?, ?, ...` placeholders, or `NULL` based on whether the parameter is declared as multi‑value (`multi_value_params`) and the values provided.

**Rules:**

1. **Parameter absent from URL** (key not present): The placeholder expands to `NULL`.
2. **Parameter present with empty value** (`param=`): The placeholder expands to `NULL`.
3. **Parameter present with non‑empty value(s)**:
   - If the parameter is **not** listed in `multi_value_params`:
     - Single value: expands to `?`
     - Multiple values (error): only the first value is used (or validation error)
   - If the parameter **is** listed in `multi_value_params`:
     - Single value: expands to `?`
     - Multiple values: expands to `?, ?, ...` (comma‑separated placeholders)

**Important:** The placeholder expands to the placeholders only, not the comparison operator. The SQL author must write the appropriate SQL around the placeholder:

- For single‑value parameters: `WHERE column = {{param}}`
- For multi‑value parameters: `WHERE column IN ({{param}})`

The placeholder does **not** automatically add `=` or `IN`. The SQL must already contain the correct operator and parentheses.

**Example expansions:**

```sql
-- Original SQL: WHERE status = {{status}}  (status not in multi_value_params)
-- status absent: WHERE status = NULL
-- status present empty: WHERE status = NULL
-- status = "active": WHERE status = ?
-- args: ["active"]

-- Original SQL: WHERE org_id IN ({{org_id}})  (org_id in multi_value_params)
-- org_id absent: WHERE org_id IN (NULL)
-- org_id present empty: WHERE org_id IN (NULL)
-- org_id = ["1"]: WHERE org_id IN (?)
-- args: ["1"]
-- org_id = ["1","2"]: WHERE org_id IN (?, ?)
-- args: ["1", "2"]
```

#### 3.2.2 Default values: `{{param:'default'}}`, `{{param:-10}}`, and `{{param:1,2,3}}`

A placeholder can specify a default value that is used when the parameter is absent from the URL. The syntax is:

- `{{param:'default'}}` for a single string literal (single‑quoted)
- `{{param:-10}}` for a single numeric literal (unquoted)
- `{{param:1,2,3}}` for multiple default values (comma‑separated, no spaces)
- `{{param:'a','b','c'}}` for multiple string defaults (each quoted)

The default is applied when the parameter is absent OR when the parameter is present but empty (`param=`). Empty values are treated as if the parameter were absent, allowing defaults to be used.

**Rules with defaults:**

1. **Parameter absent** (key not present): Use default values (single or multiple). If default is a single value, expands to `?` with that value. If default is multiple values, expands to `?, ?, ...` with those values.
2. **Parameter present with empty value** (`param=`): Use default values (same as absent). An empty value is treated as if the parameter were absent.
3. **Parameter present with non‑empty value(s)**: Use provided values (overrides default). Only actual values (non‑empty strings) cause the default to be ignored.

**Note:** If a parameter is declared in `multi_value_params` and a multi‑value default is provided, the default values are used as a fallback when the parameter is absent. If the parameter absent and default is a single value, the placeholder expands to `?` with that single value (not `IN`). The SQL author must ensure the SQL syntax matches the number of placeholders.

**Examples:**

```sql
-- Single‑value default:
SELECT * FROM orders WHERE discount = {{discount:0}}
-- discount absent: WHERE discount = ?  args: [0]
-- discount present empty: WHERE discount = ?  args: [0]
-- discount = "10": WHERE discount = ?  args: ["10"]

-- Multi‑value default:
SELECT * FROM users WHERE role IN ({{role:'admin','user'}})
-- role absent: WHERE role IN (?, ?)  args: ["admin", "user"]
-- role present empty: WHERE role IN (?, ?)  args: ["admin", "user"]
-- role = ["admin"]: WHERE role IN (?)  args: ["admin"]
-- role = ["admin","manager"]: WHERE role IN (?, ?)  args: ["admin", "manager"]
```

**Parsing notes:**
- Default values are parsed after the colon until the closing `}}`.
- Commas separate multiple default values.
- String literals must be single‑quoted; quotes are stripped.
- Numeric literals are unquoted and passed as strings (the SQL driver will handle conversion).
- Spaces are not allowed within the default value list (except inside quotes).
- To include a literal single quote in a string default, double it: `{{param:'O''Reilly'}}`.

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
                buf.WriteString("|")
            }
            buf.WriteString(url.QueryEscape(v))
        }
    }
    return buf.String()
}
```

**Consequences:**
- Canonical form changes from `key=value&key2=value2` to `key=val1|val2&key2=val3`
- Values within each key are always sorted alphabetically for consistency and determinism (so `org=2,1` becomes `org=1,2`)
- Values are pipe‑separated (`|`) within a key — `url.QueryEscape` encodes pipe as `%7C`, avoiding ambiguity with parameter values
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

This is the **most critical** change. The current `InjectParams` function replaces each `{{param}}` with a single `?` and appends one arg. It must be rewritten to handle multi‑value expansion and default values.

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
    // Regex captures parameter name and optional default clause.
    // Default clause can be a single quoted string, a numeric literal, or comma‑separated values.
    re := regexp.MustCompile(`\{\{(\w+)(?::([^}]+))?\}\}`)
    
    var result strings.Builder
    result.Grow(len(sql))
    var args []interface{}
    
    lastIndex := 0
    for _, match := range re.FindAllStringSubmatchIndex(sql, -1) {
        result.WriteString(sql[lastIndex:match[0]])
        
        paramName := sql[match[2]:match[3]]
        defaultClause := ""
        if match[4] != -1 {
            defaultClause = sql[match[4]:match[5]]
        }
        
        // Determine values to use
        var values []string
        var fromDefaults bool
        if vals, ok := params[paramName]; ok && vals != nil {
            // Parameter present in URL
            // Check if all values are empty strings (i.e., `param=`)
            allEmpty := true
            for _, v := range vals {
                if v != "" {
                    allEmpty = false
                    break
                }
            }
            if allEmpty && defaultClause != "" {
                // Empty parameter (`param=`) → treat as absent, use default
                values = parseDefaultClause(defaultClause)
                fromDefaults = true
            } else {
                // Non‑empty values present
                values = vals
            }
        } else if defaultClause != "" {
            // Parameter absent, use default
            values = parseDefaultClause(defaultClause)
            fromDefaults = true
        }
        // If values is nil → will expand to NULL
        
        // Determine if parameter supports multiple values
        isMulti := report.IsMultiValue(paramName)
        
        // Expand placeholder
        if len(values) == 0 {
            // Missing or empty → NULL
            result.WriteString("NULL")
            // No arg added
        } else if len(values) == 1 {
            // Single value
            result.WriteString("?")
            // If the value came from defaults or is non‑empty, use it
            // Empty values from defaults shouldn't happen (parseDefaultClause returns nil for empty clause)
            if values[0] == "" && !fromDefaults {
                // Empty value from URL (should have been caught above as allEmpty)
                args = append(args, nil)
            } else {
                args = append(args, values[0])
            }
        } else {
            // Multiple values
            if !isMulti {
                // Parameter not declared multi‑value: take first value only
                // (Could also raise an error at validation time)
                result.WriteString("?")
                if values[0] == "" && !fromDefaults {
                    args = append(args, nil)
                } else {
                    args = append(args, values[0])
                }
            } else {
                // Multi‑value: expand to comma‑separated placeholders
                placeholders := make([]string, len(values))
                for i, v := range values {
                    placeholders[i] = "?"
                    if v == "" && !fromDefaults {
                        // Empty value from URL (should have been caught above)
                        args = append(args, nil)
                    } else {
                        args = append(args, v)
                    }
                }
                result.WriteString(strings.Join(placeholders, ","))
            }
        }
        
        lastIndex = match[1]
    }
    
    result.WriteString(sql[lastIndex:])
    return result.String(), args
}

// parseDefaultClause parses a default clause like "'default'", "-10", "1,2,3", or "'a','b','c'".
// It returns a slice of strings (already stripped of quotes).
// Note: Default values in the SQL placeholder use commas as separators (e.g., `{{param:1,2,3}}`),
// but the HMAC canonical form uses pipes for value separation.
func parseDefaultClause(clause string) []string {
    clause = strings.TrimSpace(clause)
    if clause == "" {
        return nil
    }
    
    // Split by commas, respecting quoted strings
    // Simple implementation: split on commas, trim spaces, strip surrounding single quotes
    var values []string
    // This simple parser assumes no embedded commas or quotes inside values.
    // For production, use a proper CSV parser or a more robust tokenizer.
    parts := strings.Split(clause, ",")
    for _, part := range parts {
        part = strings.TrimSpace(part)
        // Remove surrounding single quotes if present
        if len(part) >= 2 && part[0] == '\'' && part[len(part)-1] == '\'' {
            part = part[1 : len(part)-1]
            // Unescape doubled single quotes
            part = strings.ReplaceAll(part, "''", "'")
        }
        values = append(values, part)
    }
    return values
}
```

**Consequences:**
- Function signature changes: `InjectParams(sql string, params map[string][]string, report *core.Report)` — the `report` parameter is needed for `IsMultiValue` checks.
- Regex changes from `\{\{(\w+)\}\}` to `\{\{(\w+)(?::([^}]+))?\}\}` to capture optional default clause after a colon.
- No modes (`in`, `eq`). The placeholder expansion is determined solely by the `multi_value_params` declaration and the number of values.
- When a parameter is absent from the URL and no default is specified, the placeholder expands to `NULL` (literal SQL NULL, no argument).
- When a parameter is present but empty (`param=`), the placeholder uses default values if specified; otherwise expands to `NULL` (literal SQL NULL, no argument).
- When a parameter has a single non‑empty value, the placeholder expands to `?` with that value.
- When a parameter is declared in `multi_value_params` and has multiple values, the placeholder expands to `?, ?, ...` (comma‑separated placeholders).
- Default values can be single (quoted string or numeric literal) or multiple (comma‑separated). They are used when the parameter is absent OR when the parameter is present but empty (`param=`). Only non‑empty values override defaults.
- Empty string values within a multi‑value set are converted to `nil` arguments (SQL NULL).
- The `args` slice grows dynamically: one entry per value per placeholder.
- **Important SQL authoring rule:** The placeholder expands to placeholders (or `NULL`), not to an `=` or `IN` clause. The SQL must already contain the appropriate operator and parentheses. For example:
  - Single‑value: `WHERE column = {{param}}`
  - Multi‑value: `WHERE column IN ({{param}})`
- **Performance:** For a parameter with 100 values, this produces 100 `?` placeholders. MySQL has a `max_allowed_packet` limit and a query parameter limit (~65535). For practical multi‑select filters, 10‑50 values is typical and safe.

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
- The HMAC canonical form uses pipe‑separated values: `key=val1|val2`
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

**Implementation:** Use a global configuration variable (e.g., `MAX_VALUES_PER_PARAM`) set via environment variables with a default of 50. Allow per‑parameter override in the report YAML via `max` field.

**Configuration example:**

```go
// In config struct
MaxValuesPerParam int `env:"MAX_VALUES_PER_PARAM" default:"50"`
```

**YAML syntax:**

```yaml
multi_value_params:
  organization_id:
    max: 50    # uses global default or explicit limit
  status:
    max: 20    # custom limit
  region:      # no max specified, uses global default
```

**Consequences:**
- Prevents abuse: a user sending `&status=a&status=b&status=c...` with thousands of values could cause query performance issues or hit MySQL parameter limits
- Validation happens in `ValidateParams` or in the refresh handler before SQL execution
- Global default provides consistency across reports; per‑parameter override allows flexibility
- If exceeded: return HTTP 400 with error message
- The limit applies only to parameters declared in `multi_value_params`

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

### 10.3 Validation of default values

Since modes (`in`, `eq`) have been removed, the only extension to the placeholder syntax is the optional default clause (`{{param:default}}`). Validation of default values is not strictly necessary because the `parseDefaultClause` function handles parsing errors gracefully (returns empty slice). However, we may want to add a warning if a default clause contains syntax errors (unmatched quotes, etc.) to help report authors catch mistakes early.

Alternatively, validation can be omitted, and any parsing errors will result in the default being ignored (treated as absent). This is safe but may cause confusion.

We'll keep the validation minimal.

---

## 11. Migration and Deployment

### 11.1 Breaking Changes Summary

| Component | Change | Impact |
|-----------|--------|--------|
| HMAC canonical form | `key=val` → `key=val1,val2` | **All existing signed URLs become invalid** |
| `ParamSet` types | `map[string]string` → `map[string][]string` | Compile-time breakage; all callers must update |
| `InjectParams` signature | `map[string]string` → `map[string][]string` plus default clause support | Compile-time breakage; new default value syntax |
| Placeholder syntax | Removal of `:in`/`:eq` modes; addition of default values `{{param:default}}` | Report authors must write `IN` for multi‑value params; defaults provide fallback |
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
---
### 12.4 Problem: Immutable multi-value with `=` operator

The SQL `WHERE organization_id = {{organization_id}}` with `organization_id=[1,2]` produces:
```
WHERE organization_id = IN (?, ?)
```

This is **invalid SQL**. The `=` operator cannot be used with multi-value params.

**Solution:** Report authors must write the SQL with explicit `IN` clause for parameters declared in `multi_value_params`:

```sql
WHERE organization_id IN ({{organization_id}})
```

With `organization_id=[1,2]`:
```
WHERE organization_id IN (?, ?)
```

**This means:**
- For parameters declared in `multi_value_params`, the SQL MUST use `IN ({{param}})` syntax with parentheses
- The placeholder expands to placeholders only, not the `IN` keyword
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
| `main.go` | Modify | `generateURL` flag parses comma‑separated values (in default clauses) and pipe‑separated values (in canonical form), `generateNextURL` emits repeated keys |
| `reports/*/report.yaml` | Report author | Add `multi_value_params` list |

### 14. Open Questions

1. **Delimiter choice:** Use pipe `|` to separate values in the HMAC canonical form. The pipe character is not a reserved character in URL encoding (`url.QueryEscape` encodes it as `%7C`), which avoids ambiguity with parameter values that might contain commas. **Decision:** Use pipe `|` delimiter.

2. **Order preservation:** Values within each key should always be sorted alphabetically for consistency and determinism in HMAC signatures. The thick client and API should preserve the order as provided, but canonicalization sorts them. **Decision:** Always sort values in canonical form.

3. **Default `max_values`:** Use a global configuration variable (e.g., from `.env`) with a default of 50 values per parameter. Allow per‑parameter override in the report YAML via `max` field. **Decision:** Global default with per‑parameter override.

4. **Empty multi-value:** Empty parameters (`param=`) should use default values if specified, otherwise expand to `NULL`. Missing parameters (key not present) should also use defaults if specified, otherwise expand to `NULL`. Only non‑empty values override defaults. **Decision:** Empty/missing → use default if specified, otherwise `NULL`.
