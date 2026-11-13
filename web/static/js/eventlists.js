// A page of lists (a Linktree of the day's free books) saved as one event per
// list: each list is read and saved on its own, so a big page never reaches
// an event's limit, and a kids' list stays apart from the adult ones.
import { post } from "./api.js";

const today = () => new Date().toLocaleDateString(undefined, { month: "short", day: "numeric" });

// saveEachList reads and saves the picked lists one after another; progress
// is told each step. It returns the events made and the lists that failed.
export async function saveEachList(picked, endsAt, progress) {
  const made = [];
  const failed = [];
  for (const [i, l] of picked.entries()) {
    progress(`Reading ${l.name} (${i + 1} of ${picked.length})…`);
    try {
      const r = await post("/api/events/preview", { urls: [l.url] });
      if (!r.books?.length) {
        failed.push(l.name);
        continue;
      }
      const ev = await post("/api/events", { name: `${l.name} · ${today()}`.slice(0, 100), source_url: l.url, ends_at: endsAt, books: r.books });
      made.push({ id: ev.id, name: l.name, books: r.books.length });
    } catch {
      failed.push(l.name);
    }
  }
  return { made, failed };
}
