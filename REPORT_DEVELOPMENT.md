# Report Development Guide

## Overview

This guide covers everything you need to know to build, test, and deploy reports in the Reporting App. The platform uses a **thick client architecture** where all data flows through a secure JavaScript layer (`window.ReportApp`) that manages HMAC-signed URLs, parameter validation, and server communication.

### Key Concepts

| Concept | Description |
|---------|-------------|
| **Report** | A collection of datasources (SQL queries) and visualizations that display data from a specific database. Defined by a `report.yaml` file and optional `dashboard.html` template. |
| **Datasource** | A named SQL query that provides data to visualizations. Each datasource has a SQL statement with parameter placeholders, row limits, and optional caching. |
| **Thick Client** | JavaScript layer (`window.ReportApp`) that acts as the sole data bridge between your report and the server. Handles security validation, parameter management, and data refreshing. |
| **Immutable Parameters** | Security-critical parameters included in HMAC signatures (e.g., `organization_id`, `user_id`). Cannot be changed after URL signing. |
| **Mutable Parameters** | User-changeable filters (e.g., `start_date`, `status`). Not included in HMAC signatures and can be freely updated via the thick client. |
| **Multi-Value Parameters** | Parameters that accept multiple values (arrays) for SQL `IN` clauses. Must be explicitly declared in `multi_value_params` in the report YAML. |
| **Signed URL** | Cryptographically signed URL containing a report ID, expiry timestamp, nonce, HMAC signature, and parameter values. Provides secure, time-limited access to reports. |

## 1. Creating a New Report

### 1.1 Quick Start with Template

The fastest way to create a new report is to copy the provided template:

```bash
cd ~/Go/reporting_app/reports
cp -r report_template my_new_report
cd my_new_report
```

Then edit `report.yaml` and `dashboard.html` to match your requirements.

### 1.2 Directory Structure

```
reports/my_new_report/
├── report.yaml      # Report definition (required)
├── dashboard.html   # HTML/JS dashboard (optional)
└── custom.js        # Additional JavaScript (optional)
```

### 1.3 Report YAML Structure

Each report must have a `report.yaml` file with the following structure:

```yaml
# Basic Information
id: my_new_report                    # Must match directory name
name: "My New Report"                # Display name
description: "Description of the report"
database: default                    # Database connection from databases.yaml
visibility: public                   # "public" or "private"
expires_after: 3600                  # URL expiration in seconds
max_rows: 10000                      # Maximum rows per datasource

# Parameter Configuration
immutable_params:
  - organization_id                  # Security boundary parameters

mutable_params:
  - start_date                       # User-changeable filters
  - end_date
  - status

# Multi-Value Parameters (optional)
multi_value_params:
  - organization_id                  # Supports multiple organization IDs
  - status                           # Multiple status values

# Datasources (SQL Queries)
datasources:
  summary_stats:
    sql: |
      SELECT COUNT(*) as total_orders,
             SUM(amount) as total_revenue
      FROM orders
      WHERE organization_id = {{organization_id}}
        AND created_at BETWEEN {{start_date}} AND {{end_date}}
    row_limit: 100                   # Maximum rows returned (0 = use default)
    cache_ttl: 300                   # Cache duration in seconds (optional)

  daily_metrics:
    sql: |
      SELECT DATE(created_at) as date,
             COUNT(*) as orders,
             SUM(amount) as revenue
      FROM orders
      WHERE organization_id = {{organization_id}}
        AND created_at BETWEEN {{start_date}} AND {{end_date}}
      GROUP BY DATE(created_at)
      ORDER BY date
    row_limit: 1000
```

### 1.4 Dashboard HTML Template

Create a `dashboard.html` file that includes:

1. **HTML structure** for your visualizations
2. **JavaScript** that interacts with the thick client
3. **CSS** for styling

The thick client JavaScript (`/static/thick_client.js`) is automatically injected by the server. Your dashboard HTML should load any required charting libraries (Chart.js, D3, Plotly, etc.) and your report-specific JavaScript.

**Critical Script Load Order:**
```html
<!-- CORRECT: Report JS first, thick client last -->
<script src="report.js"></script>
<script src="/static/thick_client.js"></script>

<!-- WRONG: Thick client first (events fire before listeners are attached) -->
<script src="/static/thick_client.js"></script>
<script src="report.js"></script>
```

See the `report_template/dashboard.html` for a complete example with loading states, error handling, and debug panel.

## 2. Parameter System

### 2.1 Parameter Types

