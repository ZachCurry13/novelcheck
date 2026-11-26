// Home network address (admins, Integrations page): NovelCheck's address on the home network. The Kindle app and the KOReader plugin
// learn it by themselves and try it first (a short knock), then use the main address, so at home they use the Wi-Fi alone.
import { get, put } from "./api.js";
import { esc, attempt } from "./ui.js";

export async function homeAddressCard(view) {
  const host = document.createElement("section");
  host.className = "card mb-4 space-y-2";
  view.append(host);
  const d = await attempt(() => get("/api/admin/home-address"));
  if (!d) return;
  host.innerHTML = `<h2 class="font-semibold">Home network address</h2>
    <p class="text-sm text-slate-400">NovelCheck's address on your home network, like http://192.168.1.20:8080. Kindles with the NovelCheck app or the KOReader plugin try it first when they are at home, with no internet needed, and use the main address anywhere else. They pick it up the next time they check for updates. Leave it empty to turn this off.</p>
    <div class="flex flex-wrap gap-2"><input id="home-addr" class="input min-w-0 flex-1" placeholder="http://192.168.1.20:8080" maxlength="100" value="${esc(d.address || "")}">
    <button type="button" id="home-save" class="btn-secondary py-1 text-sm">Save</button></div>`;
  host.querySelector("#home-save").onclick = async () => {
    const r = await attempt(() => put("/api/admin/home-address", { address: host.querySelector("#home-addr").value }), "Saved");
    if (r) host.querySelector("#home-addr").value = r.address || "";
  };
}
