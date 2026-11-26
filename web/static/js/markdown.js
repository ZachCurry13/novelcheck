// Tiny, safe Markdown renderer for release notes and help text. Everything is
// HTML-escaped first; only headings, lists, bold, inline code and http(s)
// links are turned back into markup.
import { esc } from "./ui.js";

function inline(s) {
  return esc(s)
    .replace(/`([^`]+)`/g, '<code class="rounded bg-slate-800 px-1">$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
    .replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g,
      '<a href="$2" target="_blank" rel="noopener noreferrer" class="underline">$1</a>');
}

export function renderMarkdown(md) {
  const out = [];
  let list = null; // "ul" | "ol" | null
  const closeList = () => {
    if (list) out.push(`</${list}>`);
    list = null;
  };
  const openList = (type) => {
    if (list === type) return;
    closeList();
    out.push(type === "ol" ? '<ol class="list-decimal space-y-1 pl-5">' : '<ul class="list-disc space-y-1 pl-5">');
    list = type;
  };
  for (const raw of String(md || "").split("\n")) {
    const line = raw.trimEnd();
    const h = line.match(/^(#{1,4})\s+(.*)$/);
    const ol = line.match(/^\d+\.\s+(.*)$/);
    const ul = line.match(/^\s*[-*]\s+(.*)$/);
    if (h) {
      closeList();
      const size = ["text-xl", "text-lg", "text-base", "text-sm"][h[1].length - 1];
      out.push(`<h${h[1].length + 1} class="${size} mt-4 mb-1 font-semibold">${inline(h[2])}</h${h[1].length + 1}>`);
    } else if (ol) {
      openList("ol");
      out.push(`<li>${inline(ol[1])}</li>`);
    } else if (ul && list === "ol" && /^\s+/.test(line)) {
      // indented bullet under a numbered step: keep it inside that step
      const last = out.length - 1;
      out[last] = out[last].replace(/<\/li>$/, `<br><span class="ml-2 inline-block">• ${inline(ul[1])}</span></li>`);
    } else if (ul) {
      openList("ul");
      out.push(`<li>${inline(ul[1])}</li>`);
    } else if (!line.trim()) {
      if (list !== "ol") closeList(); // numbered steps may be separated by blank lines
    } else {
      closeList();
      out.push(`<p class="my-2">${inline(line)}</p>`);
    }
  }
  closeList();
  return out.join("\n");
}

// Returns the "## [x.y.z]" sections of CHANGELOG.md as {version, body}.
export function changelogSections(md) {
  const parts = String(md || "").split(/^## \[/m).slice(1);
  return parts.map((p) => {
    const nl = p.indexOf("\n");
    return { version: p.slice(0, p.indexOf("]")), body: p.slice(nl + 1).trim() };
  });
}

// forViewer cuts a release note down to what applies to this person. A bullet that ends in {admin} is for admins and editors
// only; the tag itself is never shown. A heading with nothing left under it goes too, and "" means nothing is left.
export function forViewer(body, manager) {
  const kept = [];
  for (const line of String(body || "").split("\n")) {
    const tagged = /\s*\{admin\}\s*$/.test(line);
    if (tagged && !manager) continue;
    kept.push(line.replace(/\s*\{admin\}\s*$/, ""));
  }
  const out = [];
  for (let i = 0; i < kept.length; i++) {
    if (/^###\s/.test(kept[i])) {
      let j = i + 1;
      while (j < kept.length && !kept[j].trim()) j++;
      if (j >= kept.length || /^#{2,3}\s/.test(kept[j])) continue;
    }
    out.push(kept[i]);
  }
  return out.join("\n").trim();
}
