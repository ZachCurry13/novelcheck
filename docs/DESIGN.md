# NovelCheck design guide

NovelCheck follows **Material Design 3** (m3.material.io), Google's published guide for Android apps. This page says how each Material 3 rule is applied here, so every screen is built from the same pieces. When a screen needs something this page doesn't cover, add it here first, then build it.

Status: written for v1.71 (the all-screens redesign). Until that release, the app still uses the older styles.

## 0. Direction (from the unslop review, 2026-11-12)

The owner asked for the [unslop](../.claude/skills/unslop/SKILL.md) skill (by Damir Sokolovsky, CC BY 4.0) to guide the restyle. Its direction for NovelCheck, agreed with the owner:

- **Product.** Tells a family how much romance and what content is in each book before someone reads it, and keeps track of what everyone reads.
- **Character.** Careful, warm, bookish.
- **World.** A home library in the evening: paper, cloth spines, a reading lamp.
- **Moments.** The book covers, and the 0 to 5 level colours, which carry meaning. Everything else is quiet.
- **Signature.** The coloured level square (`levelBadge` in `js/peppers.js`), used wherever a level shows, like a library spine label.

What that means in practice: surfaces are a neutral graphite ramp (not navy), the main button is an off-white fill (dark fill on the light theme), the indigo accent marks only the selection, links and a key figure, emoji never decorate headings, buttons, tabs or status (they stay only where they are the content: trophies, awards, Wrapped, content flags), headings get their weight from size (page titles 28px at 500, `.page-title`), labels are sentence case, interface text has no exclamation marks, and no text is smaller than 11px. Material's fully round buttons are kept (the owner's choice of Material Design 3); unslop's smaller corners apply to cards and fields.

## 1. How it is wired

- Styles live in `web/tailwind.input.css` and compile to `web/static/css/app.css` with `make css`.
- Pages never choose colours, corners or sizes themselves. They use the shared classes in section 5 (`btn-filled`, `card`, `chip`, ...). A page that needs raw Tailwind colour or radius classes is a sign a shared piece is missing.
- Colours are CSS variables, so the light and dark themes, and any later accent colour, change in one place.
- The old class names (`btn-primary`, `btn-secondary`, `btn-ghost`, `nav-link`) stay as aliases of the new ones while screens are moved over, and are removed at the end of v1.71.

## 2. Colour

Material 3 names colours by **role**, not by shade. Each role has an "on" colour for text and icons placed on it.

| Role | Used for | Dark | Light |
|---|---|---|---|
| `primary` / `on-primary` | The one main action on a screen, active tab, links | indigo 200 / indigo 900 | indigo 600 / white |
| `primary-container` / `on-primary-container` | Selected navigation item, tonal emphasis | indigo 800 / indigo 100 | indigo 100 / indigo 900 |
| `secondary-container` / `on-secondary-container` | Tonal buttons, selected chips | slate 700 / slate 100 | slate 200 / slate 900 |
| `surface` / `on-surface` | Page background / main text | slate 950 / slate 100 | near-white / slate 900 |
| `surface-container-low`, `-container`, `-high`, `-highest` | Cards, bars, sheets, dialogs, fields (each one step lighter in dark, darker in light) | slate 900 to slate 700 | white to slate 200 |
| `on-surface-variant` | Secondary text, icons | slate 400 | slate 600 |
| `outline` / `outline-variant` | Field borders / dividers | slate 500 / slate 800 | slate 400 / slate 200 |
| `error` / `error-container` | Destructive actions, failures | rose 300 / rose 900 | rose 700 / rose 100 |

- The seed colour stays **indigo**, so the app keeps its identity.
- Depth is shown by the surface-container steps, not by shadows. Shadows are kept for things that float: menus, the search results list, snackbars.
- **Meaning colours stay outside the theme**: pepper levels and content chips keep emerald (none), amber (closed door), rose (open door), teal (Deep Scan), violet (flags), sky (categories). They are information, not decoration, and must read the same on every screen.
- Text contrast is at least 4.5:1 on its surface; large text and icons at least 3:1.

## 3. Type

System font stack (no download; OpenDyslexic replaces it for people who choose it). Sizes follow the Material 3 scale:

| Style | Size / line / weight | Used for |
|---|---|---|
| Headline small | 24 / 32 / 400 | Page title in the top bar on wide screens, big numbers |
| Title large | 22 / 28 / 400 | Page title on phones, dialog titles |
| Title medium | 16 / 24 / 500 | Card titles, list row titles |
| Title small | 14 / 20 / 500 | Section headings inside a card |
| Body large | 16 / 24 / 400 | Reading text, field text |
| Body medium | 14 / 20 / 400 | Default text |
| Body small | 12 / 16 / 400 | Supporting text, timestamps |
| Label large | 14 / 20 / 500 | Buttons, tabs |
| Label medium | 12 / 16 / 500 | Chips, navigation labels |

Sentence case everywhere. No all-capitals labels (the old `.label` style goes).

## 4. Shape, spacing, touch, motion

- **Corners:** 4 (text fields), 8 (chips, menus), 12 (cards), 16 (large cards, navigation drawer), 28 (dialogs, sheets, search view), full (buttons, search bar, badges).
- **Spacing:** a 4-point grid. 16 at the screen edge on phones, 24 on wide screens; 8 between related things, 16 between groups, 24 between sections.
- **Touch targets:** at least 48 by 48, even when the visible control is smaller.
- **State layers:** hover adds 8% of the content colour, focus and press 10%. Focus always shows a visible ring for keyboard users.
- **Motion:** 200 ms standard easing for small changes, 300 ms emphasised easing for sheets and dialogs. All of it is off under Reduce motion.
- **Breakpoints:** compact (under 600), medium (600 to 839), expanded (840 and up).