| Type | Description | HMAC Signed | Can Be Changed |
|------|-------------|-------------|----------------|
| **Immutable** | Security boundaries (e.g., `organization_id`, `user_id`) | ✅ Yes | ❌ Never |
| **Mutable** | User filters (e.g., `start_date`, `status`, `region`) | ❌ No | ✅ Yes |
| **Multi-Value** | Supports multiple values (arrays) for SQL `IN` clauses | Same as base type | Same as base type |

### 2.2 Parameter Declaration

All parameters used in SQL must be declared in the report YAML:

```yaml
immutable_params:
  - organization_id
  - user_id

mutable_params:
  - start_date
  - end_date
  - status
  - region

# Optional: Multi-value support
multi_value_params:
  - organization_id    # Immutable multi-value
  - status             # Mutable multi-value
```

### 2.3 SQL Parameter Placeholders

Use `{{parameter_name}}` syntax in SQL queries:

```sql
-- Basic single-value comparison
WHERE organization_id = {{organization_id}}

-- BETWEEN clause (requires :value mode for dates)
WHERE created_at BETWEEN {{start_date:value}} AND {{end_date:value}}

-- Optional filter (empty → NULL)
WHERE ({{status}} IS NULL OR status = {{status}})

-- Multi-value IN clause
WHERE status IN ({{status}})  -- status in multi_value_params

-- Multi-value with explicit mode
WHERE status IN ({{status:in}})

-- Default values
WHERE discount = {{discount:0}}                     -- Numeric default
WHERE name LIKE CONCAT('%', {{search:'default'}}, '%')  -- String default
WHERE role IN ({{role:'admin','user'}})            -- Multi-value default
```

### 2.4 Parameter Expansion Rules

| Context | Placeholder | Values | Expands To |
|---------|-------------|---------|------------|
| **Comparison** | `{{param}}` | 0 values | `NULL` |
| | | 1 value | `= ?` (or `?` if `:value` mode) |
| | | Multiple values | `IN (?, ?, ...)` (if in `multi_value_params`) |
| **Value Context** | `{{param:value}}` | Any | `?` (single placeholder) |
| **IN Clause** | `{{param:in}}` | Multiple | `IN (?, ?, ...)` |
| **With Default** | `{{param:'default'}}` | Absent/empty | `?` with default value |

**Important:** The placeholder expands to SQL placeholders (`?`) or `NULL`, not the actual values. The SQL driver handles proper escaping.

### 2.5 Multi-Value Parameters

#### Declaration
Add parameter names to `multi_value_params` list:

```yaml
multi_value_params:
  - organization_id   # Immutable but can have multiple values
  - status            # Mutable with multiple values
```

#### URL Format
Multi-value parameters appear as repeated keys in the URL:
```
/api/embed?...&status=active&status=pending&status=cancelled
```

#### SQL Usage
When a parameter is declared as multi-value, use `IN` clause syntax:

```sql
-- Correct for multi-value parameters
WHERE status IN ({{status}})

-- WRONG: Single comparison operator
WHERE status = {{status}}  -- Fails with multiple values
```

#### Thick Client API (JavaScript)
```javascript
// Set multiple values
window.ReportApp.setParam('status', ['active', 'pending']);

// Add a value
window.ReportApp.addParamValue('status', 'cancelled');

// Remove a value
window.ReportApp.removeParamValue('status', 'pending');

// Get all values
const allStatuses = window.ReportApp.getParamValues('status'); // Returns array

// Get first value (backward compatibility)
const firstStatus = window.ReportApp.getParam('status'); // Returns string
```

#### SQL Patterns for Multi-Value
```sql
-- Basic IN clause
WHERE status IN ({{status}})

-- With optional filter (empty array → show all)
WHERE ({{status}} IS NULL OR status IN ({{status}}))

-- Multi-column IN (requires advanced SQL)
WHERE (col1, col2) IN (SELECT ...)
```

### 2.6 Optional Parameters & NULL Handling

Mutable parameters can be optionally provided with empty values (`?status=`), which become SQL `NULL`. Design queries to handle NULL:

#### SQL Patterns for Optional Filters
```sql
-- Single optional filter
WHERE ({{status}} IS NULL OR status = {{status}})

-- Multiple optional filters
WHERE organization_id = {{organization_id}}
  AND ({{status}} IS NULL OR status = {{status}})
  AND ({{region}} IS NULL OR region = {{region}})

-- LIKE with optional search
WHERE ({{search}} IS NULL OR name LIKE CONCAT('%', {{search}}, '%'))

-- Date range with optional start/end
WHERE created_at >= COALESCE({{start_date}}, '2000-01-01')
  AND created_at <= COALESCE({{end_date}}, '2100-12-31')
```

