// NovelCheck app bootstrap: session check, hash router, PWA install prompt.
import { get, post, setUnauthorizedHandler } from "./api.js";
import { $, $$, attempt } from "./ui.js";
import { renderLibrary } from "./library.js";
import { renderQueue } from "./queue.js";
import { renderImport } from "./scanner.js";
import { renderAdmin } from "./admin.js";
import { renderProfile } from "./profile.js";

export const state = { user: null };

const routes = {
  library: renderLibrary,
  queue: renderQueue,
  import: renderImport,
  admin: renderAdmin,
  profile: renderProfile,
};
const adminRoutes = new Set(["import", "admin"]);

function showLogin() {
  state.user = null;
  $("#app-view").classList.add("hidden");
  $("#login-view").classList.remove("hidden");
}

function showApp() {
  $("#login-view").classList.add("hidden");
  $("#app-view").classList.remove("hidden");
  const isAdmin = state.user.role === "admin";
  $$(".admin-only").forEach((el) => el.classList.toggle("hidden", !isAdmin));
  route();
}

let currentCleanup = null;
async function route() {
  if (!state.user) return;
  let name = (location.hash.replace(/^#\/?/, "").split("?")[0]) || "library";
  if (!routes[name] || (adminRoutes.has(name) && state.user.role !== "admin")) name = "library";
  $$("#nav .nav-link").forEach((a) => a.classList.toggle("active", a.dataset.route === name));
  if (typeof currentCleanup === "function") currentCleanup();
  // Fresh container per route so listeners never leak between views.
  const view = document.createElement("div");
  $("#view").replaceChildren(view);
  currentCleanup = await routes[name](view, state);
}

async function boot() {
  setUnauthorizedHandler(showLogin);
  $("#login-form").addEventListener("submit", async (e) => {
    e.preventDefault();
    const fd = new FormData(e.target);
    const err = $("#login-error");
    err.classList.add("hidden");
    try {
      state.user = await post("/api/auth/login", {
        username: fd.get("username"),
        password: fd.get("password"),
      });
      e.target.reset();
      showApp();
    } catch (ex) {
      err.textContent = ex.message;
      err.classList.remove("hidden");
    }
  });
  $("#logout-btn").addEventListener("click", async () => {
    await attempt(() => post("/api/auth/logout"));
    showLogin();
  });
  window.addEventListener("hashchange", route);
  setupInstallPrompt();

  try {
    state.user = await get("/api/me");
    showApp();
  } catch {
    showLogin();
  }
}

export async function refreshUser() {
  state.user = await get("/api/me");
  return state.user;
}

// Android/desktop Chrome fire beforeinstallprompt; iOS uses Share → Add to Home Screen.
function setupInstallPrompt() {
  let deferred = null;
  const btn = $("#install-btn");
  window.addEventListener("beforeinstallprompt", (e) => {
    e.preventDefault();
    deferred = e;
    btn.classList.remove("hidden");
  });
  btn.addEventListener("click", async () => {
    if (!deferred) return;
    deferred.prompt();
    await deferred.userChoice;
    deferred = null;
    btn.classList.add("hidden");
  });
  if ("serviceWorker" in navigator) {
    navigator.serviceWorker.register("/sw.js").catch(() => {});
  }
}

boot();
