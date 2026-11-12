// Small status marks in place of emoji (✅ ⬜ ⚠️ ⏳): a glyph in a muted or meaning colour, the same size as the
// text around it. kind: done, todo, warn, busy, background.
const MARKS = {
  done: ["✓", "text-emerald-400", "Done"],
  todo: ["○", "text-slate-500", "To do"],
  warn: ["!", "text-amber-400", "Needs a look"],
  busy: ["…", "text-slate-400", "Working"],
  background: ["…", "text-slate-400", "Running in the background"],
  skip: ["–", "text-slate-500", "Skipped"],
};

export const mark = (kind) => {
  const [glyph, cls, label] = MARKS[kind] || MARKS.todo;
  return `<span class="inline-flex w-4 justify-center font-semibold ${cls}" role="img" aria-label="${label}">${glyph}</span>`;
};
