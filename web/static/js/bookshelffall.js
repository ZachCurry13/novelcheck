// What falls (or rises) in the bookshelf scene: snow, leaves, petals, embers or fireflies, on a canvas over it. It
// stops by itself when the scene leaves the page, and stands still for people who asked for less motion.
export function startFalling(cv, scene, T) {
  const cx = cv.getContext("2d");
  const still = matchMedia("(prefers-reduced-motion: reduce)").matches;
  const up = T.dir === -1;
  let ps = [], raf = 0, stopped = false;
  const make = (r, any) => ({
    x: Math.random() * r.width,
    y: any ? Math.random() * r.height : up ? r.height + 10 : -10,
    s: T.shape === "leaf" ? 4 + Math.random() * 4 : T.shape === "fly" ? 1.8 + Math.random() * 1.6 : 1.2 + Math.random() * 2.6,
    v: (T.shape === "fly" ? 0.25 : 0.35) + Math.random() * (up ? 0.5 : 0.9),
    d: Math.random() * 6, c: T.fallC[(Math.random() * T.fallC.length) | 0], rot: Math.random() * 6,
  });
  const r0 = scene.getBoundingClientRect();
  const ratio = window.devicePixelRatio || 1;
  cv.width = r0.width * ratio;
  cv.height = r0.height * ratio;
  for (let i = 0, n = Math.round(r0.width / (T.shape === "fly" ? 22 : 9)); i < n; i++) ps.push(make(r0, true));
  function tick(t) {
    if (stopped || !cv.isConnected) return;
    const r = scene.getBoundingClientRect();
    if (cv.width !== Math.round(r.width * ratio) || cv.height !== Math.round(r.height * ratio)) { // the scene grew (covers loaded)
      cv.width = r.width * ratio;
      cv.height = r.height * ratio;
    }
    cx.setTransform(ratio, 0, 0, ratio, 0, 0);
    cx.clearRect(0, 0, r.width, r.height);
    for (const p of ps) {
      if (!still) {
        p.y += up ? -p.v : p.v;
        p.x += Math.sin(t / 1100 + p.d) * 0.5;
        p.rot += 0.02;
        if (p.y > r.height + 12 || p.y < -12) Object.assign(p, make(r, false));
      }
      cx.fillStyle = p.c;
      cx.globalAlpha = T.shape === "fly" ? 0.35 + 0.65 * Math.abs(Math.sin(t / 600 + p.d)) : 0.85;
      cx.beginPath();
      if (T.shape === "leaf") {
        cx.save();
        cx.translate(p.x, p.y);
        cx.rotate(p.rot);
        cx.ellipse(0, 0, p.s, p.s * 0.55, 0, 0, 7);
        cx.fill();
        cx.restore();
      } else if (T.shape === "fly") {
        cx.shadowColor = "#ffe46b";
        cx.shadowBlur = 10;
        cx.arc(p.x, p.y, p.s, 0, 7);
        cx.fill();
        cx.shadowBlur = 0;
      } else {
        cx.arc(p.x, p.y, p.s, 0, 7);
        cx.fill();
      }
    }
    if (!still) raf = requestAnimationFrame(tick);
  }
  raf = requestAnimationFrame(tick);
  return () => {
    stopped = true;
    cancelAnimationFrame(raf);
  };
}
