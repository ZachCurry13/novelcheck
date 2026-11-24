// /kindleapp: a page to hand someone ("go to this address, sign in, download"): it asks them to sign in, then offers the
// Kindle app already set up for their account, so there is nothing to type on the Kindle. The zip is made for the person
// who is signed in here (api/koplugin_handlers.go), so each person gets their own.
import { get, post } from "./api.js";
import { esc } from "./ui.js";

const host = document.getElementById("ka");
const remember = (k, v) => { try { v === undefined ? localStorage.removeItem(k) : localStorage.setItem(k, v); } catch {} };
const recall = (k) => { try { return localStorage.getItem(k) || ""; } catch { return ""; } };

const head = `<h1 class="page-title">NovelCheck for your Kindle</h1>`;

function loginHTML(error = "") {
  host.innerHTML = `${head}<p class="text-slate-300">Sign in with your NovelCheck name and password. You can then download the Kindle app set up for your account.</p>
    <form id="login" class="card space-y-3"><label class="block text-sm">Name<input name="username" autocomplete="username" autocapitalize="none" required class="input mt-1"></label>
      <label class="block text-sm">Password<input name="password" type="password" autocomplete="current-password" required class="input mt-1"></label>
      ${error ? `<p class="text-sm text-rose-300">${esc(error)}</p>` : ""}<button class="btn-primary w-full">Sign in</button></form>`;
  host.querySelector("#login").onsubmit = async (e) => {
    e.preventDefault();
    const f = e.target;
    try {
      await post("/api/auth/login", { username: f.username.value.trim(), password: f.password.value });
      start();
    } catch (err) {
      loginHTML(err.status === 401 || err.status === 400 ? "That name and password don't match." : err.message);
    }
  };
}

function downloadHTML(me) {
  const name = recall("nc-kindle-name") || "";
  host.innerHTML = `${head}<p class="text-slate-300">Signed in as <b>${esc(me.username)}</b>. <button id="out" class="underline">Not you? Sign out</button></p>
    <section class="card space-y-3"><label class="block text-sm">What is this Kindle called? <span class="text-slate-400">(like “Jenna's Paperwhite”; if it also has the KOReader plugin, use the plugin's name so NovelCheck sees them as one Kindle)</span>
      <input id="kname" class="input mt-1" maxlength="40" placeholder="Jenna's Paperwhite" value="${esc(name)}"></label>
      <a id="zip" class="btn-primary inline-flex" href="#">⬇ Download the Kindle app</a>
      <p class="text-xs text-slate-400">Set up for <b>${esc(me.username)}</b>: there is nothing to type on the Kindle. Keep the file private: it works as your sign-in.</p></section>
    <section class="card space-y-2 text-sm text-slate-300"><h2 class="font-semibold">Before you start</h2>
      <p>The app needs a <b>jailbroken Kindle with KindleForge</b> on it. It can't be installed on a Kindle that isn't jailbroken, and a Kindle can't download it by itself: the file goes onto it from a computer.</p>
      <p class="text-amber-200">Newer Kindles may not be jailbreakable yet. Check your model and its software version on <a class="underline" href="https://kindlemodding.org" target="_blank" rel="noopener noreferrer">kindlemodding.org</a> before you try anything, and don't update the Kindle's software first.</p></section>
    <section class="card space-y-2 text-sm text-slate-300"><h2 class="font-semibold">Installing it</h2>
      <ol class="list-decimal space-y-1 pl-5"><li>Download the file above on a computer.</li>
        <li>Connect the Kindle to the computer with its cable and unzip the file onto it, merging the <b>documents</b> and <b>extensions</b> folders.</li>
        <li>Eject the Kindle. <b>NovelCheck</b> appears in its library: tap it.</li>
        <li>If it says it needs KindleForge's helper, open KindleForge once, then NovelCheck again.</li>
        <li>In the app, press <b>Sync now</b>. It updates itself from then on.</li></ol></section>`;
  const zip = host.querySelector("#zip"), kname = host.querySelector("#kname");
  const paint = () => { zip.href = `/api/me/kindle-app.zip?name=${encodeURIComponent(kname.value.trim())}`; remember("nc-kindle-name", kname.value.trim() || undefined); };
  kname.oninput = paint;
  paint();
  host.querySelector("#out").onclick = async () => { await post("/api/auth/logout").catch(() => {}); loginHTML(); };
}

async function start() {
  try {
    downloadHTML(await get("/api/me"));
  } catch {
    loginHTML();
  }
}
start();
