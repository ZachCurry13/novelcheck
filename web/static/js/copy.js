// Copy / download helpers for logs and error messages.
import { toast } from "./ui.js";

// copyText copies text and confirms with a toast (or silently with quiet,
// for buttons that show their own ✓). Returns whether it worked.
export async function copyText(text, quiet = false) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    // Older browsers, or plain http on the home network: fall back to a hidden textarea.
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.setAttribute("readonly", "");
    ta.className = "fixed -left-[9999px] top-0";
    document.body.append(ta);
    ta.select();
    const ok = document.execCommand("copy");
    ta.remove();
    if (!ok) {
      if (!quiet) toast("Couldn't copy automatically. Select the text and press Ctrl+C.", true);
      return false;
    }
  }
  if (!quiet) toast("Copied to clipboard");
  return true;
}

export function downloadText(filename, text) {
  const url = URL.createObjectURL(new Blob([text], { type: "text/plain" }));
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

// copyBar renders Copy / Download buttons for the element with id target
// (or, with getText registered via bindCopy, for generated text).
export function copyBar(target, filename) {
  return `<span class="inline-flex gap-1">
    <button type="button" data-copy-for="${target}" class="btn-ghost px-2 py-0.5 text-xs" title="Copy to clipboard">📋 Copy</button>
    ${filename ? `<button type="button" data-download-for="${target}" data-filename="${filename}" class="btn-ghost px-2 py-0.5 text-xs" title="Save as a text file">⬇ Download</button>` : ""}
  </span>`;
}

// bindCopy handles copyBar clicks inside root. sources maps a target to a
// function returning the text; otherwise the element #target's text is used.
export function bindCopy(root, sources = {}) {
  root.addEventListener("click", (e) => {
    const b = e.target.closest("[data-copy-for], [data-download-for]");
    if (!b) return;
    e.preventDefault();
    e.stopPropagation();
    const target = b.dataset.copyFor || b.dataset.downloadFor;
    const text = sources[target] ? sources[target]() : (document.getElementById(target)?.textContent ?? "");
    if (b.dataset.copyFor) copyText(text);
    else downloadText(b.dataset.filename, text);
  });
}

// setLogText updates a live log without destroying the reader's selection or
// scroll position: it skips the update while text inside is selected.
export function setLogText(el, text) {
  if (el.textContent === text) return;
  const sel = window.getSelection();
  if (sel && !sel.isCollapsed && el.contains(sel.anchorNode)) return;
  const atBottom = el.scrollTop + el.clientHeight >= el.scrollHeight - 8;
  el.textContent = text;
  if (atBottom) el.scrollTop = el.scrollHeight;
}