#### JavaScript Handling
```javascript
// Clear a filter (sets to empty string → SQL NULL)
window.ReportApp.setParam('status', '');

// Check if filter is active
function isFilterActive(paramName) {
  const value = window.ReportApp.getParam(paramName);
  return value !== '' && value !== null && value !== undefined;
}

// Apply optional filter
async function applyOptionalFilter(paramName, value) {
  const finalValue = value || '';  // Convert falsy to empty string
  return await window.ReportApp.refresh({ [paramName]: finalValue });
}
```

## 3. SQL Query Best Practices

### 3.1 Security First

**ALWAYS include organization_id filter** (or equivalent immutable parameter) in your WHERE clause to maintain data isolation:

```sql
-- Good
SELECT * FROM orders WHERE organization_id = {{organization_id}}

-- Better for JOINs
SELECT o.*, c.name 
FROM orders o
JOIN customers c ON o.customer_id = c.id
WHERE o.organization_id = {{organization_id}}
  AND c.organization_id = {{organization_id}}
```

### 3.2 Performance Optimization

1. **Use indexes**: Ensure `organization_id`, date columns, and frequently filtered columns are indexed.
2. **Set row limits**: Use `row_limit` in datasource definition to prevent excessive data transfer.
3. **Add query limits**: Use SQL `LIMIT` for large result sets.
4. **Enable caching**: Use `cache_ttl` for slow-changing data (summary statistics, reference data).
5. **Avoid N+1 queries**: Use joins instead of multiple queries when possible.

### 3.3 SQL Patterns for Common Scenarios

#### Date Range Filtering
```sql
-- Between with NULL handling
WHERE created_at BETWEEN 
  COALESCE({{start_date:value}}, '2000-01-01') 
  AND COALESCE({{end_date:value}}, '2100-12-31')

-- Optional start date only
WHERE created_at >= COALESCE({{start_date:value}}, '2000-01-01')
```

#### Multi-Select with Optional Values
```sql
-- Show all when no values selected
WHERE ({{status}} IS NULL OR status IN ({{status}}))

-- Multi-select with "All" option
WHERE ({{category}} IS NULL OR category IN ({{category}}))
  AND ({{region}} IS NULL OR region IN ({{region}}))
```

#### Search with Multiple Columns
```sql
WHERE ({{search}} IS NULL OR 
      name LIKE CONCAT('%', {{search}}, '%') OR
      email LIKE CONCAT('%', {{search}}, '%') OR
      id = {{search}})
```

### 3.4 Parameter Mode Reference

| Mode | Syntax | Use Case | Example |
|------|--------|----------|---------|
| **Auto** | `{{param}}` | Comparison context | `WHERE status = {{status}}` |
| **Value** | `{{param:value}}` | Value context | `BETWEEN {{start_date:value}} AND {{end_date:value}}` |
| **IN** | `{{param:in}}` | Explicit IN clause | `WHERE id IN ({{ids:in}})` |
| **Default** | `{{param:'default'}}` | Fallback value | `WHERE status = {{status:'active'}}` |
| **Multi-Value Default** | `{{param:'val1','val2'}}` | Fallback array | `WHERE role IN ({{role:'admin','user'}})` |

**Note:** When using `:value` mode, the placeholder expands to a single `?` regardless of how many values are provided. For multi-value parameters, use `IN ({{param}})` without `:value` mode.

### 3.5 Parameter Modes and SQL Context

#### The Problem with Auto Mode

The default "auto" mode (`{{param}}`) assumes the placeholder appears in a **comparison context** and adds comparison operators (`=` or `IN`). This causes SQL syntax errors when the placeholder appears in other contexts:

```sql
-- WRONG: Auto mode adds "=" before placeholder
WHERE created_at BETWEEN {{start_date}} AND {{end_date}}
-- Expands to: BETWEEN = ? AND = ? (SYNTAX ERROR)

-- WRONG: Auto mode in function argument
WHERE value = COALESCE({{discount}}, 0)
-- Expands to: COALESCE(= ?, 0) (SYNTAX ERROR)

-- WRONG: Auto mode in mathematical expression
SELECT price * {{tax_rate}} as total
-- Expands to: price * = ? as total (SYNTAX ERROR)
```

#### Solution: Use Appropriate Modes

