# FlashDB Frontend — Professional UI/UX Specification

> Design System, Component Architecture, and Implementation Guide  
> Goal: The most beautiful database dashboard in existence

---

## 1. Design Philosophy

### Core Principles
1. **Minimalism** — Remove everything that doesn't serve the user. White space is a feature.
2. **Intelligence** — The UI should anticipate needs, not just display data.
3. **Speed** — Every interaction must feel instant. No loading spinners visible for >200ms.
4. **Precision** — Pixel-perfect alignment. Consistent spacing. Harmonious typography.
5. **Delight** — Subtle animations that make interactions feel alive, never distracting.

### Design References
- **Linear** — Navigation, command palette, keyboard-first design
- **Vercel** — Dashboard cards, deployment status patterns
- **Raycast** — Command bar, search-as-navigation
- **Supabase** — Database management UX, table editors
- **Grafana** — Real-time metric visualization (but much more polished)

---

## 2. Design Tokens

### Color System
```css
/* === Background Layers === */
--bg-base:       #09090b;   /* App background — zinc-950 */
--bg-surface:    #18181b;   /* Cards, panels — zinc-900 */
--bg-elevated:   #27272a;   /* Popovers, modals — zinc-800 */
--bg-overlay:    rgba(0,0,0,0.6);  /* Modal backdrop */

/* === Border === */
--border-default: #27272a;  /* Subtle borders — zinc-800 */
--border-hover:   #3f3f46;  /* Hover state — zinc-700 */
--border-focus:   #3b82f6;  /* Focus ring — blue-500 */

/* === Text === */
--text-primary:   #fafafa;  /* Primary content — zinc-50 */
--text-secondary: #a1a1aa;  /* Secondary — zinc-400 */
--text-tertiary:  #71717a;  /* Disabled, hints — zinc-500 */
--text-inverse:   #09090b;  /* On accent buttons */

/* === Accent Colors === */
--accent:         #3b82f6;  /* Primary actions — blue-500 */
--accent-hover:   #2563eb;  /* Hover — blue-600 */
--accent-subtle:  rgba(59,130,246,0.1);  /* Backgrounds */

/* === Semantic === */
--success:        #22c55e;  /* green-500 */
--success-subtle: rgba(34,197,94,0.1);
--warning:        #f59e0b;  /* amber-500 */
--warning-subtle: rgba(245,158,11,0.1);
--error:          #ef4444;  /* red-500 */
--error-subtle:   rgba(239,68,68,0.1);
--info:           #06b6d4;  /* cyan-500 */

/* === Special === */
--gradient-brand: linear-gradient(135deg, #3b82f6, #8b5cf6);
--glow-accent:    0 0 20px rgba(59,130,246,0.3);
```

### Typography
```css
/* Font Stack */
--font-sans:  'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
--font-mono:  'JetBrains Mono', 'Fira Code', 'SF Mono', monospace;

/* Scale */
--text-xs:    0.75rem;    /* 12px — badges, timestamps */
--text-sm:    0.875rem;   /* 14px — secondary text, table cells */
--text-base:  1rem;       /* 16px — body text */
--text-lg:    1.125rem;   /* 18px — section headers */
--text-xl:    1.25rem;    /* 20px — page titles */
--text-2xl:   1.5rem;     /* 24px — hero stats */
--text-3xl:   1.875rem;   /* 30px — dashboard KPIs */

/* Weights */
--font-normal: 400;
--font-medium: 500;
--font-semibold: 600;
--font-bold: 700;
```

### Spacing
```css
/* 4px base grid */
--space-1: 0.25rem;   /* 4px */
--space-2: 0.5rem;    /* 8px */
--space-3: 0.75rem;   /* 12px */
--space-4: 1rem;      /* 16px */
--space-5: 1.25rem;   /* 20px */
--space-6: 1.5rem;    /* 24px */
--space-8: 2rem;      /* 32px */
--space-10: 2.5rem;   /* 40px */
--space-12: 3rem;     /* 48px */
```

### Borders & Radius
```css
--radius-sm:  0.375rem;  /* 6px — inputs, badges */
--radius-md:  0.5rem;    /* 8px — cards, buttons */
--radius-lg:  0.75rem;   /* 12px — modals, large cards */
--radius-xl:  1rem;      /* 16px — panels */
--radius-full: 9999px;   /* Pills */
```

### Shadows
```css
--shadow-sm:   0 1px 2px rgba(0,0,0,0.3);
--shadow-md:   0 4px 6px -1px rgba(0,0,0,0.4);
--shadow-lg:   0 10px 15px -3px rgba(0,0,0,0.5);
--shadow-glow: 0 0 20px rgba(59,130,246,0.15);
```

---

## 3. Layout Architecture

