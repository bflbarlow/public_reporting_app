# Thick Client Design

## Overview

The Thick Client is a lightweight JavaScript layer that runs inside the embedded report iframe. Its sole responsibility is **keeping the report's data fresh** — refreshing from the database, managing signed URLs, and exposing a clean data store to the report layer.

Everything else — rendering, coordination, filtering, export, theming — belongs to the report's own JavaScript and visualization libraries.

---

## Responsibility Split

### Thick Client (Data Layer)

| Responsibility | Details |
|---|---|
| **Data refresh** | POST `/refresh` with current signed URL → receives fresh data + new signed URL |
| **Signed URL exchange** | Validates current URL, extracts report ID + parameters, returns next URL |
| **URL state management** | Tracks `current_url`, `report_name`, `params` — what's currently loaded |
| **Browser history** | `history.replaceState()` to update the iframe URL without reload |
| **Parameter extraction** | Parses URL query params, excludes HMAC fields (`report_id`, `expires`, `nonce`, `sig`) |
| **SQL placeholder substitution** | Extracts filter parameters for backend substitution |
| **Data store** | Holds original data and filtered copies per chart — the single source of truth |
| **Error handling** | Catches refresh failures, surfaces them to the report layer |
| **Loading states** | Optional: signals when data is being fetched (spinner, disabled filters) |

### Report (Visualization Layer)

| Responsibility | Details |
|---|---|
| **Chart rendering** | Each chart's visualization library (Chart.js, D3, ApexCharts, etc.) |
| **Cross-filter coordination** | When chart A filters, propagate that filter to chart B, C, etc. |
| **UI controls** | Filter inputs, buttons, dropdowns, date pickers — whatever the report needs |
| **Drill-down** | Click a bar → show detail view → trigger refresh with new parameters |
| **Tooltips / hover states** | Show data details on mouse interaction |
| **Data transformation** | Aggregation, sorting, grouping — any math before rendering |
| **Export** | CSV, PDF, image — whatever the report needs |
| **Responsive layout** | How charts resize, wrap, stack on different screen sizes |
| **Theming** | Colors, fonts, spacing — the visual identity of the report |
| **Animation** | Transitions between filter states, chart redraws |

---

## The Contract Between Them

```
Thick Client                          Report Layer
─────────────                         ────────────
                                      
  ┌──────────────┐                    ┌──────────────┐
  │  /refresh     │◄────────────────►│  Chart.js /   │
  │  endpoint     │  fresh data +    │  D3 / Apex    │
  │              │   next URL        │  / custom lib │
  └──────┬───────┘                    └───────┬──────┘
         │                                     │
  ┌──────┴───────┐                    ┌───────┴──────┐
  │  DataStore    │                    │  render()    │
  │  (original +  │                    │  refresh()   │
  │   filtered)   │                    │  export()    │
  └──────┬───────┘                    └───────┬──────┘
         │                                     │
         └─── chartData ───────────────────────┘
              (the shared contract)
```

**The contract is simple:** Thick Client gives the Report a `chartData` object. The Report owns everything after that — how it renders, how it coordinates between charts, how it responds to user interaction.

---

## Thick Client API

The thick client exposes a single public object: `window.ReportApp`.

### Methods

#### `refresh(params?: object) → Promise<ChartData>`

Triggers a server refresh. The thick client takes the current signed URL, merges in the optional new params, POSTs to `/refresh`, and returns fresh data plus a new signed URL.

- **params** (optional) — New or changed filter parameters. Merged into the current URL before the POST.
- **returns** — `Promise<ChartData>` — resolves with the fresh data, rejects on failure.
- **side effects** — updates the data store, updates the browser URL via `history.replaceState()`.

```javascript
// No params — refresh with current filters
const data = await window.ReportApp.refresh();

// With params — refresh with new filters
const data = await window.ReportApp.refresh({
  start_date: '2023-01-01',
  end_date: '2023-12-31',
});
```

#### `getData() → ChartData`

Read-only access to the data store. Returns the current original and filtered data for all charts.

```javascript
const data = window.ReportApp.getData();
// {
//   revenue_by_month: { columns: ['month', 'revenue'], rows: [['Jan', 100], ...] },
//   product_sales: { columns: ['product', 'total'], rows: [['Widget A', 50], ...] }
// }
```

#### `setFilteredData(chartId: string, filteredRows: any[][]) → void`

Sets filtered rows for a specific chart. Used for cross-filtering — when one chart's selection filters another chart's data without a server call.

```javascript
// Chart A's selection filters Chart B's data
window.ReportApp.setFilteredData('product_sales', filteredRows);
```

