# DPlaneOS Design System

**Source of truth** synthesized from `src/index.css` and the component library.
Reference this document before generating or editing any screen.

---

## Identity

**Product:** DPlaneOS — NixOS-exclusive ZFS NAS management OS  
**Audience:** Homelab operators, small-office sysadmins, infrastructure engineers  
**Vibe:** Deep-space dark glass · enterprise precision · zero visual noise  
**Platform:** Desktop web (1280px+), dark-only, no mobile breakpoints  
**Font stack:** Outfit (UI) · JetBrains Mono (code, paths, hashes, IPs)

---

## Color Palette

All colors are HSL-rooted and should always be referenced via CSS variables.

### Primary (blue-violet accent)
| Token | Value | Use |
|---|---|---|
| `--primary` | `hsl(236, 60%, 67%)` | CTAs, active states, links |
| `--primary-bg` | `hsla(236, 60%, 67%, 0.10)` | Tinted backgrounds |
| `--primary-hover` | `hsl(236, 60%, 74%)` | Hover state |
| `--primary-glow` | `hsla(236, 60%, 67%, 0.18)` | Glow effects |

### Semantic
| Token | Value | Use |
|---|---|---|
| `--success` | `hsl(148, 80%, 45%)` | Healthy, online, complete |
| `--error` | `hsl(350, 85%, 60%)` | Faults, failures, danger |
| `--warning` | `hsl(38, 92%, 55%)` | Degraded, attention, caution |
| `--info` | `hsl(208, 95%, 62%)` | Informational, NVMe type |

Each semantic color has `*-bg` (12% alpha), `*-border` (25% alpha), `*-glow` (25% alpha) variants.

### Backgrounds (deep space)
| Token | Value | Use |
|---|---|---|
| `--bg` | `hsl(228, 18%, 4%)` | Page root |
| `--bg-elevated` | `hsla(228, 18%, 7%, 0.7)` | Modals, flyouts |
| `--bg-card` | `hsla(228, 18%, 10%, 0.5)` | Cards (glass) |
| `--surface` | `hsla(228, 20%, 18%, 0.4)` | Inputs, interactive surfaces |

### Borders
| Token | Value | Use |
|---|---|---|
| `--border` | `hsla(0,0%,100%,0.08)` | Default card/element border |
| `--border-strong` | `hsla(0,0%,100%,0.14)` | Table headers, card borders |
| `--border-subtle` | `hsla(0,0%,100%,0.04)` | Row separators, very soft dividers |
| `--border-highlight` | `hsla(0,0%,100%,0.20)` | Modal borders, active elements |

### Text
| Token | Use |
|---|---|
| `--text` | Primary (95% white) |
| `--text-secondary` | Body descriptions |
| `--text-tertiary` | Labels, captions, placeholders |
| `--text-on-primary` | Dark text on primary-colored buttons |

---

## Typography

**Scale:** 9 · 10 · 12 · 14 · 15 · 16 · 18 · 22 · 28 · 36px  
Every size has a CSS variable: `--text-3xs` through `--text-3xl`.

| Role | Size | Weight | Notes |
|---|---|---|---|
| Page title | `--text-3xl` (36px) | 700 | `letter-spacing: -1.2px` |
| Section heading | `--text-base` (15px) | 700 | Inside cards |
| Table header | `--text-xs` (12px) | 600 | UPPERCASE, 0.8px tracking |
| Body / descriptions | `--text-sm`–`--text-base` | 400–500 | |
| Labels / field labels | `--text-xs` (12px) | 600 | UPPERCASE, 0.5px tracking |
| Mono values | `--font-mono` | — | 13px, paths/IPs/hashes |
| Badges | `--text-xs` (12px) | 600 | 0.3px tracking |

---

## Spacing

Use `--space-N` tokens in inline styles where possible:

| Token | Value |
|---|---|
| `--space-1` | 4px |
| `--space-2` | 8px |
| `--space-3` | 12px |
| `--space-4` | 16px |
| `--space-5` | 20px |
| `--space-6` | 24px |
| `--space-8` | 32px |
| `--space-10` | 40px |
| `--space-12` | 48px |

**Card padding standard:** `24px 28px` (`.card` class default). Use `32px` (`.card-xl`) for primary focus cards only. Avoid ad-hoc values like 18px, 22px, 10px.

---

## Border Radius

| Token | Value | Use |
|---|---|---|
| `--radius-xs` | 6px | Badges, tiny elements |
| `--radius-sm` | 10px | Small cards, tooltips, tags, `btn-sm` |
| `--radius-md` | 16px | Inputs, standard buttons, dropdowns |
| `--radius-lg` | 20px | Section cards, tabs container |
| `--radius-xl` | 28px | Page-level cards, modals |
| `--radius-full` | 9999px | Status dots, avatar circles, pills |

---

## Shadows

| Token | Use |
|---|---|
| `--shadow-sm` | Subtle: tags, small chips |
| `--shadow-md` | Cards, dropdowns |
| `--shadow-lg` | Modals, elevated panels |
| `--shadow-xl` | Fullscreen overlays |
| `--shadow-glow` | Primary glow (decorative) |

---

## Z-Index Layers

