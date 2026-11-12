// How NovelCheck looks for each person (Settings → Appearance): the theme
// (match the device, dark or light), the OpenDyslexic font, and reduced
// motion. Saved with the account, and on the device too, so the sign-in page
// and the first paint already look right.
const KEY = "nc:appearance";
const darkDevice = matchMedia("(prefers-color-scheme: dark)");
const calmDevice = matchMedia("(prefers-reduced-motion: reduce)");
let current = read();

function read() {
  try {
    return JSON.parse(localStorage.getItem(KEY) || "{}");
  } catch {
    return {};
  }
}

function paint() {
  const root = document.documentElement;
  // Theme is "" / "dark" / "light" (Paperback and Ember) or "blue" / "blue-dark" / "blue-light" (Classic blue).
  const blue = (current.theme || "").startsWith("blue");
  const mode = blue ? (current.theme.split("-")[1] || "") : current.theme || "";
  const light = mode === "light" || (!mode && !darkDevice.matches);
  root.classList.toggle("light", light);
  root.classList.toggle("theme-blue", blue);
  root.classList.toggle("font-dyslexic", current.font === "dyslexic");
  root.classList.toggle("reduce-motion", current.motion === "reduce");
  document.querySelector('meta[name="theme-color"]')?.setAttribute("content", blue ? (light ? "#f1f5f9" : "#020617") : light ? "#f6efe2" : "#17130f");
}

// applyAppearance uses the signed-in person's choices (or the device's last ones).
export function applyAppearance(user) {
  if (user) {
    current = { theme: user.theme || "", font: user.font || "", motion: user.motion || "" };
    try {
      localStorage.setItem(KEY, JSON.stringify(current));
    } catch {
      /* private mode */
    }
  }
  paint();
}

// reduceMotion is true when animations and smooth scrolling should be skipped.
export const reduceMotion = () => current.motion === "reduce" || calmDevice.matches;

darkDevice.addEventListener("change", paint);
paint();
