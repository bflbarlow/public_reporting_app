# Multi-Database Cross-Reference Report

Demonstrates a single report querying **two different databases**:

| Datasource | Database | Data Source |
|------------|----------|-------------|
| `seeker_demographics` | `customer_sql` | seeker_profiles |
| `customer_overview` | `classicmodels` | customers |
| `product_summary` | `classicmodels` | productlines, products |
| `program_status` | `customer_sql` | programs |

## Key Feature

Each datasource specifies its own `database` field in `report.yaml`:

```yaml
datasources:
  seeker_demographics:
    database: customer_sql    # ← datasource-level override
    sql: SELECT ... FROM seeker_profiles ...
  
  customer_overview:
    database: classicmodels   # ← datasource-level override
    sql: SELECT ... FROM customers ...
```

When `database` is omitted, the report falls back to `report.database` (`default`).

## Test URL

```bash
go run main.go -genurl -report simple_multi_db_report -params "subdomain=example"
```

## Visualizations

- **Seeker Demographics** (bar chart) — seekers by subdomain from customer_sql
- **Customer Overview** (table) — country/customer count from classicmodels
- **Product Summary** (bar chart) — product lines/avg price from classicmodels
- **Program Status** (pie chart) — programs by published status from customer_sql
