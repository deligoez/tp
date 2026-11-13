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

  // containView is the first view of a W by H scoped view in a box of bw by
  // bh pixels: the whole drawing, centred, within SCALE; along an axis it
  // does not fit on, it opens at its start.
  P.containView = function (W, H, bw, bh) {
    const fit = Math.min(bw / (W + 2 * VIEW_PAD), bh / (H + 2 * VIEW_PAD));
    const s = Math.min(P.SCALE.max, Math.max(P.SCALE.min, fit));
    const w = bw / s;
    const h = bh / s;
    return { x: w >= W ? W / 2 - w / 2 : -VIEW_PAD, y: h >= H ? H / 2 - h / 2 : -VIEW_PAD, w, h };
  };

  // placeLayout turns a /api/layout payload into drawing points: a column of
  // dims.colW per layer from dims.left, a row of dims.rowH per order below
  // dims.top and dims.head, a layer shorter than the longest centred against
  // it, each node's point dims.inset into its column. It also returns each
  // band's x span, named for its milestone and those sharing its span, and
  // the drawing's size.
  P.placeLayout = function (layout, dims) {
    const rows = [];
    (layout.nodes || []).forEach((n) => { rows[n.layer] = (rows[n.layer] || 0) + 1; });
    const most = Math.max(1, ...rows.filter(Boolean));
    const pos = {};
    (layout.nodes || []).forEach((n) => {
      const shift = (most - rows[n.layer]) / 2;
      pos[n.id] = { x: dims.left + n.layer * dims.colW + dims.inset, y: dims.top + dims.head + (n.order + shift) * dims.rowH + dims.rowH / 2 };
    });
    const bands = (layout.bands || []).map((b) => ({ name: [b.milestone].concat(b.with || []).join(" + "), x: dims.left + b.from_layer * dims.colW, w: (b.to_layer - b.from_layer + 1) * dims.colW }));
    return { pos, bands, W: 2 * dims.left + (layout.layers || 0) * dims.colW, H: 2 * dims.top + dims.head + most * dims.rowH };
  };

  // centreView is v moved to centre the drawing point p in a box rw pixels
  // wide: at v's zoom, or zoomed in to SCALE.min when v is further out.
  P.centreView = function (v, p, rw) {
    const f = rw / v.w < P.SCALE.min ? rw / P.SCALE.min / v.w : 1;
    const w = v.w * f;
    const h = v.h * f;
    return { x: p.x - w / 2, y: p.y - h / 2, w, h };
  };

  // ROVE names each arrow's direction as [dx, dy].
  const ROVE = { ArrowRight: [1, 0], ArrowLeft: [-1, 0], ArrowDown: [0, 1], ArrowUp: [0, -1] };

  // roveNext is the id an arrow key moves the focus to from cur, over pos
  // (ids to drawing points): the nearest point that way, a step across the
  // arrow counting thrice a step along it; cur itself when none lies that way
  // or the key is no arrow.
  P.roveNext = function (pos, cur, key) {
    const dir = ROVE[key];
    const p = pos[cur];
    if (!dir || !p) return cur;
    let best = cur;
    let score = Infinity;
    Object.keys(pos).forEach((id) => {
      const dx = pos[id].x - p.x;
      const dy = pos[id].y - p.y;
      const along = dx * dir[0] + dy * dir[1];
      if (along <= 0) return;
      const s = along + 3 * Math.abs(dx * dir[1] - dy * dir[0]);
      if (s < score) { score = s; best = id; }
    });
    return best;
  };

  // BLOCKER_RANK orders a blocker's states from the readiest; an any-of
  // group takes its readiest member's.
  const BLOCKER_RANK = ["startable", "claimed", "decision", "waiting"];

  // blockerState is what an epic others wait on is doing: startable when tp
  // epic ready lists it, a decision when it is a condition, claimed when it
  // is under way, else waiting itself.
  const blockerState = (e, ready) => {
    if (!e) return "waiting";
    if (ready.includes(e.id)) return "startable";
    if (e.condition) return "decision";
    return e.status && e.status !== "planned" && e.status !== "done" ? "claimed" : "waiting";
  };

  // waitingGroups groups waiting, the summary's waiting epic ids, by what
  // they wait on directly, read from epics (the export's epics by id): an
  // epic sits under each undone dependency and each any-of group it waits
  // on. Each group carries its blocker's state; groups run from the largest,
  // ties in the order their blocker is first met. The waiting epics of
  // milestones open does not mark open are set apart in closed, and open
  // counts the distinct epics the groups hold.
  P.waitingGroups = function (waiting, epics, ready, open) {
    const groups = new Map();
    const closed = [];
    let count = 0;
    const add = (key, blocker, anyOf, state, id) => {
      if (!groups.has(key)) groups.set(key, { blocker, any_of: anyOf, state, epics: [] });
      groups.get(key).epics.push(id);
    };
    waiting.forEach((id) => {
      const e = epics[id];
      if (!e) return;
      if (!open[e.milestone]) {
        closed.push(id);
        return;
      }
      count++;
      (e.blocked_by || []).forEach((b) => add("e:" + b, b, null, blockerState(epics[b], ready), id));
      (e.blocked_by_any_of || []).forEach((g) => {
        const best = g.map((m) => blockerState(epics[m], ready)).sort((x, y) => BLOCKER_RANK.indexOf(x) - BLOCKER_RANK.indexOf(y))[0];
        add("g:" + g.join("\u0000"), null, g, best, id);
      });
    });
    const list = [...groups.values()];
    list.sort((x, y) => y.epics.length - x.epics.length);
    return { groups: list, closed, open: count };
  };

  // describedCounts counts, per milestone, the graph's epics and those with
  // a description, one holding more than whitespace, as tp treats it.
  P.describedCounts = function (epics) {
    const out = {};
    (epics || []).forEach((e) => {
      const c = out[e.milestone] || (out[e.milestone] = { described: 0, all: 0 });
      c.all++;
      if (String(e.description || "").trim()) c.described++;
    });
    return out;
  };

  // relTime says how long ago ms milliseconds is, in its largest whole
  // unit; under ten seconds, or a clock gone backwards, is just now.
  P.relTime = function (ms) {
    const s = Math.floor(ms / 1000);
    if (!(s >= 10)) return "just now";
    if (s < 60) return s + " s ago";
    if (s < 3600) return Math.floor(s / 60) + " min ago";
    return Math.floor(s / 3600) + " h ago";
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
