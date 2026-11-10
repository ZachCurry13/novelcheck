// The full-screen story player for Wrapped: a gradient slide at a time, progress segments across the top,
// tap the right side (or press →) to go on, the left side (or ←) to go back, ✕ or Esc to leave.
const THEMES = [
  ["#4f46e5", "#0f172a"], ["#be185d", "#1e1b4b"], ["#0f766e", "#0f172a"], ["#b45309", "#1c1917"],
  ["#7c3aed", "#0f172a"], ["#0369a1", "#082f49"], ["#15803d", "#052e16"], ["#c2410c", "#1c1917"],
];

// openStory(slides, { onShare }): slides are HTML strings. Returns nothing; closes itself.
export function openStory(slides, { onShare, start = 0 } = {}) {
  let at = Math.max(0, Math.min(start, slides.length - 1));
  const root = document.createElement("div");
  root.className = "story-full";
  root.setAttribute("role", "dialog");
  root.setAttribute("aria-label", "Wrapped");
  root.tabIndex = -1;
  const prevFocus = document.activeElement;
  const close = () => {
    document.removeEventListener("keydown", onKey, true);
    document.body.classList.remove("story-open");
    root.remove();
    prevFocus?.focus?.({ preventScroll: true });
  };
  const draw = () => {
    const [a, b] = THEMES[at % THEMES.length];
    root.style.background = `radial-gradient(120% 90% at 20% 0%, ${a}, ${b} 70%)`;
    root.innerHTML = `<div class="story-bars" aria-hidden="true">${slides.map((_, i) => `<span class="${i < at ? "done" : i === at ? "now" : ""}"></span>`).join("")}</div>
      <div class="story-top"><button type="button" class="story-btn" data-sgo="-1" aria-label="Back" ${at === 0 ? "disabled" : ""}>‹</button>
        <button type="button" class="story-btn" data-sclose aria-label="Close">✕</button></div>
      <div class="story-body">${slides[at]}</div>
      <button type="button" class="story-zone story-zone-l" data-sgo="-1" aria-label="Previous"></button>
      <button type="button" class="story-zone story-zone-r" data-sgo="1" aria-label="Next"></button>`;
    const donut = root.querySelector("[data-donut]");
    if (donut) donut.style.background = `conic-gradient(from 0deg,${donut.dataset.donut})`; // styles are set here: the page forbids inline style attributes
    root.querySelector(".story-body [data-share]")?.addEventListener("click", () => onShare?.());
    root.querySelector(".story-body [data-sdone]")?.addEventListener("click", close);
  };
  const go = (d) => {
    if (at + d >= slides.length) return close();
    at = Math.max(0, at + d);
    draw();
  };
  root.addEventListener("click", (e) => {
    if (e.target.closest("[data-sclose]")) return close();
    const g = e.target.closest("[data-sgo]");
    if (g) go(Number(g.dataset.sgo));
  });
  function onKey(e) {
    if (e.key === "Escape") { e.stopPropagation(); close(); }
    else if (e.key === "ArrowRight" || e.key === " ") { e.preventDefault(); go(1); }
    else if (e.key === "ArrowLeft") { e.preventDefault(); go(-1); }
  }
  document.addEventListener("keydown", onKey, true);
  document.body.classList.add("story-open");
  document.body.append(root);
  draw();
  root.focus({ preventScroll: true });
}
