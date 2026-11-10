// The browser's Back button (and a phone's back swipe) closes an open window, such as a book's, instead of
// leaving the page behind it. Each window opened with showModal() adds one history entry; closing the window by
// its own button takes that entry away again, but only when nothing else has happened to the address since: a
// link that closes its window and goes to another page must not have the trip undone by going back.
let ownBacks = 0; // history.back() calls this file made itself: their popstate is not the person pressing Back
const original = HTMLDialogElement.prototype.showModal;

function whenClosed(d) {
  if (d.dataset.ncPushed !== "1") return; // closed by Back already
  delete d.dataset.ncPushed;
  const hash = d.dataset.ncHash;
  // A moment later: a link that closes its window also navigates, and the address changes right after the close.
  setTimeout(() => {
    if (history.state && history.state.ncDialog && location.hash === hash) {
      ownBacks++;
      history.back(); // the entry this window added
    }
  }, 80);
}

HTMLDialogElement.prototype.showModal = function showModal(...args) {
  const wasOpen = this.open;
  original.apply(this, args);
  if (!wasOpen && this.open) {
    history.pushState({ ncDialog: true }, "");
    this.dataset.ncPushed = "1";
    this.dataset.ncHash = location.hash;
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

window.addEventListener("popstate", (event) => {
  if (ownBacks > 0) {
    ownBacks--;
    return;
  }
  if (event.state && event.state.ncDialog) return; // Forward onto a window's entry: nothing to reopen
  const open = [...document.querySelectorAll("dialog[open]")].filter((d) => d.dataset.ncPushed === "1");
  const top = open[open.length - 1];
  if (top) {
    delete top.dataset.ncPushed; // its entry is gone: closing it must not go back again
    // Like Esc: a window with its own "cancel" handling (the welcome guide marks itself seen) gets to use it.
    const cancel = new Event("cancel", { cancelable: true });
    top.dispatchEvent(cancel);
    if (top.open && !cancel.defaultPrevented) top.close();
    else if (top.open) setTimeout(() => top.open && top.close(), 0); // it said it would close itself: make sure
  }
});