1. **Value Context (BETWEEN, COALESCE, mathematical expressions)**
   Use `:value` mode to suppress operator generation:
   ```sql
   WHERE created_at BETWEEN {{start_date:value}} AND {{end_date:value}}
   WHERE value = COALESCE({{discount:value}}, 0)
   SELECT price * {{tax_rate:value}} as total
   ```

2. **Comparison Context (WHERE clauses)**
   Use default auto mode or explicit `:eq`/`:in` modes:
   ```sql
   WHERE status = {{status}}           -- Auto mode
   WHERE status = {{status:eq}}        -- Explicit equality
   WHERE id IN ({{ids}})               -- Auto mode with multi-value param
   WHERE id IN ({{ids:in}})            -- Explicit IN mode
   ```

3. **Default Values**
   Use `:default` syntax when you need fallback values:
   ```sql
   WHERE status = {{status:'active'}}                    -- Single default
   WHERE role IN ({{role:'admin','user'}})              -- Multi-value default
   WHERE discount = COALESCE({{discount:value:0}}, 0)   -- Default with value mode
   ```

#### When to Use Which Mode

| SQL Context | Recommended Syntax | Why |
|-------------|-------------------|-----|
| WHERE column = value | `{{param}}` or `{{param:eq}}` | Auto mode works correctly |
| WHERE column IN (values) | `{{param}}` (with multi_value_params) | Auto detects array values |
| BETWEEN clauses | `{{param:value}}` | Suppresses operator generation |
| Function arguments | `{{param:value}}` | Value context only |
| Mathematical expressions | `{{param:value}}` | Value context only |
| Optional filters with NULL | `WHERE ({{param}} IS NULL OR col = {{param}})` | Handles empty values |
| Default fallback values | `{{param:'default'}}` or `{{param:'val1','val2'}}` | Provides fallback |

#### Common Pitfalls and Fixes

1. **BETWEEN clause errors**: Always use `:value` mode for dates in BETWEEN.
2. **COALESCE arguments**: Use `:value` mode for parameters inside COALESCE.
3. **Multi-value in non-IN contexts**: If you need multiple values but not in an IN clause (e.g., multiple columns), you'll need to write custom SQL.
4. **Default values with value mode**: Chain modes: `{{param:value:'default'}}` (not yet supported). Instead use `COALESCE({{param:value}}, 'default')`.

#### Migration Checklist for Existing Reports

If you have existing reports that use `{{param}}` in BETWEEN, COALESCE, or mathematical expressions:

1. [ ] **Identify problematic patterns**: Search for `BETWEEN {{` and `COALESCE({{` in your SQL.
2. [ ] **Add `:value` mode**: Change `{{param}}` to `{{param:value}}` in value contexts.
3. [ ] **Test with empty values**: Ensure NULL handling still works.
4. [ ] **Test with single values**: Verify expansion produces valid SQL.
5. [ ] **Test with multi-values**: Ensure IN clauses still work where needed.

## 4. Thick Client API Reference

The thick client (`window.ReportApp`) provides the following API for report developers:

### 4.1 Core Methods

#### `refresh(params?: Object) → Promise<Data>`
Fetch fresh data from the server with optional new parameters.

```javascript
// Refresh with current parameters
const data = await window.ReportApp.refresh();

// Refresh with new mutable parameters
const data = await window.ReportApp.refresh({
  start_date: '2024-01-01',
  end_date: '2024-12-31',
  status: ['active', 'pending']  // Multi-value param
});
```

#### `getParam(key: string) → string | null`
Get the first value of a parameter (backward compatibility).

```javascript
const orgId = window.ReportApp.getParam('organization_id');
```

#### `getParamValues(key: string) → string[]`
Get all values for a parameter (supports multi-value).

```javascript
const allStatuses = window.ReportApp.getParamValues('status');
// Returns ['active', 'pending'] or [] if not set
```

#### `setParam(key: string, value: string | string[])`
Set a parameter value. For multi-value parameters, accepts array.

```javascript
// Single value
window.ReportApp.setParam('start_date', '2024-01-01');

// Multi-value (array)
window.ReportApp.setParam('status', ['active', 'pending']);

// Clear parameter (sets to empty string → SQL NULL)
window.ReportApp.setParam('status', '');
```

#### `setParamValues(key: string, values: string[])`
Explicitly set multiple values for a parameter.

```javascript
window.ReportApp.setParamValues('status', ['active', 'pending', 'cancelled']);
```

#### `addParamValue(key: string, value: string)`
Add a value to a multi-value parameter (appends).

```javascript
window.ReportApp.addParamValue('status', 'cancelled');
```

