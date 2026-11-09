// The browser's Back button (and a phone's back swipe) closes an open window, such as a book's,
// instead of leaving the page behind it. Each window opened with showModal() adds one history
// entry; closing the window by its own button takes that entry away again.
let ownBacks = 0; // history.back() calls this file made itself: their popstate is not the person pressing Back
const original = HTMLDialogElement.prototype.showModal;

function whenClosed(d) {
  if (d.dataset.ncPushed !== "1") return; // closed by Back already
  delete d.dataset.ncPushed;
  if (history.state && history.state.ncDialog) {
    ownBacks++;
    history.back(); // the entry this window added
  }
}

HTMLDialogElement.prototype.showModal = function showModal(...args) {
  const wasOpen = this.open;
  original.apply(this, args);
  if (!wasOpen && this.open) {
    history.pushState({ ncDialog: true }, "");
    this.dataset.ncPushed = "1";
    // Watching the open attribute works in every browser, and the moment it is closed.
    const watch = new MutationObserver(() => {
      if (!this.open) {
        watch.disconnect();
        whenClosed(this);
      }
    });
    watch.observe(this, { attributes: true, attributeFilter: ["open"] });
  }
};

window.addEventListener("popstate", () => {
  if (ownBacks > 0) {
    ownBacks--;
    return;
  }
  const open = [...document.querySelectorAll("dialog[open]")].filter((d) => d.dataset.ncPushed === "1");
  const top = open[open.length - 1];
  if (top) {
    delete top.dataset.ncPushed; // its entry is gone: closing it must not go back again
    top.close();
  }
});
