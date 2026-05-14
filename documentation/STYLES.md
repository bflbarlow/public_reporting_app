# CSS Style Guide

This document summarizes the CSS styles used in the application, extracted from a comprehensive stylesheet. The styles follow a consistent color palette, typography, and component design system tailored for a social services/findhelp platform.

## Color Palette

### Primary Colors
- **Dark Blue**: `#33485e` (headings, navigation)
- **Primary Blue**: `#3a70a0` (links, buttons)
- **Dark Teal**: `#167e8e` (info labels, badges)
- **Green**: `#3c763d` (success, apply buttons)
- **Red/Danger**: `#bc473a` (errors, danger buttons)
- **Dark Red Accent**: `#d14232` (accent)

### Background Colors
- **Light Gray 1**: `#f8f8f8` (navbar, light backgrounds)
- **Light Gray 2**: `#f0f0f0` (muted backgrounds)
- **Light Blue**: `#eaf4fb` (info alerts, light blue backgrounds)
- **Light Green**: `#ebfceb` (success alerts)
- **Light Red**: `#fceceb` (danger alerts)
- **Light Teal**: `#e6fbfc` (light teal backgrounds)
- **Light Yellow**: `#f9f2b4` (favorite flags, warnings)

### Text Colors
- **Dark Gray 1**: `#666` (copyright, help text)
- **Dark Gray 3**: `#444` (body text, close buttons)
- **Light Gray 1**: `#f8f8f8` (light text on dark backgrounds)
- **Primary/Blue**: `#3a70a0` (links)
- **Hover Blue**: `#23527c` (link hover)

## Typography

### Font Families
- **Primary**: System fonts stack (`-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif`)
- **Icons**: Custom "berthicons" font for category icons
- **Materialized UI**: Uses Nunito Sans and Montserrat

### Font Sizes
- **h1**: 30px (page headers)
- **h2**: 28px 
- **h3**: 22px
- **h4**: 20px
- **h5**: 14px
- **h6**: 12px
- **Body**: Default (approximately 16px)
- **Small text**: 12px-14px

### Font Weights
- **Regular**: 400
- **Medium**: 500 (links)
- **Bold**: 600-700 (headings, important text)

## Component Styles

### Buttons

#### Primary Buttons
```css
.btn-primary, .btn-info {
  background-color: #3a70a0;
  border-color: #3a70a0;
  color: #fff;
}
.btn-primary:hover {
  background-color: #235273;
  border-color: #235273;
}
```

#### Success Buttons (Apply/Connect)
```css
.btn-success, .connect-button {
  background-color: green;
  border-color: green;
  color: #fff;
}
.btn-success:hover {
  background-color: #3c763d;
  border-color: #3c763d;
}
```

#### Danger Buttons
```css
.btn-danger, .btn-red {
  background-color: #fceceb;
  color: #666;
}
.btn-danger:hover {
  background-color: #fae0de;
  color: #444;
}
```

#### Default Buttons
```css
.btn-default {
  border: 1px solid #3a70a0;
  color: #3a70a0;
}
.btn-default:hover {
  background-color: #f0f0f0;
  border: 1px solid #23527c;
  color: #23527c;
}
```

#### Special Buttons
- **Orange**: `#ef840f` (alert/warning actions)
- **Negative**: `#d14232` (destructive actions)
- **Grey Dark**: `#444` (secondary actions)
- **Grey**: `#666` (tertiary actions)

### Forms

#### Form Controls
```css
.form-control {
  border-color: #666;
}
.input-group-addon {
  background-color: #f8f8f8;
  border-color: #666;
  color: #444;
}
```

#### Validation States
- **Success**: Green text `#3c763d`, light green background `#ebfceb`
- **Error**: Red text `#bc473a`, light red background `#fceceb`
- **Required field indicator**: Red asterisk `#bc473a`

### Navigation

#### Main Navigation Bar
```css
.navbar-default {
  background-color: #f8f8f8;
}
.navbar {
  box-shadow: 0 5px 10px -3px rgba(0,0,0,.1);
}
```

#### Category Navigation Bar
```css
#category-nav-bar {
  background-color: #167e8e;
  min-height: 90px;
  box-shadow: 0 1px 3px 1px rgba(0,0,0,.15);
}
```

#### Mobile Navigation
```css
#mobile-category-navbar li {
  background-color: #f0f0f0;
  border-bottom: 2px solid #d1d1d1;
  color: #33485e;
}
```

### Alerts & Notifications

#### Info Alerts
```css
.alert-info {
  background-color: #eaf4fb;
  border-color: transparent;
  color: #23727c;
}
```

#### Success Alerts
```css
.alert-success {
  background-color: #ebfceb;
  color: #3c763d;
}
```

#### Warning Alerts
```css
.alert-orange {
  background-color: #f9f1d7;
  color: #000;
}
```

#### Danger Alerts
```css
.alert-danger {
  background-color: #fceceb;
  color: #bc473a;
}
```

### Cards & Wells

