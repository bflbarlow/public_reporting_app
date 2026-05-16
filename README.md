# Reporting App

A secure, embeddable reporting platform serving html+javascript reports with HMAC-signed URLs and thick client architecture.

## Core Requirements

These details are to *ALWAYS* be maintained with every code change.

1. **Embeddability** - The sole purpose of this application is to produce embeddable reports into a parent application.
2. **Security** - This application must secure access to data for only those that the application intends. This includes HMAC Signatures (including immutable parameters), NONCE values, and expired URLs.
3. **Thick Client** - Only one mechanism handles the movement of data on the client side. The thick client's sole purpose is to coordinate requests from the client to the reporting app to refresh data and provide that data back to the client.

## The Four-Component Architecture

| Component | Location | Responsibilities | Key Technologies |
|-----------|----------|------------------|------------------|
| **Parent Application** | Your web app (SaaS, portal) | • User authentication & permissions<br>• Generate signed URLs<br>• Embed iframes with parameters<br>• Control report lifecycle | Any web framework |
| **Reporting App** | Go server binary | • Load reports & execute SQL<br>• Validate HMAC signatures<br>• Render HTML templates<br>• Manage database connections | Go, MySQL, HTML templates |
| **Thick Client** | Browser JavaScript | • Client-side filtering & charting<br>• AJAX data refresh<br>• UI interactions & cross-filtering<br>• Data management | Vanilla JS, Chart.js (optional) |
| **Report** | Report developer's code | • Custom visualization rendering<br>• Complex interactivity and coordination<br>• Integration with specialized charting libraries | React, D3, Plotly, etc. |

## Definitions

| Term | Definition |
|------|------------|
| **Client** | The web browser that loads both the Parent Application and the Reporting App via iframes. Executes client-side JavaScript and manages user interaction with embedded reports. |
| **Report** | A named collection of charts that display data from one or more databases. Defined by a `report.yaml` file and a `report.html` template. Each datasource can specify its own database via `datasources.{name}.database`, falling back to `report.database`. |
| **Page** | The HTML document served by the Reporting App that contains a fully hydrated report. The page includes inline data, JavaScript for interactivity, and strict security headers. |
| **Chart** | A single visual element within a report (e.g., line chart, bar chart, table). Each chart has a unique ID, a title, a SQL query, and rendering configuration. |
| **Thick Client** | The JavaScript code (`/static/thick_client.js`) that runs inside the iframe and handles all client-side interactivity-filtering, charting, drill-down, and AJAX refresh. |
| **Parent Application** | Your main web application (SaaS, portal, internal tool) that embeds reports via iframes. It generates HMAC-signed URLs and decides which users can access which reports. |
| **Reporting App** | The Go server binary that validates HMAC signatures, executes SQL queries, and renders hydrated report pages. It serves as the secure backend for embedded reporting. |
| **Iframe** | An HTML `<iframe>` element that isolates the report from the parent application. The iframe has strict sandbox attributes and cannot communicate with the parent page. |
| **Embed** | The act of inserting a report into a web page using an iframe with an HMAC-signed URL. |
| **HMAC-signed URL** | A cryptographically signed web address that grants temporary, single-use access to a specific report. Contains a nonce, expiry timestamp, and HMAC-SHA256 signature. |
| **Nonce** | A random, single-use token that prevents replay attacks. Each signed URL includes a unique nonce that can only be used once. |
| **CSP** | Content Security Policy-a set of HTTP headers that restrict which resources (scripts, styles, etc.) the browser can load, preventing XSS and other attacks. |
| **CORS** | Cross-Origin Resource Sharing-headers that control which external domains are allowed to embed the Reporting App's pages via iframes. |
| **Database Connection** | A read-only connection to a SQL database, defined in `databases.yaml`. Reports can use a single database (via `report.database`) or multiple databases (via `datasources.{name}.database`). |
| **Parameterized Query** | A SQL query that contains placeholders like `{{start_date}}` which are safely replaced with values from the URL query string, preventing SQL injection. |
| **Refresh Endpoint** | The `/refresh` API endpoint that accepts a signed URL and returns fresh data plus a new signed URL, enabling AJAX-based updates without page reloads. |
| **Chart Data** | The JSON-encoded query results (columns and rows) for a chart, embedded inline in the page as `window.__chart_data__`. |
| **report.html** | The report held in an html file defining the look (css), structure (html), and interactivity (JavaScript). Location: `reports/{report}/report.html`. |
| **report.yaml** | A YAML file defining datasource-based reports. Contains datasource definitions, parameter schemas, and configuration for JavaScript-first development. Location: `reports/{report}/report.yaml`. |
| **Datasource** | A named SQL query in a manifest that provides data to JavaScript visualizations. Has parameter substitution, caching, and execution limits. |
| **Client API** | The `window.__reportData` object injected into datasource-based reports, providing structured access to datasources via `getRows()` and `getColumns()` methods. These methods use the thick client (`window.ReportApp.refreshDatasource()`) as the sole data bridge to the reporting app. Direct API endpoints are disabled. |
| **Parameter Classification** | The system of categorizing parameters as immutable (security boundaries, HMAC-signed) or mutable (user filters, not signed). Applies to both chart-based and datasource-based reports. |

