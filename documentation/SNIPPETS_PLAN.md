# SQL Snippets: Plan

**Version:** 1.1  
**Date:** 2026-05-15  
**Status:** Implemented — Active Feature

---

## 1. Overview

SQL snippets are reusable blocks of SQL code stored as individual YAML files in a `snippets/` directory. Reports reference snippets inline in their SQL using `{{snippet:name}}` syntax. At query time, the server resolves these references by reading the corresponding snippet files and performing wholesale text replacement.

**Goal:** One place to change a piece of SQL that can impact multiple reports, without requiring versioning, parameter injection, or complex tooling.

**Status:** ✅ **Implemented** (2026-05-15)

**Scope:** This is a read-only, file-based feature. No database tables, no API endpoints, no UI. Anyone with repository access can create or edit snippets.

---

## 2. Design Decisions

### 2.1 Syntax: `{{snippet:name}}`

Snippet references use the same `{{...}}` delimiter as existing parameter placeholders, with a `snippet:` prefix to distinguish them:

```sql
SELECT * FROM ({{snippet:base_sales_query}}) AS src
WHERE {{snippet:default_date_filter}}
```

### 2.2 Storage: YAML Files in `snippets/` Directory

Each snippet is a single YAML file:

```yaml
name: base_sales_query
description: "Base query for all sales reports"
sql: |
  SELECT s.*, c.region
  FROM sales s
  JOIN customers c ON s.id = c.customer_id
  WHERE s.status = 'active'
```

- **`name`** — Required. Must match the filename (without `.yaml` extension).
- **`description`** — Optional. Human-readable metadata. Never used in logic.
- **`sql`** — Required. Multi-line SQL via YAML literal block scalar (`|`).

### 2.3 Resolution: Query Time, Wholesale Text Replacement

Snippet resolution happens at query time, not startup. The server:

1. Reads all referenced snippet YAML files from disk
2. Performs wholesale text replacement of `{{snippet:name}}` → snippet SQL
3. Then performs parameter substitution (`{{param_name}}`) on the expanded SQL
4. Executes the final SQL

This guarantees the latest snippet content is always used. No caching, no invalidation logic.

### 2.4 Scope: Global

All snippets are globally accessible. Any report can reference any snippet. No scoping, no namespaces.

### 2.5 Name Restrictions

Snippet names must match the regex `[a-zA-Z0-9_-]+`. This prevents:

