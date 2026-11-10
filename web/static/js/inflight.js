// once(fn) wraps a polling function so a new call is skipped while the last one is still running. NovelCheck's
// server answers one database request at a time, so a poll that takes longer than its interval must not stack
// up more of itself behind it (that is how a slow page turns into a minute-long one).
export function once(fn) {
  let running = false;
  return async (...args) => {
    if (running) return undefined;
    running = true;
    try {
      return await fn(...args);
    } finally {
      running = false;
    }
  };
}
