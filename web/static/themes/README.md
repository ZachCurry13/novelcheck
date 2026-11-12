# Themes

Everything that decides how NovelCheck looks, as opposed to how it is laid out, lives in this folder:

- `ember.css`: the dark theme (the default).
- `paperback.css`: the light theme (on when someone picks Light, or their device is in light mode).
- `classic.css`: Classic blue, the navy and indigo from before v1.73, dark and light (picked in Settings → Appearance).
- `fonts.css`: the fonts the themes name, stored in `web/static/fonts` (nothing loads from outside).

To change a colour, edit the number in one of these files. Colours are written as `R G B` (0 to 255) so the
rest of the app can make see-through versions of them; `241 231 214` is `#f1e7d6`. These files are plain CSS
served as they are, so a change needs no `make css`; it ships with the next release like any other file.

What each name changes:

| Name | What it colours |
| --- | --- |
| `--slate-950` | the page |
| `--slate-900` | cards, the bottom bar, pop-up windows |
| `--slate-800` | raised things (secondary buttons, fields) and thin lines |
| `--slate-400` | secondary text (authors, notes, labels) |
| `--slate-100` | main text |
| `--indigo-300` | links, the selected page in the menu, small highlights |
| `--indigo-800`, `--indigo-900` | tinted backgrounds behind highlights |
| `--nc-button`, `--nc-on-button` | the main button on each screen, and its text |
| `--nc-chip-on`, `--nc-on-chip-on` | a selected filter chip |
| `--nc-banner`, `--nc-on-banner`, `--nc-banner-bar` | the "Read this year" banner on the Library |
| `--nc-font-head`, `--nc-font-body` | heading font, and the font for everything else |

The other ramps (`slate` and `indigo` here are names kept from the first design; they are now the warm
neutrals and the orange accent) are shared and not themed: the 0 to 5 level colours, and the green, amber, red,
teal, violet and sky used for content and status, stay the same in every theme because they carry meaning.

To add a third theme, copy `ember.css`, change the selector (for example `html.sepia`) and the values, add a
`<link>` for it in `index.html` and `reader.html`, and an option for it in Settings → Appearance (`js/appearance.js`).

The direction behind these choices is in `docs/DESIGN.md`.
