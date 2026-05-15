# Report Template

A comprehensive starting point for creating new reports with the thick client architecture.

## Quick Start

1. **Copy the template:**
   ```bash
   cp -r reports/report_template reports/my_new_report
   ```

2. **Update `report.yaml`:**
   - Change `id`, `name`, `description`
   - Configure `immutable_params` and `mutable_params`
   - Replace SQL queries with your actual queries
   - Ensure all queries include `organization_id = {{organization_id}}`

3. **Update `report.html`:**
   - Update report title and description in HTML
   - Replace `Chart.js` with your preferred visualization library
   - Modify JavaScript to process your specific datasources
   - Customize UI layout and styling as needed

4. **Test your report:**
   ```bash
   # Generate a test URL
   go run main.go -genurl -report my_new_report -params "organization_id=1,start_date=2024-01-01,end_date=2024-12-31"
   ```

## Template Features

### 🛡️ **Security-First Design**
- All queries include `organization_id` filtering
- Clear separation of immutable vs mutable parameters
- Proper thick client integration with validation

### 📊 **Production-Ready UI**
- Skeleton loading states
- Smooth CSS transitions
- Responsive design
- Comprehensive error handling
- Debug panel for development

### 🔧 **Developer Experience**
- Well-organized JavaScript modules
- Console utilities for debugging (`window.ReportTemplate`)
- Event handling with debouncing
- State management pattern

### 📱 **Visualization Ready**
- Chart.js integration (easily replaceable)
- Example charts and data table
- No-data states for empty results

## File Structure

```
report_template/
├── report.yaml           # Report definition with SQL queries
├── report.html        # Complete HTML/CSS/JS report
└── README.md            # This file
```

## Key Sections to Customize

### 1. **`report.yaml` - Data Definition**
```yaml
# Update these sections:
id: your_report_id                    # Must match directory name
name: "Your Report Name"              # Display name
description: "Your description"

# Add/remove parameters as needed
immutable_params:
  - organization_id                    # Keep this for security
  
mutable_params:
  - start_date
  - end_date
  # Add your custom mutable parameters

# Replace example datasources with your actual queries
datasources:
  your_datasource_name:
    sql: |
      SELECT * FROM your_table
      WHERE organization_id = {{organization_id}}
        -- Add your filters
```

### 2. **`report.html` - Visualization**

#### CSS Customization:
- Update colors in the `<style>` section
- Modify grid layouts in `.charts-grid`
- Adjust responsive breakpoints

#### JavaScript Customization:
- Update `processData()` function to handle your datasources
- Modify chart creation in `updateTimeSeriesChart()`, `updateCategoryChart()`
- Customize `updateSummaryCards()` for your summary metrics
- Update `updateDataTable()` for your table data

#### Library Replacement:
To use a different chart library (Plotly, D3, etc.):

1. Replace the Chart.js CDN script tag
2. Update chart creation/destruction functions
3. Ensure library is in `ALLOWED_CDNS` environment variable

## Best Practices

### 🎯 **Parameter Design**
- **Immutable parameters:** For security (organization, user, tenant IDs)
- **Mutable parameters:** For user-adjustable filters (dates, statuses, categories)
- **Always include `organization_id`** in WHERE clauses for data isolation

### 🔍 **Query Optimization**
- Use appropriate `row_limit` values
- Consider `cache_ttl` for slow-changing data
- Add indexes on `organization_id` and date columns
- Use `LIMIT` in SQL for large datasets

### 🎨 **UI/UX Considerations**
- Implement loading states for all async operations
- Debounce rapid filter changes (300ms default)
- Provide clear error messages
- Show empty states for no data
- Make controls accessible and intuitive

### 🐛 **Debugging & Testing**
- Use the debug panel (click 🔧 button)
- Access `window.ReportTemplate` utilities in console
- Test with different parameter values
- Verify data isolation with different `organization_id` values

## Common Tasks

### Adding a New Filter Control

1. Add HTML input in `.filters-container`:
   ```html
   <div class="filter-group">
       <label for="status-filter">Status</label>
       <select id="status-filter">
           <option value="">All</option>
           <option value="active">Active</option>
           <option value="inactive">Inactive</option>
       </select>
   </div>
   ```

2. Add to `ReportState.filters` initialization
3. Update `updateFiltersFromUI()` to read the new control
4. Ensure parameter is declared in `report.yaml` `mutable_params`

### Adding a New Chart

1. Add HTML container to `charts-grid`:
   ```html
   <div class="chart-card">
       <div class="chart-header">
           <div class="chart-title">New Chart</div>
           <div class="chart-description">Description here</div>
       </div>
       <div class="chart-container">
           <div class="chart-wrapper">
               <canvas id="new-chart"></canvas>
               <div id="new-chart-no-data" class="no-data">No data</div>
           </div>
       </div>
   </div>
   ```

2. Add chart instance to `ReportState.charts`
3. Create chart update function in `DataManager`
4. Call it from `processData()`

### Customizing Data Processing

Modify the `processData()` function in `DataManager`:

```javascript
processData(data) {
    // Your custom data processing logic
    this.updateYourCustomChart(data);
    this.updateSummaryCards(data);
    this.updateNoDataStates(data);
}
```

## Testing Checklist

- [ ] Report loads with skeleton screens
- [ ] Data loads successfully with default parameters
- [ ] Filters apply correctly with debouncing
- [ ] Charts render with actual data
- [ ] No-data states show when appropriate
- [ ] Error messages display for failed loads
- [ ] Debug panel works and shows correct info
- [ ] Responsive design works on mobile/tablet
- [ ] Date validation works (start ≤ end)
- [ ] Console utilities available (`window.ReportTemplate`)

## Troubleshooting

### "Thick client not available"
- Ensure you're accessing via `/api/embed` URL
- Check browser console for script errors
- Verify `thick_client.js` is loaded (auto-injected by server)

### "Cannot change immutable parameter"
- You're trying to modify a parameter marked as immutable
- Use `window.ReportApp.isImmutable()` to check parameter type
- Only mutable parameters can be changed with `setParam()`

### Charts not rendering
- Check datasource names match between `report.yaml` and JavaScript
- Verify data structure: `{ columns: [], rows: [] }`
- Check browser console for Chart.js errors
- Ensure CDN is in `ALLOWED_CDNS` environment variable

### No data returned
- Verify SQL queries work in database directly
- Check parameter values are being passed correctly
- Ensure `organization_id` filter is in WHERE clause
- Test with `ENABLE_PUBLIC_PATHS=true` for development

## Support & Resources

- **Thick Client Guide:** `THICK_CLIENT_FOR_REPORT_DEVS.md`
- **Security Audit:** `SECURITY_AUDIT.md`
- **Blink/Flicker Discussion:** `BLINK_DISCUSSION.md`
- **Example Reports:** `reports/example_dashboard/`, `reports/customer_*/`

For additional help, contact the reporting app maintainers.

---

*Template Version: 2.0*  
*Last Updated: 2026-05-07*