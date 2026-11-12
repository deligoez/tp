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

  if (typeof module === "object" && module.exports) module.exports = P;
  else root.tpPure = P;
})(this);
