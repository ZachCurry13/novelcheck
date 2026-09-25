// Live ISBN barcode scanning with the phone camera (Check a book). Uses the
// browser's own BarcodeDetector where it exists (Chrome on Android) and the
// bundled ZXing library everywhere else (iPhone). Needs the https address.
import { esc } from "./ui.js";

let zxingLoad = null;
function loadZXing() {
  zxingLoad ??= new Promise((resolve, reject) => {
    if (window.ZXing) return resolve(window.ZXing);
    const s = document.createElement("script");
    s.src = "/vendor/zxing.min.js";
    s.onload = () => resolve(window.ZXing);
    s.onerror = () => {
      zxingLoad = null;
      reject(new Error("The barcode scanner couldn't load."));
    };
    document.head.append(s);
  });
  return zxingLoad;
}

// liveBlocker says why live scanning can't start here ("" if it can).
export function liveBlocker() {
  if (!window.isSecureContext) return "Live scanning needs NovelCheck's secure https:// address (Admin → Delivery & Services → Remote access). You can still take a photo.";
  if (!navigator.mediaDevices?.getUserMedia) return "This browser can't use the camera for live scanning. You can still take a photo.";
  return "";
}

const isISBN = (v) => /^97[89]\d{10}$/.test(v || "");

function cameraError(err) {
  switch (err?.name) {
    case "NotAllowedError":
    case "SecurityError":
      return "Camera access was blocked. Allow the camera for NovelCheck in the browser's settings, or close this and take a photo instead.";
    case "NotFoundError":
    case "OverconstrainedError":
      return "No camera was found. Close this and choose a photo instead.";
    case "NotReadableError":
      return "The camera is busy in another app. Close that app and try again.";
    default:
      return `${esc(err?.message || "The camera couldn't start.")} Close this and take a photo instead.`;
  }
}

// scanLive shows the camera in dlg and resolves with an ISBN-13, or "" when
// the person closes it.
export async function scanLive(dlg) {
  let stop = () => {};
  let done;
  const result = new Promise((r) => (done = r));
  const finish = (isbn) => {
    stop();
    if (dlg.open) dlg.close();
    done(isbn);
  };
  dlg.innerHTML = `<div class="space-y-3 p-4">
    <div class="flex items-center justify-between gap-3"><h2 class="font-semibold">Point the camera at the barcode</h2>
      <button data-close class="btn-ghost px-2 text-xl" aria-label="Close">✕</button></div>
    <video class="w-full rounded-lg bg-black" playsinline muted></video>
    <p data-msg class="text-sm text-slate-400">Hold the barcode on the back of the book steady, about a hand's width away.</p></div>`;
  dlg.onclick = (e) => {
    if (e.target === dlg || e.target.closest("[data-close]")) finish("");
  };
  dlg.oncancel = () => finish("");
  dlg.showModal();
  const video = dlg.querySelector("video");
  const msg = dlg.querySelector("[data-msg]");
  const back = { video: { facingMode: "environment" } };
  try {
    const native = "BarcodeDetector" in window && (await window.BarcodeDetector.getSupportedFormats?.())?.includes("ean_13");
    if (native) {
      const stream = await navigator.mediaDevices.getUserMedia(back);
      let alive = true;
      stop = () => {
        alive = false;
        stream.getTracks().forEach((t) => t.stop());
      };
      video.srcObject = stream;
      await video.play();
      const detector = new window.BarcodeDetector({ formats: ["ean_13"] });
      const tick = async () => {
        if (!alive) return;
        const codes = await detector.detect(video).catch(() => []);
        const hit = codes.map((c) => c.rawValue).find(isISBN);
        if (hit) return finish(hit);
        setTimeout(tick, 200);
      };
      tick();
    } else {
      const ZXing = await loadZXing();
      const hints = new Map([[ZXing.DecodeHintType.POSSIBLE_FORMATS, [ZXing.BarcodeFormat.EAN_13]]]);
      const reader = new ZXing.BrowserMultiFormatReader(hints);
      stop = () => reader.reset();
      await reader.decodeFromConstraints(back, video, (res) => {
        const text = res?.getText();
        if (isISBN(text)) finish(text);
      });
    }
  } catch (err) {
    stop();
    msg.className = "text-sm text-rose-300";
    msg.innerHTML = cameraError(err);
  }
  return result;
}