## Quick Start

```bash
# Clone and setup
cp .env.example .env
# Edit .env with your settings (especially HMAC_SECRET)

# Install dependencies
go mod tidy

# Run the server
go run main.go

# Generate a signed URL for the example report
go run main.go -genurl -report example_dashboard -params "organization_id=1"
```

## Project Structure

```
reporting_app/
├── main.go                       # Entry point, configuration
├── internal/core/                # Shared types and constants
├── internal/loader/              # Report discovery and parsing
├── internal/database/            # Connection pooling, queries
├── internal/security/            # HMAC, nonce tracking, CSP
├── internal/server/              # HTTP server
├── internal/handler/             # Embed and refresh handlers
├── static/                       # Thick client JavaScript
├── reports/                      # User report definitions
│   ├── report_template/          # Template for new reports
│   ├── example_dashboard/        # Simple example
├── databases.yaml                # Database connections
└── README.md
```

## Creating a Report

### Quick Start with Template
For the fastest start, use the included template:
```bash
cp -r reports/report_template reports/my_new_report
# Then customize the files in reports/my_new_report/
```

### Manual Creation
1. Create a directory in `reports/` (e.g., `reports/my_report/`)
2. Add `report.yaml`:

```yaml
id: my_report
name: "My Report"
description: "A sample report"
database: default
visibility: public
expires_after: 3600
max_rows: 10000

# Immutable parameters (in HMAC signature)
immutable_params:
  - organization_id
  - user_id

# Mutable parameters (can be changed)
mutable_params:
  - start_date
  - end_date

# Datasources (SQL queries)
# Each datasource can optionally specify its own database connection.
# If omitted, the report-level 'database' field is used as the fallback.
datasources:
  summary:
    database: default    # optional — falls back to report.database if omitted
    sql: |
      SELECT COUNT(*) as count
      FROM orders
      WHERE organization_id = {{organization_id}}
        AND order_date BETWEEN {{start_date}} AND {{end_date}}
    row_limit: 100
    cache_ttl: 300  # Optional caching
```

3. Optionally add `report.html` for custom UI

4. Generate a signed URL (tools coming soon)

## Security Model

### URL Structure
```
/api/embed?
  report_id=example&
  expires=1735689200&      # Unix timestamp
  nonce=abc123&           # Single-use token
  sig=hmac(...)&          # HMAC signature
  organization_id=1&      # immutable parameter
  start_date=2024-01-01   # mutable parameter
```

### HMAC Composition
Only immutable parameters are included in the HMAC signature:
```
message = "report_id:expires:nonce:sorted_immutable_keys:sorted_immutable_values"
sig = hmac_sha256(message, secret)
```

### Refresh Flow
1. Thick client POSTs to `/refresh` with new mutable parameters
2. Server validates: cannot change immutable parameters
3. New nonce generated for replay protection
4. Data returned with new signed URL for next refresh

### Testing Mode (ENABLE_PUBLIC_PATHS=true)
When the `ENABLE_PUBLIC_PATHS` environment variable is set to `true`, all security validation is bypassed:
- No HMAC signature required
- No nonce tracking
- No expiration checking
- Immutable parameters can still not be changed (maintains data integrity)
- Useful for development and testing

### Configurable Security Parameters (Phase 1)
All security timeouts and limits are configurable via environment variables:

**URL Expiration**
- `URL_EXPIRY_DEFAULT` — default URL lifetime (default: `5m`)
- `URL_EXPIRY_MIN` — minimum allowed expiration (default: `1m`)
- `URL_EXPIRY_MAX` — maximum allowed expiration (default: `24h`)
- `REFRESH_GRACE_PERIOD` — grace period after expiry (default: `0s`)

**NONCE Settings**
- `NONCE_BYTES` — random bytes (16‑64, default: `32`)
- `NONCE_ENCODING` — encoding format (`urlsafe-base64`, `base64`, `hex`, default: `urlsafe-base64`)
- `NONCE_MAX_AGE` — maximum nonce lifetime (default: `24h`)
- `NONCE_CLEANUP_INTERVAL` — cleanup frequency (default: `60s`)
- `NONCE_MAX_USES` — max uses per nonce (default: `1`, single‑use)
- `NONCE_USE_WINDOW` — sliding window for multi‑use (default: `5m`)

**Defaults match previous hardcoded values** — zero‑downtime migration.

**Nonce Usage Pattern:** `/api/embed` consumes a nonce use; `/refresh` does not. This allows multiple refreshes from the same signed URL while preventing replay of the embed itself.

## Thick Client API