- **Colon (`:`)** — ambiguous delimiter conflict
- **Forward slash (`/`)** — would resolve to subdirectory paths, fragile
- **Backslash (`\`)** — path separator on Windows
- **Null byte (`\0`)** — truncates strings in Go
- **Empty name** — would produce invalid filename

Names are validated at load time. Invalid names are silently skipped with a warning logged.

### 2.6 No Parameter Validation

Snippets may contain `{{param_name}}` references. No validation is performed — they are treated as plain text during snippet resolution and expanded later during parameter substitution. Typos in parameter names within snippets will surface as runtime query errors.

### 2.7 No Nesting

A snippet's SQL must not contain `{{snippet:...}}` references. Nested snippets are not supported and will not be expanded. If a snippet contains `{{snippet:...}}`, it is left as-is in the expanded SQL (which will likely cause a SQL syntax error, acting as implicit validation).

### 2.8 No Versioning

Snippets are always resolved from their latest file content. No versioning, no pinning. Breaking changes propagate immediately to all consuming reports.

### 2.9 Snippet Types and Valid Usage Positions

**Critical:** Snippets are wholesale text fragments — not self-contained SQL. The user must ensure the expanded SQL is valid. Snippets fall into categories based on their content:

| Type | Content | Valid Positions | Example |
|------|---------|-----------------|---------|
| **Full SELECT** | Complete `SELECT ... FROM ...` | `FROM ({{snippet:name}}) AS src` — must be wrapped as subquery | `base_sales_query` |
| **WHERE fragment** | `col = 'val' AND ...` | `WHERE {{snippet:name}}` or `AND {{snippet:name}}` | `default_date_filter` |
| **JOIN fragment** | `JOIN table ON ...` or `LEFT JOIN ...` | `FROM users {{snippet:name}}` | `customer_join` |
| **SELECT fragment** | Column list or expressions | `SELECT {{snippet:name}}` | `user_columns` |
| **GROUP BY fragment** | `GROUP BY col1, col2` | `... GROUP BY {{snippet:name}}` | `date_grouping` |
| **ORDER BY fragment** | `ORDER BY col DESC` | `... ORDER BY {{snippet:name}}` | `date_sort` |

**Invalid usage examples (will produce SQL errors):**

```sql
-- ❌ Full SELECT snippet in FROM without subquery wrapper
SELECT * FROM {{snippet:base_sales_query}}
-- Expands to: SELECT * FROM SELECT s.*, c.region FROM sales s ... (INVALID)

-- ❌ WHERE fragment in FROM position
SELECT * FROM {{snippet:default_date_filter}}
-- Expands to: SELECT * FROM s.date >= '...' AND ... (INVALID)

-- ❌ WHERE fragment in SELECT position
SELECT {{snippet:default_date_filter}}
-- Expands to: SELECT s.date >= '...' AND ... (INVALID)
```

**Recommended patterns:**

```yaml
# ✅ Full SELECT — wrap as subquery
sql: |
  SELECT * FROM ({{snippet:base_sales_query}}) AS src
  WHERE {{snippet:default_date_filter}}

# ✅ WHERE fragment — use in WHERE/AND position
sql: |
  FROM users u
  WHERE {{snippet:default_date_filter}}
  AND {{snippet:user_status_filter}}

# ✅ JOIN fragment — use in FROM/JOIN position
sql: |
  FROM users u
  {{snippet:customer_join}}
  WHERE {{snippet:default_date_filter}}
```

### 2.10 Error Handling

- **Missing snippet file** — Query fails with a clear error: `snippet "name" not found in snippets directory`
- **Invalid YAML** — Query fails with a clear error: `failed to parse snippet "name": <yaml error>`
- **Invalid name format** — Silently skipped with a warning logged at startup
- **Malformed snippet YAML** (missing `name` or `sql`) — Rejected with error: `snippet "name" is missing required field: <field>`
- **Invalid SQL after expansion** — Query fails with database error; no validation of expanded SQL is performed

---

## 3. Technical Architecture

### 3.1 Directory Structure

```
reporting_app/
├── snippets/                    # New directory (env-configurable, default: ./snippets)
│   ├── base_sales_query.yaml
│   ├── default_date_filter.yaml
│   └── ...
├── internal/
│   ├── loader/
│   │   ├── snippet.go           # New file: snippet type + loader
│   │   └── ...                  # Existing files (unchanged)
│   └── database/
│       └── ...                  # Existing files (minor integration)
├── main.go                      # Updated: add SNIPPETS_DIR env var
├── .env                         # Updated: add SNIPPETS_DIR
└── .env.example                 # Updated: add SNIPPETS_DIR
```

### 3.2 New File: `internal/loader/snippet.go`

This file is **self-contained** and does not modify any existing files. It defines:

#### Types

```go
type Snippet struct {
    Name        string `yaml:"name"`
    Description string `yaml:"description"`
    SQL         string `yaml:"sql"`
}
```

#### Functions

```go
// LoadSnippet loads and validates a single snippet file.
func LoadSnippet(path string) (*Snippet, error)

// LoadSnippets loads all snippets from the given directory.
func LoadSnippets(dir string) (map[string]*Snippet, error)

// ExpandSnippets replaces {{snippet:name}} with snippet SQL in the given SQL string.
func ExpandSnippets(snippets map[string]*Snippet, sql string) (string, error)
```

#### Validation Rules (in `LoadSnippet`)

1. File must exist and be readable
2. YAML must parse successfully
3. `name` field must be present and non-empty
4. `name` must match `[a-zA-Z0-9_-]+`
5. `sql` field must be present and non-empty
6. `name` must match the filename (without `.yaml`)

### 3.3 Integration Points

#### A. `main.go` — Configuration

Add `SNIPPETS_DIR` environment variable to the configuration loading. Default: `./snippets`.

```go
snippetsDir := getEnv("SNIPPETS_DIR", "./snippets")
```

**Risk:** Minimal. This is a new env var with a safe default. No existing code is modified.

#### B. `main.go` — Startup (Optional)

At startup, attempt to load snippets from the configured directory. If the directory doesn't exist, log a debug message and continue (snippets are optional). If snippets exist but fail to load, log a warning.

```go
if snippetsDir, err := os.Stat(snippetsDir); err == nil && snippetsDir.IsDir() {
    snippets, err := loader.LoadSnippets(snippetsDir)
    if err != nil {
        log.Printf("⚠️  Failed to load snippets from %s: %v (snippets are optional)", snippetsDir, err)
    } else {
        log.Printf("📦 Loaded %d snippets from %s", len(snippets), snippetsDir)
    }
} else {
    log.Printf("ℹ️  No snippets directory found at %s (snippets are optional)", snippetsDir)
}
```

**Risk:** Minimal. Snippets are optional — the server starts successfully even if no snippets directory exists.

#### C. Datasource Query Execution — SQL Expansion

In the datasource resolver (where SQL is prepared/executed), expand snippets before parameter substitution:

```go
// Before: sql = "SELECT * FROM {{dataset_name}} WHERE id = {{id}}"
expanded, err := loader.ExpandSnippets(snippets, sql)
// After: sql = "SELECT * FROM base_sales_query WHERE id = {{id}}"
// Then parameter substitution proceeds as normal
```

**Integration location:** The exact file depends on where the datasource resolver lives (likely `internal/database/` or `internal/loader/`). This is the **only** place that needs to call `ExpandSnippets`.

**Risk:** 
- The expansion happens **before** parameter substitution, so `{{param_name}}` references in snippets are preserved for later expansion.
- The original SQL string is never mutated — a new string is returned.
- If expansion fails, the query fails early with a clear error.

### 3.4 `ExpandSnippets` Implementation Details

The expansion function must:

1. **Scan** the SQL string for `{{snippet:name}}` patterns using a regex
2. **For each match:**
   - Extract the name
   - Look up the snippet in the map
   - If found, replace the match with the snippet's `sql` content
   - If not found, return an error
3. **Return** the expanded SQL string

**Regex pattern:** `{{snippet:([a-zA-Z0-9_-]+)}}`

This regex is **greedy by default** but the character class `[a-zA-Z0-9_-]` prevents ambiguity. No snippet name can contain `}` or `:`, so the pattern is unambiguous.

**Edge cases handled:**

| Case | Behavior |
|------|----------|
| `{{snippet:missing}}` | Error: snippet not found |
| `{{snippet:}}` | No match (regex won't match empty name) |
| `{{snippet:bad name}}` | No match (space not in character class) |
| `{{snippet:name}} {{snippet:name2}}` | Both expanded |
| `{{snippet:name}} in middle of text` | Expanded in place |
| Duplicate references to same snippet | Each expanded independently (file read once per reference) |

---

## 4. Implementation Plan

### Phase 1: Core Implementation (Low Risk)

#### Step 1: Create `internal/loader/snippet.go`

- Define `Snippet` struct
- Implement `LoadSnippet(path string) (*Snippet, error)`
- Implement `LoadSnippets(dir string) (map[string]*Snippet, error)`
- Implement `ExpandSnippets(snippets map[string]*Snippet, sql string) (string, error)`
- Add unit tests for each function

**Files changed:** `internal/loader/snippet.go` (new)

**Risk:** None. New file, no existing code modified.

#### Step 2: Add `SNIPPETS_DIR` to Configuration

- Add `SNIPPETS_DIR` env var parsing in `main.go`
- Add to `.env.example` with default `./snippets`
- Add to `.env` with default `./snippets`

**Files changed:** `main.go`, `.env.example`, `.env`

**Risk:** Minimal. New env var, safe default. No existing logic modified.

#### Step 3: Integrate Snippet Loading into Startup

- Add startup snippet loading in `main.go`
- Log snippet count or absence
- Handle missing directory gracefully (no error)

**Files changed:** `main.go`

**Risk:** Minimal. Optional loading, no impact on startup if directory missing.

#### Step 4: Integrate SQL Expansion into Datasource Resolver

- Identify the datasource resolver location (where SQL is prepared/executed)
- Add call to `ExpandSnippets` before parameter substitution
- Handle expansion errors (fail query with clear message)

**Files changed:** `internal/database/` or `internal/loader/` (existing file, one function call added)

**Risk:** Low. One function call added in one location. Expansion happens before parameter substitution, so existing parameter logic is untouched. If expansion fails, the query fails early.

### Phase 2: Validation & Testing (Medium Risk)

#### Step 5: Add `--reload-snippets` CLI Flag

- Add `--reload-snippets` flag to `main.go`
- When flag is provided, reload all snippets from disk and log the count
- If snippets directory doesn't exist, log a message and exit
- Print snippet names and descriptions after reload
- Exit after printing (no server startup)

**Files changed:** `main.go`

**Risk:** Low. New flag, no server impact when not used.

#### Step 6: Add Snippet Validation at Startup

- At startup, validate all snippet files in the directory
- Log warnings for invalid snippets (bad YAML, missing fields, invalid names)
- Continue startup even if some snippets are invalid

**Files changed:** `main.go`, `internal/loader/snippet.go`

**Risk:** Low. Warnings, not errors. Startup succeeds regardless.

#### Step 7: Add Unit Tests

- Test `LoadSnippet` with valid/invalid YAML
- Test `LoadSnippets` with mixed valid/invalid files
- Test `ExpandSnippets` with various patterns
- Test edge cases (missing snippets, duplicate references, special characters)

**Files changed:** `internal/loader/snippet_test.go` (new)

**Risk:** None. New test file.

### Phase 3: Documentation (Low Risk)

#### Step 8: Update Documentation

- Create `SNIPPETS_PLAN.md` (this document)
- Update `README.md` with snippets section
- Add example snippets to repository

**Files changed:** `README.md`, `documentation/` (new docs), `snippets/` (example files)

**Risk:** None. Documentation only.

---

## 5. Risk Analysis

### 5.1 Low-Risk Items

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| **Breaking existing code** | Very Low | Low | Snippet loading is optional; expansion is a single function call added in one location |
| **File read errors** | Low | Low | Graceful error handling; missing directory = no snippets, not a failure |
| **YAML parse errors** | Low | Low | Invalid YAML = snippet skipped with warning |
| **Performance (file I/O)** | Low | Low | Unlikely to be a bottleneck; snippets are small files read infrequently |
| **SQL injection via snippets** | Medium | High | Snippets are trusted input (same as report.yaml SQL); no new attack surface |

### 5.2 Medium-Risk Items

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| **Parameter name collision** | Low | Medium | Snippets may contain `{{param_name}}` that conflicts with report params; document this as a known limitation |
| **Malicious snippet content** | Low | High | Snippets are trusted input (same as report.yaml SQL); documented as a trust boundary |
| **Very large snippet SQL** | Low | Low | No size limit; could impact query performance if snippet is extremely large |
| **Duplicate snippet names** | Low | Low | File-based; last file loaded wins (alphabetical order); documented as a known limitation |

### 5.3 What This Feature Does NOT Change

- **HMAC signatures** — unchanged
- **Nonce tracking** — unchanged
- **Parameter validation** — unchanged (parameter names are still validated as before)
- **Immutable/mutable parameter classification** — unchanged
- **Report YAML parsing** — unchanged (snippet references are just text in the SQL field)
- **Database connections** — unchanged
- **CSP headers** — unchanged
- **Thick client architecture** — unchanged
- **Refresh endpoint** — unchanged

---

## 6. File I/O Behavior

### 6.1 Read Frequency

Snippet files are read **once per query execution**, not cached. For a report with 5 datasources, each referencing 2 snippets:

- **10 file reads per request**
- **~20 file reads per page load** (embed + refresh)

For typical reporting workloads (hundreds of requests per hour), this is negligible. If the app handles thousands of requests per minute, consider adding caching later.

### 6.2 File Read Behavior

- `LoadSnippets` reads all `.yaml` files in the directory
- Each file is opened, parsed, and closed
- No file descriptors are held open
- No directory watching

### 6.3 Error Behavior on Read Failure

| Error | Behavior |
|-------|----------|
| Directory doesn't exist | Log debug message; continue (snippets optional) |
| File not readable | Log warning; skip snippet; continue |
| YAML parse error | Log warning; skip snippet; continue |
| Missing required field | Log warning; skip snippet; continue |
| Invalid name format | Log warning; skip snippet; continue |

---

## 7. Example Usage

### 7.1 Snippet File: `snippets/base_sales_query.yaml`

```yaml
name: base_sales_query
description: "Base query for all sales reports"
sql: |
  SELECT s.*, c.region
  FROM sales s
  JOIN customers c ON s.id = c.customer_id
  WHERE s.status = 'active'
