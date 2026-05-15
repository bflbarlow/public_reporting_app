# Report Development: Best Practices & Conventions

## Executive Summary

This document defines the most efficient and effective patterns for building reports in the Reporting App platform. Following these conventions ensures:

1. **Consistent quality** across all reports
2. **Optimal performance** for users and databases  
3. **Robust security** and data isolation
4. **Maintainable code** that's easy to debug and extend
5. **Professional user experience** with polished interfaces

## 1. Security-First Architecture

### 1.1 Immutable Parameters: The Foundation

**Golden Rule:** Every report MUST use immutable parameters for data isolation.

```yaml
# REQUIRED - Every report.yaml
immutable_params:
  - organization_id    # Primary data isolation boundary
  - tenant_id         # Multi-tenant environments
  - user_id           # User-level access control
```

### 1.2 Database Configuration

Reports can use a **single database** (via `report.database`) or **multiple databases** (via `datasources.{name}.database`):

```yaml
# Single-database report (simpler)
id: simple_report
database: default    # All datasources use this
datasources:
  summary:
    sql: SELECT * FROM default_table   # uses report.database

# Multi-database report (each datasource picks its DB)
id: cross_db_report
database: default    # fallback for datasources without a database field
datasources:
  customer_data:
    database: customer_sql    # datasource-level override
    sql: SELECT * FROM customers

  program_data:
    database: classicmodels   # another datasource uses a different DB
    sql: SELECT * FROM products

  fallback_report:
    # No database field — uses report.database ("default") as fallback
    sql: SELECT * FROM default_table
```

**Rules:**
- `report.database` is **required** as the fallback for all datasources
- `datasources.{name}.database` is **optional** — if omitted, the report-level database is used
- At least one of `report.database` or a datasource-level `database` must be set for the report to be valid
- Database names must exist in `databases.yaml` — validation errors are logged at startup

**Implementation Checklist:**
- [ ] **ALWAYS** include `organization_id` as immutable parameter
- [ ] **ALWAYS** filter by `organization_id` in every SQL query
- [ ] **NEVER** expose organization_id in UI controls
- [ ] **NEVER** allow users to change immutable parameters

### 1.2 SQL Security Patterns

**Secure Query Structure:**
```sql
-- CORRECT: Organization filter in main WHERE clause
SELECT * FROM orders 
WHERE organization_id = {{organization_id}}
  AND created_at BETWEEN {{start_date}} AND {{end_date}}

-- CORRECT: Organization filter in JOINs
SELECT o.*, c.name 
FROM orders o
JOIN customers c ON o.customer_id = c.id
WHERE o.organization_id = {{organization_id}}
  AND c.organization_id = {{organization_id}}  -- Critical for JOINs
```

**Security Anti-Patterns (NEVER DO THIS):**
```sql
-- WRONG: No organization filter
SELECT * FROM orders WHERE created_at > '2024-01-01'

-- WRONG: Filter in subquery only
SELECT * FROM orders 
WHERE id IN (SELECT id FROM orders WHERE organization_id = {{organization_id}})

-- WRONG: Incomplete JOIN filtering
SELECT o.*, c.* 
FROM orders o
JOIN customers c ON o.customer_id = c.id
WHERE o.organization_id = {{organization_id}}  -- Missing c.organization_id filter!
```

### 1.3 Parameter Validation

**Server-Side Enforcement:**
- Immutable parameters are HMAC-signed and cannot be changed
- Mutable parameters are validated before SQL execution
- SQL injection protection via parameterized queries

**Client-Side UX:**
```javascript
// Disable UI controls for immutable parameters
if (window.ReportApp.isImmutable('organization_id')) {
  document.getElementById('org-select').disabled = true;
  document.getElementById('org-select').title = 'Cannot change organization';
}

// Show visual indication
<div class="param-control">
  <label>Organization ID 
    <span class="badge badge-immutable">Immutable</span>
  </label>
  <input type="text" value="{{organization_id}}" readonly>
</div>
```

## 2. SQL Query Excellence

### 2.1 Performance Optimization Patterns