### App Shell
```
┌─────────────────────────────────────────────────────────┐
│ ┌────────┐ ┌─────────────────────────────────────────┐  │
│ │        │ │ Top Bar                                 │  │
│ │        │ │ ┌─────────┐  ┌──────┐  ┌─────────────┐│  │
│ │  Side  │ │ │Breadcrumb│  │Search│  │Status • User ││  │
│ │  bar   │ │ └─────────┘  └──────┘  └─────────────┘│  │
│ │        │ ├─────────────────────────────────────────┤  │
│ │  ⚡    │ │                                         │  │
│ │        │ │                                         │  │
│ │ 📊 Dash│ │         Main Content Area               │  │
│ │ 💻 Con │ │                                         │  │
│ │ 🔍 Exp │ │         (Scrollable)                    │  │
│ │ 📈 Mon │ │                                         │  │
│ │ ⚙️ Set │ │                                         │  │
│ │        │ │                                         │  │
│ │        │ │                                         │  │
│ │ ───── │ │                                         │  │
│ │ 🟢 OK  │ │                                         │  │
│ └────────┘ └─────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘

Sidebar: 240px expanded, 60px collapsed (icon-only)
Top Bar: 56px height, sticky
```

---

## 4. Page Designs

### 4.1 Dashboard (`/`)
```
┌─────────────────────────────────────────────────┐
│ Dashboard                              ⌘K Search│
├─────────────────────────────────────────────────┤
│                                                  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌─────┐│
│  │ 📊 Keys  │ │ 💾 Memory│ │ ⚡ Ops/s │ │ ⏱️  ││
│  │ 12,847   │ │ 24.3 MB  │ │ 48,291   │ │2d5h ││
│  │ +2.4%  ↑ │ │ +0.8% ↑  │ │ -1.2% ↓  │ │     ││
│  └──────────┘ └──────────┘ └──────────┘ └─────┘│
│                                                  │
│  ┌──────────────────────┐ ┌────────────────────┐│
│  │ Operations / Second   │ │ Memory Usage       ││
│  │ ┌───────────────────┐│ │ ┌────────────────┐ ││
│  │ │ ▁▃▅▇█▇▅▃▁▃▅▇█▇▅ ││ │ │ ▇▇▇▇░░░░░░░░ │ ││
│  │ │ Real-time chart   ││ │ │ 24.3MB / 256MB │ ││
│  │ └───────────────────┘│ │ └────────────────┘ ││
│  └──────────────────────┘ └────────────────────┘│
│                                                  │
│  ┌──────────────────────┐ ┌────────────────────┐│
│  │ Recent Commands       │ │ Key Distribution   ││
│  │ SET user:123 ...  2ms│ │ ┌────────────────┐ ││
│  │ GET session:...   0ms│ │ │   Treemap of    │ ││
│  │ ZADD leaders...  1ms│ │ │   namespaces    │ ││
│  │ DEL temp:...     0ms│ │ └────────────────┘ ││
│  └──────────────────────┘ └────────────────────┘│
└─────────────────────────────────────────────────┘
```

### 4.2 Console (`/console`)
```
┌─────────────────────────────────────────────────┐
│ Console                          Clear  History  │
├─────────────────────────────────────────────────┤
│ ┌───────────────────────────────────────────────┐│
│ │ flashdb> SET user:1 "John Doe"                ││
│ │ OK                                             ││
│ │                                                ││
│ │ flashdb> GET user:1                            ││
│ │ "John Doe"                                     ││
│ │                                                ││
│ │ flashdb> HSET user:1:meta email john@doe.com   ││
│ │ (integer) 1                                    ││
│ │                                                ││
│ │ flashdb> _                                     ││
│ │                                                ││
│ │                                                ││
│ └───────────────────────────────────────────────┘│
│                                                  │
│  ┌─ Quick Commands ─────────────────────────────┐│
│  │ PING  │ DBSIZE │ INFO │ KEYS * │ FLUSHDB    ││
│  └──────────────────────────────────────────────┘│
│                                                  │
│  ┌─ Auto-complete ──────────────────────────────┐│
│  │ SET   SETEX   SETNX   SETRANGE              ││
│  └──────────────────────────────────────────────┘│
└─────────────────────────────────────────────────┘
```

### 4.3 Key Explorer (`/explorer`)
```
┌─────────────────────────────────────────────────┐
│ Explorer            🔍 Search keys...    + Add   │
├───────────────────┬─────────────────────────────┤
│ Key Tree           │ Key Details                 │
│                    │                             │
│ 📁 user: (1,247)  │ user:123                    │
│  ├─ user:1        │ ┌─────────────────────────┐ │
│  ├─ user:2        │ │ Type:  string           │ │
│  └─ user:3        │ │ TTL:   -1 (no expiry)   │ │
│ 📁 session: (89)  │ │ Size:  42 bytes         │ │
│ 📁 cache: (3,401) │ │ Encoding: raw           │ │
│ 📁 counter: (12)  │ └─────────────────────────┘ │
│                    │                             │
│                    │ Value:                      │
│                    │ ┌─────────────────────────┐ │
│                    │ │ {"name": "John Doe",    │ │
│                    │ │  "email": "j@doe.com",  │ │
│                    │ │  "role": "admin"}       │ │
│                    │ └─────────────────────────┘ │
│                    │                             │
│                    │ [Copy] [Edit] [Delete] [TTL]│
│ ──────────────────│─────────────────────────────│
│ Showing 4,749 keys │                            │
└───────────────────┴─────────────────────────────┘
```

