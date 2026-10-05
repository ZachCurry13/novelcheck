// NovelCheck app bootstrap: session check, hash router, PWA install prompt.
import { get, post, setUnauthorizedHandler } from "./api.js";
import { $, $$, attempt, canManage, can } from "./ui.js";
import { renderLibrary } from "./library.js";
import { renderCheck } from "./check.js";
import { renderQueue } from "./queue.js";
import { renderImport } from "./scanner.js";
import { renderAdmin } from "./admin.js";
import { renderProfile } from "./profile.js";
import { renderWhatsNew } from "./whatsnew.js";
import { openGuide } from "./guide.js";
import { checkForUpdates } from "./updatebanner.js";
import { showSafeMode } from "./safemodebanner.js";
import { applyAppearance } from "./appearance.js";
import { renderSystem } from "./system.js";
import { renderUsage } from "./usage.js";
import { renderDuplicates } from "./duplicates.js";
import { renderDeletions } from "./deletions.js";
import { renderDeepScanAdmin } from "./deepscanadmin.js";
import { renderWishlist } from "./wishlist.js";
import { renderDiscover } from "./discover.js";
import { renderShelf } from "./shelf.js";
import { renderCollections } from "./collections.js";
import { renderSeries } from "./series.js";
import { renderEvents } from "./events.js";
import { initBell } from "./notifications.js";
import { on, applyModules } from "./modules.js";
import { buildMobileNav, markMobileNav } from "./mobilenav.js";
import { keepUpToDate } from "./appupdate.js";
import { initActivity } from "./activity.js";
import { showFamily } from "./family.js";

export const state = { user: null };

const routes = {
  check: renderCheck,
  library: renderLibrary,
  queue: renderQueue,
  import: renderImport,
  admin: renderAdmin,
  profile: renderProfile,
  whatsnew: renderWhatsNew,
  system: renderSystem,
  usage: renderUsage,
  duplicates: renderDuplicates,
  deletions: renderDeletions,
  deepscan: renderDeepScanAdmin,
  wishlist: renderWishlist,
  discover: renderDiscover,
  shelf: renderShelf,
  collections: renderCollections,
  series: renderSeries,
  events: renderEvents,
};
const managerRoutes = new Set(["check", "import", "shelf", "admin", "duplicates"]);
// Admin pages and the area each needs (the main admin gives areas).
const ROUTE_AREA = { system: "system", usage: "system", deletions: "calibre", deepscan: "deep" };
const moduleRoutes = { queue: "queue", import: "import", shelf: "import" }; // pages an admin can turn off
// Pages without a menu item of their own light up the one they belong to.
const NAV_OF = { series: "collections", wishlist: "discover", events: "discover", import: "profile", shelf: "profile", deepscan: "admin", usage: "admin", system: "admin",
  duplicates: "admin", deletions: "admin" };

function showOnly(id) {
  for (const v of ["#setup-view", "#login-view", "#family-view", "#app-view"]) $(v).classList.toggle("hidden", v !== id);
}

function showLogin(family = false) {
  state.user = null;
  $("#back-to-family").classList.toggle("hidden", !family);
  showOnly("#login-view");
}

// toStartPage drops the page from the address, so route() opens the
// person's start page (after signing in, or switching profile).
function toStartPage() {
  if (location.hash) history.replaceState(null, "", location.pathname + location.search);
}

// launchedFromHomeScreen: the installed app opened fresh (not a reload). Older
// installs open on "#/library", their start address before 1.30.
function launchedFromHomeScreen() {
  const standalone = matchMedia("(display-mode: standalone)").matches || navigator.standalone;
  return standalone && performance.getEntriesByType?.("navigation")[0]?.type === "navigate";
}

// showSignIn opens "Who's reading?" on a family device, else the sign-in form.
async function showSignIn() {
  state.user = null;
  const signedIn = (u) => {
    state.user = u;
    toStartPage();
    showApp();
  };
  if (await showFamily(signedIn, () => showLogin(true))) showOnly("#family-view");
  else showLogin();
}

