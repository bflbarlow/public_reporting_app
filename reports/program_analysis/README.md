# Program Analysis Dashboard

Advanced Chart.js visualizations for program performance and referral analysis using the customer_sql database.

## Overview

This report showcases four advanced Chart.js chart types with premium styling and interactive features designed to deliver a "WOW!" user experience.

## Features

### 📊 **Chart Visualizations**
1. **Radar Chart with Simulated Skip Points** - Program performance metrics across categories
2. **Polar Area Chart with Enhanced Tooltips** - Referral status distribution (centered labels via tooltips)
3. **Line Chart with Multiple Datasets** - Referral trends over time on a single scale
4. **True Stacked Line Chart** - Cumulative daily referral volume by top program categories

### 🎨 **Visual Design**
- **Gradient header** with purple-to-pink gradient
- **Dark/light theme toggle** with localStorage persistence
- **CSS variable-based theming** across all components
- **Progress bars** in summary cards (completion rate)
- **Animated transitions** and hover effects
- **Responsive design** for all screen sizes

### ⚡ **Interactive Features**
- **Live timestamp** showing last update time
- **Pulse animation** on Apply Filters button
- **Debounced filter updates** (300ms)
- **Smooth chart animations** (easeOutQuart, 1500-2000ms duration)
- **Theme-aware chart updates** (charts refresh on theme change)
- **Print/PDF export** with optimized formatting (🖨️ button in header)

### 🛡️ **Technical Foundation**
- **Thick client integration** with proper security patterns
- **Error handling** with user-friendly messages
- **Loading states** with skeleton screens
- **Debug panel** for development (🔧 button)
- **Performance optimized** with GPU-accelerated animations

## Database Schema

Uses the `customer_sql` database with focus on:
- `referrals` table (primary analysis)
- `programs` table (category mapping)
- `referral_status` table (status history)
- `seeker_profiles` table (seeker information)

See [DATABASE_DETAILS.md](DATABASE_DETAILS.md) for complete schema documentation.

## Parameters

### Immutable Parameters (Security)
- `organization_id` - Automatically included via thick client

### Mutable Parameters (User-adjustable)
- `start_date` - Start date for date range filters
- `end_date` - End date for date range filters  
- `status_filter` - Optional: Filter by referral status
- `category_filter` - Optional: Filter by program category

## Datasources

Five SQL queries defined in [report.yaml](report.yaml):

1. **radar_metrics** - Program category metrics for radar chart
2. **status_distribution** - Referral status counts for polar area chart
3. **time_series_metrics** - Daily referral metrics for line chart
4. **category_time_series** - Daily counts by category for stacked line chart
5. **summary_stats** - Summary statistics for dashboard cards

## Setup & Testing

### Generate Test URL
```bash
go run main.go -genurl -report program_analysis -params "organization_id=1,start_date=2024-01-01,end_date=2024-12-31"
```

### Development Mode
Set `ENABLE_PUBLIC_PATHS=true` in `.env` to bypass HMAC for testing.

## Implementation Notes

### Chart.js Version
Uses Chart.js 4.x from CDN with custom configurations for each chart type.

### Theme System
CSS variables defined in `:root` and `body.dark-theme` scopes allow dynamic theming without JavaScript color manipulation.

### Animation Strategy
- **CSS transitions** for UI elements (300ms)
- **Chart.js animations** for data visualizations (1500-2000ms)
- **Progress bar animations** with delay for visual appeal

### Backend Feature Requests
For complete implementation of requested features, see [CHANGES_SUMMARY.md](CHANGES_SUMMARY.md#backend-support-required-thick-clientreporting-app).

## Browser Support
- Modern browsers with CSS Custom Properties support
- ES6+ JavaScript compatibility required
- Canvas API for Chart.js

## Performance Considerations
- Chart animations disabled during theme changes (`update('none')`)
- Debounced filter updates prevent excessive API calls
- LocalStorage used minimally (theme preference only)
- Efficient chart destruction/recreation patterns

## Troubleshooting

### Common Issues
1. **"Thick client not available"** - Access via `/api/embed` URL
2. **Charts not rendering** - Check browser console for Chart.js errors
3. **No data returned** - Verify SQL queries and parameter values
4. **Theme not persisting** - Clear browser localStorage and retry

### Debugging Tools
- Click the 🔧 button to open debug panel
- Access `window.ReportTemplate` utilities in console
- Check network tab for `/refresh` API calls

## Enhancement History

See [CHANGES_SUMMARY.md](CHANGES_SUMMARY.md) for detailed documentation of enhancements made to deliver the "WOW!" factor.

---

*Report Version: 2.2*  
*Last Updated: 2026-05-09*