#### `removeParamValue(key: string, value: string)`
Remove a specific value from a multi-value parameter.

```javascript
window.ReportApp.removeParamValue('status', 'pending');
```

#### `getParams() → Object`
Get all current parameters (returns object with arrays for multi-value).

```javascript
const params = window.ReportApp.getParams();
// { organization_id: ["123"], status: ["active", "pending"] }
```

#### `isImmutable(key: string) → boolean`
Check if a parameter is immutable (security boundary).

```javascript
if (window.ReportApp.isImmutable('organization_id')) {
  console.log('Cannot change organization_id');
}
```

#### `isMutable(key: string) → boolean`
Check if a parameter is mutable (user-changeable).

#### `isMultiValue(key: string) → boolean`
Check if a parameter supports multiple values.

### 4.2 Events

The thick client emits DOM events for decoupled communication:

```javascript
// Show spinner when refresh starts
window.addEventListener('report:refresh-start', () => {
  document.body.classList.add('loading');
});

// Hide spinner when refresh completes
window.addEventListener('report:refresh-complete', (event) => {
  document.body.classList.remove('loading');
  const { data, url } = event.detail;
  updateVisualizations(data);
});

// Handle errors
window.addEventListener('report:refresh-error', (event) => {
  const { error, url } = event.detail;
  showError(`Refresh failed: ${error.message}`);
});

// Initial data ready
window.addEventListener('report:data-ready', (event) => {
  const { data } = event.detail;
  initializeDashboard(data);
});
```

### 4.3 Initialization Pattern

Always check for thick client availability and wait for DOM ready:

```javascript
document.addEventListener('DOMContentLoaded', async function() {
  // 1. Check thick client is available
  if (!window.ReportApp || typeof window.ReportApp.refresh !== 'function') {
    showFatalError('Thick client not available. Access via reporting app.');
    return;
  }
  
  // 2. Parse configuration from server
  const config = parseReportConfig();
  
  // 3. Wait for thick client ready event (if needed)
  await new Promise((resolve) => {
    if (window.ReportApp.getParam) {
      resolve();
    } else {
      window.ReportApp.on('ready', resolve);
    }
  });
  
  // 4. Initialize your dashboard
  initializeDashboard(config);
  
  // 5. Load initial data
  await loadInitialData();
});

function parseReportConfig() {
  const config = window.ReportConfig || {};
  let params = {};
  
  if (config.params) {
    try {
      // Parameters are JSON strings
      params = JSON.parse(config.params);
    } catch (e) {
      console.error('Failed to parse params:', e);
    }
  }
  
  return { params, config };
}
```

### 4.5 Datasource-Based Reporting Model

The reporting app also supports a **datasource-based reporting model** optimized for JavaScript‑first development. This model provides a structured client API (`window.__reportData`) alongside the thick client for maximum flexibility.

#### Architecture Comparison

| Aspect | Chart‑Based Model | Datasource‑Based Model |
|--------|-------------------|------------------------|
| **Data Access** | Thick client only (`window.ReportApp`) | Client API + thick client hybrid |
| **SQL Definition** | In `report.yaml` per datasource | In `manifest.yaml` as datasources |
| **Rendering** | Built‑in templates or simple Chart.js | Any JavaScript library (React, D3, Plotly, etc.) |
| **Interactivity** | Via thick client API | Via client API or custom JavaScript |
| **Coordination** | Limited cross‑filtering | Full custom coordination |
| **Best For** | Simple dashboards, SQL‑centric teams | Complex visualizations, JavaScript‑heavy teams |

#### Client API (`window.__reportData`)

For datasource‑based reports, the server injects a structured API:

```javascript
window.__reportData = {
  reportID: 'sales_dashboard',
  datasources: {
    monthly_sales: {
      getRows: function(params) { /* fetch from server */ },
      getColumns: function() { /* fetch metadata */ },
      // Inline data available on page load
      data: {
        columns: ['month', 'revenue'],
        rows: [['2024‑01', 10000], ...]
      }
    }
  },
  parameters: {
    schema: { /* parameter definitions */ },
    current: { /* current values */ },
    immutable: ['customer_id'],
    mutable: ['start_date', 'end_date']
  }
};
```

#### Using Datasource API with Thick Client

You can use both APIs together for maximum flexibility:

```javascript
// Option 1: Use client API only
const monthlySales = window.__reportData.datasources.monthly_sales;
const data = await monthlySales.getRows({ start_date: '2024‑01‑01' });

// Option 2: Use thick client for URL state, client API for data
window.ReportApp.setParam('start_date', '2024‑01‑01');
const data = await window.__reportData.datasources.monthly_sales
  .getRows(window.ReportApp.getParams());

// Option 3: Hybrid approach
function refreshData(params) {
  // Update thick client state
  window.ReportApp.setParam('start_date', params.start_date);
  
  // Fetch via datasource API
  return window.__reportData.datasources.monthly_sales.getRows(params);
}
```

#### Parameter Classification Still Applies

The same immutable/mutable parameter classification works for both models. Security boundaries are maintained regardless of which API you use.

#### Choosing the Right Model

**Use chart‑based model when:**
- You need simple dashboards quickly
- Your team is SQL‑centric with limited JavaScript expertise
- Standard chart types (Chart.js) are sufficient
- Legacy compatibility is required

**Use datasource‑based model when:**
- Complex interactive visualizations are needed (D3, Plotly, custom)
- JavaScript/React expertise is available
- Advanced coordination between visualizations
- Hot reload during development is desired (manifests support file watching)

#### Configuration

To enable datasource‑based reports:
1. Set `MANIFESTS_DIR=./manifests` environment variable
2. Create `manifests/{report}.yaml` files
3. Enable hot reload: `ENABLE_HOT_RELOAD=true` (optional)

The system automatically routes requests to the appropriate handler based on whether a manifest exists for the report ID.

## 5. Multi-Value Parameter Implementation Guide

### 5.1 Complete Example: Multi-Select Report

#### Report YAML
```yaml
id: multi_select_demo
name: "Multi‑Select Demo"
database: default

immutable_params:
  - organization_id

mutable_params:
  - status
  - region

multi_value_params:
  - organization_id
  - status

datasources:
  orders_by_status:
    sql: |
      SELECT status, COUNT(*) as count, SUM(amount) as total
      FROM orders
      WHERE organization_id IN ({{organization_id}})
        AND status IN ({{status}})
      GROUP BY status
    row_limit: 100
```

#### JavaScript: Multi‑Select UI
```javascript
// Initialize multi‑select UI
function initMultiSelect() {
  const statusSelect = document.getElementById('status-select');
  
  // Load available options (from reference datasource)
  loadStatusOptions().then(options => {
    options.forEach(option => {
      const opt = document.createElement('option');
      opt.value = option.value;
      opt.textContent = option.label;
      statusSelect.appendChild(opt);
    });
    
    // Make it a multi‑select
    statusSelect.multiple = true;
    
    // Set current values
    const currentStatuses = window.ReportApp.getParamValues('status');
    Array.from(statusSelect.options).forEach(opt => {
      opt.selected = currentStatuses.includes(opt.value);
    });
  });
  
  // Handle selection changes
  statusSelect.addEventListener('change', () => {
    const selected = Array.from(statusSelect.selectedOptions)
                         .map(opt => opt.value);
    
    // Update thick client
    window.ReportApp.setParamValues('status', selected);
    
    // Trigger refresh (debounced)
    debouncedRefresh();
  });
}

// Debounced refresh
let refreshTimeout;
function debouncedRefresh() {
  clearTimeout(refreshTimeout);
  refreshTimeout = setTimeout(() => {
    window.ReportApp.refresh({});
  }, 300);
}
```

#### SQL: Handling Empty Multi‑Select
```sql
-- Show all when no statuses selected
WHERE ({{status}} IS NULL OR status IN ({{status}}))

-- Alternative: Use default values
WHERE status IN ({{status:'active','pending','cancelled'}})
```

### 5.2 Migration from Single‑Value to Multi‑Value

If you need to upgrade an existing parameter to support multiple values:

1. **Update report.yaml**: Add parameter to `multi_value_params`
2. **Update SQL queries**: Change `=` to `IN` for that parameter
3. **Update JavaScript**: Use array‑aware methods (`getParamValues`, `setParamValues`)
4. **Test thoroughly**: With 0, 1, and multiple values

## 6. Best Practices for Report Development

### 6.1 Security

1. **Never modify immutable parameters** – this violates HMAC security.
2. **Validate all user inputs** before passing to `refresh()`.
3. **Sanitize data before rendering** to prevent XSS.
4. **Use HTTPS** in production for all CDN resources.
5. **Respect CSP** – only load scripts from allowed CDNs.

### 6.2 Performance

1. **Debounce rapid filter changes** – use 300ms delay for refresh calls.
2. **Implement loading states** – show spinners/skeletons during refreshes.
3. **Cache data client‑side** when appropriate (but respect data isolation).
4. **Use server‑side pagination** for large datasets (>1000 rows).
5. **Limit concurrent requests** – avoid multiple simultaneous refreshes.