## 5. Components (one definition each)

| Piece | Class | Rule |
|---|---|---|
| **Top bar** | `top-bar` | 64 high, `surface`, turns `surface-container` once the page scrolls. Left: page title. Middle or right: search (section 6). Right: notifications, then the account menu (Help, What's new, Sign out). No navigation links in it. |
| **Navigation bar** (compact) | `nav-bar` | Bottom, 80 high, 4 or 5 destinations, icon over label, a pill behind the active icon. The last one is **More**. |
| **Navigation drawer** (768 and wider) | `#nav`, `nav-link` | Left side, 216 wide, every page as a 48-high row with an icon and a pill behind the open one. (A rail holds at most seven pages; NovelCheck has eleven.) |
| **Tabs** | `tabs`, `tab` | Primary tabs: 48 high, label large, 3-high indicator under the active tab, scroll sideways when they don't fit. One strip per page (`tabs.js`). |
| **Buttons** | `btn-filled`, `btn-tonal`, `btn-outlined`, `btn-text`, `btn-danger` | 40 high, full round, label large. One filled button per screen or dialog (the main action). Tonal for the next most important, outlined or text for the rest. `btn-danger` is filled in `error`. |
| **Icon button** | `icon-btn` | 40 round with a 48 target and an `aria-label`. |
| **FAB** | `fab` | 56, corner 16, `primary-container`. Only where a screen has one constructive action (Check a book: take a photo). |
| **Cards** | `card` (filled), `card-outlined`, `card-elevated` | Corner 12, padding 16. Filled (`surface-container-low`) is the default; outlined for cards inside cards; elevated only for things that float. |
| **Chips** | `chip-assist`, `chip-filter`, `chip-input`, `chip-info` | 32 high (24 for `chip-info` inside rows), corner 8. Filter chips show a tick when on. `chip-info` carries the meaning colours of section 2. |
| **Lists** | `list`, `list-row` | Rows 56 (one line) or 72 (two lines): leading icon or cover, title medium, supporting text in body medium, trailing action. Dividers in `outline-variant`. |
| **Text fields** | `field` | Outlined, 50 high, corner 4, the label above the field. Supporting or error text below. 16 px text so phones don't zoom. |
| **Switch, checkbox, radio** | `switch`, `check`, `radio` | Switch for settings that apply at once; checkbox for picking several things from a list. |
| **Dialogs** | `dialog` | Corner 28, `surface-container-low`, title large, actions bottom right as text buttons with the confirming one last. Compact screens: a bottom sheet with a drag handle and the same content. |
| **Menus** | `menu` | Corner 8, `surface-container`, rows 48 high. |
| **Snackbar** | `snackbar` | Replaces the toast: bottom, above the navigation bar, one line plus an optional action (Undo). Errors stay until dismissed. |
| **Progress** | `progress-bar`, `progress-ring` | Bar 4 high; ring for goals and "Next up". Unknown length uses the moving form. |
| **Badges** | `badge` | Small `error` dot or count on an icon (notifications, tab counts). |
| **Empty and error states** | `state` | Icon, one sentence saying what is going on, one button for the next step. |

**Icons:** Material Symbols (Rounded, Apache 2.0), as a small font in `web/static/fonts` that holds only the icons the app uses, so nothing loads from outside. Write `<span class="ms" aria-hidden="true">icon_name</span>`; a new icon means fetching the font again with its name added. Icons in the font: account_circle, add, admin_panel_settings, arrow_back, bar_chart, barcode_scanner, bed, bug_report, check, close, collections_bookmark, delete, download, expand_more, explore, filter_list, group, help, local_library, logout, menu, more_vert, notifications, person, photo_camera, search, settings, star. Emoji stay only where they are content (pepper levels, trophies), not as button or menu icons.

## 6. Search in the top bar

A full-round search bar, 56 high on the page, opening a search view (full screen on phones, a docked list on wide screens). It finds **settings, admin tools, pages and actions**: not books, people or collections, which keep their own search on Library. Each result is a list row with an icon, a name, and where it lives ("Admin → Library cleanup"). Admin results show only to people who can open them.

## 7. Page patterns

- **Reading stats: one page, five tabs**: Highlights, Books, Calendar, Records, Trophies.
  - *Records:* period tiles (week, month, year, all time) each with the change against the period before and **Play it as Wrapped**; personal bests as a list.
  - *Trophies:* "N of M earned" with a progress bar; **Next up** with rings and an estimated date; named milestones as list rows with a tick when earned.
- **Settings and Admin:** tabs across the top, one card per subject, a sticky Save bar when something has changed.
- **Lists of books** (Library, Nightstand, Discover, Collections): the same book card and the same book row everywhere; filters as filter chips with the full set in a sheet.
- **Reading view** (planned, after v1.71): the one screen that hides the bars. Page turns by tapping the sides, the menu by tapping the top, a slim status line at the bottom. Its menus and sheets use the pieces above.

## 8. Checks before a release

1. Every screen at 360 wide, 768 wide and 1280 wide, in dark and light.
2. Keyboard only: every control reachable, focus visible.
3. OpenDyslexic and Reduce motion on.
4. No raw colour or radius classes left in page code (search `web/static/js` for `bg-slate-`, `rounded-xl`, `ring-slate-`).
