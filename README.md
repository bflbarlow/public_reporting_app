# Reporting App

A secure, embeddable reporting platform with HMAC-signed URLs and thick client architecture.

## Architecture Principles

1. **Single Loader, Single Truth** - One way to load all reports
2. **No Fallback Logic** - Code reads linearly, no "try A then B then C"
3. **Zero Duplication** - One source of truth for types and logic
4. **Thick Client is the ONLY Data Bridge** - All data flows through `window.ReportApp`
5. **Simple Over Clever** - Readable code wins over clever abstractions
6. **No Backwards Compatibility** - This project is in development. The end result of all coding changes must be a purely clean codebase.
7. **With One Voice** - The codebase should be considered a single voice as if created by a single developer with a singular vision.
8. **Focus On Destination** - Any development and code changes must define the destination and define the path for which to get there.


## Core Requirements

These details are to *ALWAYS* be maintained with every code change.

1. **Embeddability** - The sole purpose of this application is to produce embeddable reports into a parent application.
2. **Security** - This application must secure access to data for only those that the application intends. This includes HMAC Signatures (including immutable parameters), NONCE values, and expired URLs.
3. **Thick Client** - Only one mechanism handles the movement of data on the client side. The thick client's sole purpose is to coordinate requests from the client to the reporting app to refresh data and provide that data back to the client.

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
│   └── customer_*/               # Example customer reports
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
datasources:
  summary:
    sql: |
      SELECT COUNT(*) as count
      FROM orders
      WHERE organization_id = {{organization_id}}
        AND order_date BETWEEN {{start_date}} AND {{end_date}}
    row_limit: 100
    cache_ttl: 300  # Optional caching
```

3. Optionally add `dashboard.html` for custom UI

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

## Thick Client API

**📖 Complete Guide:** See `THICK_CLIENT_FOR_REPORT_DEVS.md` for detailed usage instructions.

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
- `PORT` - HTTP port (default: 8080)
- `HMAC_SECRET` - **Required** for signing URLs (unless `ENABLE_PUBLIC_PATHS=true`)
- `REPORTS_DIR` - Reports directory (default: ./reports)
- `STATIC_DIR` - Static files directory (default: ./static)
- `DATABASES_CONFIG` - Database config path (default: ./databases.yaml)
- `ENABLE_PUBLIC_PATHS` - Set to `true` to bypass HMAC/nonce/expired security (testing only, default: false)
- `ALLOW_ORIGINS` - Comma-separated list of origins allowed to embed reports (default: `*` meaning any origin). Example: `http://localhost:8080,https://example.com`
- `ALLOWED_CDNS` - Comma-separated list of CDN origins for scripts and connections (default: empty). Example: `https://cdn.jsdelivr.net,https://cdn.plot.ly`

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