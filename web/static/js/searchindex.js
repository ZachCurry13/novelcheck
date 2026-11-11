// What the top-bar search (search.js) can find: pages, settings, admin tools and actions. Not books, people or
// collections: those have their own search on the Library page. Each entry is
// { name, where, icon, href | click, words }: href opens a page, click presses a button that is already on the page.
import { canHere, canManage } from "./ui.js";
import { tabsFor } from "./adminnav.js";

const shown = (a) => !a.classList.contains("hidden") && !a.classList.contains("module-off");
const label = (a) => [...a.childNodes].filter((n) => n.nodeType === 3).map((n) => n.textContent).join("").trim();

const STATS = [["report", "Highlights", "report week month year"], ["books", "Books", "read finished"], ["calendar", "Calendar", "days month"],
  ["records", "Records and Wrapped", "personal bests awards play wrapped year in review"], ["trophies", "Trophies", "milestones achievements levels next up"]];

// [section, name, extra words, managers only]
const SETTINGS = [
  ["general", "Add books", "import kindle drive goodreads amazon paper barcode", true],
  ["general", "Appearance: theme", "dark light mode colours"],
  ["general", "Appearance: font", "opendyslexic dyslexia text"],
  ["general", "Appearance: motion", "reduce animation"],
  ["general", "Start page", "home first page opens"],
  ["general", "“Start Reading” delivery", "send to kindle email koreader deliver"],
  ["general", "Notifications on this device", "push alerts bell"],
  ["account", "Change password", "sign in security"],
  ["account", "Sharing", "readers profile private reading now"],
  ["account", "Family devices", "who's reading pin kids tablet"],
  ["kindle", "KOReader setup", "kindle e-reader opds progress sync"],
  ["kindle", "KOReader plugin", "download install update kindle"],
  ["kindle", "Reading sync", "sync code webdav statistics kosync devices"],
  ["connections", "Hardcover", "token ratings goals want to read sync"],
  ["connections", "Libby libraries", "overdrive library card borrow"],
  ["connections", "Audiobookshelf", "audiobooks listen"],
];
const SECTION = { general: "General", account: "Account", kindle: "Kindle & KOReader", connections: "Connections" };

const PROFILE = [["My reading", "summary"], ["Your reading taste", "rate books likes"], ["Content rules on this account", "hidden filters peppers"]];

// Tools on Admin → Library cleanup.
const CLEANUP = [["Books listed twice in NovelCheck", "join same book"], ["Duplicates in Calibre", "copies remove keep"], ["Tidy titles in Calibre", "rename numbers"],
  ["Formats", "epub convert remove other formats"], ["Genres", "tags fill"], ["Box sets", "split omnibus"], ["Books with no cover", "find covers"],
  ["Covers reported as wrong", "review"], ["Read the Calibre library now", "sync refresh re-read"], ["Calibre's recycle bin", "put back restore removed trash"]];
// Words that should also find an admin tab.
const ADMIN_WORDS = { ai: "model ollama api key rating errors spent cost tokens", users: "accounts kids family roles rules add person", cleanup: "duplicates formats covers calibre",
  delivery: "email smtp calibre server content server remote access tunnel", system: "features turn off modules backup safe mode updates", "@deepscan": "whole book epub review",
  "@usage": "cost tokens spend", "@system": "checks health services versions" };

export function entries(user) {
  const out = [];
  const add = (name, where, icon, target, words = "") => out.push({ name, where, icon, words, ...target });
  for (const a of document.querySelectorAll("#nav .nav-link")) {
    if (shown(a)) add(label(a), "Page", a.querySelector(".ms")?.textContent || "menu", { href: a.getAttribute("href") });
  }
  add("What's new", "Page", "star", { href: "#/whatsnew" }, "release notes version changes");
  for (const [k, name, words] of STATS) add(name, "Stats", "bar_chart", { href: `#/stats?tab=${k}` }, words);
  for (const [s, name, words, managers] of SETTINGS) {
    if (!managers || canManage(user)) add(name, `Settings → ${SECTION[s]}`, "settings", { href: `#/settings?s=${s}` }, words);
  }
  for (const [name, words] of PROFILE) add(name, "Profile", "person", { href: "#/profile" }, words);
  if (canManage(user)) {
    const strip = (t) => t.replace(/^[^\p{L}]+/u, "");
    for (const [k, , text] of tabsFor()) {
      add(strip(text), "Admin", "admin_panel_settings", { href: k.startsWith("@") ? `#/${k.slice(1)}` : `#/admin?tab=${k}` }, ADMIN_WORDS[k] || "");
    }
    if (canHere("calibre")) {
      for (const [name, words] of CLEANUP) add(name, "Admin → Library cleanup", "admin_panel_settings", { href: "#/admin?tab=cleanup" }, words);
      add("Delete requests", "Admin → Library cleanup", "delete", { href: "#/deletions" }, "remove books family asked");
    }
  }
  add("How to use NovelCheck", "Action", "help", { click: "#help-btn" }, "help guide tour");
  add("Report a problem or idea", "Action", "bug_report", { click: "#report-link" }, "bug feedback");
  add("Notifications", "Action", "notifications", { click: "#bell-btn" }, "bell alerts");
  add(document.getElementById("logout-btn")?.textContent.trim() || "Sign out", "Action", "logout", { click: "#logout-btn" }, "log out switch profile");
  return out;
}

// find: every word of the query must appear somewhere in the entry; names that start with the query come first.
export function find(list, query) {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (!words.length) return list.filter((e) => e.where === "Page");
  const q = words.join(" ");
  return list.map((e) => ({ e, name: e.name.toLowerCase(), all: `${e.name} ${e.where} ${e.words}`.toLowerCase() }))
    .filter((x) => words.every((w) => x.all.includes(w)))
    .sort((a, b) => (b.name.startsWith(q) - a.name.startsWith(q)) || (b.name.includes(q) - a.name.includes(q)))
    .slice(0, 12).map((x) => x.e);
}