**Index-Friendly Queries:**
```sql
-- GOOD: Uses indexed columns
WHERE organization_id = {{organization_id}}
  AND created_at BETWEEN {{start_date}} AND {{end_date}}
  AND status = {{status}}

-- BAD: Non-sargable query (can't use index)
WHERE YEAR(created_at) = 2024  -- Function on column
WHERE amount * 1.1 > 100       -- Calculation on column
WHERE name LIKE '%search%'     -- Leading wildcard
```

**Row Limiting Strategies:**
```yaml
datasources:
  summary_data:
    sql: |
      SELECT * FROM large_table
      WHERE organization_id = {{organization_id}}
      ORDER BY created_at DESC
      LIMIT 100  -- SQL-level limit
    row_limit: 100  # System-level limit (redundant safety)
```

**Efficient JOIN Patterns:**
```sql
-- GOOD: Filter early, join small datasets
WITH filtered_orders AS (
  SELECT * FROM orders 
  WHERE organization_id = {{organization_id}}
    AND created_at BETWEEN {{start_date}} AND {{end_date}}
)
SELECT o.*, c.name 
FROM filtered_orders o
JOIN customers c ON o.customer_id = c.id
WHERE c.organization_id = {{organization_id}}  -- JOIN filter

-- BAD: Join before filtering
SELECT o.*, c.name 
FROM orders o
JOIN customers c ON o.customer_id = c.id
WHERE o.organization_id = {{organization_id}}
  AND c.organization_id = {{organization_id}}
  AND o.created_at BETWEEN {{start_date}} AND {{end_date}}
```

### 2.2 Parameter Handling Patterns

**Optional Filter Pattern (NULL Handling):**
```sql
-- Optional date range with COALESCE fallback
WHERE created_at >= COALESCE({{start_date}}, '2000-01-01')
  AND created_at <= COALESCE({{end_date}}, '2100-12-31')

-- Optional value with default fallback
WHERE role = COALESCE({{role}}, 'user')
```

**Multi-Value Parameter Best Practices:**
```yaml
# Report.yaml declaration
multi_value_params:
  - status           # For IN clauses
  - category_ids     # Large lists
  - region_codes     # Code-based filters

# SQL with multi-value parameters
WHERE status IN ({{status}})
  AND category_id IN ({{category_ids}})
  AND ({{region_codes}} IS NULL OR region_code IN ({{region_codes}}))
```

**Default Value Syntax:**

| Use Case | Syntax | Example | Why |
|----------|--------|---------|-----|
| **Default Value** | `{{param:'default'}}` | `WHERE role = {{role:'user'}}` | Fallback when parameter is empty or absent |
| **Multi Default** | `{{param:'a','b'}}` | `WHERE role IN ({{role:'admin','user'}})` | Array fallback for multi-value params |

### 2.3 Complex Query Patterns

**CTEs for Readability:**
```sql
WITH 
  filtered_referrals AS (
    SELECT * FROM referrals
    WHERE organization_id = {{organization_id}}
      AND referral_date BETWEEN {{start_date}} AND {{end_date}}
  ),
  status_summary AS (
    SELECT 
      status,
      COUNT(*) as count,
      COUNT(*) * 100.0 / SUM(COUNT(*)) OVER () as percentage
    FROM filtered_referrals
    GROUP BY status
  )
SELECT * FROM status_summary
ORDER BY count DESC;
```

**Window Functions for Analytics:**
```sql
SELECT 
  date,
  category,
  referrals,
  SUM(referrals) OVER (PARTITION BY category ORDER BY date) as running_total,
  AVG(referrals) OVER (PARTITION BY category ORDER BY date ROWS 6 PRECEDING) as weekly_avg,
  RANK() OVER (PARTITION BY date ORDER BY referrals DESC) as daily_rank
FROM daily_category_stats
WHERE organization_id = {{organization_id}}
ORDER BY date, category;
```

### 2.4 Performance Anti-Patterns to Avoid

**❌ SELECT ***
```sql
-- BAD: Returns all columns
SELECT * FROM large_table

-- GOOD: Only needed columns
SELECT id, name, created_at, amount FROM orders
```

**❌ N+1 Query Patterns**
```sql
-- BAD: Multiple queries in JavaScript loop
// JavaScript
for (const category of categories) {
  const data = await fetchData({ category: category.id });
}

-- GOOD: Single query with IN clause
WHERE category_id IN ({{category_ids}})
```

