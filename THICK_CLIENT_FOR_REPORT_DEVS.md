# Thick Client Guide for Report Developers

## Overview

The **Thick Client** (`window.ReportApp`) is the **sole data bridge** between your report and the reporting application. It handles:
- Data fetching with security validation
- Parameter management (immutable vs. mutable)
- State tracking across refreshes
- Error handling and retry logic

**Golden Rule:** ALL data must flow through `window.ReportApp`. Never make direct API calls or attempt to bypass this layer.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                     Report HTML/JS/CSS                       │
│  ┌─────────────────────────────────────────────────────┐    │
│  │           Your Visualization Code                   │    │
│  │    (Chart.js, Plotly, D3, vanilla JS, etc.)         │    │
│  └─────────────────────────────────────────────────────┘    │
│                    │                                         │
│                    ▼                                         │
│  ┌─────────────────────────────────────────────────────┐    │
│  │           Thick Client (window.ReportApp)           │    │
│  │                                                     │    │
│  │  • refresh() - Fetch new data                      │    │
│  │  • getParam()/setParam() - Parameter management    │    │
│  │  • isImmutable()/isMutable() - Parameter checks    │    │
│  │  • on()/emit() - Event system                      │    │
│  └─────────────────────────────────────────────────────┘    │
│                    │                                         │
│                    ▼                                         │
│  ┌─────────────────────────────────────────────────────┐    │
│  │            Reporting App Backend                     │    │
│  │  • HMAC validation                                   │    │
│  │  • Database queries                                  │    │
│  │  • Result caching                                    │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

## Initialization & Availability Check

### Step 1: Wait for DOM and Thick Client

```javascript
document.addEventListener('DOMContentLoaded', async function() {
    // ALWAYS check if thick client is available
    if (!window.ReportApp || typeof window.ReportApp.refresh !== 'function') {
        console.error('Thick client not available');
        showError('Thick client not loaded. Ensure you are accessing through the reporting app.');
        return;
    }
    
    // Optional: Wait for ready event
    window.ReportApp.on('ready', function(eventData) {
        console.log('Thick client ready for report:', eventData.reportId);
        initializeDashboard();
    });
    
    // If already ready, initialize immediately
    if (window.ReportApp.getParam) {
        initializeDashboard();
    }
});
```

### Step 2: Access Report Configuration

The server injects `window.ReportConfig` with:

```javascript
// Structure of window.ReportConfig
{
    "reportId": "customer_dashboard",
    "reportName": "Customer Dashboard",
    "params": "{\"organization_id\":\"123\",\"start_date\":\"2024-01-01\",\"end_date\":\"2024-12-31\"}", // JSON string!
    "immutableParams": ["organization_id"],
    "mutableParams": ["start_date", "end_date", "status"],
    "datasources": "{\"sales_summary\":{\"SQL\":\"SELECT ...\",\"RowLimit\":100,\"CacheTTL\":300}}",
    "currentUrl": "/api/embed?report_id=customer_dashboard&..."
}
```

**Important:** `params` and `datasources` are JSON strings that need parsing:

```javascript
function parseReportConfig() {
    const config = window.ReportConfig || {};
    
    // Parse params (it's a JSON string!)
    let params = {};
    if (config.params) {
        try {
            params = JSON.parse(config.params);
        } catch (e) {
            console.error('Failed to parse params:', e);
        }
    }
    
    // Parse datasources (optional, mostly for debugging)
    let datasources = {};
    if (config.datasources) {
        try {
            datasources = JSON.parse(config.datasources);
        } catch (e) {
            console.error('Failed to parse datasources:', e);
        }
    }
    
    return { params, datasources, config };
}
```

## Parameter Management

### Understanding Parameter Types

| Type | Description | Can be changed? | In HMAC signature? |
|------|-------------|-----------------|-------------------|
| **Immutable** | Core security parameters (e.g., organization_id) | ❌ Never | ✅ Yes |
| **Mutable** | User-changeable filters (e.g., dates, status) | ✅ Yes | ❌ No |

### Working with Parameters

```javascript
// Get a single parameter
const orgId = window.ReportApp.getParam('organization_id');
const startDate = window.ReportApp.getParam('start_date');

// Get all current parameters
const allParams = window.ReportApp.getParams();
console.log('Current params:', allParams);

// Check parameter type
if (window.ReportApp.isImmutable('organization_id')) {
    console.log('organization_id is immutable - cannot be changed');
}

if (window.ReportApp.isMutable('start_date')) {
    console.log('start_date is mutable - can be updated');
}

// Update a mutable parameter
try {
    window.ReportApp.setParam('start_date', '2024-02-01');
    console.log('Parameter updated successfully');
} catch (error) {
    console.error('Failed to update parameter:', error.message);
    // Common errors:
    // - "Cannot change immutable parameter: organization_id"
    // - "Unknown parameter: invalid_param_name"
}
```

