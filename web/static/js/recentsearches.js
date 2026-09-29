// Recent searches under the Library's search box: each person's last 10,
// kept on the server so they follow them to every device.
import { get, post, del } from "./api.js";
import { esc } from "./ui.js";

export function recentSearches(input, onPick) {
  const box = document.createElement("div");
  box.className = "hidden col-span-full flex flex-wrap items-center gap-1 text-xs";
  input.parentElement.after(box);
  let list = [];
  let timer;
  const paint = () => {
    box.innerHTML = list.length
      ? `<span class="text-slate-500">Recent:</span> ${list.map((q) => `<button type="button" data-q="${esc(q)}" class="chip-cat">🕘 ${esc(q)}</button>`).join(" ")}
        <button type="button" data-clear class="ml-1 text-slate-500 underline">Clear</button>`
      : "";
  };
  const show = (on) => box.classList.toggle("hidden", !on || !list.length);
  get("/api/me/searches").then((l) => {
    list = l || [];
    paint();
    if (document.activeElement === input) show(true);
  }).catch(() => {});
  input.addEventListener("focus", () => show(true));
  input.addEventListener("blur", () => setTimeout(() => show(false), 150));
  box.addEventListener("pointerdown", (e) => e.preventDefault()); // keeps the search box focused
  box.addEventListener("click", async (e) => {
    const q = e.target.closest("[data-q]")?.dataset.q;
    if (q !== undefined) {
      input.value = q;
      show(false);
      return onPick();
    }
    if (e.target.closest("[data-clear]")) {
      list = (await del("/api/me/searches").catch(() => null)) ?? list;
      paint();
      show(false);
    }
  });
  return {
    // remember keeps a search that found books, once typing has settled.
    remember(q) {
      clearTimeout(timer);
      q = String(q || "").trim();
      if (q.length < 2) return;
      timer = setTimeout(async () => {
        list = (await post("/api/me/searches", { query: q }).catch(() => null)) || list;
        paint();
      }, 2000);
    },
  };
}