**❌ Expensive String Operations**
```sql
-- BAD: Slow string operations
WHERE LOWER(name) = LOWER({{search}})
WHERE CONCAT(first_name, ' ', last_name) LIKE '%{{search}}%'

-- GOOD: Index-friendly alternatives
WHERE name = {{search}} COLLATE utf8mb4_general_ci
WHERE first_name LIKE CONCAT({{search}}, '%')
  OR last_name LIKE CONCAT({{search}}, '%')
```

**❌ Unbounded Result Sets**
```sql
-- BAD: No limits
SELECT * FROM history_table

-- GOOD: Reasonable limits
SELECT * FROM history_table 
WHERE organization_id = {{organization_id}}
ORDER BY created_at DESC
LIMIT 1000  -- SQL limit
-- PLUS: row_limit: 1000 in YAML
```

## 3. Thick Client Patterns

### 3.1 Initialization & Error Handling

**Robust Initialization Pattern:**
```javascript
document.addEventListener('DOMContentLoaded', async function() {
  // 1. Check thick client availability
  if (!window.ReportApp || typeof window.ReportApp.refresh !== 'function') {
    showFatalError('Thick client not available. Access via reporting app.');
    return;
  }
  
  // 2. Parse configuration
  const config = parseReportConfig();
  
  // 3. Wait for ready event if needed
  await new Promise((resolve) => {
    if (window.ReportApp.getParam) {
      resolve();
    } else {
      window.ReportApp.on('ready', resolve);
    }
  });
  
  // 4. Initialize dashboard
  initializeDashboard(config);
  
  // 5. Load initial data
  await loadInitialData();
});

function showFatalError(message) {
  document.body.innerHTML = `
    <div class="fatal-error">
      <h2>❌ Report Error</h2>
      <p>${message}</p>
      <p>Please ensure you are accessing via the reporting application.</p>
    </div>
  `;
}
```

**Configuration Parsing:**
```javascript
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

### 3.2 Data Fetching Patterns

**Debounced Refresh for Interactive Filters:**
```javascript
let refreshTimeout;

function onFilterChange(newParams) {
  // Clear any pending refresh
  if (refreshTimeout) {
    clearTimeout(refreshTimeout);
  }
  
  // Debounce by 300ms
  refreshTimeout = setTimeout(() => {
    fetchData(newParams);
  }, 300);
}
```

**Safe Refresh with Validation:**
```javascript
async function refreshWithValidation(newParams = {}) {
  // Validate before sending
  const currentParams = window.ReportApp.getParams();
  const finalParams = { ...currentParams, ...newParams };
  
  // Check for required parameters
  if (!finalParams.start_date || !finalParams.end_date) {
    showError('Start date and end date are required');
    return;
  }
  
  // Validate date range
  if (finalParams.start_date > finalParams.end_date) {
    showError('Start date must be before end date');
    return;
  }
  
  // Ensure we're not changing immutable params
  for (const key in newParams) {
    if (window.ReportApp.isImmutable(key)) {
      const currentValue = currentParams[key];
      if (currentValue && newParams[key] !== currentValue) {
        showError(`Cannot change ${key} from ${currentValue} to ${newParams[key]}`);
        return;
      }
    }
  }
  
  return await fetchData(newParams);
}
```

**Multi-Value Parameter Handling:**
```javascript
// Set a parameter (multi-value params use comma-separated strings internally)
window.ReportApp.setParam('status', 'active');

// Get all current parameters
const params = window.ReportApp.getParams();

// Check parameter type
if (window.ReportApp.isImmutable('organization_id')) {
  // Show read-only indicator
}
if (window.ReportApp.isMutable('status')) {
  // Show filter control
}
```

### 3.3 Event-Driven Architecture

**Thick Client Event System:**

The thick client provides a basic `on()`/`emit()` event system. It emits a `'ready'` event when initialization is complete:

```javascript
// Wait for thick client to be ready
window.ReportApp.on('ready', (data) => {
  console.log('Report ready:', data.reportId);
  initializeDashboard();
});

