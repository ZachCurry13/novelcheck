// How NovelCheck looks for each person (Profile → Appearance): the theme
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
  const light = current.theme === "light" || (!current.theme && !darkDevice.matches);
  root.classList.toggle("light", light);
  root.classList.toggle("font-dyslexic", current.font === "dyslexic");
  root.classList.toggle("reduce-motion", current.motion === "reduce");
  document.querySelector('meta[name="theme-color"]')?.setAttribute("content", light ? "#f1f5f9" : "#0f172a");
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