### 6.3 Error Handling

```javascript
async function refreshWithErrorHandling(params) {
  showLoading();
  
  try {
    const data = await window.ReportApp.refresh(params);
    updateVisualizations(data);
    hideError();
  } catch (error) {
    console.error('Refresh failed:', error);
    
    // User‑friendly error messages
    if (error.message.includes('HMAC')) {
      showError('Security validation failed. Please reload the report.');
    } else if (error.message.includes('SQL')) {
      showError('Database query failed. Check your filter values.');
    } else {
      showError(`Failed to load data: ${error.message}`);
    }
    
    // Show empty states
    showEmptyStates();
  } finally {
    hideLoading();
  }
}
```

### 6.4 Testing

#### Generate Test URLs
```bash
# Single‑value parameters
go run main.go -genurl -report my_report \
  -params "organization_id=123,start_date=2024-01 01"

# Multi‑value parameters (comma‑separated values)
go run main.go -genurl -report multi_select_demo \
  -params "organization_id=1,2,3&status=active,pending"
```

#### Development Mode
Set `ENABLE_PUBLIC_PATHS=true` in `.env` to bypass HMAC for testing:

1. No HMAC signature required
2. No nonce tracking
3. No expiration checking
4. Immutable parameters still cannot be changed

#### Console Debugging
```javascript
// Check thick client availability
console.log('ReportApp:', window.ReportApp);
console.log('ReportConfig:', window.ReportConfig);

// Monitor refresh calls
const originalRefresh = window.ReportApp.refresh;
window.ReportApp.refresh = async function(...args) {
  console.log('refresh() called with:', args);
  const result = await originalRefresh.apply(this, args);
  console.log('refresh() returned:', result);
  return result;
};

// Log parameter changes
window.ReportApp.on('param-changed', (event) => {
  console.log('Parameter changed:', event.detail);
});
```

## 7. Common Issues & Solutions

### Issue 1: "Thick client not available"
**Cause**: Directly opening HTML file instead of through reporting app.
**Solution**: Always access via `/api/embed?...` signed URL.

### Issue 2: "Cannot change immutable parameter"
**Cause**: Trying to modify a parameter marked as immutable.
**Solution**: Only use `setParam()` for mutable parameters. Check with `isImmutable()`.

### Issue 3: "params is a string, not an object"
**Cause**: Forgetting to parse `window.ReportConfig.params`.
**Solution**: Always parse: `JSON.parse(window.ReportConfig.params)`.

### Issue 4: "SQL syntax error" with BETWEEN
**Cause**: Using `{{param}}` in value context without `:value` mode.
**Solution**: Use `{{start_date:value}}` and `{{end_date:value}}` in BETWEEN clauses.

### Issue 5: Multi‑value parameter not working in IN clause
**Cause**: Parameter not declared in `multi_value_params` or using `=` instead of `IN`.
**Solution**: Add to `multi_value_params` and use `WHERE column IN ({{param}})`.

### Issue 6: Empty optional filter shows no data
**Cause**: SQL not handling NULL properly.
**Solution**: Use `WHERE ({{param}} IS NULL OR column = {{param}})` pattern.

### Issue 7: Blink/flicker during refresh
**Solution**: Implement CSS transitions and skeleton screens.

### Issue 8: "Report:data-ready event not firing"
**Cause**: Thick client loaded before report JavaScript.
**Solution**: Load thick client script LAST: `<script src="/static/thick_client.js"></script>` should be the last script tag.

## 8. Advanced Patterns

### 8.1 Cross‑Filtering
```javascript
// When chart A filters chart B
function setupCrossFilter(chartA, chartB) {
  chartA.on('click', async (event) => {
    const selectedValue = event.detail.value;
    
    // Update filter parameter
    window.ReportApp.setParam('category', selectedValue);
    
    // Refresh data
    const data = await window.ReportApp.refresh();
    
    // Update all charts
    updateChart(chartA, data);
    updateChart(chartB, data);
  });
}
```

### 8.2 Real‑Time Updates
```javascript
// Poll for updates every 30 seconds
let pollInterval;

function startPolling(interval = 30000) {
  pollInterval = setInterval(async () => {
    try {
      const data = await window.ReportApp.refresh();
      updateVisualizations(data);
    } catch (error) {
      console.error('Polling failed:', error);
      stopPolling();
    }
  }, interval);
}

function stopPolling() {
  clearInterval(pollInterval);
}
```

