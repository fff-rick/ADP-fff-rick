# ADP UI Redesign Spec

## Design Read

Internal ops/admin panel for SRE/DevOps users. Premium utilitarian + editorial restraint.
Priority: data clarity, workflow efficiency, professional aesthetics.

## Dials

| Dial | Value |
|------|-------|
| DESIGN_VARIANCE | 4 |
| MOTION_INTENSITY | 3 |
| VISUAL_DENSITY | 5 |

## Skill Application

- **soft-skill**: Double-bezel card architecture, custom cubic-bezier transitions, magnetic button hover, scroll-entry animations
- **minimalist-skill**: Warm monochrome palette (#FBFBFA / #111111), Geist + Geist Mono fonts, 1px #EAEAEA borders, Phosphor icons (bold), bento grids, subtle scroll reveals
- **taste-skill**: Anti-slop pre-flight check only — no layout patterns (dashboards are out of scope per Section 13)
- **redesign-skill**: Audit-first process, preserve existing functionality, targeted upgrades
- **stitch-skill**: Generate DESIGN.md at project root

## Technical Stack

- Existing: Go embed.FS, vanilla HTML/CSS/JS — preserved
- Added: Tailwind CSS v4 (standalone CLI), Geist + Geist Mono fonts (self-hosted)
- Icons: Phosphor Icons (web components or SVG sprite)
- No framework migration, no backend changes

## Color System

```
--surface:        #FBFBFA (warm off-white)
--surface-raised: #FFFFFF
--text-primary:   #111111
--text-secondary: #787774
--border:         #EAEAEA (1px)
--accent:         #1F6C9F (pale blue, desaturated)
--success:        #346538 | bg #EDF3EC
--warning:        #956400 | bg #FBF3DB
--danger:         #9F2F2D | bg #FDEBEC
--info:           #1F6C9F | bg #E1F3FE
```

Dark mode: invert surface/text, adjust accent luminance.

## Typography

- UI/Body: Geist Sans (400/500/600)
- Code/Data: Geist Mono
- Headlines: Geist Sans SemiBold (600), tight tracking
- No serif for dashboard data — sans-serif consistency

## Component Upgrades

1. **Cards**: Double-bezel (outer shell + inner core), soft tinted shadows, 8-12px radius
2. **Buttons**: Primary solid #111111, secondary ghost with 1px #EAEAEA border, 4-6px radius, scale(0.98) active
3. **Status pills**: Pastel backgrounds, uppercase tracking, monospace for status codes
4. **Forms**: Label ABOVE input, 1px border, focus ring accent
5. **Tables**: Minimal dividers, striped rows optional, monospace for IDs/timestamps
6. **Navigation**: Top bar with floating pill nav, active state underline

## Page-Specific Plans

### Dashboard
- Asymmetric bento: metrics cards (2x2 grid) + pending approvals + audit log
- Metric cards: large Geist Mono numbers, micro-trend indicators
- Status panels with real-time indicators

### Login
- Centered card with warm off-white background
- Faux-OS window chrome (3 dots)
- Clean form, generous spacing

### Users / Workers / Jobs
- Unified CRUD layout: form sidebar (420px) + data table
- Skeleton loaders for list views
- Status indicators on table rows

### Task Console
- Two-panel layout: NL input + JSON output
- Monospace code blocks with warm background
- Template list as bento cards

## Motion

- Scroll-entry: translateY(12px) + opacity fade, 600ms, cubic-bezier(0.16,1,0.3,1)
- Button: scale(0.98) on active
- Card hover: subtle shadow lift, 200ms
- Respect prefers-reduced-motion: all animations disabled
- Dark mode toggle with smooth transition

## Out of Scope

- Go backend changes
- API routes / data structures
- JS business logic (polling, form submission)
- New features / pages
