# Blink/Flicker Analysis for Reporting App

**Date:** 2026-05-07  
**Context:** Thick Client refresh operations causing visible UI disruption  
**Goal:** Identify causes and propose solutions for seamless report updates

## Executive Summary

When reports use `window.ReportApp.refresh()` to update data, users experience a visible "blink" or flicker effect. This document analyzes the root causes within the current architecture and proposes solutions ranging from quick wins to architectural improvements.

## Current Architecture & Data Flow

```
Report UI → window.ReportApp.refresh() → POST /refresh → Server Processing
      ↑                                      ↓
      └──── Returns Data ────────┬───── Response (JSON)
                                  ↓
                          Report JavaScript → DOM Update → UI Blink
```

**Key Characteristics:**
- Request/response cycle for each refresh
- Complete data replacement in report UI
- No visual feedback during processing
- Synchronous DOM updates after async fetch

## Root Causes of Blink Effect

### 1. **Synchronous DOM Replacement**
Report JavaScript typically does:
```javascript
const data = await window.ReportApp.refresh(params);
// BLINK OCCURS HERE ⬇️
document.getElementById('chart').innerHTML = generateChartHTML(data);
```
The element becomes empty briefly while the HTML generation function executes.

### 2. **Chart Library Re-initialization**
Most chart libraries (Chart.js, D3.js, Plotly) destroy and recreate the entire visualization canvas, causing:
- Canvas clearing (white flash)
- Complete redraw from scratch
- No incremental updates

### 3. **CSS Layout Shifts (CLS)**
New data can change:
- Chart dimensions
- Table row counts
- Container heights
Cumulative Layout Shift causes visible content jumps.

### 4. **Network Latency Visibility**
No visual feedback during the ~100-500ms fetch period makes updates feel abrupt and discontinuous.

### 5. **Multiple Component Updates**
Dashboards with multiple visualizations trigger multiple simultaneous updates, amplifying the blink effect.

## Impact Assessment

| Severity | User Experience Impact | Technical Complexity |
|----------|-----------------------|---------------------|
| High | Professional dashboards lose polish | Medium |
| Medium | User confidence decreases | Low-Medium |
| Low | Acceptable for internal tools | Low |

## Solution Matrix

### Immediate Solutions (Quick Wins)

#### A. CSS Transitions & Opacity Fading
```css
/* Add to report CSS */
.report-component {
  transition: opacity 0.2s ease;
}
.report-component.updating {
  opacity: 0.7;
}
```

**Implementation:**
1. Add CSS class before refresh
2. Remove class after DOM update
3. Fade between states instead of hard cut

**Effort:** Low (report-level changes only)

#### B. Skeleton Screens
Replace content with placeholder skeletons during refresh:

```html
<div id="chart-container">
  <!-- Normal content -->
</div>

<!-- Skeleton template -->
<template id="skeleton-template">
  <div class="skeleton-chart">
    <div class="skeleton-bar" style="height: 60%"></div>
    <div class="skeleton-bar" style="height: 80%"></div>
    <!-- ... -->
  </div>
</template>
```

**Effort:** Medium (report-level HTML/CSS/JS)

### Medium-Term Solutions

#### C. Thick Client Loading States API
Enhance `window.ReportApp` with event system:

```javascript
// Proposed API
window.ReportApp.on('refresh:start', () => {
  document.body.classList.add('refreshing');
});

window.ReportApp.on('refresh:complete', () => {
  document.body.classList.remove('refreshing');
});

// Or promise-based
window.ReportApp.refresh(params, {
  onStart: () => { /* show loading */ },
  onProgress: (percent) => { /* update progress */ },
  onComplete: (data) => { /* transition */ }
});
```

**Effort:** Medium (thick client changes + report adoption)

#### D. Client-Side Caching
Implement `stale-while-revalidate` pattern:

```javascript
// In thick_client.js
const cache = new Map();

async function refresh(params) {
  // Return cached data immediately
  const cacheKey = generateCacheKey(params);
  if (cache.has(cacheKey)) {
    emit('data:cached', cache.get(cacheKey));
  }
  
  // Fetch fresh data
  const fresh = await fetchFresh(params);
  cache.set(cacheKey, fresh);
  emit('data:fresh', fresh);
}
```

**Effort:** Medium (thick client implementation)

### Architectural Solutions

#### E. WebSocket/Server-Sent Events (SSE)
**WebSocket:**
- Persistent bidirectional connection
- Real-time push updates
- Eliminates request/response cycle

**SSE (Server-Sent Events):**
- Simpler, unidirectional from server
- Automatic reconnection
- Built-in event types

**Implementation Sketch:**
```go
// In server
func (h *RefreshHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  flusher, ok := w.(http.Flusher)
  // Send incremental updates
  fmt.Fprintf(w, "event: progress\ndata: 25\n\n")
  flusher.Flush()
}
```