// Custom events for report author use
customEvent: function(event, data) {
  if (window.ReportApp.emit) {
    window.ReportApp.emit(event, data);
  }
}
```

### 3.4 Loading State Patterns

**Skeleton Screens:**
```css
/* CSS for skeleton loading */
.skeleton {
  background: linear-gradient(90deg, #f0f0f0 25%, #e0e0e0 50%, #f0f0f0 75%);
  background-size: 200% 100%;
  animation: shimmer 1.5s infinite;
  border-radius: 4px;
}

.skeleton-chart {
  height: 400px;
  width: 100%;
}

.skeleton-card {
  height: 120px;
  width: 100%;
}

@keyframes shimmer {
  from { background-position: -200% 0; }
  to { background-position: 200% 0; }
}
```

**Loading Overlay:**
```javascript
function showLoadingOverlay() {
  const overlay = document.getElementById('loading-overlay') || createLoadingOverlay();
  overlay.classList.add('active');
}

function hideLoadingOverlay() {
  const overlay = document.getElementById('loading-overlay');
  if (overlay) {
    overlay.classList.remove('active');
  }
}

function createLoadingOverlay() {
  const overlay = document.createElement('div');
  overlay.id = 'loading-overlay';
  overlay.className = 'loading-overlay';
  overlay.innerHTML = `
    <div class="spinner-container">
      <div class="spinner"></div>
      <p>Loading data...</p>
    </div>
  `;
  document.body.appendChild(overlay);
  return overlay;
}
```

## 4. UI/UX Patterns

### 4.1 Consistent Visual Design

**Follow the STYLES.md Color Palette:**
```css
/* Use established color palette */
:root {
  --primary-blue: #3a70a0;
  --dark-blue: #33485e;
  --dark-teal: #167e8e;
  --success-green: #3c763d;
  --danger-red: #bc473a;
  --light-gray: #f8f8f8;
  --medium-gray: #f0f0f0;
}

/* Report-specific overrides */
.report-header {
  background: var(--dark-blue);
  color: white;
  border-left: 6px solid var(--dark-teal);
}

.summary-card {
  background: white;
  border-radius: 12px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
}

.error-message {
  background: #fceceb;
  border-left: 6px solid var(--danger-red);
  color: var(--danger-red);
}
```

**Responsive Design Patterns:**
```css
/* Mobile-first responsive grid */
.dashboard-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 20px;
}

