# Customer SQL Reporting - Development Discussion

## Overview
This document summarizes the conversation and requirements for creating the new `customer_sql_reporting` report with advanced Plotly.js visualizations.

## Project Context
- **Project Location**: `/Users/bflbarlow/Go/reporting_app/reports/customer_sql_reporting/`
- **Template Source**: Copied from `reports/report_template/`
- **Database**: `customer_sql` (as defined in `databases.yaml`)
- **Reporting App Framework**: Using thick client (`window.ReportApp`) for data fetching and parameter management

## Requirements Summary

### 1. Core Constraints
- **Report Developer Role**: Can only change code in the reports directory
- **Cannot change**: Thick client or reporting_app code
- **Can change**: Code in the reports directory only
- **Database Structure**: Defined by `/Users/bflbarlow/Python/excel_to_md/combined.md` (Customer SQL database schema)

### 2. Technical Requirements
- **Visualization Library**: Plotly.js (must be clever in presenting data)
- **Goal**: Create something new and unique to "wow" stakeholders
- **Starting Point**: `/Users/bflbarlow/Go/reporting_app/reports/report_template`
- **Available Documentation**: 
  - `/Users/bflbarlow/Go/reporting_app/THICK_CLIENT_FOR_REPORT_DEVS.md`
  - Template README.md

### 3. Visualization Vision
The report should feature advanced Plotly.js visualizations including:
- **Sankey diagrams**: For customer journey flow visualization
- **Sunburst charts**: For hierarchical data representation  
- **Parallel coordinates**: For multidimensional analysis
- **3D scatter plots**: For complex relationship visualization

## Database Schema Analysis

### Key Tables Identified
From the database documentation (`combined.md`), the following tables are available for the customer journey analytics:

1. **assessments** - Form submissions with questions and answers
2. **form_responses** - Program and screener form submissions
3. **goals** - Goals set for seekers
4. **referrals** - Referral records with status tracking
5. **referral_status** - Historical referral status changes
6. **programs** - Program information with categories and tags
7. **seeker_profiles** - Seeker demographic information
8. **seeker_context** - Seeker session context data
9. **notes** - Notes on goals or referrals
10. **user_activity** - Site activity tracking

### Critical Join Relationships
- `seeker_profiles.seeker_id` → `referrals.seeker_id`
- `referrals.program_numeric_id` → `programs.program_numeric_id`
- `referrals.referral_id` → `referral_status.referral_id`
- `goals.seeker_id` → `seeker_profiles.seeker_id`
- `assessments.seeker_id` → `seeker_profiles.seeker_id`

## Planned Implementation

### 1. Report Configuration (`report.yaml`)
- **ID**: `customer_sql_reporting`
- **Name**: "Customer Journey Analytics"
- **Description**: Advanced visual analytics of customer journeys from assessment to outcomes
- **Database**: `customer_sql`
- **Parameters**:
  - **Immutable**: `organization_id` (required for security)
  - **Mutable**: `start_date`, `end_date`, `domain_filter`, `program_category`, `referral_status_filter`

### 2. Data Sources (SQL Queries)
Need to create datasources that:
1. Track customer journey from assessment → referral → program → outcome
2. Support hierarchical categorization (domains → categories → programs)
3. Provide multidimensional metrics for parallel coordinates
4. Enable Sankey flow visualization between journey stages

### 3. Dashboard Design (`dashboard.html`)
- Replace Chart.js with Plotly.js CDN
- Implement interactive visualization grid with:
  - Sankey diagram for journey flow
  - Sunburst for program hierarchy
  - Parallel coordinates for multidimensional analysis
  - 3D scatter for complex relationships
- Maintain thick client integration patterns
- Include advanced controls for visualization parameters

### 4. Thick Client Integration
- Follow patterns from `THICK_CLIENT_FOR_REPORT_DEVS.md`
- Use `window.ReportApp.refresh()` for data fetching
- Implement proper parameter management
- Include loading states and error handling

## Technical Considerations

### 1. Security Requirements
- All SQL queries MUST include `organization_id = {{organization_id}}` filter
- Never expose raw SQL in client-side code
- Respect immutable vs mutable parameter separation

### 2. Performance Considerations
- Use appropriate `row_limit` values
- Implement `cache_ttl` for slow-changing data
- Consider pagination for large datasets

### 3. Visualization Complexity
- Plotly.js supports complex visualizations but requires careful data preparation
- Sankey diagrams need source-target-value format
- Sunburst charts require hierarchical data structure
- Parallel coordinates need normalized numerical data

## Next Steps

### Immediate Actions
1. **Update `report.yaml`** with actual SQL queries for customer journey data
2. **Rewrite `dashboard.html`** with Plotly.js visualizations
3. **Test** with generated URL: `go run main.go -genurl -report customer_sql_reporting -params "organization_id=1,start_date=2024-01-01,end_date=2024-12-31"`

### SQL Query Development
Need to create queries that:
- Join assessments, referrals, programs, and seeker_profiles
- Track journey progression through assessment domains → referral categories → program outcomes
- Calculate metrics for multidimensional analysis (completion rates, time intervals, engagement scores)

### Visualization Implementation
- Create data transformation functions for Plotly.js formats
- Implement interactive controls for visualization parameters
- Ensure responsive design and mobile compatibility

## Potential Challenges

### 1. Data Complexity
Customer journey data spans multiple tables and requires careful joining to maintain data integrity.

### 2. Visualization Performance
Complex Plotly.js visualizations with large datasets may impact performance - need to implement data sampling or aggregation.

### 3. Thick Client Integration
Must ensure proper error handling and loading states during data refresh cycles.

## Success Criteria
1. Report loads successfully via thick client
2. All Plotly.js visualizations render correctly
3. Interactive features work (zoom, pan, hover details)
4. Data reflects actual customer journey patterns
5. Performance is acceptable with realistic dataset sizes

## References
1. Thick Client Guide: `THICK_CLIENT_FOR_REPORT_DEVS.md`
2. Database Schema: `combined.md`
3. Plotly.js Documentation: https://plotly.com/javascript/
4. Reporting App Structure: Existing reports in `reports/` directory

---
*Discussion created: 2026-05-08*
*Report ID: customer_sql_reporting*
*Target Database: customer_sql*