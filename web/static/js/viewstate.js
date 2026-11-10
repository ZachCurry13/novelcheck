// Making the browser's Back button take you back to where you were: the scroll position of the
// page you return to, and (for the Library) its search and filters. Going to a page by a link
// starts at the top as before; only Back and Forward restore.
const SCROLL = "nc:scroll:";
const FORM = "nc:form:";
let backAt = 0; // when the last Back or Forward happened
let leaving = location.hash; // the page on screen until the next page change
let back = false; // whether the page being drawn now was reached with Back or Forward

// beginRoute is called as a page change starts: it decides whether it was Back or Forward.
export function beginRoute() {
  back = Date.now() - backAt < 1500;
  backAt = 0;
  // The page being left is still on screen: keep where it was scrolled to.
  if (leaving && window.scrollY > 0 && leaving !== location.hash) {
    try {
      sessionStorage.setItem(SCROLL + leaving, String(Math.round(window.scrollY)));
    } catch {
      /* private mode: not kept */
    }
  }
  leaving = location.hash;
}

// isBack says whether the page being drawn was reached with Back or Forward.
export const isBack = () => back;

export function startViewState() {
  try {
    history.scrollRestoration = "manual"; // the app draws pages itself, so it restores the place itself
  } catch {
    /* an old browser: it keeps its own behaviour */
  }
  // A tapped link to another #/page also fires popstate in some browsers (iPhone Safari): that is not Back.
  let interactAt = 0;
  for (const ev of ["click", "keydown"]) {
    window.addEventListener(ev, () => (interactAt = Date.now()), { capture: true, passive: true });
  }
  window.addEventListener("popstate", () => {
    if (Date.now() - interactAt > 700) backAt = Date.now();
  });
  let pending = null;
  let timer;
  window.addEventListener("scroll", () => {
    pending = { key: SCROLL + location.hash, y: Math.round(window.scrollY) }; // the page it belongs to, now
    clearTimeout(timer);
    timer = setTimeout(() => {
      try {
        sessionStorage.setItem(pending.key, String(pending.y));
      } catch {
        /* private mode: not kept */
      }
    }, 150);
  }, { passive: true });
}

// restoreScroll goes back to where the page was left (after Back), or to the top.
export function restoreScroll() {
  let y = 0;
  const wasBack = back;
  back = false;
  if (wasBack) {
    try {
      y = Number(sessionStorage.getItem(SCROLL + location.hash)) || 0;
    } catch {
      y = 0;
    }
  }
  window.scrollTo(0, y);
  if (y) setTimeout(() => window.scrollTo(0, y), 300); // lists that fill in a moment later
}

// saveForm keeps a form's fields (search, filters) for when the page is come back to.
export function saveForm(key, form) {
  try {
    const out = {};
    for (const el of form.elements) {
      if (!el.name || el.type === "button" || el.type === "submit") continue;
      if (el.type === "checkbox") {
        out[el.name] = out[el.name] || [];
        if (el.checked) out[el.name].push(el.value);
      } else out[el.name] = el.value;
    }
    sessionStorage.setItem(FORM + key, JSON.stringify(out));
  } catch {
    /* private mode: not kept */
  }
}

// restoreForm puts the kept fields back; it says whether there was anything.
export function restoreForm(key, form) {
  let saved = null;
  try {
    saved = JSON.parse(sessionStorage.getItem(FORM + key) || "null");
  } catch {
    saved = null;
  }
  if (!saved) return false;
  for (const el of form.elements) {
    if (!el.name || !(el.name in saved) || el.type === "button" || el.type === "submit") continue;
    if (el.type === "checkbox") el.checked = Array.isArray(saved[el.name]) && saved[el.name].includes(el.value);
    else el.value = saved[el.name];
  }
  return true;
}
