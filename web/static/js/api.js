// Thin fetch wrapper for the NovelCheck JSON API.
// Every state-changing request carries X-NovelCheck: 1 (CSRF guard).

export class ApiError extends Error {
  constructor(status, message) {
    super(message);
    this.status = status;
  }
}

let onUnauthorized = () => {};
export function setUnauthorizedHandler(fn) {
  onUnauthorized = fn;
}

// A phone often resumes the app before its network is back, and the remote
// tunnel answers 502/503/530 while it reconnects. Reads (GET) are retried up
// to 3 times, waiting for the "online" event, before any error shows. Changes
// (POST/PUT/…) are never retried, so nothing is done twice.
const RETRY_STATUS = new Set([502, 503, 504, 522, 524, 530]);
const RETRIES = 3;
const wait = (ms) => new Promise((r) => setTimeout(r, ms));

function backOnline(ms) {
  if (navigator.onLine) return wait(ms);
  return new Promise((resolve) => {
    const done = () => {
      clearTimeout(timer);
      window.removeEventListener("online", done);
      resolve();
    };
    const timer = setTimeout(done, 10000);
    window.addEventListener("online", done);
  });
}

async function send(path, opts) {
  for (let attempt = 0; ; attempt++) {
    try {
      const res = await fetch(path, opts);
      if (opts.method === "GET" && RETRY_STATUS.has(res.status) && attempt < RETRIES) {
        await backOnline(700 * 2 ** attempt);
        continue;
      }
      return res;
    } catch {
      if (opts.method !== "GET" || attempt >= RETRIES) {
        throw new ApiError(0, "Can't reach NovelCheck right now. Check your connection and try again.");
      }
      await backOnline(700 * 2 ** attempt);
    }
  }
}

export async function api(path, { method = "GET", body, raw = false } = {}) {
  const opts = { method, credentials: "same-origin", headers: {} };
  if (method !== "GET") opts.headers["X-NovelCheck"] = "1";
  if (body !== undefined) {
    opts.headers["Content-Type"] = "application/json";
    opts.body = JSON.stringify(body);
  }
  const res = await send(path, opts);
  if (res.status === 401 && !path.startsWith("/api/auth/")) {
    onUnauthorized();
    throw new ApiError(401, "Please sign in");
  }
  if (raw) return res;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError(res.status, data.error || res.statusText);
  return data;
}

export const get = (p) => api(p);
export const post = (p, body = {}) => api(p, { method: "POST", body });
export const put = (p, body) => api(p, { method: "PUT", body });
export const patch = (p, body) => api(p, { method: "PATCH", body });
export const del = (p) => api(p, { method: "DELETE" });

export function qs(params) {
  const u = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== "" && v !== false) u.set(k, v === true ? "1" : v);
  }
  const s = u.toString();
  return s ? "?" + s : "";
}
