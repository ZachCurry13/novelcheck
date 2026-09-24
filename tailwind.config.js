/** Tailwind config: scans the embedded HTML/JS so only used classes ship. */
module.exports = {
  content: ["./web/static/**/*.html", "./web/static/js/**/*.js"],
  darkMode: "class",
  theme: { extend: {} },
  plugins: [],
};