#### Well Component
```css
.well {
  background-color: #fff;
  border-radius: 1px;
  box-shadow: 0 0 2px -1px rgba(0,0,0,.3);
  margin-bottom: 7px;
  padding: 15px;
}
```

#### Search Results
```css
.search-result {
  color: rgba(0,0,0,.6);
  padding: 0 30px 30px !important;
}
```

#### Card V3 (Materialized Design)
```css
.card-v3.search-result {
  border: 1px solid #c7c7c7 !important;
  border-radius: 8px !important;
  padding: 16px !important;
}
.card-v3.search-result:hover {
  border: 1px solid #a8a8a8 !important;
  box-shadow: 0 4px 6px -1px rgba(0,0,0,.1), 0 2px 4px -2px rgba(0,0,0,.05);
}
```

### Badges & Labels

#### Info Label
```css
.label-info {
  background-color: #167e8e;
  color: #fff;
}
```

#### Warning Label
```css
.label-warning {
  background-color: #167e8e;
  color: #fff;
}
```

#### Eligibility Badges
- **Eligible**: Green background, white text
- **Ineligible**: Red background, white text  
- **Undetermined**: Gray background `#bdbdbd`
- **Partial Match**: Teal background `#167e8e`
- **Exact Match**: Dark blue background `#34495e`

### Tags & Chips

#### Result Tags
```css
.result-tag {
  background-color: transparent;
  border: 0;
  border-radius: 4px;
  margin-bottom: 5px;
  margin-right: -2px;
  padding: 2px 8px;
  position: relative;
}
```

#### Favorite Flag
```css
.favorite-flag {
  background-color: #f9f2b4;
  border: 1px solid #444;
  color: #444;
}
.favorite-flag:hover {
  background-color: #f1c40f;
}
```

### Tabs & Panels

#### Tab Navigation
```css
.nav-pills > li.active > a {
  background-color: #3498db;
}
.nav-tabs > li > a:hover {
  border-color: #f0f0f0 #f0f0f0 #ddd;
}
```

#### Tab Content
```css
.tab-content {
  background-color: #fff;
  border: 1px solid #ddd;
  border-radius: 0 0 4px 4px;
  border-top-color: transparent;
  padding: 15px 5px 15px 0;
}
```

### Modals & Dialogs

#### Connect/Referral Windows
```css
.connect, .share_box {
  border-radius: 5px;
  clear: both;
  margin-top: 10px;
  padding: 30px;
}
```

#### Exit Suggest Modal
```css
.exit-suggest-modal {
  background-color: #fff;
  border-radius: 8px;
  box-shadow: 0 4px 10px rgba(0,0,0,.15);
  padding: 30px !important;
}
```

### Footer
```css
footer {
  background-color: #f8f8f8;
  border-top: 1px solid #e7e7e7;
  padding: 8px 10px 0;
}
footer .navbar {
  box-shadow: 0 -5px 10px -3px rgba(0,0,0,.1);
}
```

## Layout & Grid

### Container
```css
.container-fluid {
  margin: 0 auto;
  max-width: 1690px;
  padding-left: 30px;
  padding-right: 30px;
}
```

### Responsive Breakpoints
- **Extra small**: < 342px (mobile portrait)
- **Small**: 342px - 767px (mobile landscape)
- **Medium**: 768px - 991px (tablet)
- **Large**: 992px - 1690px (desktop)

### Flexbox Layouts
```css
.flexbox, .row-eq-height {
  display: -webkit-box;
  display: -webkit-flex;
  display: -ms-flexbox;
  display: flex;
}
```

## Interactive States

### Hover States
- **Links**: Color changes from `#3a70a0` to `#23527c`
- **Buttons**: Background darkens, box-shadow added
- **Navigation items**: Background color changes, text decoration

### Focus States
```css
a:focus-visible {
  border-radius: 5px;
  box-shadow: 0 0 0 2px #192a56;
}
```

### Active States
- **Buttons**: Background darkens further
- **Navigation**: Color changes to `#286090`

## Accessibility Features

### Screen Reader Only
```css
.screenreader {
  height: 1px;
  overflow: hidden;
  position: absolute;
  top: -10px;
  width: 1px;
}
```

### Skip Links
```css
.skiplink {
  color: #666;
  left: -10000px;
  padding: 1em;
  position: absolute;
  z-index: 1000;
}
.skiplink:focus {
  background: #fff;
  border: 3px solid #4593ff;
  border-radius: 3px;
  left: 0;
  position: absolute;
  top: 4em;
  z-index: 100000;
}
```

## Icons

### Custom Icon Font (berthicons)
Icons are implemented via custom font with character codes:
- Care: `\61`
- Education: `\62`
- Food: `\63`
- Goods: `\64`
- Health: `\65`
- Housing: `\66`
- Legal: `\67`
- Money: `\68`
- Transportation: `\69`
- Work: `\6a`
- And more specialty icons

### Font Awesome Integration
Used alongside custom icons for common UI elements (close, info, etc.)