function showApp() {
  showOnly("#app-view");
  document.body.dataset.role = state.user.role;
  document.body.dataset.areas = (state.user.areas || []).join(" ");
  applyAppearance(state.user);
  $$(".manager-only").forEach((el) => el.classList.toggle("hidden", !canManage(state.user)));
  $$(".admin-only").forEach((el) => el.classList.toggle("hidden", state.user.role !== "admin"));
  applyModules(state.user);
  initBell(state);
  initActivity(state);
  $("#admin-tab").textContent = state.user.role === "admin" ? "Admin" : "Manage";
  $("#logout-btn").textContent = state.user.family_device ? "👥 Switch profile" : "Sign out";
  buildMobileNav();
  route();
  if (!state.user.guide_seen) openGuide(state);
  checkForUpdates(state);
  showSafeMode(state.user);
}

let currentCleanup = null;
async function route() {
  if (!state.user) return;
  // Everyone lands on their chosen start page; by default parents on Check a
  // book (the main feature), kids on the Library.
  let name = (location.hash.replace(/^#\/?/, "").split("?")[0]) || state.user.start_page || (canManage(state.user) ? "check" : "library");
  if (!routes[name] || (managerRoutes.has(name) && !canManage(state.user)) ||
    (ROUTE_AREA[name] && !can(state.user, ROUTE_AREA[name])) ||
    (moduleRoutes[name] && !on(state.user, moduleRoutes[name]))) name = "library";
  const menu = NAV_OF[name] || name;
  $$("#nav .nav-link").forEach((a) => a.classList.toggle("active", a.dataset.route === menu));
  markMobileNav(menu);
  if (typeof currentCleanup === "function") currentCleanup();
  // Fresh container per route so listeners never leak between views.
  const view = document.createElement("div");
  $("#view").replaceChildren(view);
  currentCleanup = await routes[name](view, state);
}

async function boot() {
  setUnauthorizedHandler(showSignIn);
  $("#back-to-family").addEventListener("click", showSignIn);
  $("#login-form").addEventListener("submit", async (e) => {
    e.preventDefault();
    const fd = new FormData(e.target);
    const err = $("#login-error");
    err.classList.add("hidden");
    try {
      state.user = await post("/api/auth/login", {
        username: fd.get("username"),
        password: fd.get("password"),
        remember: fd.get("remember") === "on",
      });
      e.target.reset();
      toStartPage();
      showApp();
    } catch (ex) {
      err.textContent = ex.message;
      err.classList.remove("hidden");
    }
  });
  $("#logout-btn").addEventListener("click", async () => {
    await attempt(() => post("/api/auth/logout"));
    showSignIn();
  });
  window.addEventListener("hashchange", route);
  for (const id of ["#help-btn", "#help-link"]) {
    $(id).addEventListener("click", (e) => {
      e.preventDefault();
      openGuide(state);
    });
  }
  $("#report-link").addEventListener("click", (e) => {
    e.preventDefault();
    import("./problems.js").then((m) => m.openReportProblem());
  });
  setupInstallPrompt();

  $("#setup-form").addEventListener("submit", submitSetup);

  try {
    state.user = await get("/api/me");
    if (launchedFromHomeScreen() && location.hash === "#/library") toStartPage();
    showApp();
  } catch {
    const setup = await get("/api/setup").catch(() => ({}));
    if (setup.needed) showOnly("#setup-view");
    else showSignIn();
  }
}

// First-run: create the admin account from the browser, then sign in.
async function submitSetup(e) {
  e.preventDefault();
  const fd = new FormData(e.target);
  const err = $("#setup-error");
  err.classList.add("hidden");
  if (fd.get("password") !== fd.get("confirm")) {
    err.textContent = "The two passwords don't match.";
    err.classList.remove("hidden");
    return;
  }
  try {
    state.user = await post("/api/setup", { username: fd.get("username"), password: fd.get("password") });
    e.target.reset();
    showApp();
  } catch (ex) {
    err.textContent = ex.message;
    err.classList.remove("hidden");
    if (ex.status === 409) setTimeout(showLogin, 1500);
  }
}

export async function refreshUser() {
  state.user = await get("/api/me");
  applyModules(state.user);
  buildMobileNav();
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
  keepUpToDate();
}

boot();
