// The Library's ⋯ menu (more actions): a pick closes it, and it opens under its button but is moved back on
// screen when it would spill over an edge (on a phone the button sits near the left edge).
export function bindMoreMenu(form) {
  form.addEventListener("click", (e) => {
    if (e.target.closest(".fmenu-list")) e.target.closest("details").open = false; // pick an action, the menu closes
  });
  form.addEventListener("toggle", (e) => {
    const list = e.target.querySelector?.(".fmenu-list");
    if (!list || !e.target.open) return;
    list.style.transform = "";
    const r = list.getBoundingClientRect(), edge = 8;
    const dx = r.left < edge ? edge - r.left : r.right > innerWidth - edge ? innerWidth - edge - r.right : 0;
    if (dx) list.style.transform = `translateX(${Math.round(dx)}px)`;
  }, true); // toggle does not bubble
}
