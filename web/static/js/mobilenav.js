// Phones: a bottom tab bar with the main pages and "More" for the rest,
// because the full top menu doesn't fit a phone screen. It's built from the
// top menu's links, so it shows exactly what this account may open.

const MAIN = [["check", "📷", "Check"], ["library", "📚", "Library"], ["queue", "▶️", "Up Next"], ["wishlist", "⭐", "Wishlist"], ["profile", "👤", "Profile"]];
const ICON = { import: "💾", admin: "🛠️", usage: "📈", system: "🩺", profile: "👤", wishlist: "⭐" };

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
    + (more.length ? `<button type="button" data-more aria-haspopup="dialog"><span class="ico" aria-hidden="true">☰</span>More</button>` : "");
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

function openMore(links) {
  let d = document.getElementById("more-dialog");
  if (!d) {
    d = document.createElement("dialog");
    d.id = "more-dialog";
    d.className = "dialog";
    document.body.append(d);
  }
  const item = (href, ico, label, attrs = "") => `<a href="${href}" ${attrs} class="flex items-center gap-3 rounded-lg px-3 py-3 text-base hover:bg-slate-800">
    <span class="w-6 text-center text-xl" aria-hidden="true">${ico}</span>${label}</a>`;
  d.innerHTML = `<div class="space-y-1 p-4">
    <div class="flex items-center justify-between"><h2 class="text-lg font-bold">More</h2>
      <button data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button></div>
    ${links.map((a) => item(a.getAttribute("href"), ICON[a.dataset.route] || "•", a.textContent.replace(/^[^\p{L}]+/u, "").trim())).join("")}
    <hr class="my-2 border-slate-800">
    ${item("#", "❔", "How to use NovelCheck", "data-help")}
    ${item("#", "🐞", "Report a problem or idea", "data-report")}
  </div>`;
  d.onclick = (e) => {
    const a = e.target.closest("a");
    if (e.target === d || e.target.closest("[data-close]") || a) d.close();
    if (e.target.closest("[data-help]")) {
      e.preventDefault();
      document.getElementById("help-btn")?.click();
    } else if (e.target.closest("[data-report]")) {
      e.preventDefault();
      document.getElementById("report-link")?.click();
    }
  };
  d.showModal();
}