**Effort:** High (server + client changes)

#### F. Streaming Responses
Send data as it becomes available from database:

```javascript
// Client receives stream
const response = await fetch('/refresh-stream', options);
const reader = response.body.getReader();

while (true) {
  const {done, value} = await reader.read();
  if (done) break;
  // Process partial data, update UI incrementally
  updateChartPartial(decoder.decode(value));
}
```

**Effort:** High (database layer + HTTP streaming)

#### G. Virtual DOM / Incremental Updates
For chart libraries that support it:

```javascript
// Instead of: chart.update() (destroys and recreates)
// Use: chart.addData() and chart.removeData() incrementally

// Patch-based updates
const patches = diff(oldData, newData);
applyPatches(chart, patches);
```

**Effort:** High (report-level library choices)

## Recommendation Priority

### Phase 1: Immediate (1-2 weeks)
1. **CSS Transitions** - Add to all report templates
2. **Skeleton Screens** - Create reusable skeleton components
3. **Document Best Practices** for report developers

### Phase 2: Short-term (1 month)
1. **Thick Client Events API** - Add `refresh:start`, `refresh:progress`, `refresh:complete`
2. **Request Debouncing** - In thick client for rapid parameter changes
3. **Error State Handling** - Graceful degradation

### Phase 3: Medium-term (2-3 months)
1. **Client-Side Caching** - Configurable caching strategy
2. **Performance Budgets** - Monitor and alert on slow updates
3. **Report Template Library** - Pre-built, optimized components

### Phase 4: Long-term (Optional)
1. **WebSocket/SSE Support** - For real-time dashboards
2. **Streaming Responses** - For large datasets
3. **Incremental Chart Updates** - Partner with visualization libraries

## Implementation Guidelines

### For Report Developers:
```javascript
// Current pattern (causes blink):
async function updateChart() {
  const data = await window.ReportApp.refresh(params);
  chart.data = data;
  chart.update(); // Blinks
}

// Improved pattern:
async function updateChartSmoothly() {
  // 1. Show loading state
  chartContainer.classList.add('updating');
  
  // 2. Use skeleton if data will take > 200ms
  if (expectedDelay > 200) {
    showSkeleton();
  }
  
  // 3. Get data
  const data = await window.ReportApp.refresh(params);
  
  // 4. Update with transition
  setTimeout(() => {
    applyDataWithAnimation(data);
    chartContainer.classList.remove('updating');
  }, 10);
}
```

### CSS Framework Additions:
```css
/* Add to static/styles.css */
.report-transitioning {
  transition: opacity 0.2s ease;
}

.report-skeleton {
  background: linear-gradient(90deg, #f0f0f0 25%, #e0e0e0 50%, #f0f0f0 75%);
  background-size: 200% 100%;
  animation: shimmer 1.5s infinite;
}

@keyframes shimmer {
  from { background-position: -200% 0; }
  to { background-position: 200% 0; }
}
```

## Measurement & Validation

### Metrics to Track:
1. **First Paint After Refresh** (target: < 50ms)
2. **Cumulative Layout Shift** (target: < 0.1)
3. **User Perception Score** (survey: 1-5 smoothness rating)
4. **Refresh Abandonment Rate** (users canceling during update)

### A/B Testing Approach:
1. Implement solutions in select reports first
2. Measure against control group
3. User feedback sessions

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| CSS transitions cause performance issues | Use `transform` and `opacity` only (GPU accelerated) |
| Skeleton screens increase bundle size | Use shared CSS, lightweight templates |
| Caching causes stale data | Implement TTL, manual refresh override |
| WebSocket adds server load | Implement connection pooling, scale horizontally |

## Conclusion

The blink effect is solvable through a layered approach:

1. **Immediate:** CSS transitions and skeleton screens (80% improvement)
2. **Short-term:** Enhanced thick client API for coordinated updates
3. **Long-term:** Architectural improvements for specific use cases

**Recommended starting point:** Implement CSS transitions across all reports while designing the thick client events API. This provides immediate user experience improvement with minimal technical risk.

---

## Appendix: Technical Spike Questions

1. **What percentage of refreshes exceed 200ms?**  
   *Answer needed to decide between skeleton screens vs. simple transitions.*

2. **Which chart libraries are most commonly used?**  
   *Answer needed to prioritize incremental update implementations.*

3. **What is the average dataset size per report?**  
   *Answer influences caching and streaming decisions.*

4. **Are reports typically viewed on mobile or desktop?**  
   *Answer affects performance budgets and interaction patterns.*

---
*Document maintained by: Reporting App Architecture Team*  
*Last updated: 2026-05-07*