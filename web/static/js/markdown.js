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
  let list = false;
  const closeList = () => {
    if (list) out.push("</ul>");
    list = false;
  };
  for (const raw of String(md || "").split("\n")) {
    const line = raw.trimEnd();
    const h = line.match(/^(#{1,4})\s+(.*)$/);
    const li = line.match(/^\s*[-*]\s+(.*)$/);
    if (h) {
      closeList();
      const size = ["text-xl", "text-lg", "text-base", "text-sm"][h[1].length - 1];
      out.push(`<h${h[1].length + 1} class="${size} mt-4 mb-1 font-semibold">${inline(h[2])}</h${h[1].length + 1}>`);
    } else if (li) {
      if (!list) out.push('<ul class="list-disc space-y-1 pl-5">');
      list = true;
      out.push(`<li>${inline(li[1])}</li>`);
    } else if (!line.trim()) {
      closeList();
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
