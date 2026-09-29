// Safe mode: NovelCheck started without its background work (Calibre sync,
// rating, Deep Scans…). Admins see why, and can leave it here.
import { post } from "./api.js";
import { esc, attempt } from "./ui.js";

const WHY = {
  crash: "NovelCheck stopped unexpectedly several times in a row, so it started without its background work.",
  flag: "Safe mode was turned on with the command novelcheck safe-mode on.",
  env: "Safe mode is set by NOVELCHECK_SAFE_MODE in the app's settings in TrueNAS. Remove it there and restart the app to leave.",
};

export function showSafeMode(user) {
  document.getElementById("safe-banner")?.remove();
  const reason = user?.safe_mode;
  if (!reason) return;
  const bar = document.createElement("div");
  bar.id = "safe-banner";
  bar.className = "border-b border-amber-800 bg-amber-950/60";
  bar.innerHTML = `<div class="mx-auto flex max-w-7xl flex-wrap items-center gap-2 px-4 py-2 text-sm text-amber-200">
    <span class="min-w-0 flex-1"><b>🛟 Safe mode:</b> Calibre sync, rating and Deep Scans are paused. ${esc(WHY[reason] || "")}</span>
    ${reason === "env" ? "" : `<button type="button" data-leave class="btn-secondary py-1 text-sm">Leave safe mode</button>`}
  </div>`;
  document.getElementById("update-banner").after(bar);
  bar.querySelector("[data-leave]")?.addEventListener("click", async () => {
    if (await attempt(() => post("/api/admin/safe-mode/leave"), "Safe mode is off: background work has started")) bar.remove();
  });
}