## Fetching Data with `refresh()`

### Basic Refresh Pattern

```javascript
async function fetchData(newParams = {}) {
    showLoadingState();
    
    try {
        // Call thick client refresh
        const data = await window.ReportApp.refresh(newParams);
        
        // Process and display data
        updateVisualizations(data);
        
        console.log('Data refreshed successfully');
        return data;
        
    } catch (error) {
        console.error('Refresh failed:', error);
        showError(`Failed to load data: ${error.message}`);
        throw error;
        
    } finally {
        hideLoadingState();
    }
}
```

### Understanding the Response Format

```javascript
// Data structure returned by refresh()
const data = {
    // Each datasource from report.yaml
    "sales_summary": {
        "columns": ["total_orders", "total_revenue"],
        "rows": [[150, 45000.75]]
    },
    "daily_sales": {
        "columns": ["day", "orders", "revenue"],
        "rows": [
            ["2024-01-01", 10, 3000],
            ["2024-01-02", 12, 3600]
        ]
    }
};

// Convert to more usable format
function processDataSource(data, datasourceName) {
    const ds = data[datasourceName];
    if (!ds) return [];
    
    const { columns, rows } = ds;
    
    // Convert array rows to objects (optional but recommended)
    const rowsAsObjects = rows.map(row => {
        const obj = {};
        columns.forEach((col, index) => {
            obj[col] = row[index];
        });
        return obj;
    });
    
    return rowsAsObjects;
}

// Usage
const salesData = processDataSource(data, 'sales_summary');
salesData.forEach(row => {
    console.log(`Orders: ${row.total_orders}, Revenue: ${row.total_revenue}`);
});
```

### Advanced Refresh Patterns

#### 1. **Debounced Refresh** (for interactive filters)
```javascript
let refreshTimeout = null;

function onFilterChange(newParams) {
    // Clear any pending refresh
    if (refreshTimeout) {
        clearTimeout(refreshTimeout);
    }
    
    // Debounce refresh by 300ms
    refreshTimeout = setTimeout(() => {
        fetchData(newParams);
    }, 300);
}
```

#### 2. **Partial Parameter Updates**
```javascript
async function updateDateRange(startDate, endDate) {
    // Only update the parameters we're changing
    const newParams = {
        start_date: startDate,
        end_date: endDate
    };
    
    // Keep all other parameters as-is
    return await fetchData(newParams);
}
```

#### 3. **Validation Before Refresh**
```javascript
async function refreshWithValidation(newParams) {
    // Validate parameters before sending
    const currentParams = window.ReportApp.getParams();
    const finalParams = { ...currentParams, ...newParams };
    
    // Check for empty required parameters
    if (!finalParams.start_date || !finalParams.end_date) {
        showError('Start date and end date are required');
        return;
    }
    
    // Ensure we're not trying to change immutable params
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

## UI Patterns & Best Practices

### Loading States

```javascript
// CSS for loading states
const css = `
/* Fade transition for smooth updates */
.report-component {
    transition: opacity 0.2s ease;
}
.report-component.loading {
    opacity: 0.5;
    pointer-events: none;
}

/* Skeleton screen */
.skeleton {
    background: linear-gradient(90deg, #f0f0f0 25%, #e0e0e0 50%, #f0f0f0 75%);
    background-size: 200% 100%;
    animation: shimmer 1.5s infinite;
    border-radius: 4px;
}
@keyframes shimmer {
    from { background-position: -200% 0; }
    to { background-position: 200% 0; }
}
`;

// Show/hide loading state
function showLoadingState() {
    document.body.classList.add('loading');
    // Or show specific skeleton screens
    document.getElementById('chart-container').innerHTML = `
        <div class="skeleton" style="height: 400px;"></div>
    `;
}

