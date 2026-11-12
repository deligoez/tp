// tp dashboard's pure functions: geometry and grouping that read their
// arguments alone and touch no document, so the tests run them under node.
// The page loads this file before app.js and reads them as tpPure.
(function (root) {
  "use strict";
  const P = {};

  // VIEW_PAD is the margin, in drawing units, a first view leaves around the
  // drawing; SCALE bounds the first view's screen pixels per drawing unit:
  // below min the 11-unit labels read under 11 pixels, above max a small
  // drawing is blown up.
  const VIEW_PAD = 8;
  P.SCALE = { min: 1, max: 1.5 };

  // initialView is the first view of a W by H drawing in a box of bw by bh
  // pixels: the drawing's width fitted to the box, within SCALE, from its
  // top left; what does not fit is panned to.
  P.initialView = function (W, H, bw, bh) {
    const s = Math.min(P.SCALE.max, Math.max(P.SCALE.min, bw / (W + 2 * VIEW_PAD)));
    return { x: -VIEW_PAD, y: -VIEW_PAD, w: bw / s, h: bh / s };
  };

  // pinchView is the view after two fingers moved from a0 and b0 to a1 and
  // b1 (pixels from the box's top left, the box rw by rh) over the view v0
  // they started on: zoomed by the ratio of their spans, with the drawing
  // point under their first midpoint kept under their current one.
  P.pinchView = function (v0, a0, b0, a1, b1, rw, rh) {
    const span = (a, b) => Math.hypot(b.x - a.x, b.y - a.y) || 1;
    const f = span(a0, b0) / span(a1, b1);
    const m0 = { x: (a0.x + b0.x) / 2, y: (a0.y + b0.y) / 2 };
    const m1 = { x: (a1.x + b1.x) / 2, y: (a1.y + b1.y) / 2 };
    const w = v0.w * f;
    const h = v0.h * f;
    const px = v0.x + (m0.x / rw) * v0.w;
    const py = v0.y + (m0.y / rh) * v0.h;
    return { x: px - (m1.x / rw) * w, y: py - (m1.y / rh) * h, w, h };
  };

  if (typeof module === "object" && module.exports) module.exports = P;
  else root.tpPure = P;
})(this);
