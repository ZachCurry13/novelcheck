// Reading offline. sw.js keeps the last 5 books opened in the reader, the Nightstand and its covers on this
// device; here, a place saved while offline waits (one per book) and is sent once NovelCheck answers again.
// Sign-out and "Switch profile" send what is waiting, then clear everything kept, so the next person starts clean.
const KEY = "nc-pos-";

const send = (id, { cfi, fraction, xpointer }) => fetch(`/api/books/${id}/position`, {
  method: "PUT", credentials: "same-origin", headers: { "X-NovelCheck": "1", "Content-Type": "application/json" },
  body: JSON.stringify({ cfi, fraction, xpointer }),
});

const waiting = () => {
  try {
    return Object.keys(localStorage).filter((k) => k.startsWith(KEY));
  } catch {
    return [];
  }
};

// pendingPosition is the place saved offline for this book, or null.
export function pendingPosition(id) {
  try {
    return JSON.parse(localStorage.getItem(KEY + id));
  } catch {
    return null;
  }
}

// savePosition sends the place; with no connection it waits on this device instead.
export async function savePosition(id, place) {
  try {
    await send(id, place);
    localStorage.removeItem(KEY + id);
  } catch {
    try {
      localStorage.setItem(KEY + id, JSON.stringify({ ...place, at: new Date().toISOString() }));
    } catch { /* storage blocked: the place is lost, as before */ }
  }
}

// flushPositions sends the places saved offline (stops at the first that can't be sent).
export async function flushPositions() {
  for (const k of waiting()) {
    const place = pendingPosition(k.slice(KEY.length));
    try {
      if (place) await send(k.slice(KEY.length), place);
      localStorage.removeItem(k);
    } catch {
      return;
    }
  }
}

// clearOffline forgets everything kept for offline reading on this device.
export async function clearOffline() {
  await flushPositions();
  for (const k of waiting()) localStorage.removeItem(k);
  await globalThis.caches?.delete("novelcheck-mine").catch(() => {});
}