## Materialized Design System (Card V3)

### Color Scheme
- **Primary**: `#00838f` (teal)
- **Primary Dark**: `#006064`
- **Text**: `rgba(0,0,0,.84)` (87% black)
- **Secondary Text**: `rgba(0,0,0,.6)` (60% black)
- **Disabled**: `rgba(0,0,0,.38)` (38% black)

### Buttons (Materialized)
```css
.materialized .btn {
  background-color: transparent;
  border: none;
  border-radius: 5px;
  color: #00838f;
  font-size: 15px;
  letter-spacing: .5px;
  padding: 8px 16px;
  text-transform: uppercase;
  transition: all .3s cubic-bezier(.25,.8,.25,1);
}
.materialized .btn:hover {
  background-color: rgba(0,151,167,.18);
  box-shadow: 0 1px 3px rgba(0,0,0,.12), 0 1px 2px rgba(0,0,0,.24);
}
```

### Cards (Materialized)
- Border radius: 8px
- Box shadow on hover
- Clean typography with Montserrat/Nunito Sans
- Flexible grid with next-steps module

## Print Styles

### Print Optimization
```css
@media only print {
  * {
    background: transparent !important;
    filter: none !important;
    -ms-filter: none !important;
    text-shadow: none !important;
  }
  body {
    font: 12pt Helvetica, Arial, sans-serif;
    line-height: 1.3;
  }
  .print-button, .share-button, .favorite-flag {
    display: none;
  }
}
```

## Utility Classes

### Spacing
- `.no-padding`: `padding: 0`
- `.no-margin`: `margin-left: 0; margin-right: 0`
- `.no-pad-l`: `padding-left: 0 !important`
- `.no-pad-x`: `padding-left: 0 !important; padding-right: 0 !important`

### Text Utilities
- `.uppercase`: `text-transform: uppercase`
- `.text-wrap`: `white-space: normal`
- `.nowrap`: `white-space: nowrap`
- `.preserve-line-breaks`: `white-space: pre-line`
- `.word-wrap`: `word-wrap: break-word`

### Display Utilities
- `.screenreader`: Hide visually but available to screen readers
- `.visible-print`: Display only when printing
- `.d-xs-none`, `.d-sm-none`, `.d-md-none`: Responsive display classes

### Color Utilities
- `.text-dark-blue-1` through `-4`: Various blue text colors
- `.text-dark-teal`: Teal text
- `.text-dark-red`: Red text
- `.text-dark-green`: Green text
- `.bg-light-blue`, `.bg-light-green`, etc.: Light background colors

## Specialized Components

### Eligibility Display
```css
.seeker-eligibility-display {
  border-radius: 64px;
  font-family: Roboto, sans-serif;
  font-size: 12px;
  font-weight: 500;
  letter-spacing: .14px;
  line-height: 20px;
  text-align: center;
}
.ineligible-eligibility {
  background: #c62828;
  color: #fff;
  width: 85px;
}
.eligible-eligibility {
  background: #2e7d32;
  color: #fff;
  width: 75px;
}
```

### Claim Badge
```css
.claim-badge {
  background-color: #ece9de;
  border-radius: 20px;
  color: #351417;
  height: 40px;
  padding: 11px 10px;
  transition: all .3s cubic-bezier(.25,.8,.25,1);
}
.unclaimed .claim-badge {
  background-color: #fff;
  box-shadow: 0 1px 3px rgba(0,0,0,.12), 0 1px 2px rgba(0,0,0,.24);
  color: #00838f;
}
```

### Loading Indicators
```css
#loading-indicator {
  background-color: #fff;
  opacity: .75;
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 99;
}
.search-results-map-area-loading {
  animation: rotation 2s linear infinite;
  background-image: url(/images/spinner.png);
  background-position: 50%;
  background-repeat: no-repeat;
}
```

## Responsive Design Patterns

### Mobile-First Adjustments
1. **Navigation**: Collapses to hamburger menu
2. **Search forms**: Stack vertically, full width buttons
3. **Action buttons**: Smaller, icon-only or shortened text
4. **Cards**: Stack vertically, simplified layout

### Tablet Optimizations
- Side-by-side forms where possible
- Adjusted padding and margins
- Category navigation as dropdowns

## Browser Compatibility

### Vendor Prefixes
- WebKit prefixes for flexbox, box-shadow, transitions
- Mozilla prefixes for border-radius
- MS prefixes for transforms

### Touch Optimization
```css
body {
  -webkit-overflow-scrolling: touch !important;
  height: 100% !important;
  overflow: auto !important;
}
```

## Performance Considerations

### Image Optimization
- Spinner as PNG sprite
- SVG icons where possible
- Responsive images with max-width constraints

### Animation Performance
- Uses CSS transforms for smooth animations
- Hardware acceleration where beneficial
- Transition durations kept short (0.3s typical)

---

*This style guide documents the CSS architecture as of April 2026. The system combines traditional Bootstrap-like patterns with a newer materialized design system for cards and interactive elements.*