@media (min-width: 768px) {
  .dashboard-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (min-width: 1200px) {
  .dashboard-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

/* Chart responsiveness */
.chart-container {
  position: relative;
  height: 400px;
  width: 100%;
}

@media (max-width: 768px) {
  .chart-container {
    height: 300px;
  }
}
```

### 4.2 Filter Controls Design

**Parameter Type Indicators:**
```html
<div class="filter-group">
  <label for="start-date">
    Start Date
    <span class="badge badge-mutable">Mutable</span>
  </label>
  <input type="date" id="start-date" class="form-control">
</div>

<div class="filter-group">
  <label for="organization-id">
    Organization ID
    <span class="badge badge-immutable">Immutable</span>
  </label>
  <input type="text" id="organization-id" 
         value="{{organization_id}}" readonly
         class="form-control" disabled>
</div>
```

**Multi-Select UI Patterns:**
```javascript
function initMultiSelect(parameterName, currentValues = []) {
  const container = document.getElementById(`${parameterName}-container`);
  
  // Fetch available options
  fetchOptions(parameterName).then(options => {
    // Create multi-select
    const select = document.createElement('select');
    select.id = parameterName;
    select.multiple = true;
    select.className = 'multi-select';
    
    // Add options
    options.forEach(option => {
      const opt = document.createElement('option');
      opt.value = option.value;
      opt.textContent = option.label;
      opt.selected = currentValues.includes(option.value);
      select.appendChild(opt);
    });
    
    // Handle changes
    select.addEventListener('change', () => {
      const selected = Array.from(select.selectedOptions)
                          .map(opt => opt.value);
      // Multi-value params use comma-separated strings
      window.ReportApp.setParam(parameterName, selected.join(','));
      debouncedRefresh();
    });
    
    container.appendChild(select);
  });
}
```

**Date Range Picker Pattern:**
```javascript
function initDateRangePicker() {
  const startDate = document.getElementById('start-date');
  const endDate = document.getElementById('end-date');
  
  // Set default to last 30 days
  const end = new Date();
  const start = new Date();
  start.setDate(start.getDate() - 30);
  
  startDate.value = start.toISOString().split('T')[0];
  endDate.value = end.toISOString().split('T')[0];
  
  // Apply initial filters
  window.ReportApp.setParam('start_date', startDate.value);
  window.ReportApp.setParam('end_date', endDate.value);
  
  // Listen for changes
  [startDate, endDate].forEach(input => {
    input.addEventListener('change', () => {
      if (startDate.value && endDate.value) {
        window.ReportApp.setParam('start_date', startDate.value);
        window.ReportApp.setParam('end_date', endDate.value);
        debouncedRefresh();
      }
    });
  });
}
```

### 4.3 Error States & Empty Data

**Graceful Empty States:**
```html
<div class="chart-container">
  <canvas id="main-chart"></canvas>
  <div id="no-data-message" class="no-data" style="display: none;">
    <div class="no-data-icon">📊</div>
    <h3>No Data Available</h3>
    <p>Try adjusting your filters or date range.</p>
    <button onclick="resetFilters()" class="btn btn-secondary">
      Reset Filters
    </button>
  </div>
</div>
```

```javascript
function updateChart(data, datasourceName) {
  const ds = data[datasourceName];
  const noDataEl = document.getElementById('no-data-message');
  
  if (!ds || ds.rows.length === 0) {
    // Show empty state
    noDataEl.style.display = 'flex';
    document.getElementById('main-chart').style.display = 'none';
    return;
  }
  
  // Hide empty state, show chart
  noDataEl.style.display = 'none';
  document.getElementById('main-chart').style.display = 'block';
  
  // Update or create chart
  // ... chart rendering logic
}
```

**User-Friendly Error Messages:**
```javascript
function showError(message, details = '') {
  const errorEl = document.getElementById('error-message') || createErrorContainer();
  const detailsEl = errorEl.querySelector('.error-details');
  
  // User-friendly message mapping
  let userMessage = message;
  if (message.includes('HMAC')) {
    userMessage = 'Security validation failed. Please reload the report.';
  } else if (message.includes('SQL')) {
    userMessage = 'Database query failed. Check your filter values.';
  } else if (message.includes('network') || message.includes('fetch')) {
    userMessage = 'Network error. Check your connection and try again.';
  }
  
  errorEl.querySelector('.error-text').textContent = userMessage;
  if (details) {
    detailsEl.textContent = details;
    detailsEl.style.display = 'block';
  } else {
    detailsEl.style.display = 'none';
  }
  
  errorEl.classList.add('show');
  
  // Auto-dismiss after 10 seconds
  setTimeout(() => {
    errorEl.classList.remove('show');
  }, 10000);
}
```

## 5. Performance Optimization

### 5.1 Server-Side Optimization

**Row Limits:**
```yaml
datasources:
  summary_stats:
    sql: |
      SELECT COUNT(*), SUM(amount), AVG(amount)
      FROM orders
      WHERE organization_id = {{organization_id}}
        AND created_at BETWEEN {{start_date}} AND {{end_date}}
    row_limit: 100

  reference_data:
    sql: |
      SELECT id, name FROM categories
      WHERE organization_id = {{organization_id}}
    row_limit: 1000

  real_time_data:
    sql: |
      SELECT * FROM recent_activity
      WHERE organization_id = {{organization_id}}
        AND created_at > NOW() - INTERVAL 1 HOUR
    row_limit: 1000
```

**Batch Operations:**
```sql
-- GOOD: Single query with all needed data
SELECT 
  DATE(created_at) as date,
  category,
  COUNT(*) as count,
  SUM(amount) as total
FROM orders
WHERE organization_id = {{organization_id}}
  AND created_at BETWEEN {{start_date}} AND {{end_date}}
GROUP BY DATE(created_at), category
ORDER BY date, category;

-- BAD: Multiple separate queries
-- (Avoid this pattern in report.yaml)
```

### 5.2 Client-Side Optimization

**Data Transformation Efficiency:**
```javascript
// Efficient data transformation
function processDataSource(data, datasourceName) {
  const ds = data[datasourceName];
  if (!ds) return [];
  
  const { columns, rows } = ds;
  
  // Convert to objects only when needed
  if (needsObjectFormat(columns)) {
    return rows.map(row => {
      const obj = {};
      columns.forEach((col, index) => {
        obj[col] = row[index];
      });
      return obj;
    });
  }
  
  // Return raw arrays for chart libraries that prefer them
  return { columns, rows };
}

// Cache processed data
const dataCache = new Map();

function getCachedData(datasourceName, params) {
  const cacheKey = `${datasourceName}:${JSON.stringify(params)}`;
  if (dataCache.has(cacheKey)) {
    return dataCache.get(cacheKey);
  }
  
  const data = processDataSource(rawData, datasourceName);
  dataCache.set(cacheKey, data);
  
  // Clear cache after 5 minutes
  setTimeout(() => {
    dataCache.delete(cacheKey);
  }, 300000);
  
  return data;
}
```

**Chart Rendering Optimization:**
```javascript
// Debounced chart updates
let chartUpdateTimeout;

function updateChartsDebounced(data) {
  clearTimeout(chartUpdateTimeout);
  chartUpdateTimeout = setTimeout(() => {
    updateAllCharts(data);
  }, 50); // 50ms debounce for smooth updates
}

// Batch DOM updates
function updateAllCharts(data) {
  // Use requestAnimationFrame for smooth animations
  requestAnimationFrame(() => {
    // Batch updates to minimize reflows
    document.body.classList.add('updating-charts');
    
    updateChart('chart1', data.chart1_data);
    updateChart('chart2', data.chart2_data);
    updateChart('chart3', data.chart3_data);
    
    requestAnimationFrame(() => {
      document.body.classList.remove('updating-charts');
    });
  });
}
```

**Memory Management:**
```javascript
// Clean up chart instances
const chartInstances = new Map();

function updateChart(chartId, data) {
  if (chartInstances.has(chartId)) {
    const chart = chartInstances.get(chartId);
    // Update existing chart
    chart.data = data;
    chart.update('none'); // 'none' prevents animation
  } else {
    // Create new chart
    const ctx = document.getElementById(chartId).getContext('2d');
    const chart = new Chart(ctx, { /* config */ });
    chartInstances.set(chartId, chart);
  }
}

// Clean up when leaving page
window.addEventListener('beforeunload', () => {
  chartInstances.forEach(chart => {
    chart.destroy();
  });
  chartInstances.clear();
});
```

### 5.3 Network Optimization

**Payload Size Management:**
```yaml
datasources:
  large_dataset:
    sql: |
      SELECT 
        id,
        name,
        created_at,
        amount
      FROM large_table
      WHERE organization_id = {{organization_id}}
      ORDER BY created_at DESC
      LIMIT 500  -- Limit rows
    row_limit: 500  # Enforce limit
    # Consider pagination for >1000 rows
```

**Sequential Loading:**
```javascript
// Load data sequentially with setTimeout for UX
async function loadDataSequential() {
  // Load primary data first
  const primaryData = await window.ReportApp.refresh({});
  updatePrimaryVisualizations(primaryData);
  
  // Load secondary data after a short delay
  setTimeout(async () => {
    const secondaryData = await window.ReportApp.refresh({});
    updateSecondaryVisualizations(secondaryData);
  }, 100);
}
```

## 6. Development Workflow

### 6.1 Report Template Usage

**Start with the Template:**
```bash
# Use the provided template
cd ~/Go/reporting_app/reports
cp -r report_template my_new_report
cd my_new_report

# Edit report.yaml
vim report.yaml  # Update id, name, description, SQL

# Edit report.html
vim report.html  # Update visualizations
```

**Template Customization Checklist:**
- [ ] **Update `report.yaml`**:
  - Change `id` to match directory name
  - Update `name` and `description`
  - Modify `immutable_params` and `mutable_params`
  - Add `database` to each datasource (optional — falls back to `report.database`)
  - Replace SQL queries with your actual queries
  - Adjust `row_limit` values

- [ ] **Update `report.html`**:
  - Change page title and headers
  - Update filter controls to match your parameters
  - Replace chart configurations with your visualizations
  - Update data processing functions
  - Customize CSS styles if needed

- [ ] **Test thoroughly**:
  - Generate test URL
  - Test with different parameter values
  - Verify empty states work
  - Test on mobile devices

### 6.2 Testing Strategies

**Generate Test URLs:**
```bash
# Basic test URL
 go run main.go -genurl -report my_report \
  -params "organization_id=123,start_date=2024-01-01,end_date=2024-12-31"

# Multi-value parameters (use | to separate key=value pairs)
 go run main.go -genurl -report my_report \
  -params "organization_id=123|status=active,pending|category=1,2,3"

# Empty optional parameters
 go run main.go -genurl -report my_report \
  -params "organization_id=123,start_date=,end_date=,status="
```

**Development Mode:**
```bash
# Set in .env file
ENABLE_PUBLIC_PATHS=true  # Bypass HMAC for testing
ALLOW_ORIGINS=http://localhost:3000  # Allow local dev
ALLOWED_CDNS=https://cdn.jsdelivr.net  # Allow Chart.js CDN

# Run server
go run main.go
```

**Test Scenarios:**
1. **Happy Path**: Normal parameters with valid data
2. **Empty Results**: Parameters that return no data
3. **Edge Cases**: Date ranges, empty strings, NULL values
4. **Multi-Value**: 0, 1, and multiple values
5. **Security**: Attempt to change immutable parameters
6. **Performance**: Large date ranges, many parameters

### 6.3 Debugging Patterns

**Console Debugging:**
```javascript
// Add to report.html for development
window.enableDebug = true;

if (window.enableDebug) {
  console.log('ReportConfig:', window.ReportConfig);
  console.log('ReportApp available:', !!(window.ReportApp));
  
  // Monitor refresh calls
  const originalRefresh = window.ReportApp.refresh;
  window.ReportApp.refresh = async function(...args) {
    console.log('🔄 refresh() called with:', args);
    const startTime = Date.now();
    const result = await originalRefresh.apply(this, args);
    const duration = Date.now() - startTime;
    console.log(`✅ refresh() completed in ${duration}ms`);
    console.log('Data keys:', Object.keys(result));
    return result;
  };
  
  // Monitor custom events (if report emits them)
  window.ReportApp.on('custom-event', (data) => {
    console.log('Custom event:', data);
  });
}
```

**Network Inspection:**
1. **Open Browser DevTools** → Network tab
2. **Filter by `/refresh`** to see API calls
3. **Check request payloads** for parameter format
4. **Verify response structure** matches expectations
5. **Monitor response times** for performance issues

**SQL Debugging:**
```sql
-- Test SQL directly in database
EXPLAIN 
SELECT * FROM orders
WHERE organization_id = 123
  AND created_at BETWEEN '2024-01-01' AND '2024-12-31';

-- Check query performance
SHOW PROFILES;
SHOW PROFILE FOR QUERY 1;
```

### 6.4 Version Control & Collaboration

**Git Structure:**
```bash
reports/
├── my_report/
│   ├── report.yaml      # Tracked
│   ├── report.html   # Tracked
│   ├── custom.js        # Tracked (if separate)
│   └── .gitignore       # Ignore backup files
└── report_template/     # Reference only
```

**.gitignore for Reports:**
```
# Backup files
*.backup
*.backup*
*.bak

# Development files
.env.local
.DS_Store

# Large data files
*.csv
*.xlsx
```

**Commit Messages:**
```
feat(report): Add customer dashboard with multi-select
fix(report): Fix SQL syntax in program_analysis_v2
perf(report): Optimize large_dataset query with indexes
docs(report): Add README for referral_funnel_dashboard
```

**Code Review Checklist:**
- [ ] **Security**: Organization_id filtering in all queries
- [ ] **Performance**: Appropriate limits and indexing
- [ ] **UX**: Loading states, error handling, empty states
- [ ] **Accessibility**: Semantic HTML, keyboard navigation
- [ ] **Responsive**: Works on mobile devices
- [ ] **Testing**: Works with edge cases
- [ ] **Documentation**: Clear comments and structure

## 7. Summary of Key Conventions

### 7.1 Mandatory Conventions (MUST FOLLOW)

1. **Security First**:
   - ALWAYS include `organization_id` as immutable parameter
   - ALWAYS filter by `organization_id` in EVERY SQL query
   - ALWAYS include `organization_id` filter in JOINed tables
   - NEVER expose immutable parameters in UI controls

2. **Performance Essentials**:
   - ALWAYS set `row_limit` in datasource definitions
   - ALWAYS use SQL `LIMIT` for queries that could return large datasets
   - ALWAYS debounce rapid filter changes (300ms)
   - ALWAYS implement loading states and skeleton screens

3. **Error Handling**:
   - ALWAYS check for thick client availability on initialization
   - ALWAYS handle empty data states gracefully
   - ALWAYS provide user-friendly error messages
   - ALWAYS validate date ranges and required parameters

### 7.2 Strongly Recommended Patterns

1. **SQL Excellence**:
   - Use CTEs for complex queries
   - Implement proper NULL handling for optional filters
   - Use window functions for analytical queries
   - Use default value syntax (`{{param:'default'}}`) for fallback values

2. **UI/UX Quality**:
   - Follow the `STYLES.md` color palette
   - Implement responsive design (mobile-first)
   - Show parameter type badges (immutable/mutable)
   - Use consistent spacing and typography

3. **Development Workflow**:
   - Start with `report_template` as base
   - Test with `ENABLE_PUBLIC_PATHS=true` during development
   - Generate test URLs for all parameter combinations
   - Use meaningful commit messages

### 7.3 Efficiency Patterns by Report Type

**Simple Dashboard Reports:**
- 3-5 datasources maximum
- Single-page layout
- Basic Chart.js visualizations
- Standard date range filters

**Complex Analytical Reports:**
- Use CTEs and window functions
- Implement progressive loading
- Multi-tab or drill-down interfaces
- Advanced chart types (radar, polar, stacked)

**Data Exploration Reports:**
- Many optional filters
- Multi-select parameters
- Real-time updates (frequent filter changes)
- Export functionality

### 7.4 Anti-Patterns to Avoid

**❌ Security Anti-Patterns:**
- Missing organization_id filters
- Exposing immutable parameters in UI
- Allowing users to change security boundaries
- Incomplete JOIN filtering

**❌ Performance Anti-Patterns:**
- Unbounded queries (no LIMIT/row_limit)
- N+1 query patterns
- SELECT * in large tables
- No caching for slow-changing data

**❌ UX Anti-Patterns:**
- No loading indicators
- Technical error messages
- No empty states
- Non-responsive layouts

### 7.5 Quick Reference Checklist

**New Report Setup:**
1. [ ] Copy `report_template` directory
2. [ ] Update `report.yaml` (id, name, description)
3. [ ] Define immutable and mutable parameters
4. [ ] Write secure SQL queries with organization_id filters
5. [ ] Set appropriate row_limit values
6. [ ] Customize `report.html` with your visualizations
7. [ ] Test with generated URLs
8. [ ] Verify security, performance, and UX

**Report Maintenance:**
1. [ ] Regularly review query performance
2. [ ] Review query performance and row limits periodically
3. [ ] Test with new edge cases
4. [ ] Monitor user feedback and error rates
5. [ ] Keep dependencies (Chart.js, etc.) updated

---

## Adoption & Compliance

**For New Reports:**
All new reports MUST follow these conventions. The `report_template` directory provides a starting point that implements most required patterns.

**For Existing Reports:**
Existing reports should be gradually updated to comply with these conventions, prioritizing security fixes first, then performance improvements, then UX enhancements.

**Code Reviews:**
Use the code review checklist in Section 6.4 as the standard for all report submissions.

**Training:**
New report developers should:
1. Read `REPORT_DEVELOPMENT.md` for technical details
2. Study `report_template` for implementation patterns
3. Review `program_analysis_v2` as an advanced example
4. Practice with the test URL generator

---

*Document Version: 1.0*  
*Last Updated: 2026-05-11*  
*Based on Reporting App v2.1+ with multi-value parameter support*