| Token | Value | Use |
|---|---|---|
| `--z-topbar` | 40 | Fixed topbar + sidebar nav |
| `--z-dropdown` | 100 | Dropdowns, context menus (backdrop), tooltips |
| `--z-modal` | 200 | Modals, inline overlays, flyout panels |
| `--z-toast` | 300 | Toast notifications |
| `--z-overlay` | 400 | Full-screen overlays (GlobalSearch, KeyboardHelp) |
| `--z-supreme` | 9999 | Force-password-change wall (blocks all UI) |

**Backdrop pattern:** Render backdrop `<div>` BEFORE the panel `<div>` in DOM order. Give both the same z-index — the later-rendered panel wins the stacking order.

---

## Motion

| Token | Curve | Use |
|---|---|---|
| `--transition-fast` | `0.15s cubic-bezier(0.2,0,0,1)` | Hover states |
| `--transition-base` | `0.25s cubic-bezier(0.2,0,0,1)` | Card transforms |
| `--transition-slow` | `0.4s cubic-bezier(0.2,0,0,1)` | Page transitions |
| `--transition-bounce` | `0.4s cubic-bezier(0.34,1.56,0.64,1)` | Tabs, toggles |

---

## Component Patterns

### Buttons
```
.btn .btn-primary   — filled accent, dark text on primary
.btn .btn-ghost     — transparent, subtle border
.btn .btn-danger    — red tinted, for destructive actions
.btn .btn-sm        — smaller padding (6px 12px)
.btn .btn-xs        — smallest (4px 10px)
```
- Always `disabled` attribute during loading/pending — never remove the button
- Gap between icon and label: 6px (default btn), 4px (btn-xs)

### Cards
```
.card               — glass card, 24px 28px padding, radius-xl
.card.interactive   — adds hover lift + glow cursor pointer
.card-xl            — 32px padding, primary focus sections only
```

### Alerts
```
.alert .alert-warning
.alert .alert-error
.alert .alert-info
.alert .alert-success
```
Always include an `<Icon>` as the first child. Alert text is colored to match the semantic color.

### Empty States
```tsx
<div className="empty-state">
  <Icon name="..." size={48} className="empty-state-icon" />
  <h3 className="empty-state-title">No items</h3>
  <p className="empty-state-body">Descriptive helper text.</p>
</div>
```

### Badges
```
.badge .badge-success / .badge-error / .badge-warning / .badge-primary / .badge-neutral
```

### Form Fields
```tsx
<div className="field">
  <label className="field-label">LABEL</label>
  <input className="input" />
</div>
```

### Data Tables
Always inside `<div className="card" style={{ padding: 0, overflow: 'hidden' }}>`.

### Modals
Use `<Modal title="..." onClose={fn}>` from `@/components/ui/Modal`. Provides portal, focus trap, Escape key, ARIA. Content goes in children; footer buttons in `<div className="modal-footer">`.

---

## Layout Structure

```
┌─ Sidebar (260px / 72px collapsed) ──── z-topbar (40) ─────┐
│  Fixed left nav                                            │
└────────────────────────────────────────────────────────────┘
┌─ TopBar (64px height) ───────────────── z-topbar (40) ─────┐
│  Fixed, blurred glass header                               │
└────────────────────────────────────────────────────────────┘
┌─ Page content (offset by sidebar+topbar) ──────────────────┐
│  max-width typically 900–1000px                            │
│  32px margin-bottom on header                              │
│  24px gap between card sections                            │
└────────────────────────────────────────────────────────────┘
```

---

## Chart Colors

For SVG fill/stroke contexts where CSS variables are not supported:

```ts
const C_ARC    = 'hsl(260, 78%, 76%)' // purple — ZFS ARC, distinct from primary
const C_LOAD   = 'hsl(208, 95%, 62%)' // matches --info
const C_IOWAIT = 'hsl(38, 92%, 55%)'  // matches --warning
```

---

## Glass Morphism Rules

- Card backgrounds: `hsla(...)` at low opacity + `backdrop-filter: var(--blur-glass)` + `border: 1px solid var(--border)`
- Modal backgrounds: `var(--bg-elevated)` + `backdrop-filter: var(--blur-glass)` + `border: 1px solid var(--border-highlight)`
- Allowed raw rgba: `rgba(0,0,0,x)` for backdrop overlays · `rgba(255,255,255,x)` for glass highlights · xterm.js ITheme hex colors (library requirement)
- Never use raw hex colors (`#abc123`) or specific-color rgba outside of the above exceptions

---

## What NOT to Do

- No hardcoded hex colors outside xterm.js themes
- No raw `rgba(r,g,b,a)` with specific colors — use `--error-bg`, `--info-bg`, etc.
- No raw `zIndex: N` numbers — use `var(--z-*)` tokens
- No `<a href="/path">` for internal navigation — use `router.navigate()` or TanStack `<Link>`
- No `alert()`, `confirm()`, `prompt()` — use `<ConfirmDialog>` and `toast.*`
- No padding values like 17px, 22px, 18px — stay on the token grid (4, 8, 12, 16, 20, 24, 28, 32px)
- No `.card` padding overridden to ad-hoc values — extend with `padding: 'var(--space-6) var(--space-8)'`
- No light mode — dark only
