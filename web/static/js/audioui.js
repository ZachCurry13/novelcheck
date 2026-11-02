// Audiobook bits shared by the cards, the book window and the player.
import { esc } from "./ui.js";

// fmtLen is a length like "12h 30m".
export function fmtLen(sec) {
  let h = Math.floor(sec / 3600), m = Math.round((sec % 3600) / 60);
  if (m === 60) {
    h++;
    m = 0;
  }
  return h ? `${h}h ${m}m` : `${m}m`;
}

// fmtClock is a place in the book like "1:02:03".
export function fmtClock(sec) {
  const s = Math.max(0, Math.floor(sec));
  const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), r = s % 60;
  return `${h ? `${h}:${String(m).padStart(2, "0")}` : m}:${String(r).padStart(2, "0")}`;
}

// audioChip is the 🎧 length on a card.
export const audioChip = (b) => b.duration_sec
  ? `<span class="chip-fmt" title="Audiobook${b.narrator ? ` · read by ${esc(b.narrator)}` : ""}">🎧 ${fmtLen(b.duration_sec)}</span>` : "";

// Length choices for the Library's filter: [label, min hours, max hours].
export const LENGTHS = [["Any length", 0, 0], ["Under 5 hours", 0, 5], ["5 to 10 hours", 5, 10], ["10 to 20 hours", 10, 20], ["Over 20 hours", 20, 0]];
