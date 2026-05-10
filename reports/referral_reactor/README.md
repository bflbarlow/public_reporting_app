# Referral Reactor Core

A pulsing energy-core visualization of referrals, programs, and engagement.

## Features

- **Animated reactor core** - Pulsing energy core representing total engagement heat
- **Orbiting program rings** - Each program orbits the core at its own speed
- **Status particles** - Referrals orbit their programs as colored particles
- **Real-time animation** - Continuous motion with physics-based orbits
- **Fully Interactive Controls**:
  - **Speed adjustment**: 0.1x to 3.0x speed slider
  - **Zoom & Pan**: Mouse wheel zoom, click & drag pan, touch pinch zoom
  - **Tooltips**: Hover over any particle to see detailed referral data
  - **Animation controls**: Pause/resume with button or Space key
- **Visual encoding**:
  - Particle color = referral status
  - Particle size = engagement level (goals + notes + activity)
  - Orbit distance = referral duration (days to latest status)
  - Core pulse intensity = total engagement heat

## Data Sources

- **referrals** - Core referral data
- **programs** - Program names and IDs  
- **referral_status** - Latest status and duration
- **goals**, **notes**, **user_activity** - Engagement metrics

## Status Color Mapping

- **Created**: Blue (`#60a5fa`)
- **In Progress**: Yellow (`#fbbf24`)
- **Completed**: Green (`#34d399`)
- **Cancelled**: Red (`#f87171`)
- **Other/Unknown**: Default blue

## Technical Implementation

- HTML5 Canvas with radial gradients for glow effects
- Trigonometric functions for orbital mechanics
- RequestAnimationFrame for smooth animation
- Physics-based orbital speeds and radii

## Development Notes

- Programs are assigned random orbital radii (120-320px) and speeds
- Particles orbit their program centers with additional animation
- Core pulse frequency tied to total engagement heat
- Canvas clears and redraws each frame for smooth animation

## Testing

With `ENABLE_PUBLIC_PATHS=true`, you can test without HMAC signatures.

## Interactive Controls

### Mouse/Trackpad
- **Zoom**: Scroll wheel
- **Pan**: Click and drag empty space
- **Hover**: Hover over any particle to see detailed tooltip
- **Click**: Click on particle to lock tooltip
- **Speed Control**: Use slider in control panel (top-right)

### Touch Devices
- **Zoom**: Pinch with two fingers
- **Pan**: Drag with one finger
- **Tap**: Tap particle to see tooltip
- **Speed Control**: Use slider in control panel

### Keyboard Shortcuts
- **Space**: Toggle animation
- **R**: Reset zoom/pan view
- **+** or **=**: Zoom in
- **-**: Zoom out

### On-Screen Controls (Top-Right)
- **Speed Slider**: Adjust animation speed (0.1x to 3.0x, default 0.5x)
- **Zoom Buttons**: +/- for zoom control
- **Reset View**: Returns to default zoom/pan
- **Pause/Resume**: Toggles the animation

### Status Display (Bottom-Left)
Shows current:
- Animation state (Running/Paused)
- Zoom level
- Pan coordinates
- Speed multiplier
- Usage hints

## Future Enhancements

- **Filter by status or program**: Toggle visibility of specific statuses/programs
- **Program highlighting**: Click to highlight a specific program's particles
- **Export view**: Capture current visualization as image
- **Time range filtering**: Visualize referrals within specific date ranges
- **Statistical overlay**: Show completion rates, engagement metrics
- **Search**: Find specific seeker ID or referral ID