#### `getParam(key: string) → string | null`

Gets a single parameter value from the current URL state.

```javascript
const customerId = window.ReportApp.getParam('customer_id');
```

#### `getParams() → { [key: string]: string }`

Gets all parameters from the current URL state (excluding HMAC fields).

```javascript
const params = window.ReportApp.getParams();
// { start_date: '2023-01-01', end_date: '2023-12-31', customer_id: '103' }
```

#### `setParam(key: string, value: string) → void`

Updates a staged parameter value. Does not trigger a refresh — just updates the local param store. When the report calls `refresh()`, the thick client uses the staged params.

```javascript
window.ReportApp.setParam('start_date', '2024-01-01');
// The URL doesn't change yet. When refresh() is called, it will include start_date=2024-01-01.
```

#### `getCurrentUrl() → string`

Gets the current signed URL (the one currently loaded in the iframe).

```javascript
const url = window.ReportApp.getCurrentUrl();
// '/embed/sales_dashboard?report_id=sales_dashboard&expires=1717200000&nonce=abc123&sig=xyz789'
```

### Events

The thick client emits DOM events for decoupled notification. Multiple listeners can react without tight coupling to the report that triggered the refresh.

| Event | Detail | When |
|---|---|---|
| `report:refresh-start` | `{ chartIds: string[] }` | Before the POST to `/refresh` |
| `report:refresh-complete` | `{ data: ChartData, url: string }` | After data is loaded and store updated |
| `report:refresh-error` | `{ error: Error, url: string }` | After a failed refresh |
| `report:data-ready` | `{ data: ChartData }` | On initial page load, after data is parsed |

```javascript
// Show a spinner when refresh starts
window.addEventListener('report:refresh-start', (e) => {
  showSpinner();
});

// Hide the spinner when refresh completes
window.addEventListener('report:refresh-complete', (e) => {
  hideSpinner();
});

// Handle errors at the page level
window.addEventListener('report:refresh-error', (e) => {
  showToast('Failed to refresh data: ' + e.detail.error.message);
});
```

---

## How the Report Talks to the Thick Client

The thick client is a **dumb pipe**. It doesn't know about SQL placeholders, user intent, or chart semantics. It only:

1. Takes a signed URL
2. POSTs it to `/refresh`
3. Returns data + new URL

The report is the **brain**. It knows the semantics. It decides what to do with the data.

### The Data Flow

```
User action (click, input change, timer)
    ↓
Report decides: "this needs a server refresh with params X, Y"
    ↓
Report calls window.ReportApp.refresh({ X: 'val', Y: 'val' })
    ↓
Thick client:
  1. Merge params into current URL
  2. POST to /refresh
  3. Receive data + next_url
  4. Update data store
  5. Update browser URL
  6. Emit events
  7. Return Promise
    ↓
Report receives data (via Promise or event)
    ↓
Report re-renders charts
```

### Client-Side vs Server-Side Decisions

The report must decide **before** calling refresh whether a change needs a server round-trip:

| Change Type | Needs Refresh? | Why |
|---|---|---|
| SQL param changes (dates, customer IDs, status) | **Yes** | Data must be re-queried from the database |
| Client-side filter (product name, category) | **No** | Data already loaded, just filter in memory |
| Drill-down | **Yes** (usually) | Need detail rows from the database |
| Aggregation (subtotal, percentage) | **No** | Computed from existing data |
| Export (CSV, PDF) | **No** | Uses existing data |

The thick client does **not** make this decision. The report does.

---

## Refresh Trigger Patterns

Every refresh starts from the report. Below are the five common patterns with explicit code.

### Pattern 1: Explicit User Action

User clicks a button, changes a date range, selects a category. The report decides this needs a server refresh and calls `refresh()` with the new params.

```javascript
// The report owns this UI control logic
document.getElementById('refresh-btn').addEventListener('click', () => {
  const startDate = document.getElementById('start-date').value;
  const endDate = document.getElementById('end-date').value;

  // Stage the params (updates local store, not the URL yet)
  window.ReportApp.setParam('start_date', startDate);
  window.ReportApp.setParam('end_date', endDate);

  // Trigger the refresh
  window.ReportApp.refresh()
    .then(data => {
      // Data is now fresh, re-render charts
      renderCharts(data);
    })
    .catch(error => {
      // Report decides how to surface the error
      showToast('Failed to refresh data: ' + error.message);
    });
});
```

### Pattern 2: Cross-Filter Propagation

