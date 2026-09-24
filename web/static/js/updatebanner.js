// Shows "Update available" to admins/editors once per version, and the running
// version in the footer for everyone.
import { get } from "./api.js";
import { $ } from "./ui.js";

const DISMISS_KEY = "novelcheck.dismissedUpdate";

function dismissed() {
  try {
    return localStorage.getItem(DISMISS_KEY);
  } catch {
    return null;
  }
}

export async function checkForUpdates() {
  let data;
  try {
    data = await get("/api/updates");
  } catch {
    return; // update checks are best-effort
  }
  const st = data.status;
  if (!st) return;
  $("#version-label").textContent = `Version ${st.current}`;
  if (!st.update_available || dismissed() === st.latest) return;
  $("#update-text").textContent = `NovelCheck ${st.latest} is available (you have ${st.current}).`;
  $("#update-banner").classList.remove("hidden");
  $("#update-dismiss").onclick = () => {
    try {
      localStorage.setItem(DISMISS_KEY, st.latest);
    } catch {
      /* private mode: dismiss for this page view only */
    }
    $("#update-banner").classList.add("hidden");
  };
}
