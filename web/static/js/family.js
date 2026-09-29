// 👥 Who's reading? On a family device (a parent set it up in Profile), the
// app opens on everyone's profiles: kids tap their name, and anyone with a
// PIN types it on a big keypad; parents without a PIN use their password.
import { get, post } from "./api.js";
import { esc } from "./ui.js";

const $ = (s) => document.querySelector(s);
const initial = (name) => esc((name || "?").trim().charAt(0).toUpperCase());
// Colors the light theme keeps white text on (see html.light in tailwind.input.css).
const COLORS = ["bg-indigo-600", "bg-emerald-600", "bg-rose-600", "bg-teal-600"];

function tiles(data) {
  return `<div class="mb-6 flex items-center justify-center gap-3"><img src="/icons/icon.svg" alt="" class="h-10 w-10">
      <h1 class="text-2xl font-bold">Who's reading?</h1></div>
    <ul class="grid grid-cols-2 gap-3 sm:grid-cols-3">${data.profiles.map((p, i) => `<li>
      <button type="button" data-pick="${p.id}" class="card flex w-full flex-col items-center gap-2 py-5 hover:ring-indigo-600">
        <span class="flex h-16 w-16 items-center justify-center rounded-full ${COLORS[i % COLORS.length]} text-2xl font-bold text-white">${initial(p.username)}</span>
        <span class="max-w-full break-words px-2 text-center font-semibold leading-snug line-clamp-2">${esc(p.username)}</span>
        <span class="text-xs text-slate-400">${p.needs === "pin" ? "🔒 PIN" : p.needs === "password" ? "🔑 Password" : p.role === "restricted" ? "Tap to read" : ""}</span>
      </button></li>`).join("")}</ul>
    <button type="button" data-password class="btn-ghost mx-auto mt-6 block text-sm">Sign in with a username and password</button>`;
}

function pinPad(p) {
  const key = (k, label = k) => `<button type="button" data-key="${k}" class="btn-secondary h-16 text-2xl">${label}</button>`;
  return `<div class="mx-auto max-w-xs space-y-5 text-center">
    <p class="text-lg font-semibold">${esc(p.username)}, enter your PIN</p>
    <div data-dots class="flex justify-center gap-3" aria-live="polite">${"<span class='h-4 w-4 rounded-full bg-slate-700'></span>".repeat(4)}</div>
    <p data-err class="hidden text-sm text-rose-400"></p>
    <div class="grid grid-cols-3 gap-3">${[1, 2, 3, 4, 5, 6, 7, 8, 9].map((n) => key(n)).join("")}
      <button type="button" data-back class="btn-ghost h-16 text-sm">‹ Back</button>${key(0)}${key("del", "⌫")}</div></div>`;
}

function passwordForm(p) {
  return `<form data-pw class="mx-auto max-w-xs space-y-3">
    <p class="text-center text-lg font-semibold">${esc(p.username)}, enter your password</p>
    <input name="password" type="password" autocomplete="current-password" required class="input" aria-label="Password">
    <p data-err class="hidden text-sm text-rose-400"></p>
    <button class="btn-primary w-full">Continue</button>
    <button type="button" data-back class="btn-ghost w-full">‹ Back</button>
    <p class="text-center text-xs text-slate-500">Set a PIN in Profile to use a PIN here instead.</p></form>`;
}

// showFamily shows the picker if this is a family device (false if not).
// onIn gets the signed-in user; onPassword opens the usual sign-in form.
export async function showFamily(onIn, onPassword) {
  const data = await get("/api/family").catch(() => null);
  if (!data?.family) return false;
  const host = $("#family-picker");
  const switchTo = async (p, extra, errBox) => {
    try {
      onIn(await post("/api/family/switch", { user_id: p.id, ...extra }));
    } catch (ex) {
      if (!errBox) return alert(ex.message);
      errBox.textContent = ex.message;
      errBox.classList.remove("hidden");
      return false;
    }
    return true;
  };
  const home = () => {
    host.innerHTML = tiles(data);
    host.onclick = (e) => {
      if (e.target.closest("[data-password]")) return onPassword();
      const p = data.profiles.find((x) => x.id === Number(e.target.closest("[data-pick]")?.dataset.pick));
      if (!p) return;
      if (p.needs === "none") return switchTo(p, {});
      if (p.needs === "password") return askPassword(p);
      askPIN(p);
    };
  };
  const askPIN = (p) => {
    let pin = "";
    host.innerHTML = pinPad(p);
    const paint = () => host.querySelectorAll("[data-dots] span").forEach((d, i) => d.classList.toggle("bg-indigo-400", i < pin.length));
    host.onclick = async (e) => {
      if (e.target.closest("[data-back]")) return home();
      const k = e.target.closest("[data-key]")?.dataset.key;
      if (k === undefined) return;
      pin = k === "del" ? pin.slice(0, -1) : (pin + k).slice(0, 4);
      paint();
      if (pin.length === 4 && !(await switchTo(p, { pin }, host.querySelector("[data-err]")))) {
        pin = "";
        paint();
      }
    };
  };
  const askPassword = (p) => {
    host.innerHTML = passwordForm(p);
    const form = host.querySelector("[data-pw]");
    form.password.focus();
    host.onclick = (e) => e.target.closest("[data-back]") && home();
    form.onsubmit = (e) => {
      e.preventDefault();
      switchTo(p, { password: form.password.value }, form.querySelector("[data-err]"));
    };
  };
  home();
  return true;
}