User clicks a bar in chart A. Chart A's selection changes the filter for chart B. The report decides chart B's filter is a SQL parameter (needs server refresh) and calls `refresh()` with the new params.

```javascript
// In chart A's click handler
document.getElementById('chart-revenue_by_month').addEventListener('click', (e) => {
  const selectedMonth = e.detail.month; // from chart library

  // Check if this month is a SQL param
  if (selectedMonth) {
    // Stage the new param
    window.ReportApp.setParam('month', selectedMonth);

    // Trigger refresh — all charts will update
    window.ReportApp.refresh()
      .then(data => {
        // Re-render all charts with fresh data
        renderCharts(data);
      })
      .catch(error => {
        console.error('Cross-filter refresh failed:', error);
      });
  } else {
    // Client-side filter — no server call needed
    const filteredRows = filterRowsByMonth(data, selectedMonth);
    window.ReportApp.setFilteredData('product_sales', filteredRows);
    renderChart('product_sales', filteredRows);
  }
});
```

### Pattern 3: Drill-Down

User clicks a data point. The report extracts drill-down params and calls `refresh()`.

```javascript
// In chart B's click handler
document.getElementById('chart-product_sales').addEventListener('click', (e) => {
  const selectedProduct = e.detail.product; // from chart library

  if (selectedProduct) {
    // Stage the drill-down param
    window.ReportApp.setParam('product_name', selectedProduct);

    // Trigger refresh to get detail rows
    window.ReportApp.refresh()
      .then(data => {
        // Show detail view in a modal or expand the chart
        showDetailModal(data);
      })
      .catch(error => {
        console.error('Drill-down refresh failed:', error);
      });
  }
});
```

### Pattern 4: Periodic Polling

Dashboard on a wall. The report calls `refresh()` every N seconds.

```javascript
// Polling is the report's responsibility
let pollInterval;

function startPolling(intervalMs = 30000) {
  pollInterval = setInterval(() => {
    window.ReportApp.refresh()
      .then(data => {
        renderCharts(data);
      })
      .catch(error => {
        console.error('Poll refresh failed:', error);
        // Stop polling on persistent failure
        stopPolling();
      });
  }, intervalMs);
}

function stopPolling() {
  if (pollInterval) {
    clearInterval(pollInterval);
    pollInterval = null;
  }
}

// Start polling when the report loads
startPolling(30000); // Every 30 seconds
```

### Pattern 5: No Refresh (Client-Side)

User selects a product from a dropdown. The report checks: is this a SQL param or a client-side filter? If client-side, it **doesn't call refresh at all** — it just filters the data store directly and re-renders.

```javascript
// Client-side filter — no server call
document.getElementById('product-dropdown').addEventListener('change', (e) => {
  const selectedProduct = e.target.value;

  if (selectedProduct === 'all') {
    // Reset to original data
    const originalData = window.ReportApp.getData();
    renderChart('product_sales', originalData.product_sales);
  } else {
    // Filter in memory — no server call
    const data = window.ReportApp.getData();
    const filteredRows = data.product_sales.rows.filter(row => row[0] === selectedProduct);
    window.ReportApp.setFilteredData('product_sales', filteredRows);
    renderChart('product_sales', filteredRows);
  }
});
```

---

## Why This Split Works

| Aspect | Thick Client | Report |
|---|---|---|
| **Why** | Security + data freshness | Visualization + UX |
| **Who writes it** | Platform team | Report developer |
| **Changes often?** | Rarely | Always |
| **Complexity** | Low (data layer) | High (visualization) |
| **Shared?** | Yes (same for all reports) | No (unique per report) |

The Thick Client is **infrastructure** — boring, reliable, secure. The Report is **art** — where the value lives. Separating them means the platform team can harden security without touching visualization code, and report developers can experiment with any charting library without worrying about HMAC or nonce management.

---

## Implementation Notes

### Data Store Structure

```javascript
{
  [chartId]: {
    columns: ['col1', 'col2', 'col3'],
    rows: [['val1', 'val2', 'val3'], ...],
    filteredRows: [['val1', 'val2', 'val3'], ...] // optional, set by report
  }
}
```

### Error Handling

```javascript
window.ReportApp.refresh().catch(error => {
  // Report decides how to surface the error:
  // show toast, disable filters, log, etc.
  console.error('Refresh failed:', error);
});
```

---

## What This Does NOT Include

The Thick Client does **not** handle:

- Chart rendering or visualization
- Cross-filter coordination between charts
- UI controls (inputs, buttons, dropdowns)
- Drill-down logic
- Tooltips or hover states
- Data transformation (aggregation, sorting, grouping)
- Export functionality
- Responsive layout or theming
- Animation or transitions
- Any business logic specific to a report

