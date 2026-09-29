// Family device settings: in Profile, your PIN and (parents) making this
// device a family device; in Admin → Users, the family devices and kids'
// PINs.
import { get, put, post, del } from "./api.js";
import { esc, attempt, canManage } from "./ui.js";
import { refreshUser } from "./app.js";

// familyCardHTML is the Profile card.
export function familyCardHTML(u) {
  return `<div id="family-card" class="card space-y-3">
    <h2 class="text-lg font-semibold">👥 Family device &amp; PIN</h2>
    ${canManage(u) ? `<p class="text-sm text-slate-400">${u.family_device
      ? "This device opens on <b>Who's reading?</b>: everyone picks their profile, kids with a tap, parents with their PIN or password."
      : "On a shared tablet or computer, everyone can pick their profile instead of typing a password: kids tap their name, parents use a PIN."}</p>
      <button type="button" data-fam="${u.family_device ? "off" : "on"}" class="btn-secondary">${u.family_device ? "Stop using this device for the family" : "Use this device for the whole family"}</button>` : ""}
    <form data-pin class="space-y-2">
      <p class="text-sm text-slate-400">${u.has_pin ? "You have a PIN for family devices." : "A 4-digit PIN signs you in on a family device."}</p>
      <div class="grid grid-cols-2 gap-2">
        <input name="pin" inputmode="numeric" pattern="[0-9]{4}" maxlength="4" autocomplete="off" placeholder="New PIN (4 digits)" class="input" aria-label="New PIN">
        <input name="password" type="password" required autocomplete="current-password" placeholder="Your password" class="input" aria-label="Your password">
      </div>
      <div class="flex flex-wrap gap-2"><button class="btn-primary">${u.has_pin ? "Change PIN" : "Set PIN"}</button>
        ${u.has_pin ? `<button type="button" data-fam="clear-pin" class="btn-ghost">Remove PIN</button>` : ""}</div>
    </form></div>`;
}

// bindFamilyCard wires the Profile card; rerender redraws the page.
export function bindFamilyCard(view, rerender) {
  const card = view.querySelector("#family-card");
  if (!card) return;
  const form = card.querySelector("[data-pin]");
  const savePIN = async (pin) => {
    if (await attempt(() => put("/api/me/pin", { pin, password: form.password.value }), pin ? "PIN saved" : "PIN removed")) {
      await refreshUser();
      rerender();
    }
  };
  form.onsubmit = (e) => {
    e.preventDefault();
    if (!/^[0-9]{4}$/.test(form.pin.value)) return alert("A PIN is 4 digits.");
    savePIN(form.pin.value);
  };
  card.onclick = async (e) => {
    const act = e.target.closest("[data-fam]")?.dataset.fam;
    if (act === "clear-pin") {
      if (!form.password.value) return alert("Type your password first.");
      return savePIN("");
    }
    if (act === "on") {
      const name = prompt("A name for this device, e.g. Kitchen tablet:", "Family tablet");
      if (name === null) return;
      if (!(await attempt(() => post("/api/me/family-device", { name }), "This device now opens on Who's reading?"))) return;
    } else if (act === "off") {
      if (!confirm("Stop using this device for the family? It will ask for a username and password again.")) return;
      if (!(await attempt(() => del("/api/me/family-device"), "This device is back to normal sign-in"))) return;
    } else return;
    await refreshUser();
    location.reload(); // the header's Sign out / Switch profile follows
  };
}

// renderFamilyDevices lists the family devices (Admin → Users), with Remove.
export async function renderFamilyDevices(host) {
  const list = await get("/api/admin/family-devices").catch(() => []);
  host.innerHTML = list.length ? `<div class="card mb-4 space-y-2">
    <h3 class="font-semibold">👥 Family devices</h3>
    <p class="text-xs text-slate-400">These open on Who's reading?. Remove one you no longer use (it will ask for a password again).</p>
    <ul class="space-y-1">${list.map((d) => `<li class="flex items-center justify-between gap-2 text-sm">
      <span class="min-w-0 truncate">${esc(d.name)} <span class="text-xs text-slate-500">· set up by ${esc(d.created_by)}</span></span>
      <button type="button" data-rm-device="${d.id}" class="btn-ghost shrink-0 py-1 text-xs">Remove</button></li>`).join("")}</ul></div>` : "";
  host.onclick = async (e) => {
    const id = e.target.closest("[data-rm-device]")?.dataset.rmDevice;
    if (id && confirm("Remove this family device?") && (await attempt(() => del(`/api/admin/family-devices/${id}`), "Removed"))) renderFamilyDevices(host);
  };
}

// setUserPIN asks a parent for a kid's (or, admins, anyone's) new PIN.
export async function setUserPIN(id, name) {
  const pin = prompt(`A 4-digit PIN for ${name} on family devices (leave empty to remove it):`, "");
  if (pin === null) return;
  if (pin && !/^[0-9]{4}$/.test(pin)) return alert("A PIN is 4 digits.");
  await attempt(() => put(`/api/admin/users/${id}/pin`, { pin }), pin ? `PIN set for ${name}` : `PIN removed for ${name}`);
}
