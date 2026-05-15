# Hover Effects Removal

## Request
Remove the "little 'lift' movement" animations on chart-card and summary-card elements.

## Changes Made

### 1. **Chart Card Hover Effect Removed**
**Location**: `.chart-card:hover` CSS rule
**Original**: 
```css
.chart-card:hover {
    transform: translateY(-5px);
    box-shadow: 0 10px 25px var(--color-shadow);
}
```
**After**: Removed entirely (replaced with comment)

### 2. **Summary Card Hover Effect Removed**
**Location**: `.summary-card:hover` CSS rule  
**Original**:
```css
.summary-card:hover {
    transform: translateY(-2px);
}
```
**After**: Removed entirely (replaced with comment)

## What Was Removed
 - `transform: translateY(-5px)` from chart cards (previously lifted cards up by 5px on hover)
 - `transform: translateY(-2px)` from summary cards (previously lifted cards up by 2px on hover)
 - Enhanced box shadow on chart card hover

## What Remains
 - All other transitions (color changes, theme transitions, etc.)
 - Card styling (gradient top bars, borders, shadows)
 - Progress bar animations
 - Theme toggle transitions
 - Filter container and input transitions

## Files Modified
- `report.html` - Updated CSS rules (lines ~340 and ~283)

## Testing
- Hover over chart cards: No vertical movement
- Hover over summary cards: No vertical movement  
- All other hover effects (theme toggle, filter inputs) remain intact
- Cards maintain static position at all times

**Note**: Cards still have `transition: all 0.3s ease;` for color and border changes when theme toggles, but no positional changes.