All of these belong to the report's JavaScript, which is loaded alongside the Thick Client and has full access to the Thick Client's data store and refresh capability.

---

## File Structure

```
reports/
└── my_dashboard/
    ├── report.yaml      # Report definition (datasources, params)
    ├── report.html   # HTML layout
    └── report.js        # Report's own JS (rendering, coordination, UI)
```

The Thick Client is loaded automatically by the base template:

```html
<script src="/static/thick_client.js"></script>
```

Each report can add its own JS:

```html
<script src="report.js"></script>
<script src="/static/thick_client.js"></script>
```

---

## Script Load Order — Critical

**The thick client script must load *after* the report's JavaScript.**

The thick client emits `report:data-ready` during its initialization, which runs when the DOM is ready. If the thick client loads before the report's `report.js`, the event fires before the report's listener is attached, and the event is lost — charts will never render.

```html
<!-- WRONG: thick client first, event fires before listener is attached -->
<script src="/static/thick_client.js"></script>
<script src="report.js"></script>

<!-- CORRECT: report first, thick client last -->
<script src="report.js"></script>
<script src="/static/thick_client.js"></script>
```

**Why this order?** The thick client's `init()` dispatches `report:data-ready` synchronously. The report's `report.js` must already be loaded and have its `addEventListener('report:data-ready', ...)` call executed before that dispatch happens. Loading thick client last guarantees the listener is attached before the event fires.

---

## Parameter Classification & Security

The thick client works with a **two-class parameter system** that distinguishes between immutable and mutable parameters:

### Immutable Parameters (Security-Critical)
- **Purpose**: Protect identity, access control, and authorization boundaries (e.g., `customer_id`, `user_id`, `region`)
- **HMAC Protection**: Included in the cryptographic signature of signed URLs
- **Security Guarantee**: Cannot be changed after URL signing — any attempt triggers HMAC validation failure
- **Report Declaration**: Must be listed in `report.yaml` under `immutable_params`
- **Example**: `customer_id=123` ensures users only see their own data

### Mutable Parameters (User-Interactive)
- **Purpose**: Enable interactive filtering and exploration (e.g., `start_date`, `end_date`, `status`, `product_category`)
- **HMAC Exclusion**: Not included in the cryptographic signature
- **User Freedom**: Can be freely changed via `ReportApp.setParam()` without breaking HMAC validation
- **Report Declaration**: Must be listed in `report.yaml` under `mutable_params`
- **Example**: `start_date=2023-01-01` can be changed to `start_date=2024-01-01` by user

### How the Thick Client Handles Parameter Classes

1. **Initialization**: Extracts all parameters from signed URL, excluding HMAC fields (`report_id`, `expires`, `nonce`, `sig`)
2. **Storage**: Stores both immutable and mutable parameters in the data store
3. **Refresh Logic**:
   - **Mutable changes**: When `ReportApp.refresh()` is called with new mutable parameters, the thick client merges them into the current URL and POSTs to `/refresh`
   - **Immutable protection**: Any attempt to change immutable parameters via `setParam()` will cause HMAC validation failure on next refresh
4. **Security Boundary**: The thick client **does not** validate parameter classification — this is handled server-side by HMAC middleware

### Report Development with Parameter Classification

**When writing `report.js`:**
1. **Check parameter type** before allowing user changes:
   ```javascript
   const mutableParams = window.ReportConfig.mutable_params || [];
   const immutableParams = window.ReportConfig.immutable_params || [];
   
   if (mutableParams.includes('start_date')) {
     // Safe to let user change this
     enableDatePicker();
   }
   ```

2. **Use `ReportApp.setParam()` appropriately**:
   ```javascript
   // Safe: Changing mutable parameter
   ReportApp.setParam('start_date', newDate);
   
   // Dangerous: Changing immutable parameter (will break on refresh)
   // ReportApp.setParam('customer_id', '456'); // DON'T DO THIS
   ```

3. **Design UI accordingly**:
   - **Mutable parameters**: Show interactive controls (date pickers, dropdowns, checkboxes)
   - **Immutable parameters**: Display as read-only labels (for context)

**Example report.yaml**:
```yaml
id: customer_dashboard
immutable_params:  # Security boundaries
  - customer_id
  - user_role
mutable_params:    # Interactive filters
  - start_date
  - end_date
  - status
  - product_category
```

### Why This Matters for Thick Client Design