### 8.3 Export to CSV
```javascript
function exportToCSV(data, datasourceName) {
  const ds = data[datasourceName];
  if (!ds) return;
  
  const { columns, rows } = ds;
  
  // Create CSV content
  const csvContent = [
    columns.join(','),
    ...rows.map(row => row.join(','))
  ].join('\n');
  
  // Trigger download
  const blob = new Blob([csvContent], { type: 'text/csv' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${datasourceName}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}
```

## 9. Reference: Report Configuration Object

The server injects `window.ReportConfig` with:

```javascript
{
  "reportId": "customer_dashboard",
  "reportName": "Customer Dashboard",
  "params": "{\"organization_id\":[\"123\"],\"start_date\":\"2024-01-01\",\"status\":[\"active\",\"pending\"]}", // JSON string!
  "immutableParams": ["organization_id"],
  "mutableParams": ["start_date", "end_date", "status"],
  "multiValueParams": ["organization_id", "status"], // New field
  "datasources": "{\"sales_summary\":{\"SQL\":\"SELECT ...\",\"RowLimit\":100}}",
  "currentUrl": "/api/embed?..."
}
```

**Note**: `params` and `datasources` are JSON strings that need parsing with `JSON.parse()`.

## 10. Migration Checklist for Multi‑Value Parameters

When upgrading to the multi‑value parameter system:

1. [ ] **Update report.yaml** – add `multi_value_params` list
2. [ ] **Update SQL queries** – change `=` to `IN` for multi‑value parameters
3. [ ] **Update JavaScript** – use array‑aware methods (`getParamValues`, `setParamValues`)
4. [ ] **Test with 0 values** – ensure SQL handles NULL/empty arrays
5. [ ] **Test with 1 value** – ensure backward compatibility
6. [ ] **Test with multiple values** – verify IN clause works
7. [ ] **Update UI controls** – implement multi‑select widgets
8. [ ] **Verify security** – immutable multi‑value params cannot be changed

---

## Appendix A: Quick Reference

### SQL Placeholder Cheat Sheet

| Use Case | Syntax | Example |
|----------|--------|---------|
| Single value comparison | `{{param}}` | `WHERE status = {{status}}` |
| Value context (BETWEEN, COALESCE) | `{{param:value}}` | `BETWEEN {{start_date:value}} AND {{end_date:value}}` |
| Multi‑value IN clause | `{{param}}` (in `multi_value_params`) | `WHERE id IN ({{ids}})` |
| Default value | `{{param:'default'}}` | `WHERE role = {{role:'user'}}` |
| Multi‑value default | `{{param:'val1','val2'}}` | `WHERE role IN ({{role:'admin','user'}})` |
| Optional filter | `WHERE ({{param}} IS NULL OR col = {{param}})` | Works with all types |

### Thick Client Method Cheat Sheet

| Task | Method |
|------|--------|
| Refresh data | `ReportApp.refresh(params)` |
| Get parameter value | `ReportApp.getParam(key)` |
| Get all values | `ReportApp.getParamValues(key)` |
| Set parameter | `ReportApp.setParam(key, value)` |
| Set multiple values | `ReportApp.setParamValues(key, values)` |
| Add value | `ReportApp.addParamValue(key, value)` |
| Remove value | `ReportApp.removeParamValue(key, value)` |
| Get all params | `ReportApp.getParams()` |
| Check immutable | `ReportApp.isImmutable(key)` |
| Check multi‑value | `ReportApp.isMultiValue(key)` |

## Appendix B: Example Reports

Study these example reports for implementation patterns:

1. **`report_template`** – Basic template with best practices
2. **`program_analysis_v2`** – Complex multi‑value parameter usage
3. **`customer_referral_analytics`** – Advanced SQL patterns
4. **`referral_funnel_dashboard`** – BETWEEN clause with `:value` mode

---

## Support & Troubleshooting

If you encounter issues:

1. **Check browser console** for JavaScript errors
2. **Verify thick client is loaded** (`console.log(window.ReportApp)`)
3. **Check network tab** for failed requests to `/refresh`
4. **Validate your `report.yaml`** syntax
5. **Test with minimal example** to isolate the issue

For persistent issues, contact the reporting app maintainers with:
- Report ID
- Error message from console
- Steps to reproduce
- Browser/OS information

---

*Document Version: 2.0 (Updated for Multi‑Value Parameters)*  
*Last Updated: 2026-05-11*  
*Based on Reporting App v2.1+*
