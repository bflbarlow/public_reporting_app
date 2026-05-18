# Classic Models - Automotive Gallery

## Overview

A luxury museum-style editorial experience transforming the ClassicModels database into a curated automotive collection. This report abandons the typical dashboard aesthetic in favor of a warm, gallery-like presentation with gold/cream/charcoal tones.

## Design Philosophy

- **Museum gallery** over **dashboard** — products displayed as collectible items
- **Warm gold palette** over **neon cyberpunk** — sophisticated, timeless
- **Editorial typography** (Playfair Display + Inter) — refined and readable
- **Data density** — every visualization tells a story, not just displays numbers

## Visualizations

| Section | Type | Data Source |
|---------|------|-------------|
| Revenue Waterfall | Grouped bar chart | revenue_waterfall |
| Sales Velocity | Dual-axis line chart | sales_velocity |
| Profitability Scatter | Bubble chart (cost vs MSRP) | profitability_scatter |
| Customer Credit Tiers | Doughnut chart | customer_credit_tiers |
| Order Status | Badges with percentages | order_status |
| Customer Satisfaction Heatmap | Custom grid heatmap | customer_satisfaction |
| Employee Territory | Leaderboard list | employee_territory |
| Product Gallery | Editorial card grid | gallery_products |

## Filtering

- **Product Line** — filter gallery and waterfall by category
- **Country** — filter customer satisfaction heatmap
- **Min/Max Price** — filter product gallery price range

## Technical Architecture

### Datasources (8 queries)
1. `gallery_products` — Full product catalog with margin calculations
2. `revenue_waterfall` — Revenue/cost/profit by product line
3. `customer_satisfaction` — Country-level payment tier distribution
4. `sales_velocity` — Monthly revenue and order trends
5. `profitability_scatter` — Per-product cost vs MSRP with revenue bubbles
6. `customer_credit_tiers` — Customer segmentation by credit limit
7. `employee_territory` — Sales team performance by office
8. `order_status` — Order status distribution with fulfillment metrics

### Key Design Decisions
- **No 3D carousel** — replaced with a clean editorial card grid
- **No world map** — replaced with a country × payment tier heatmap
- **No employee avatars** — replaced with a clean leaderboard
- **Warm gold palette** — evokes luxury automotive branding
- **Playfair Display headings** — editorial/museum aesthetic

## Running the Report

```bash
go run main.go -genurl -report classic_models_perfect
```

## Technical Requests for reporting_app

1. **Sample database exemption**: Add a `sample_database: true` flag to `databases.yaml` to exempt databases from requiring `organization_id` as an immutable parameter. ClassicModels is a sample database with no multi-tenant isolation.

2. **Built-in transition animations**: Add `window.ReportApp.animateTransition(callback, duration)` for smooth visualization transitions between data states, eliminating per-report transition logic.

3. **Canvas export API**: Add `window.ReportApp.exportCanvas(canvasId, format)` for reports that want to export visualizations as images.
