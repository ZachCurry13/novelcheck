/** Tailwind config: scans the embedded HTML/JS so only used classes ship. */
// The colors the app uses are CSS variables (set in web/tailwind.input.css),
// so the light theme swaps them all, see-through variants (bg-slate-800/60)
// included, without touching any page.
const shades = [50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950];
const themed = ["slate", "indigo", "sky", "emerald", "amber", "rose", "orange", "teal", "violet"];
const colors = Object.fromEntries(themed.map((f) => [f, Object.fromEntries(shades.map((s) => [s, `rgb(var(--${f}-${s}) / <alpha-value>)`]))]));

module.exports = {
  content: ["./web/static/**/*.html", "./web/static/js/**/*.js"],
  darkMode: "class",
  theme: { extend: { colors } },
  plugins: [],
};