**Complete Guide:** See `THICK_CLIENT_FOR_REPORT_DEVS.md` for detailed usage instructions.

Reports interact with data through `window.ReportApp`:

```javascript
// Load data with new parameters
const data = await window.ReportApp.refresh({
  start_date: '2024-01-01',
  end_date: '2024-12-31'
});

// Get current parameters
const params = window.ReportApp.getParams();

// Update a mutable parameter
window.ReportApp.setParam('start_date', '2024-02-01');

// Check parameter types
const isImmutable = window.ReportApp.isImmutable('organization_id');
```

## Configuration

### Environment Variables

#### Basic Configuration
- `PORT` - HTTP port (default: 8080)
- `HMAC_SECRET` - **Required** for signing URLs (unless `ENABLE_PUBLIC_PATHS=true`)
- `REPORTS_DIR` - Reports directory (default: ./reports)
- `STATIC_DIR` - Static files directory (default: ./static)
- `DATABASES_CONFIG` - Database config path (default: ./databases.yaml)
- `ENABLE_PUBLIC_PATHS` - Set to `true` to bypass HMAC/nonce/expired security (testing only, default: false)
- `ALLOW_ORIGINS` - Comma-separated list of origins allowed to embed reports (default: `*` meaning any origin). Example: `http://localhost:8080,https://example.com`
- `ALLOWED_CDNS` - Comma-separated list of CDN origins for scripts and connections (default: empty). Example: `https://cdn.jsdelivr.net,https://cdn.plot.ly`
- `QUERY_LOGGING` - Enable query logging for development/troubleshooting (default: false)
- `QUERY_LOG_DIR` - Directory for query logs (default: ./query_log)

#### Security Configuration (Phase 1)

**URL Expiration**
- `URL_EXPIRY_DEFAULT` - Default URL expiration (default: `5m`)
- `URL_EXPIRY_MIN` - Minimum allowed expiration (default: `1m`)
- `URL_EXPIRY_MAX` - Maximum allowed expiration (default: `24h`)
- `REFRESH_GRACE_PERIOD` - Grace period after URL expiry (default: `0s`)

**NONCE Settings**
- `NONCE_BYTES` - Random bytes for nonce (16‑64, default: `32`)
- `NONCE_ENCODING` - Encoding format (`urlsafe-base64`, `base64`, `hex`, default: `urlsafe-base64`)
- `NONCE_MAX_AGE` - Maximum nonce lifetime (default: `24h`)
- `NONCE_CLEANUP_INTERVAL` - Cleanup frequency (default: `60s`)
- `NONCE_MAX_USES` - Maximum uses per nonce (default: `1`, single‑use)
- `NONCE_USE_WINDOW` - Sliding window for multi‑use nonces (default: `5m`)

**Note:** All defaults match previous hardcoded values for zero‑downtime migration.

#### Snippets Configuration (Phase 2)

SQL snippets are reusable code blocks stored as YAML files in a `snippets/` directory. Reports reference them inline in their SQL using `{{snippet:name}}` syntax.

- `SNIPPETS_DIR` - Directory containing snippet YAML files (default: `./snippets`)

**Example snippet file** (`snippets/base_sales_query.yaml`):
```yaml
name: base_sales_query
description: "Base query for all sales reports"
sql: |
  SELECT s.*, c.region
  FROM sales s
  JOIN customers c ON s.id = c.customer_id
  WHERE s.status = 'active'
```

**Usage in report.yaml:**
```yaml
datasources:
  sales_data:
    database: default
    sql: |
      SELECT * FROM ({{snippet:base_sales_query}}) AS src
      WHERE {{snippet:default_date_filter}}
```

**CLI flag:** `--reload-snippets` — Reloads snippets from disk and prints count + names.

### Database Configuration
Edit `databases.yaml`:

```yaml
connections:
  default:
    driver: mysql
    dsn: "user:pass@tcp(localhost:3306)/database"
    max_connections: 10
    read_only: true
    timeout: "30s"
    row_limit: 10000
```

### Multi-Database Reports
Reports can query multiple databases — each datasource specifies its own database:

```yaml
id: cross_database_report
database: default          # fallback for datasources without a database field

datasources:
  customer_data:
    database: customer_sql  # datasource-level override
    sql: |
      SELECT * FROM customers WHERE ...

  program_data:
    database: classicmodels  # another datasource uses a different DB
    sql: |
      SELECT * FROM products WHERE ...

  fallback_report:
    # No database field — uses report.database ("default") as fallback
    sql: SELECT * FROM default_table WHERE ...
```

## Development

### Building
```bash
go build -o reporting_app main.go
```

### Testing
```bash
# Run unit tests
go test ./...

# Run with coverage
go test -cover ./...
```

### Code Style
- **No fallback chains** - Fail fast with clear errors
- **Single responsibility** - Each function does one thing
- **Explicit over implicit** - Clear code wins over clever code
- **Document why, not what** - Comments explain decisions, not restate code