```

### 7.2 Snippet File: `snippets/default_date_filter.yaml`

```yaml
name: default_date_filter
description: "Default date filter using start_date and end_date parameters"
sql: |
  s.date >= '{{start_date}}' AND s.date <= '{{end_date}}'
```

### 7.3 Report YAML: `reports/sales_report/report.yaml`

```yaml
id: sales_report
name: "Sales Report"

datasources:
  sales_data:
    database: default
    sql: |
      SELECT * FROM ({{snippet:base_sales_query}}) AS src
      WHERE {{snippet:default_date_filter}}
      GROUP BY region
```

### 7.4 Resolved SQL (at query time)

1. **Snippet expansion:**
   ```sql
   SELECT * FROM ({{snippet:base_sales_query}}) AS src
   WHERE {{snippet:default_date_filter}}
   GROUP BY region
   ```
   
   After expanding `base_sales_query`:
   ```sql
   SELECT * FROM (
     SELECT s.*, c.region
     FROM sales s
     JOIN customers c ON s.id = c.customer_id
     WHERE s.status = 'active'
   ) AS src
   WHERE {{snippet:default_date_filter}}
   GROUP BY region
   ```
   
   After expanding `default_date_filter`:
   ```sql
   SELECT * FROM (
     SELECT s.*, c.region
     FROM sales s
     JOIN customers c ON s.id = c.customer_id
     WHERE s.status = 'active'
   ) AS src
   WHERE s.date >= '{{start_date}}' AND s.date <= '{{end_date}}'
   GROUP BY region
   ```

2. **Parameter substitution** (after `start_date=2024-01-01`, `end_date=2024-12-31`):
   ```sql
   SELECT * FROM (
     SELECT s.*, c.region
     FROM sales s
     JOIN customers c ON s.id = c.customer_id
     WHERE s.status = 'active'
   ) AS src
   WHERE s.date >= '2024-01-01' AND s.date <= '2024-12-31'
   GROUP BY region
   ```

### 7.5 Impact of Changing a Snippet

If `base_sales_query.yaml` is updated:

```yaml
sql: |
  SELECT s.*, c.region
  FROM sales s
  JOIN customers c ON s.id = c.customer_id
  WHERE s.status = 'active'
  AND s.region IS NOT NULL
```

**All reports using `{{snippet:base_sales_query}}`** will immediately use the updated SQL on their next query. No migration, no versioning, no coordination.

---

## 8. Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SNIPPETS_DIR` | `./snippets` | Directory containing snippet YAML files |

---

## 9. Decisions

1. **`--reload-snippets` CLI flag** — ✅ Yes. Reloads snippets from disk without restarting the server.
2. **Snippet categories/tags** — ❌ No. Flat list in `snippets/` directory.
3. **`--list-snippets` CLI flag** — ❌ No.
4. **Whitespace trimming** — ❌ No. YAML `|` preserves internal newlines; the final newline is stripped by YAML (standard behavior).
5. **`--validate-snippets` CLI flag** — ❌ No.

---

## 10. Rollback Plan

If the feature causes issues:

1. **Disable snippets:** Set `SNIPPETS_DIR` to a non-existent directory (e.g., `/dev/null`)
2. **Remove snippet references:** Update report YAML files to use inline SQL instead of `{{snippet:name}}`
3. **Remove the feature:** Delete `internal/loader/snippet.go` and revert `main.go` changes

The feature is **opt-in** by design — no snippets directory = no impact on existing reports.