function hideLoadingState() {
    document.body.classList.remove('loading');
}
```

### Error Handling

```javascript
function showError(message, isFatal = false) {
    const errorDiv = document.createElement('div');
    errorDiv.className = 'error-message';
    errorDiv.innerHTML = `
        <strong>Error:</strong> ${message}
        ${!isFatal ? '<button onclick="this.parentElement.remove()">Dismiss</button>' : ''}
    `;
    
    if (isFatal) {
        // Replace entire content for fatal errors
        document.body.innerHTML = '';
        document.body.appendChild(errorDiv);
    } else {
        // Show as notification
        errorDiv.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            background: #ffeaa7;
            border-left: 4px solid #e17055;
            padding: 12px 16px;
            border-radius: 4px;
            z-index: 10000;
            max-width: 400px;
        `;
        document.body.appendChild(errorDiv);
        
        // Auto-dismiss after 5 seconds
        setTimeout(() => errorDiv.remove(), 5000);
    }
}
```

### Data Transformation Helpers

```javascript
// Helper: Convert thick client data to Chart.js format
function prepareChartJSData(data, datasourceName, labelColumn, dataColumns) {
    const ds = data[datasourceName];
    if (!ds) return null;
    
    const { columns, rows } = ds;
    
    // Find column indices
    const labelIndex = columns.indexOf(labelColumn);
    const dataIndices = dataColumns.map(col => columns.indexOf(col));
    
    // Extract labels and datasets
    const labels = rows.map(row => row[labelIndex]);
    const datasets = dataColumns.map((col, i) => ({
        label: col,
        data: rows.map(row => row[dataIndices[i]])
    }));
    
    return { labels, datasets };
}

// Helper: Convert to Plotly format
function preparePlotlyData(data, datasourceName, xColumn, yColumns) {
    const ds = data[datasourceName];
    if (!ds) return [];
    
    const { columns, rows } = ds;
    const xIndex = columns.indexOf(xColumn);
    
    return yColumns.map(yCol => {
        const yIndex = columns.indexOf(yCol);
        return {
            x: rows.map(row => row[xIndex]),
            y: rows.map(row => row[yIndex]),
            type: 'scatter',
            name: yCol
        };
    });
}
```

## Complete Example Template

```html
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Report Template</title>
    <style>
        /* Styles from above */
    </style>
    <!-- Load thick client (automatically injected by server) -->
    <!-- <script src="/static/thick_client.js"></script> -->
    <!-- Load your chart library -->
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
</head>
<body>
    <div class="dashboard">
        <div class="header">
            <h1 id="report-title">Report</h1>
            <div class="controls">
                <div class="control-group">
                    <label for="start-date">Start Date</label>
                    <input type="date" id="start-date">
                </div>
                <div class="control-group">
                    <label for="end-date">End Date</label>
                    <input type="date" id="end-date">
                </div>
                <button id="refresh-btn">Refresh</button>
            </div>
        </div>
        
        <div class="content">
            <div class="viz-container">
                <canvas id="main-chart"></canvas>
            </div>
            <div id="error-message" class="error" style="display: none;"></div>
        </div>
    </div>
    
    <script>
        // Global state
        let chartInstance = null;
        
        // DOM Ready
        document.addEventListener('DOMContentLoaded', async function() {
            // 1. Check thick client availability
            if (!window.ReportApp || typeof window.ReportApp.refresh !== 'function') {
                showError('Thick client not available. Access through reporting app.', true);
                return;
            }
            
            // 2. Parse configuration
            const config = parseReportConfig();
            document.getElementById('report-title').textContent = config.config.reportName;
            
            // 3. Setup UI controls with current values
            setupControls(config.params);
            
            // 4. Load initial data
            await loadInitialData();
            
            // 5. Setup event listeners
            document.getElementById('refresh-btn').addEventListener('click', onRefresh);
            document.getElementById('start-date').addEventListener('change', onFilterChange);
            document.getElementById('end-date').addEventListener('change', onFilterChange);
            
            console.log('Dashboard initialized');
        });
        
        function parseReportConfig() {
            const config = window.ReportConfig || {};
            let params = {};
            
            if (config.params) {
                try {
                    params = JSON.parse(config.params);
                } catch (e) {
                    console.error('Failed to parse params:', e);
                }
            }
            
            return { params, config };
        }
        
        function setupControls(params) {
            // Pre-fill controls with current parameter values
            if (params.start_date) {
                document.getElementById('start-date').value = params.start_date;
            }
            if (params.end_date) {
                document.getElementById('end-date').value = params.end_date;
            }
        }
        
        async function loadInitialData() {
            showLoading();
            
            try {
                // Get current parameters from thick client
                const currentParams = window.ReportApp.getParams();
                
                // Fetch data
                const data = await window.ReportApp.refresh({});
                
                // Update visualizations
                updateChart(data);
                
            } catch (error) {
                showError(`Failed to load data: ${error.message}`);
            } finally {
                hideLoading();
            }
        }
        
        async function onRefresh() {
            const newParams = collectParamsFromUI();
            await fetchData(newParams);
        }
        
        function onFilterChange() {
            // Debounce filter changes
            if (window.filterTimeout) clearTimeout(window.filterTimeout);
            window.filterTimeout = setTimeout(onRefresh, 300);
        }
        
        function collectParamsFromUI() {
            return {
                start_date: document.getElementById('start-date').value,
                end_date: document.getElementById('end-date').value
            };
        }
        
        async function fetchData(newParams) {
            showLoading();
            
            try {
                const data = await window.ReportApp.refresh(newParams);
                updateChart(data);
                
            } catch (error) {
                showError(`Refresh failed: ${error.message}`);
                throw error;
                
            } finally {
                hideLoading();
            }
        }
        
        function updateChart(data) {
            // Process data for your chart library
            const chartData = prepareChartJSData(data, 'sales_summary', 'day', ['orders', 'revenue']);
            
            if (!chartData) {
                showError('No data available');
                return;
            }
            
            // Update or create chart
            const ctx = document.getElementById('main-chart').getContext('2d');
            
            if (chartInstance) {
                chartInstance.data.labels = chartData.labels;
                chartInstance.data.datasets = chartData.datasets;
                chartInstance.update();
            } else {
                chartInstance = new Chart(ctx, {
                    type: 'line',
                    data: {
                        labels: chartData.labels,
                        datasets: chartData.datasets
                    },
                    options: { responsive: true }
                });
            }
        }
        
        function showError(message) {
            const el = document.getElementById('error-message');
            el.textContent = message;
            el.style.display = 'block';
            
            setTimeout(() => {
                el.style.display = 'none';
            }, 5000);
        }
        
        function showLoading() {
            document.body.classList.add('loading');
        }
        
        function hideLoading() {
            document.body.classList.remove('loading');
        }
    </script>
</body>
</html>
```

## Common Issues & Solutions

### Issue 1: "Thick client not available"
**Cause:** Directly opening the HTML file instead of through the reporting app.
**Solution:** Always access via `/api/embed?...` URL.

### Issue 2: "Cannot change immutable parameter"
**Cause:** Trying to modify a parameter marked as immutable in `report.yaml`.
**Solution:** Only use `setParam()` for mutable parameters. Use `isImmutable()` to check first.

### Issue 3: "params is a string, not an object"
**Cause:** Forgetting to parse `window.ReportConfig.params`.
**Solution:** Always parse: `JSON.parse(window.ReportConfig.params)`.

### Issue 4: Data format confusion
**Cause:** Expecting different data structure.
**Solution:** Data is always `{ datasourceName: { columns: [], rows: [] } }`.

### Issue 5: "next_url" not being used
**Cause:** Not updating internal state after refresh.
**Solution:** The thick client handles this automatically. Your `currentUrl` in `ReportConfig` will be updated.

### Issue 6: Blink/flicker during refresh
**Solution:** Implement CSS transitions and skeleton screens (see BLINK_DISCUSSION.md).

## Security Considerations

1. **Never modify immutable parameters** - this violates HMAC security.
2. **Validate all user inputs** before passing to `refresh()`.
3. **Sanitize data before rendering** to prevent XSS.
4. **Use HTTPS** in production for all CDN resources.
5. **Respect CSP** - only load scripts from allowed CDNs.

## Testing Your Report

### 1. Generate a test URL:
```bash
go run main.go -genurl -report your_report_id -params "organization_id=123,start_date=2024-01-01"
```

### 2. Development mode:
Set `ENABLE_PUBLIC_PATHS=true` in `.env` to bypass HMAC for testing.

### 3. Console debugging:
```javascript
// Check what's available
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
```

## Migration from Legacy Patterns

### Old Pattern (Deprecated):
```javascript
// Direct API calls - DON'T DO THIS
fetch('/some/api').then(...);

// Using window.__reportData - DON'T DO THIS
const data = window.__reportData.datasources.sales_summary.data;
```

### New Pattern (Correct):
```javascript
// Use thick client - DO THIS
const data = await window.ReportApp.refresh(params);
const salesData = data.sales_summary;
```

## FAQs

### Q: Can I use multiple chart libraries in one report?
**A:** Yes, but declare all required CDNs in `ALLOWED_CDNS` environment variable.

### Q: How do I handle very large datasets?
**A:** Use `row_limit` in your datasource definition and implement pagination in your UI.

### Q: Can I cache data client-side?
**A:** Yes, but be careful not to cache across different organizations. Use `organization_id` as part of cache key.

### Q: How do I implement real-time updates?
**A:** Use setInterval to periodically call `refresh()`, or implement WebSocket/SSE in your report (advanced).

### Q: Can I have multiple independent refresh buttons?
**A:** Yes, each can call `refresh()` with different parameters.

---

## Support & Troubleshooting

If you encounter issues:

1. **Check browser console** for JavaScript errors.
2. **Verify thick client is loaded** (`console.log(window.ReportApp)`).
3. **Check network tab** for failed requests to `/refresh`.
4. **Validate your `report.yaml`** syntax and parameter definitions.
5. **Test with minimal example** to isolate the issue.

For persistent issues, contact the reporting app maintainers with:
- Report ID
- Error message from console
- Steps to reproduce
- Browser/OS information

---

*Last Updated: 2026-05-07*  
*Document Version: 2.0*