### 4.4 Monitoring (`/monitoring`)
```
┌─────────────────────────────────────────────────┐
│ Monitoring                    ● Live    [1h][6h] │
├─────────────────────────────────────────────────┤
│  ┌────────────────────────────────────────────┐  │
│  │ Throughput (ops/sec)                        │  │
│  │ ▁▂▃▄▅▆▇█▇▆▅▄▃▂▁▂▃▄▅▆▇█  48,291 current  │  │
│  └────────────────────────────────────────────┘  │
│                                                  │
│  ┌─ Connected Clients (3) ────────────────────┐  │
│  │ ID │ Address        │ Age   │ Idle │ Cmds  │  │
│  │ 1  │ 127.0.0.1:8234│ 2h 3m │ 0s   │ 12847│  │
│  │ 2  │ 127.0.0.1:8235│ 1h 2m │ 3s   │ 8234 │  │
│  │ 3  │ 192.168.1.5   │ 0h 5m │ 0s   │ 291  │  │
│  └────────────────────────────────────────────┘  │
│                                                  │
│  ┌─ Slow Query Log ──────────────────────────┐   │
│  │ Duration │ Command                │ Time   │   │
│  │ 23ms     │ KEYS *                │ 14:23  │   │
│  │ 12ms     │ ZRANGEBYSCORE ...     │ 14:21  │   │
│  └───────────────────────────────────────────┘   │
└─────────────────────────────────────────────────┘
```

---

## 5. Component Library

### Core UI Components
| Component | Purpose | Variants |
|-----------|---------|----------|
| `Button` | Actions | `primary`, `secondary`, `ghost`, `danger` |
| `Input` | Text entry | `default`, `search`, `code` |
| `Badge` | Status labels | `default`, `success`, `warning`, `error` |
| `Card` | Content containers | `default`, `interactive`, `stat` |
| `Table` | Data display | Sortable, filterable |
| `Modal` | Overlays | `default`, `alert`, `fullscreen` |
| `Tooltip` | Hover info | Top, bottom, left, right |
| `Tabs` | Section switching | `underline`, `pill` |
| `Select` | Dropdowns | Single, multi |
| `Toast` | Notifications | `success`, `error`, `info` |
| `Kbd` | Keyboard shortcuts | — |
| `CommandPalette` | ⌘K search | — |
| `Sidebar` | Navigation | Expandable/collapsible |

### Chart Components
| Component | Purpose |
|-----------|---------|
| `AreaChart` | Time-series metrics (ops/sec, memory) |
| `BarChart` | Key distribution, command frequency |
| `Treemap` | Namespace size visualization |
| `Sparkline` | Inline micro-charts in stat cards |
| `GaugeChart` | Memory utilization, connection usage |

---

## 6. Interaction Patterns

### Command Palette (⌘K)
- Global search across keys, commands, documentation, settings
- Fuzzy matching with highlighted results
- Recent actions section
- Keyboard-navigable (↑↓ to select, Enter to execute)

### Keyboard Shortcuts
| Shortcut | Action |
|----------|--------|
| `⌘K` | Open command palette |
| `⌘/` | Focus console |
| `⌘1` | Go to Dashboard |
| `⌘2` | Go to Console |
| `⌘3` | Go to Explorer |
| `⌘4` | Go to Monitoring |
| `Esc` | Close modal / clear search |

### Animation Guidelines
- **Page transitions**: 200ms ease-out opacity + translateY(8px)
- **Modal entrance**: 150ms ease-out scale(0.98→1) + opacity
- **Hover states**: 150ms ease color transitions
- **Charts**: 300ms ease-out entrance animation
- **Number counters**: Spring animation for value changes
- **Skeleton loading**: Shimmer animation for async data

---

## 7. Responsive Breakpoints

| Breakpoint | Width | Layout |
|-----------|-------|--------|
| Mobile | <768px | Sidebar hidden, bottom nav |
| Tablet | 768-1024px | Collapsed sidebar (icons only) |
| Desktop | 1024-1440px | Full sidebar |
| Wide | >1440px | Full sidebar + wider content |

---

## 8. Accessibility Requirements

- WCAG 2.1 AA compliance
- All interactive elements keyboard accessible
- Focus visible indicators on all focusable elements
- Screen reader compatible (ARIA labels, roles)
- Color contrast ratio >4.5:1 for text
- Reduced motion support (`prefers-reduced-motion`)
- Semantic HTML throughout

---

*This specification is the definitive guide for frontend implementation. All UI decisions should reference this document.*
