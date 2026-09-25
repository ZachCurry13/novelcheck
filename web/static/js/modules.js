// Features the admin can turn off under Admin → System & Toggles → Features. The server refuses
// their API calls too; this hides them so the app stays uncluttered.

// on(user, "queue") is true unless the admin turned that feature off.
export const on = (user, name) => user?.modules?.[name] !== false;

// Anything marked data-module="queue" (etc.) disappears while it's off.
export function applyModules(user) {
  document.querySelectorAll("[data-module]").forEach((el) => el.classList.toggle("module-off", !on(user, el.dataset.module)));
}

// Delivery choices for "Start Reading", minus the ones turned off.
export function deliveryOptions(user, selected) {
  return [
    ["none", "No delivery (just track progress)"],
    ["email", "Email EPUB via Send-to-Kindle", "send_to_kindle"],
    ["koreader", "KOReader catalog (OPDS download)", "koreader"],
  ].filter(([v, , mod]) => !mod || on(user, mod) || v === selected)
    .map(([v, label]) => `<option value="${v}" ${v === selected ? "selected" : ""}>${label}</option>`).join("");
}
