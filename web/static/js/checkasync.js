// A book check that runs on the server on its own, so a photo that takes longer than the connection allows (the
// remote address gives up after 100 seconds) still gets its answer: ask, then wait for it.
import { post, get, ApiError } from "./api.js";

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

export async function postCheck(body) {
  const j = await post("/api/check?async=1", body);
  if (!j.job) return j; // an older server answered directly
  for (let i = 0; i < 150; i++) { // up to 5 minutes
    await sleep(2000);
    let r;
    try {
      r = await get(`/api/check/jobs/${j.job}`);
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) throw new ApiError(404, "The check was lost (the server restarted?): try again", {});
      continue; // a hiccup in the connection: ask again
    }
    if (r.done) {
      if (r.status >= 400) throw new ApiError(r.status, r.body?.error || "Couldn't check that book", r.body || {});
      return r.body;
    }
  }
  throw new ApiError(504, "Still working after 5 minutes: try again", {});
}
