// Phones: a bottom tab bar with the main pages and "More" for the rest,
// because the full top menu doesn't fit a phone screen. It's built from the
// top menu's links, so it shows exactly what this account may open.

const MAIN = [["check", "📷", "Check"], ["library", "📚", "Library"], ["discover", "🧭", "Discover"], ["queue", "▶️", "Up Next"], ["collections", "📚", "Collections"], ["wishlist", "⭐", "Wishlist"], ["profile", "👤", "Profile"]];
const ICON = { discover: "🧭", queue: "▶️", import: "💾", shelf: "📕", collections: "📚", admin: "🛠️", deepscan: "🧬", usage: "📈", system: "🩺", profile: "👤", wishlist: "⭐" };

let moreRoutes = [];

const shown = (a) => !a.classList.contains("hidden") && !a.classList.contains("module-off");

export function buildMobileNav() {
  const bar = document.getElementById("mobile-nav");
  if (!bar) return;
  const links = [...document.querySelectorAll("#nav .nav-link")].filter(shown);
  const main = MAIN.filter(([r]) => links.some((a) => a.dataset.route === r)).slice(0, 4);
  const more = links.filter((a) => !main.some(([r]) => r === a.dataset.route));
  moreRoutes = more.map((a) => a.dataset.route);
  bar.innerHTML = main.map(([r, ico, label]) => `<a href="#/${r}" data-route="${r}"><span class="ico" aria-hidden="true">${ico}</span>${label}</a>`).join("")
    + `<button type="button" data-more aria-haspopup="dialog"><span class="ico" aria-hidden="true">☰</span>More</button>`; // also Help and Sign out
  bar.onclick = (e) => {
    if (e.target.closest("[data-more]")) openMore(more);
  };
  markMobileNav((location.hash.replace(/^#\/?/, "").split("?")[0]) || "");
}

// markMobileNav highlights the current page (More stands for the pages in it).
export function markMobileNav(route) {
  const bar = document.getElementById("mobile-nav");
  if (!bar) return;
  bar.querySelectorAll("[data-route]").forEach((a) => a.classList.toggle("active", a.dataset.route === route));
  bar.querySelector("[data-more]")?.classList.toggle("active", moreRoutes.includes(route));
}

// More opens as a sheet from the bottom (see .dialog in tailwind.input.css):
// big tiles for the other pages, then help. A swipe down closes it.
function openMore(links) {
  let d = document.getElementById("more-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "more-dialog";
    d.className = "dialog";
    d.setAttribute("aria-label", "More pages");
    document.body.append(d);
    swipeToClose(d);
  }
  const tile = (a) => `<a href="${a.getAttribute("href")}" class="flex flex-col items-center gap-1 rounded-xl bg-slate-800/70 px-1 py-3 text-center text-sm hover:bg-slate-700">
    <span class="text-2xl" aria-hidden="true">${ICON[a.dataset.route] || "•"}</span>${a.textContent.replace(/^[^\p{L}]+/u, "").trim()}</a>`;
  const item = (ico, label, attrs) => `<a href="#" ${attrs} class="flex items-center gap-3 rounded-lg px-3 py-3 text-base hover:bg-slate-800">
    <span class="w-6 text-center text-xl" aria-hidden="true">${ico}</span>${label}</a>`;
  d.innerHTML = `<div class="space-y-3 p-4">
    <div class="mx-auto h-1.5 w-10 rounded-full bg-slate-700" aria-hidden="true"></div>
    <div class="flex items-center justify-between"><h2 class="text-lg font-bold">More</h2>
      <button data-close class="btn-ghost h-10 w-10 p-0 text-xl" aria-label="Close">✕</button></div>
    <div class="grid grid-cols-3 gap-2">${links.map(tile).join("")}</div>
    <div class="border-t border-slate-800 pt-2">
      ${item("❔", "How to use NovelCheck", "data-help")}
      ${item("🐞", "Report a problem or idea", "data-report")}
      ${document.getElementById("logout-btn")?.textContent.includes("Switch") ? item("👥", "Switch profile", "data-logout") : item("🚪", "Sign out", "data-logout")}
    </div>
  </div>`;
  d.onclick = (e) => {
    const a = e.target.closest("a");
    if (e.target === d || e.target.closest("[data-close]") || a) d.close();
    if (e.target.closest("[data-help]")) {
      e.preventDefault();
      document.getElementById("help-btn")?.click();
    } else if (e.target.closest("[data-logout]")) {
      e.preventDefault();
      document.getElementById("logout-btn")?.click();
    } else if (e.target.closest("[data-report]")) {
      e.preventDefault();
      document.getElementById("report-link")?.click();
    }
  };
  d.showModal();
}

// swipeToClose closes a bottom sheet dragged down by more than 80px.
function swipeToClose(d) {
  let startY = null;
  d.addEventListener("touchstart", (e) => {
    startY = d.scrollTop <= 0 && !d.querySelector(".overflow-y-auto")?.scrollTop ? e.touches[0].clientY : null;
  }, { passive: true });
  d.addEventListener("touchend", (e) => {
    if (startY !== null && e.changedTouches[0].clientY - startY > 80) d.close();
    startY = null;
  }, { passive: true });
}
