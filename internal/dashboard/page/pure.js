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
  // bh pixels, and what Fit shows: the whole drawing, centred, never above
  // SCALE.max; a drawing too large for SCALE.min is zoomed out until it fits,
  // since a view that cuts bands or milestones off hides that they exist.
  P.containView = function (W, H, bw, bh) {
    const fit = Math.min(bw / (W + 2 * VIEW_PAD), bh / (H + 2 * VIEW_PAD));
    const s = Math.min(P.SCALE.max, fit);
    const w = bw / s;
    const h = bh / s;
    return { x: W / 2 - w / 2, y: H / 2 - h / 2, w, h };
  };

  // containAt is the view of a W by H drawing in a box of bw by bh pixels
  // centred on the drawing point p and holding the whole drawing: as wide
  // as p's farther side needs each way, never above SCALE.max. Re-centring
  // the neighbourhood view takes it, so a new layout is never drawn at the
  // old zoom with its outer nodes outside the box.
  P.containAt = function (W, H, bw, bh, p) {
    const hx = Math.max(p.x + VIEW_PAD, W + VIEW_PAD - p.x);
    const hy = Math.max(p.y + VIEW_PAD, H + VIEW_PAD - p.y);
    const s = Math.min(P.SCALE.max, bw / (2 * hx), bh / (2 * hy));
    const w = bw / s;
    const h = bh / s;
    return { x: p.x - w / 2, y: p.y - h / 2, w, h };
  };

  // fitAtView is the view re-centring takes for a drawing of size {W, H}
  // in a box of rect {width, height} on the point p (containAt), or null
  // while the box has no size or there is no point.
  P.fitAtView = function (size, rect, p) {
    if (!p || !rect.width || !rect.height) return null;
    return P.containAt(size.W, size.H, rect.width, rect.height, p);
  };

  // LABEL_CH is the width, in drawing units, of one character of an 11-unit
  // monospaced epic id.
  const LABEL_CH = 6.7;

  // labelBox is the box behind an id label of n characters whose baseline
  // starts at x, y: opaque in the dependency views, so an edge passing
  // behind the label never runs through its text.
  P.labelBox = function (x, y, n) {
    return { x: x - 2, y: y - 10, w: n * LABEL_CH + 4, h: 14 };
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

  // layoutReady is what the dependency view shows for s, its key (the query
  // it shows, null when it needs a selection it lacks), dataKey (the query
  // its data was fetched for), data and error: none without a key, the
  // error the current key's fetch gave, the data only when it was fetched
  // for the current key, and else loading, so stale data is never drawn.
  P.layoutReady = function (s) {
    if (!s.key) return "none";
    if (s.error) return "error";
    return s.data && s.dataKey === s.key ? "draw" : "loading";
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

  // markState is what an epic of the export shows, closed saying its
  // milestone is not open, as { mark, hold }. The mark is done or its stage
  // after planned (claimed, specced, in_progress) straight from the status
  // field, so every place names a stage as the Lifecycle counts it; a
  // planned epic's mark says what holds it: moot, held by a condition,
  // blocked, its milestone closed, else ready. hold keeps the other fact for
  // an epic past planned and not done that is also held or blocked, drawn
  // over its mark, since the KPI and Waiting count it blocked; null else.
  P.markState = function (e, closed) {
    if (e.status && e.status !== "planned") {
      const under = e.status !== "done";
      return { mark: e.status, hold: under && e.held ? "held" : under && e.blocked ? "blocked" : null };
    }
    const at = (mark) => ({ mark, hold: null });
    if (e.moot) return at("moot");
    if (e.held) return at("held");
    if (e.blocked) return at("blocked");
    return at(closed ? "waiting" : "ready");
  };

  // markText is a markState result in words, read from labels by key: the
  // mark, and the hold after a comma ("claimed, blocked").
  P.markText = function (m, labels) {
    return labels[m.mark] + (m.hold ? ", " + labels[m.hold] : "");
  };

  // BLOCKER_RANK orders a blocker's states from the readiest; an any-of
  // group takes its readiest member's.
  const BLOCKER_RANK = ["startable", "in_progress", "specced", "claimed", "decision", "waiting"];

  // blockerState is what an epic others wait on is doing: startable when tp
  // epic ready lists it, a decision when it is a condition, its status when
  // it is past planned (claimed, specced or in_progress, the status field
  // as the export serves it), else waiting itself.
  const blockerState = (e, ready) => {
    if (!e) return "waiting";
    if (ready.includes(e.id)) return "startable";
    if (e.condition) return "decision";
    return e.status && e.status !== "planned" && e.status !== "done" ? e.status : "waiting";
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

  // waitingSections sorts waitingGroups' groups into one section per
  // blocker's milestone (read from epics, the export's epics by id), in
  // order, the graph file's milestones track by track, a section whose
  // milestone order does not hold last; an any-of group sits under its
  // member milestone first in order. Inside a section groups keep their
  // rank by direct waiters. Each section counts the distinct epics waiting
  // in it, and each group carries critical when its blocker, or a member of
  // its any-of group, is on critical, the critical path's ids: a mark, never
  // a sort key.
  P.waitingSections = function (groups, epics, order, critical) {
    const rank = (m) => { const i = order.indexOf(m); return i < 0 ? order.length : i; };
    const msOf = (id) => (epics[id] || {}).milestone || "";
    const crit = new Set(critical || []);
    const sections = new Map();
    groups.forEach((g) => {
      const members = g.blocker ? [g.blocker] : g.any_of;
      const m = members.map(msOf).sort((x, y) => rank(x) - rank(y))[0];
      if (!sections.has(m)) sections.set(m, { milestone: m, groups: [], ids: new Set() });
      const s = sections.get(m);
      s.groups.push(Object.assign({}, g, { critical: members.some((id) => crit.has(id)) }));
      g.epics.forEach((id) => s.ids.add(id));
    });
    return [...sections.values()].sort((x, y) => rank(x.milestone) - rank(y.milestone))
      .map((s) => ({ milestone: s.milestone, groups: s.groups, epics: s.ids.size }));
  };

  // flipWait is the Waiting view v with key flipped in its kind of set:
  // folded (a section's header) or all (a section's Show all); v itself and
  // every other key are left as they were.
  P.flipWait = function (v, kind, key) {
    const next = { all: new Set(v.all), folded: new Set(v.folded) };
    if (next[kind].has(key)) next[kind].delete(key);
    else next[kind].add(key);
    return next;
  };

  // waitSection is what a waitingSections section s shows under the view
  // v, n groups before its Show all: its key, its title ("no milestone"
  // when it has none), whether it is expanded, the groups shown (none when
  // folded) and its Show all, null when folded or when n groups hold it.
  P.waitSection = function (s, v, n) {
    const key = s.milestone;
    const name = key || "no milestone";
    const expanded = !v.folded.has(key);
    const all = v.all.has(key);
    const more = expanded && s.groups.length > n ? { expanded: all, text: all ? "Show the largest " + n : "Show all " + s.groups.length + " blockers in " + name } : null;
    return { key, title: "Blockers in " + name, expanded, groups: !expanded ? [] : all ? s.groups : s.groups.slice(0, n), more };
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

  // epicTotals is the one sentence that puts the count of every epic, which
  // the Epics tab shows, beside the work epics the share and the Lifecycle
  // count, naming the condition epics between them.
  P.epicTotals = function (epics) {
    const n = (k, one, many) => k + " " + (k === 1 ? one : many);
    const conds = (epics || []).filter((e) => e.condition).length;
    const all = (epics || []).length;
    return n(all, "epic", "epics") + ": " + n(all - conds, "work epic", "work epics") + " and " + n(conds, "condition epic", "condition epics") +
      "; the Lifecycle and the share count the work epics";
  };

  // msLine is a milestone card's one line: its gate's purpose when it has
  // one, since that says what the milestone is for, else threshold, the
  // threshold as the card words it.
  P.msLine = function (purpose, threshold) {
    const p = String(purpose || "").trim();
    return p ? { kind: "purpose", text: p } : { kind: "threshold", text: threshold };
  };

  // drawerNext is the drawer's next state from st, { detail, selected }, the
  // milestone shown and the epic selected, after act: "milestone" opens
  // act.id, or closes it when it is the one shown, an epic giving way;
  // "epic" selects act.id, or clears the selection when it is the selected
  // epic or null, a milestone giving way; "present" drops a milestone not in
  // act.milestones, the ids a push carries, so it cannot come back by itself.
  P.drawerNext = function (st, act) {
    if (act.type === "milestone") return { detail: st.detail === act.id && !st.selected ? null : act.id, selected: null };
    if (act.type === "epic") return { detail: null, selected: act.id && act.id !== st.selected ? act.id : null };
    if (act.type === "present") return { detail: st.detail && act.milestones.includes(st.detail) ? st.detail : null, selected: st.selected };
    return st;
  };

  // cardExpanded says whether milestone id's card reads expanded: while its
  // milestone is the one the drawer shows.
  P.cardExpanded = function (st, id) {
    return !st.selected && st.detail === id;
  };

  // escText escapes text for HTML, as the page's esc does.
  const escText = (s) => String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);

  // thresholdOf is a milestone's threshold text, the one place the card and
  // the drawer read it: the export's m.gate, else gate, the graph's gate.
  P.thresholdOf = function (m, gate) {
    return String((m.gate && m.gate.threshold) || (gate && gate.threshold) || "").trim();
  };

  // thresholdLine is the threshold as a milestone card words it: the text
  // of a written threshold, else its state's label from labels.
  P.thresholdLine = function (m, gate, labels) {
    const th = (m.state || {}).threshold || "absent";
    const text = P.thresholdOf(m, gate);
    return th === "written" && text ? "threshold: " + text : labels[th] || th;
  };

  // milestoneHTML is a milestone's detail in the drawer: its gate's state,
  // its threshold and purpose in full, which its card cuts to one line, and
  // dc, how many of its epics are described, every text escaped.
  P.milestoneHTML = function (m, gate, dc, labels) {
    const s = m.state || {};
    const th = s.threshold || "absent";
    const text = P.thresholdOf(m, gate);
    const n = (k, one, many) => k + " " + (k === 1 ? one : many);
    const waits = (m.waiting_on || []).length ? ", waits on " + m.waiting_on.join(", ") : "";
    const fields = [
      ["Gate", (m.open ? "open" : "closed") + waits + "; " + (s.members_done || 0) + " of " + n(s.members || 0, "member", "members") + " done"],
      ["Threshold", (labels[th] || th) + (text ? ": " + text : "")],
      ["Purpose", String((gate || {}).purpose || "").trim() || "none written"],
      ["Described", dc.described + " of " + n(dc.all, "epic", "epics") + " described"],
    ];
    return "<h3>" + escText(m.milestone) + "</h3><dl>" + fields.map(([k, v]) => "<dt>" + k + "</dt><dd>" + escText(v) + "</dd>").join("") + "</dl>";
  };

  // balancedColumns is the column count for n cards of at least minCard px
  // in a row width px wide, gap px apart: the fewest rows the width allows,
  // then the fewest columns that keep that many rows, so the rows come out
  // as even as a grid makes them (nine cards 5 + 4, not 8 + 1). Null while
  // the row has no width, as in a hidden tab, or holds no cards.
  P.balancedColumns = function (n, width, minCard, gap) {
    if (!(n > 0) || !(width > 0)) return null;
    const fit = Math.max(1, Math.min(n, Math.floor((width + gap) / (minCard + gap))));
    return Math.ceil(n / Math.ceil(n / fit));
  };

  // cardSpans lays n cards, at most cols to a row, over the fewest rows as
  // evenly as they divide, the fuller rows first, and gives the grid's
  // column count, one every row length divides, and each card's span in
  // it, so every row fills the width: 5 + 4 is 20 columns, spans 4 then 5.
  // Null for no cards or no column count.
  P.cardSpans = function (n, cols) {
    if (!(n > 0) || !(cols > 0)) return null;
    const rows = Math.ceil(n / cols);
    const q = Math.floor(n / rows);
    const r = n % rows;
    // Row lengths are q and q + 1, which share no factor.
    const columns = r ? q * (q + 1) : q;
    const spans = [];
    for (let row = 0; row < rows; row++) {
      const len = row < r ? q + 1 : q;
      for (let i = 0; i < len; i++) spans.push(columns / len);
    }
    return { columns, spans };
  };

  // closureDescribed says how many of a gate closure's nodes are in
  // described, the set of described epic ids, as the closure view's header
  // reads it.
  P.closureDescribed = function (nodes, described) {
    return nodes.filter((n) => described.has(n.id)).length + " of " + nodes.length + " in the closure described";
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

  // pointerAction is what a pointer coming down on a map does, pointers the
  // touch pointers down with it counted: a mouse's main button, a pen or one
  // finger pans, a second finger pinches, and any other button or a third
  // finger does nothing.
  P.pointerAction = function (pointers, ev) {
    if (ev.pointerType !== "touch") return ev.button === 0 ? "pan" : "none";
    if (pointers <= 1) return "pan";
    return pointers === 2 ? "pinch" : "none";
  };

  // wheelZooms says whether a wheel event zooms the map: with Ctrl or ⌘
  // held, or with the plain wheel while the map fills the screen.
  P.wheelZooms = function (ev, fullscreen) {
    return !!(ev.ctrlKey || ev.metaKey || fullscreen);
  };

  // tabStops gives each of ids its tabindex: 0 for focus, or for the first
  // when focus is none of them, -1 for the rest, so one is in the tab order.
  P.tabStops = function (ids, focus) {
    const at = Math.max(0, ids.indexOf(focus));
    return ids.map((_, i) => (i === at ? 0 : -1));
  };

  // focusAfterClose picks where focus goes back when the drawer closes:
  // "same" while the element that opened it is still on the page, else the
  // first key saved at opening that one of candidates, the keys of the
  // focusable elements on the page now, holds, else null.
  P.focusAfterClose = function (saved, candidates) {
    if (!saved) return null;
    if (saved.connected) return "same";
    return (saved.keys || []).find((k) => candidates.includes(k)) || null;
  };

  // TOGGLES are the data attributes of the buttons a redraw replaces and
  // focus must find again: a milestone card, which opens the milestone in
  // the drawer, a Waiting section's header and a Waiting section's Show all.
  const TOGGLES = ["ms", "wsec", "wall"];

  // toggleKey is the key of a toggle button from its dataset, so focus finds
  // the button a redraw put in its place; null for any other element.
  P.toggleKey = function (dataset) {
    const k = TOGGLES.find((t) => typeof dataset[t] === "string");
    return k ? k + ":" + dataset[k] : null;
  };

  // toggleOf is the attribute and value a toggleKey names.
  P.toggleOf = function (key) {
    const at = key.indexOf(":");
    return { attr: "data-" + key.slice(0, at), value: key.slice(at + 1) };
  };

  if (typeof module === "object" && module.exports) module.exports = P;
  else root.tpPure = P;
})(this);