1. **Security**: Immutable parameters establish data access boundaries that cannot be bypassed via client-side JavaScript
2. **User Experience**: Mutable parameters enable rich interactivity without constant HMAC regeneration
3. **Performance**: Only changed mutable parameters are sent to `/refresh`, reducing payload size
4. **Simplicity**: Report developers can focus on UX without worrying about HMAC mechanics for filter changes

**Key Principle**: The thick client enables interactive filtering **within** the security boundaries established by immutable parameters, not across them.

---

## Datasource-Based Reporting Model

With the introduction of the datasource-based reporting model, the thick client's role evolves to work alongside a new client API system optimized for JavaScript-first development:

### New Architecture for Datasource-Based Reports

**Traditional Chart-Based Model**:
```
Thick Client → Server → Database → Raw Data → Chart Rendering
```

**New Datasource-Based Model**:
```
Client API → Server → Database → Structured Data → Developer JavaScript → Custom Visualization
```

### Key Differences in Datasource Mode

1. **API-First Design**: The thick client is supplemented by a structured client API (`window.__reportData`)
2. **Datasource Abstraction**: SQL queries are defined in manifests, exposed as named datasources
3. **JavaScript-First**: Report developers write pure JavaScript for visualization and interactivity
4. **Flexible Rendering**: Any charting library can be used (React, D3, Plotly, etc.)

### Datasource Client API

For datasource-based reports, the server injects `window.__reportData` with:

```javascript
window.__reportData = {
  reportID: 'sales_dashboard',
  datasources: {
    monthly_sales: {
      getRows: function(params) { /* fetch from server */ },
      getColumns: function() { /* fetch metadata */ },
      // Inline data also available on page load
      data: {
        columns: ['month', 'revenue'],
        rows: [['2024-01', 10000], ...]
      }
    }
  },
  parameters: {
    schema: { /* parameter definitions */ },
    current: { /* current values */ },
    immutable: ['customer_id'],
    mutable: ['start_date', 'end_date']
  }
}
```

### Thick Client Integration with Datasource Model

**For datasource-based reports, developers have two options**:

1. **Use Client API Only**: Developers access data via `window.__reportData.datasources.{name}.getRows()`
2. **Use Thick Client Hybrid**: Developers can still use `window.ReportApp` for URL state management, but fetch data via datasource API

**Parameter Classification Still Applies**: The same immutable/mutable parameter classification system works for both models, ensuring security boundaries are maintained.

### Example: Datasource-Based Report with Client API

```javascript
// Developer's report.js for datasource-based report
window.addEventListener('DOMContentLoaded', function() {
  // Access datasource data
  const monthlySales = window.__reportData.datasources.monthly_sales;
  
  // Get columns for metadata
  const columns = monthlySales.data.columns;
  
  // Render with custom visualization library
  renderCustomChart({
    columns: columns,
    rows: monthlySales.data.rows,
    container: '#chart-container'
  });
  
  // Handle parameter changes
  document.getElementById('date-range').addEventListener('change', function(e) {
    // Fetch fresh data with new parameters
    monthlySales.getRows({ start_date: e.target.value })
      .then(data => {
        // Update visualization with fresh data
        updateChart(data);
      });
  });
});
```

### Migration Path

Existing chart-based reports continue to work unchanged. New reports can adopt the datasource model for greater flexibility:

| Task | Chart-Based | Datasource-Based |
|------|-------------|------------------|
| Define SQL queries | In `report.yaml` per datasource | In `manifest.yaml` as datasources |
| Render visualizations | Built-in or simple Chart.js | Any JavaScript library |
| Handle interactivity | Via thick client API | Via client API or custom JS |
| Coordinate charts | Limited cross-filtering | Full custom coordination |
| Update data | `ReportApp.refresh()` | `datasource.getRows(params)` |

### Choosing the Right Model

**Use Chart-Based When**:
- You need simple dashboards quickly
- SQL-centric team with limited JavaScript expertise
- Standard chart types are sufficient
- Legacy compatibility is required

**Use Datasource-Based When**:
- Complex interactive visualizations needed
- JavaScript/React expertise available
- Custom charting libraries required (D3, Plotly, etc.)
- Advanced coordination between visualizations
- Want hot reload during development (manifests support file watching)

### Configuration

To enable datasource-based reports:
1. Set `MANIFESTS_DIR=./manifests` environment variable
2. Create `manifests/{report}.yaml` files
3. Enable hot reload: `ENABLE_HOT_RELOAD=true` (optional)

The system automatically detects and routes requests to the appropriate handler based on whether a manifest exists for the report ID.
