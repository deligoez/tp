// The graph card (spec/backlog/dashboard-graph.md): the plan, the
// milestones, a gate's closure and an epic's neighbourhood, laid out by tp at
// /api/layout as a forest packed into columns (layers top to bottom, tree
// edges solid, cross edges light beneath them until K hides them, back
// edges violet, the critical path always drawn) and drawn with deck.gl.
// The page computes no place: every row and column comes from tp. deck.gl
// is vendored under
// vendor/ and loaded only when the card first scrolls into view; a browser
// without WebGL2 gets a plain message instead. A canvas has no focusable
// node, so keyboard use runs through the search, the lists, the drawer and
// the chain as text (decision 4); the canvas box itself takes the arrows.
(function () {
  "use strict";
  const app = window.tpApp;
  const pure = window.tpPure;
  if (!app || !pure) return;
  const DECK_SRC = "vendor/deckgl-9.3.12.min.js";
  const $ = (id) => document.getElementById(id);
  // escape is s safe to place in HTML.
  const escape = (s) => String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);

  // t words the card in the page's language (tpApp.tr), so every word the
  // card writes is a key of i18n/<lang>.json; tk marks a key a table holds.
  // EDGE_TEXT is each edge class's legend key.
  const t = (key, params) => app.tr()(key, params);
  const tk = (key) => key;
  const EDGE_TEXT = { tree: tk("graph.edge.tree"), cross: tk("graph.edge.cross"), back: tk("graph.edge.back"), critical: tk("graph.edge.critical"), any: tk("graph.edge.any") };
  const milestoneLabel = (n) => n.id + "  " + n.done + "/" + n.epics;
  const milestoneTitle = (n) => t("graph.msTitle", { state: n.open ? t("ms.open") : t("ms.closed"), done: n.done, n: n.epics, in: n.inbound });
  // countLine is the count line under the toolbar, in full screen the
  // toolbar's end: what the card drew and, for a gate closure or a
  // neighbourhood, what it is drawn from. The layout's layers and columns
  // and the renderer's load time stay in the card's data-* attributes.
  function countLine(P) {
    const L = P.L;
    const nodes = L.view === "milestones" ? t("graph.countMilestones", { n: P.nodes.length }) : t("count.epics", { n: P.nodes.length });
    const line = t("graph.count", { nodes, edges: P.links.length });
    if (L.view === "scope") return line + " " + t("graph.inClosure", { ms: L.milestone });
    if (L.view === "neighbourhood") return line + " " + t("graph.within", { depth: L.depth, epic: L.epic });
    return line;
  }
  // The card's info popover ends with how many of a gate closure's epics
  // are described.
  app.graphNote = () => {
    const P = G.prepared;
    return P && P.L.view === "scope" ? pure.closureDescribed(P.L.nodes, P.described, app.tr()) : "";
  };

  // World geometry, one unit a screen pixel at zoom 0: an epic is a box of
  // w by h, one column col wide, one layer row tall, its id in lines of at
  // most chars characters (tpPure.wrapId) of the fs-unit monospace font.
  const GEO = {
    epic: { w: 150, h: 28, col: 172, row: 40, fs: 11, chars: 20 },
    milestone: { w: 200, h: 54, col: 240, row: 110, fs: 22, chars: 14 },
  };
  // The neighbourhood opens no further out than this, where an epic's
  // 11-unit id reads at 12 pixels, its whole id drawn (tpPure.detailLevel).
  const NEAR_ZOOM = 0.13;
  const CURVE = 10;
  // One arrow key pans this many screen pixels; + and − zoom by ZOOM_STEP.
  const PAN_PX = 90;
  // In full screen the toolbar floats over the canvas's top this tall.
  const FS_BAR_PX = 64;
  // A Safari pinch counts as over this long after its last step.
  const GESTURE_IDLE_MS = 500;
  const ZOOM_STEP = 0.5;
  // The marks in legend order, each with the colour token it is drawn in
  // (tpPure.GRAPH_MARKS).
  const MARKS = pure.GRAPH_MARKS;
  const EDGE_CLASSES = ["tree", "cross", "back", "critical", "any"];

  const G = {
    mode: "plan", milestone: null, depth: 2, showCross: true, whole: false, centre: null, q: "",
    hiddenMarks: new Set(), hiddenEdges: new Set(),
    data: null, key: null, dataKey: null, error: null, seq: 0, seen: null,
    deck: null, deckState: "idle", visible: false, built: null, builtFor: null, prepared: null,
    vs: null, detail: "near", fitted: null, matchIdx: -1, colors: null, loadMs: null, firstDrawMs: null,
    sets: null, setsKey: null, drawing: null, msgKey: tk("graph.notYet"), pinch: 1, gesture: false, themeVer: 0,
  };
  window.tpGraph = G;

  // ---------- the address: the card's place (tpPure.graphParams) ----------
  function readPlace(get) {
    Object.assign(G, pure.graphPlace(get));
    $("gc-q").value = G.q;
  }
  readPlace((k) => new URLSearchParams(location.hash.slice(1)).get(k));
  app.graphParams = () => pure.graphParams(G);
  app.readGraphPlace = (p) => readPlace((k) => p.get(k));

  // ---------- colours, read from the page's tokens ----------
  // readColors reads every token the palette needs and builds the graph's
  // colours through tpPure.graphPalette, so the contrast the Go test checks
  // on style.css is the contrast drawn.
  function readColors() {
    const css = getComputedStyle(document.documentElement);
    const tok = {};
    pure.GRAPH_TOKENS.forEach((name) => { tok[name] = css.getPropertyValue(name).trim(); });
    G.colors = pure.graphPalette(tok);
  }
  const mix = (a, b, t) => a.map((x, i) => Math.round(x + (b[i] - x) * t));

  // ---------- loading deck.gl, once and lazily ----------
  function hasWebGL2() {
    try {
      return !!document.createElement("canvas").getContext("webgl2");
    } catch (_) {
      return false;
    }
  }
  // say shows text in the card's message, or hides it for none; a message
  // said by its key keeps the key, so a change of language words it again.
  function say(text, key) {
    const m = $("gc-msg");
    G.msgKey = key || null;
    m.textContent = text;
    m.hidden = !text;
  }
  const sayKey = (key) => say(t(key), key);
  function loadDeck() {
    if (G.deckState !== "idle") return;
    if (!hasWebGL2() || new URLSearchParams(location.search).has("nogl")) {
      G.deckState = "nogl";
      card.classList.add("nogl");
      sayKey(tk("graph.noGL"));
      return;
    }
    G.deckState = "loading";
    sayKey(tk("graph.loading"));
    const t0 = performance.now();
    G.loadFrom = t0;
    const s = document.createElement("script");
    s.src = DECK_SRC;
    s.onload = () => {
      G.loadMs = Math.round(performance.now() - t0);
      G.deckState = "ready";
      draw();
    };
    s.onerror = () => {
      G.deckState = "failed";
      sayKey(tk("graph.failed"));
    };
    document.head.appendChild(s);
  }

  // The card loads deck.gl the first time it is on screen.
  const card = $("graphcard");
  new IntersectionObserver((entries) => {
    G.visible = entries.some((e) => e.isIntersecting);
    if (G.visible) {
      loadDeck();
      draw();
    }
  }, { rootMargin: "200px" }).observe(card);

  // ---------- the layout query and its fetch ----------
  function gateMilestone(g) {
    const ms = g.milestones || [];
    const pick = ms.find((m) => m.milestone === G.milestone) || ms.find((m) => m.open && m.share_done < 1) || ms[0];
    return pick ? pick.milestone : null;
  }
  // firstReady is the first epic of the export that can start, or null.
  function firstReady(g) {
    const reports = {};
    (g.milestones || []).forEach((m) => { reports[m.milestone] = m; });
    const e = (g.epics || []).find((x) => app.markOf(x, reports[x.milestone]).mark === "ready");
    return e ? e.id : null;
  }
  function query(g) {
    const p = new URLSearchParams({ view: G.mode });
    if (G.mode === "scope") {
      const m = gateMilestone(g);
      if (!m) return null;
      p.set("milestone", m);
    } else if (G.mode === "neighbourhood") {
      // The centre is the selection, else the last epic selected, so
      // closing the drawer keeps the neighbourhood it showed, else the
      // first epic that can start, so the mode never opens empty.
      const sel = app.selected() || G.centre || firstReady(g);
      if (!sel) return null;
      p.set("epic", sel);
      p.set("depth", String(G.depth));
    }
    return p.toString();
  }
  async function fetchLayout(key) {
    const seq = ++G.seq;
    G.key = key;
    G.error = null;
    let body;
    try {
      const res = await fetch("/api/layout?" + key, { cache: "no-store" });
      body = await res.json();
    } catch (err) {
      body = { error: t("graph.fetchFailed", { message: err.message }), hint: t("err.stillRunning") };
    }
    if (seq !== G.seq) return;
    if (body && typeof body.error === "string") Object.assign(G, { data: null, dataKey: null, error: body });
    else Object.assign(G, { data: body, dataKey: key, error: null });
    draw();
  }

  // ---------- preparing the drawing from a layout ----------
  function prepareNode(L, n, i, ctx) {
    const geo = ctx.geo;
    const x = n.col * geo.col;
    const y = n.layer * geo.row;
    const out = { i, id: n.id, x, y, w: geo.w, h: geo.h, mark: null, hold: null, label: n.id, ms: n.milestone || null, title: "", cond: false,
      centre: !!n.centre, deferrable: !!n.deferrable, layer: n.layer, col: n.col, status: null };
    if (L.view === "milestones") {
      out.mark = n.open ? (n.epics && n.done === n.epics ? "done" : "ready") : "waiting";
      out.label = milestoneLabel(n);
      out.title = milestoneTitle(n);
    } else {
      const e = ctx.epics[n.id] || { id: n.id, status: "planned" };
      out.ms = e.milestone || out.ms;
      const m = app.markOf(e, ctx.reports[e.milestone]);
      out.mark = m.mark;
      out.hold = m.hold;
      out.title = (ctx.titles[n.id] && ctx.titles[n.id].title) || e.title || "";
      out.cond = !!e.condition;
      out.status = e.status;
      out.label = n.id + (ctx.described.has(n.id) ? " ¶" : "");
    }
    // The id in its fixed-width box: wrapped at a punctuation mark onto a
    // second line near (tpPure.wrapId), its first line alone at mid, the
    // box never widened.
    const lines = pure.wrapId(out.label, geo.chars);
    out.text = lines.join("\n");
    out.short = lines.length > 1 ? lines[0].slice(0, geo.chars - 1) + "…" : lines[0];
    out.band = -1;
    const { w, h } = out;
    out.poly = out.cond
      ? [[x - w / 2 + 8, y], [x + w / 2 - 8, y], [x + w / 2, y + h / 2], [x + w / 2 - 8, y + h], [x - w / 2 + 8, y + h], [x - w / 2, y + h / 2]]
      : [[x - w / 2, y], [x + w / 2, y], [x + w / 2, y + h], [x - w / 2, y + h]];
    return out;
  }
  // bandRegions is each band's region: per layer it spans, the columns its
  // nodes on that layer take, joined into one outline, so a band in one
  // track does not reach over a neighbouring track's nodes on other layers.
  function bandRegions(L, nodes, geo) {
    const pad = 12;
    return (L.bands || []).map((b, k) => {
      const names = [b.milestone].concat(b.with || []);
      const inBand = nodes.filter((n) => names.includes(n.ms));
      if (!inBand.length) return null;
      const rows = [];
      for (let layer = b.from_layer; layer <= b.to_layer; layer++) {
        const on = inBand.filter((n) => n.layer === layer);
        if (on.length) rows.push({ y0: layer * geo.row - pad, y1: layer * geo.row + geo.h + pad, x0: Math.min(...on.map((n) => n.x - n.w / 2)) - pad, x1: Math.max(...on.map((n) => n.x + n.w / 2)) + pad });
      }
      rows[0].y0 -= 16;
      // Two rows meet half-way between them, so the outline steps across
      // horizontally even where the band skips a layer.
      rows.forEach((r, i) => {
        if (i === 0) return;
        const mid = (rows[i - 1].y1 + r.y0) / 2;
        rows[i - 1].y1 = mid;
        r.y0 = mid;
      });
      const right = rows.flatMap((r) => [[r.x1, r.y0], [r.x1, r.y1]]);
      const left = rows.slice().reverse().flatMap((r) => [[r.x0, r.y1], [r.x0, r.y0]]);
      inBand.forEach((n) => { n.band = k; });
      return { k, name: names.join(" + "), poly: right.concat(left), x0: rows[0].x0, y0: rows[0].y0, count: inBand.length };
    }).filter(Boolean);
  }
  function prepare(L, g) {
    const geo = L.view === "milestones" ? GEO.milestone : GEO.epic;
    // described is the graph's epics with a description, each marked ¶
    // after its id.
    const described = new Set(((g.graph && g.graph.epics) || []).filter((e) => String(e.description || "").trim()).map((e) => e.id));
    const ctx = { geo, epics: {}, reports: {}, titles: {}, described };
    (g.epics || []).forEach((e) => { ctx.epics[e.id] = e; });
    (g.milestones || []).forEach((m) => { ctx.reports[m.milestone] = m; });
    ((g.graph && g.graph.epics) || []).forEach((e) => { ctx.titles[e.id] = e; });
    const nodes = L.nodes.map((n, i) => prepareNode(L, n, i, ctx));
    const index = {};
    nodes.forEach((n) => { index[n.id] = n.i; });
    const links = [];
    L.edges.forEach((e) => {
      const s = index[e.from];
      const d = index[e.to];
      if (s === undefined || d === undefined) return;
      links.push({ s, d, role: e.role || "cross", critical: !!e.critical, any: !!e.any_of,
        long: pure.edgeLong(nodes[d].layer - nodes[s].layer, nodes[d].col - nodes[s].col) });
    });
    const preds = nodes.map(() => []);
    const succs = nodes.map(() => []);
    links.forEach((l, k) => { succs[l.s].push(k); preds[l.d].push(k); });
    links.forEach((l) => { l.path = curve(nodes[l.s], nodes[l.d], l.role === "back"); });
    const bands = bandRegions(L, nodes, geo);
    const bounds = nodes.length ? {
      x0: Math.min(...nodes.map((n) => n.x), ...bands.map((b) => b.x0)) - geo.w / 2 - 20, x1: Math.max(...nodes.map((n) => n.x)) + geo.w / 2 + 40,
      y0: Math.min(...nodes.map((n) => n.y), ...bands.map((b) => b.y0)) - 30, y1: Math.max(...nodes.map((n) => n.y)) + geo.h + 30,
    } : { x0: 0, x1: 1, y0: 0, y1: 1 };
    const counts = { marks: {}, edges: {} };
    nodes.forEach((n) => { counts.marks[n.mark] = (counts.marks[n.mark] || 0) + 1; });
    links.forEach((l) => {
      counts.edges[l.role] = (counts.edges[l.role] || 0) + 1;
      if (l.critical) counts.edges.critical = (counts.edges.critical || 0) + 1;
      if (l.any) counts.edges.any = (counts.edges.any || 0) + 1;
    });
    return { L, geo, nodes, links, preds, succs, bands, bounds, counts, index, described, charSet: pure.glyphsOf(nodes, bands), sel: -1, anc: null, desc: null, ver: 0 };
  }

  // curve is an edge's path: a cubic from the bottom of its dependency to
  // the top of its dependent, or for a back edge from the top round to the
  // bottom, as flat [x, y] pairs.
  function curve(s, d, back) {
    let x0, y0, x1, y1, c0x, c0y, c1x, c1y;
    if (back) {
      x0 = s.x; y0 = s.y; x1 = d.x; y1 = d.y + d.h;
      const bend = 0.6 * (y0 - y1) + 80;
      c0x = x0 + 120; c0y = y0 - bend * 0.3; c1x = x1 + 120; c1y = y1 + bend * 0.3;
    } else {
      x0 = s.x; y0 = s.y + s.h; x1 = d.x; y1 = d.y;
      const dy = Math.max(24, y1 - y0);
      c0x = x0; c0y = y0 + dy * 0.5; c1x = x1; c1y = y1 - dy * 0.5;
    }
    const out = [];
    for (let i = 0; i <= CURVE; i++) {
      const t = i / CURVE;
      const u = 1 - t;
      out.push([u * u * u * x0 + 3 * u * u * t * c0x + 3 * u * t * t * c1x + t * t * t * x1,
        u * u * u * y0 + 3 * u * u * t * c0y + 3 * u * t * t * c1y + t * t * t * y1]);
    }
    return out;
  }

  // ---------- selection: a node's prerequisites and dependents ----------
  // anc and desc map each node of the chain to its steps from the selection
  // (tpPure.hops); the highlight fades with them (tpPure.chainAlpha).
  function applySelection(P) {
    const id = app.selected();
    const i = id && P.index[id] !== undefined ? P.index[id] : -1;
    if (i === P.sel) return;
    P.sel = i;
    P.anc = i < 0 ? null : pure.hops(i, (v) => P.preds[v].map((k) => P.links[k].s));
    P.desc = i < 0 ? null : pure.hops(i, (v) => P.succs[v].map((k) => P.links[k].d));
    P.ver++;
  }
  const inChain = (P, i) => P.sel < 0 || i === P.sel || P.anc.has(i) || P.desc.has(i);
  // fade is a chain member's opacity, d steps out, faded past two steps
  // unless the legend asks for the whole chain.
  const fade = (d) => pure.chainAlpha(d, G.whole);
  const stepsOf = (P, i) => (i === P.sel ? 0 : Math.min(P.anc.get(i) || Infinity, P.desc.get(i) || Infinity));
  function matches(P) {
    const q = app.fold(G.q.trim());
    if (!q) return null;
    const hit = new Set();
    P.nodes.forEach((n) => { if (app.fold(n.id).includes(q) || app.fold(n.title).includes(q)) hit.add(n.i); });
    return hit;
  }

  // ---------- drawing ----------
  // staged says whether n is in the lifecycle's stage filter; a milestone
  // node always is.
  const staged = (n) => !n.status || app.inStage(n);
  function nodeAlpha(P, hits, n) {
    if (G.hiddenMarks.has(n.mark)) return 28;
    if (P.sel >= 0) return inChain(P, n.i) ? pure.chainAlpha(stepsOf(P, n.i), G.whole) : 40;
    if (hits && !hits.has(n.i)) return 50;
    return staged(n) ? 255 : 50;
  }
  // bandFill is band k's area colour, two greys alternating.
  const bandFill = (c, k) => mix(c.page, c.band, k % 2 ? 0.55 : 0.9);
  // REGION_TINT is how far a node's fill leans to its band's area colour
  // when no text is drawn, so the far view reads as regions, not as a
  // brick texture of boxes.
  const REGION_TINT = 0.45;
  function nodeFill(P, hits) {
    const c = G.colors;
    return (n) => {
      const mark = c.mark[n.mark] || c.muted;
      const fill = G.detail !== "far" ? c.fill[n.mark] || mark : n.band >= 0 ? mix(mark, bandFill(c, n.band), REGION_TINT) : mark;
      return [...fill, nodeAlpha(P, hits, n)];
    };
  }
  function nodeLine(P, hits) {
    const c = G.colors;
    return (n) => {
      if (n.i === P.sel) return [...c.ink, 255];
      if (P.sel >= 0) {
        const a = inChain(P, n.i) ? fade(stepsOf(P, n.i)) : 60;
        if (P.anc.has(n.i)) return [...c.anc, a];
        if (P.desc.has(n.i)) return [...c.desc, a];
        return [...c.line, 60];
      }
      if (hits && hits.has(n.i)) return [...c.accent, 255];
      if (n.hold) return [...c.mark[n.hold], 255];
      if (n.centre) return [...c.accent, 255];
      return [...(c.mark[n.mark] || c.muted), staged(n) ? 230 : 60];
    };
  }
  // nodeLineWidth draws no border far out but the selection's cone, a
  // search's matches and the marks a node holds.
  function nodeLineWidth(P, hits) {
    return (n) => {
      if (n.i === P.sel) return 3;
      if (P.sel >= 0 && (P.anc.has(n.i) || P.desc.has(n.i))) return stepsOf(P, n.i) <= 2 ? 2 : 1;
      if ((hits && hits.has(n.i)) || n.hold || n.centre) return 2;
      return G.detail === "far" ? 0 : 1;
    };
  }
  // onChain says whether l lies on the selection's chain: into the selection
  // from what it waits on, or out of it to what waits on it.
  function onChain(P, l) {
    if (P.sel < 0) return false;
    const { s, d } = l;
    const up = (d === P.sel || P.anc.has(d)) && P.anc.has(s);
    const down = (s === P.sel || P.desc.has(s)) && P.desc.has(d);
    return up || down;
  }
  // EDGE_STYLE is each edge's opacity, light page then dark, and width with
  // nothing selected, as the 1.17.1 audit rendered and compared them: cross
  // edges faint under the tree, fainter far out; tree edges in the ink; a
  // long edge (three layers or eight columns, tpPure.edgeLong) fainter than
  // its class. With an epic selected, the edges of its chain draw full at
  // CHAIN_WIDTH and the rest at OFF_CHAIN.
  const EDGE_STYLE = {
    crossFar: [24, 28, 0.6], cross: [40, 45, 0.75], crossLong: [18, 22, 0.6],
    tree: [170, 150, 1.4], treeFar: [130, 120, 1.2], long: [80, 70, 1], critical: [235, 235, 2.4],
  };
  const OFF_CHAIN = 12;
  const CHAIN_WIDTH = 2.5;
  // edgeStyle is l's EDGE_STYLE entry as nothing selected draws it.
  function edgeStyle(l) {
    if (l.critical) return EDGE_STYLE.critical;
    if (l.role === "cross") return l.long ? EDGE_STYLE.crossLong : G.detail === "far" ? EDGE_STYLE.crossFar : EDGE_STYLE.cross;
    if (l.long) return EDGE_STYLE.long;
    return G.detail === "far" ? EDGE_STYLE.treeFar : EDGE_STYLE.tree;
  }
  function edgeColor(P) {
    const c = G.colors;
    return (l) => {
      const base = l.role === "back" ? c.back : l.critical ? c.crit : l.role === "cross" ? c.muted : c.tree;
      if (P.sel >= 0) {
        if (!onChain(P, l)) return [...base, OFF_CHAIN];
        const hue = l.critical ? c.crit : l.role === "back" ? c.back : P.anc.has(l.s) ? c.anc : c.desc;
        return [...hue, fade(Math.max(stepsOf(P, l.s), stepsOf(P, l.d)))];
      }
      return [...base, edgeStyle(l)[c.dark ? 1 : 0]];
    };
  }
  function edgeWidth(P) {
    return (l) => (P.sel >= 0 && onChain(P, l) ? CHAIN_WIDTH : edgeStyle(l)[2]);
  }
  // edgeSets is the edge layers' data (tpPure.edgeLayers), kept while the
  // drawing, the selection and the toggles that choose them stay the same;
  // the selection's own chain draws its cross edges whatever the toggle.
  function edgeSets(P) {
    const shown = pure.crossShown(G);
    const key = P.ver + "|" + shown + "|" + [...G.hiddenEdges].join();
    if (G.setsKey !== key || G.sets === null || G.sets.of !== P) {
      const own = P.sel >= 0 ? (l) => onChain(P, l) : null;
      G.sets = { ...pure.edgeLayers(P.links, { showCross: shown, hidden: [...G.hiddenEdges], own }), of: P };
      G.setsKey = key;
    }
    return G.sets;
  }

  // LABEL_PX is a band label's height on screen and LABEL_CH the width of
  // one of its characters, an estimate for its 13-pixel bold font; a label
  // is a pill inside its band's top-left corner, LABEL_INSET from it, or in
  // the strip just above the band when a node sits in that corner.
  const LABEL_PX = 20;
  const LABEL_CH = 7.6;
  const LABEL_INSET = 6;
  // bandLabels is each band's label as this frame draws it: its name and
  // count, its name alone where that would overlap a label or a node, or
  // none, placed top to bottom, left to right, in screen pixels at scale
  // (tpPure.placeLabels), with the world position it is drawn at.
  function bandLabels(P, scale) {
    const order = P.bands.slice().sort((a, b) => a.y0 - b.y0 || a.x0 - b.x0);
    const full = (b) => b.name + " · " + b.count;
    // A label tries its band's top-left corner, then the strip just above
    // the band; the nodes' boxes, a pixel larger all round, are obstacles,
    // so a label is shortened, moved or left out rather than laid over one.
    const boxes = order.map((b) => ({
      x: b.x0 * scale + LABEL_INSET, y: b.y0 * scale + LABEL_INSET, w: full(b).length * LABEL_CH + 12, h: LABEL_PX, sw: b.name.length * LABEL_CH + 12,
      alts: [{ x: b.x0 * scale + LABEL_INSET, y: b.y0 * scale - LABEL_PX - 2 }],
    }));
    const nodes = P.nodes.map((n) => ({ x: (n.x - n.w / 2) * scale - 1, y: n.y * scale - 1, w: n.w * scale + 2, h: n.h * scale + 2 }));
    const at = pure.placeLabels(boxes, nodes);
    return order.map((b, i) => (at[i] ? { b, text: at[i].kind === "full" ? full(b) : b.name, wx: at[i].x / scale, wy: at[i].y / scale } : null)).filter(Boolean);
  }

  // nodeText is what a node's id reads at the frame's detail: its whole
  // wrapped id near, its first line mid, nothing far (the tooltip still
  // names it).
  const nodeText = (n) => (G.detail === "near" ? n.text : n.short);

  function render() {
    const P = G.prepared;
    if (!G.deck || !P) return;
    const { PolygonLayer, PathLayer, TextLayer } = window.deck;
    const c = G.colors;
    const scale = Math.pow(2, G.vs.zoom);
    G.detail = pure.detailLevel(P.geo.fs * scale);
    const hits = matches(P);
    const v = [P.ver, G.q, G.detail, pure.crossShown(G), G.whole, [...G.hiddenMarks].join(), [...G.hiddenEdges].join(), G.themeVer, app.stage()];
    const fontSettings = { sdf: true, fontSize: 64, buffer: 6, radius: 12, cutoff: 0.22 };
    const dash = window.deck.PathStyleExtension ? [new window.deck.PathStyleExtension({ dash: true })] : [];
    const sets = edgeSets(P);
    const edgeLayer = (id, data) => new PathLayer({
      id, data, getPath: (l) => l.path, getColor: edgeColor(P), getWidth: edgeWidth(P), widthUnits: "pixels",
      capRounded: true, jointRounded: true, updateTriggers: { getColor: v, getWidth: v },
      ...(dash.length && id === "any" ? { extensions: dash, getDashArray: [4, 3], dashJustified: true } : {}),
    });
    const sel = P.sel >= 0 ? P.nodes[P.sel] : null;
    const layers = [
      new PolygonLayer({
        id: "bands", data: P.bands, getPolygon: (b) => b.poly, stroked: true, filled: true, pickable: true,
        getFillColor: (b) => [...bandFill(c, b.k), 255],
        getLineColor: [...c.line, 255], getLineWidth: 1, lineWidthUnits: "pixels", updateTriggers: { getFillColor: v },
      }),
      edgeLayer("cross", sets.cross),
      edgeLayer("any", sets.any),
      edgeLayer("long", sets.long),
      edgeLayer("tree", sets.tree),
      edgeLayer("crit", sets.crit),
      new PolygonLayer({
        id: "nodes", data: P.nodes, getPolygon: (n) => n.poly, stroked: true, filled: true, pickable: true, autoHighlight: true,
        highlightColor: [...c.accent, 70], getFillColor: nodeFill(P, hits), getLineColor: nodeLine(P, hits), getLineWidth: nodeLineWidth(P, hits),
        lineWidthUnits: "pixels", updateTriggers: { getFillColor: v, getLineColor: v, getLineWidth: v },
      }),
      new TextLayer({
        id: "ids", data: P.nodes, visible: G.detail !== "far", getPosition: (n) => [n.x, n.y + n.h / 2], getText: nodeText,
        getSize: P.geo.fs, sizeUnits: "common", getTextAnchor: "middle", getAlignmentBaseline: "center", lineHeight: 1.05,
        getColor: (n) => [...c.ink, nodeAlpha(P, hits, n) >= 170 ? 255 : 70],
        fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace", characterSet: P.charSet, fontSettings,
        updateTriggers: { getColor: v, getText: v },
      }),
      new TextLayer({
        id: "band-names", data: bandLabels(P, scale), getPosition: (l) => [l.wx, l.wy], getText: (l) => l.text,
        getPixelOffset: [6, 3], getSize: 13, sizeUnits: "pixels", getTextAnchor: "start", getAlignmentBaseline: "top", getColor: [...c.ink, 235],
        fontFamily: "system-ui, sans-serif", fontWeight: 700, characterSet: P.charSet, fontSettings,
        background: true, getBackgroundColor: [...c.page, 235], backgroundPadding: [6, 3, 6, 3],
        getBorderColor: [...c.line, 255], getBorderWidth: 1, updateTriggers: { getColor: v, getBackgroundColor: v, getBorderColor: v },
      }),
      new TextLayer({
        id: "sel-title", data: sel && sel.title ? [sel] : [], getPosition: (n) => [n.x + n.w / 2 + 8, n.y + n.h / 2], getText: (n) => n.title,
        getSize: 14, sizeUnits: "pixels", getTextAnchor: "start", getAlignmentBaseline: "center", getColor: [...c.ink, 255],
        fontFamily: "system-ui, sans-serif", fontWeight: 600, characterSet: P.charSet, fontSettings,
        outlineWidth: 4, outlineColor: [...c.page, 255], updateTriggers: { getColor: v, outlineColor: v },
      }),
    ];
    // What this frame holds, recorded on the card once deck.gl drew it.
    G.drawing = { mode: P.L.view, nodes: P.nodes.length, edges: sets.drawn, layers: P.L.layers, cols: P.L.columns };
    G.deck.setProps({ layers, viewState: G.vs });
  }
  // recordDrawn writes what the last frame drew onto the card, so the audit
  // reads per mode that the canvas holds the layout's nodes and edges, and
  // the first frame's time from the card asking for the renderer: loading
  // it, fetching the layout, preparing it and drawing. The layout's layers
  // and columns live here too, not in the count line.
  function recordDrawn() {
    const d = G.drawing;
    if (!d) return;
    card.dataset.drawnMode = d.mode;
    card.dataset.drawnNodes = String(d.nodes);
    card.dataset.drawnEdges = String(d.edges);
    card.dataset.layers = String(d.layers || 0);
    card.dataset.columns = String(d.cols || 0);
    card.dataset.detail = G.detail;
    if (G.firstDrawMs === null && d.nodes) {
      G.firstDrawMs = Math.round(performance.now() - G.loadFrom);
      card.dataset.firstDrawMs = String(G.firstDrawMs);
      card.dataset.rendererMs = String(G.loadMs);
    }
  }

  // ---------- camera ----------
  // fitView is the view that shows the whole layout: a margin of 24 pixels
  // a side and, in full screen, the floating toolbar's strip at the top; a
  // small graph is never magnified past zoom 0 and never fitted further out
  // than its labels read (tpPure.fitFloor), and then it sits at the top.
  function fitView() {
    const P = G.prepared;
    const box = $("gc-canvas");
    const w = Math.max(1, box.clientWidth);
    const h = Math.max(1, box.clientHeight);
    const b = P.bounds;
    const bar = isFull() ? FS_BAR_PX : 0;
    const whole = Math.log2(Math.min((w - 48) / (b.x1 - b.x0), (h - 48 - bar) / (b.y1 - b.y0)));
    const floor = pure.fitFloor(P.nodes.length, P.L.view, P.geo.fs);
    const zoom = Math.min(0, Math.max(floor, whole));
    const s = Math.pow(2, zoom);
    const top = zoom > whole ? b.y0 + (h / 2 - 24 - bar) / s : (b.y0 + b.y1) / 2 - bar / 2 / s;
    return { target: [(b.x0 + b.x1) / 2, top, 0], zoom, minZoom: -8, maxZoom: 3 };
  }
  // fit goes to fitView and remembers the camera sits at its fit, so the
  // card fits again when full screen changes the canvas's size.
  function fit() {
    goTo(fitView());
    G.atFit = true;
  }
  function goTo(vs) {
    G.vs = pure.cameraOf(vs);
    G.atFit = false;
    render();
  }
  function flyTo(n) {
    goTo({ ...G.vs, target: [n.x, n.y + n.h / 2, 0], zoom: Math.max(G.vs.zoom, NEAR_ZOOM) });
  }
  // pan moves the view by dx, dy screen pixels; zoomBy zooms about the
  // centre.
  function pan(dx, dy) {
    const s = Math.pow(2, G.vs.zoom);
    goTo({ ...G.vs, target: [G.vs.target[0] + dx / s, G.vs.target[1] + dy / s, 0] });
  }
  function zoomBy(dz) {
    goTo({ ...G.vs, zoom: Math.min(3, Math.max(-8, G.vs.zoom + dz)) });
  }
  // zoomAt zooms by dz about the screen point (cx, cy), so what sits under
  // the pointer stays under it.
  function zoomAt(dz, cx, cy) {
    const box = $("gc-canvas").getBoundingClientRect();
    const px = cx - box.left - box.width / 2;
    const py = cy - box.top - box.height / 2;
    const s0 = Math.pow(2, G.vs.zoom);
    const z = Math.min(3, Math.max(-8, G.vs.zoom + dz));
    const s1 = Math.pow(2, z);
    const wx = G.vs.target[0] + px / s0;
    const wy = G.vs.target[1] + py / s0;
    goTo({ ...G.vs, zoom: z, target: [wx - px / s1, wy - py / s1, 0] });
  }
  // The wheel (tpPure.wheelAction): on the canvas, Ctrl or ⌘ with it, and
  // Chrome's and Firefox's pinch, zoom about the pointer; in full screen the
  // plain wheel pans; outside it the page scrolls. The box listens in the
  // capture phase, ahead of deck.gl's canvas inside it.
  function onWheel(ev, place) {
    if (!G.vs || !G.prepared) return;
    if (!ev.ctrlKey && !ev.metaKey) G.gesture = false;
    const act = pure.wheelAction(ev, isFull(), place);
    if (act === "page") return;
    ev.preventDefault();
    // A pinch Safari reports as a gesture zooms there alone.
    if (act === "block" || (act === "zoom" && G.gesture)) return;
    const [dx, dy] = pure.wheelDelta(ev, $("gc-canvas").clientHeight);
    if (act === "zoom") zoomAt(pure.wheelZoom(dy), ev.clientX, ev.clientY);
    else pan(dx, dy);
  }
  // Safari's pinch (tpPure.gestureAction): gesture events carry the scale
  // since the pinch began, and each step zooms by its change, through
  // zoomAt like the wheel's.
  function onGesture(ev, place) {
    const act = pure.gestureAction(isFull(), place);
    if (act === "page" || !G.vs) return;
    ev.preventDefault();
    if (act === "block") return;
    if (ev.type === "gesturestart") {
      G.pinch = 1;
      G.gesture = true;
      return;
    }
    if (ev.type === "gestureend") {
      G.gesture = false;
      return;
    }
    if (!G.prepared) return;
    zoomAt(pure.pinchZoom(G.pinch, ev.scale), ev.clientX, ev.clientY);
    G.pinch = ev.scale;
    // A gestureend Safari never sends must not leave Ctrl-wheel zoom off.
    clearTimeout(G.gestureTimer);
    G.gestureTimer = setTimeout(() => { G.gesture = false; }, GESTURE_IDLE_MS);
  }
  const GESTURES = ["gesturestart", "gesturechange", "gestureend"];
  $("gc-canvas").addEventListener("wheel", (ev) => onWheel(ev, "canvas"), { passive: false, capture: true });
  GESTURES.forEach((type) => $("gc-canvas").addEventListener(type, (ev) => onGesture(ev, "canvas")));
  // In full screen the document hears what lands off the canvas: over the
  // card's floating toolbar and legend, by the pointer's point, a zoom
  // zooms the graph, and anywhere else it is cancelled, so a pinch never
  // zooms the page behind a graph that fills the screen.
  const overCard = (ev) => {
    const r = card.getBoundingClientRect();
    return ev.clientX >= r.left && ev.clientX < r.right && ev.clientY >= r.top && ev.clientY < r.bottom;
  };
  const offCanvas = (handle) => (ev) => {
    if (!$("gc-canvas").contains(ev.target)) handle(ev, overCard(ev) ? "card" : "page");
  };
  const fsWheel = offCanvas(onWheel);
  const fsGesture = offCanvas(onGesture);
  function listenOffCanvas(on) {
    const m = on ? "addEventListener" : "removeEventListener";
    document[m]("wheel", fsWheel, { passive: false, capture: true });
    GESTURES.forEach((type) => document[m](type, fsGesture, { passive: false, capture: true }));
  }

  function tooltip({ object, layer }) {
    // A band names itself on hover, so a label left out to spare a node is
    // still one pointer away.
    if (object && layer && layer.id === "bands") return { text: object.name + " · " + object.count };
    if (!object || !layer || layer.id !== "nodes") return null;
    const n = object;
    const lines = G.prepared.L.view === "milestones"
      ? "<b>" + escape(n.id) + "</b><br>" + escape(n.title)
      : '<span style="opacity:.7;font-family:ui-monospace,monospace;font-size:11px">' + escape(n.id) + (n.ms ? " · " + escape(n.ms) : "") + (n.cond ? " · " + escape(t("drawer.condition")) : "") + "</span><br>" +
        escape(n.title) + "<br>" + escape(app.markText({ mark: n.mark, hold: n.hold })) + (n.deferrable ? ", " + escape(t("graph.deferrable")) : "");
    const c = G.colors;
    return { html: lines, style: { background: "rgb(" + c.ink + ")", color: "rgb(" + c.surface + ")", borderRadius: "6px", maxWidth: "320px", fontSize: "13px", padding: "6px 10px", lineHeight: "1.4" } };
  }
  function ensureDeck() {
    if (G.deck) return;
    say("");
    G.deck = new window.deck.Deck({
      parent: $("gc-canvas"),
      views: new window.deck.OrthographicView({ id: "ortho", flipY: true }),
      controller: { scrollZoom: false, dragRotate: false, doubleClickZoom: true, touchZoom: true, keyboard: false, inertia: 200 },
      viewState: G.vs,
      onViewStateChange: ({ viewState }) => {
        G.vs = pure.cameraOf(viewState);
        G.atFit = false;
        render();
      },
      onAfterRender: recordDrawn,
      onClick: (info) => {
        if (info.object && info.layer && info.layer.id === "nodes") pick(info.object);
        else if (app.selected()) app.selectEpic(null);
      },
      getCursor: ({ isHovering, isDragging }) => (isDragging ? "grabbing" : isHovering ? "pointer" : "grab"),
      getTooltip: tooltip,
      onLoad: noTabStop,
    });
    noTabStop();
  }
  // noTabStop takes deck.gl's canvas out of the tab order, so the canvas
  // box, which takes the arrows, is the card's one tab stop.
  function noTabStop() {
    $("gc-canvas").querySelectorAll("canvas").forEach((c) => { c.tabIndex = -1; });
  }

  // setPlace changes the card's place and keeps it in the address.
  function setPlace(change) {
    Object.assign(G, change);
    app.syncURL();
    draw();
  }
  // showClosure is the milestone drawer's way to its gate's closure: the
  // card in Gate closure on ms, scrolled into view.
  app.showClosure = (ms) => {
    setPlace({ mode: "scope", milestone: ms });
    card.scrollIntoView({ block: "start", behavior: "smooth" });
  };

  // pick answers a click on a node: a milestone opens its gate's closure, an
  // epic is selected (or cleared when it is the selected one), and in the
  // neighbourhood the view re-centres on it.
  function pick(n) {
    if (G.prepared.L.view === "milestones") {
      setPlace({ mode: "scope", milestone: n.id });
      return;
    }
    app.selectEpic(n.id === app.selected() ? null : n.id);
  }
  // A selection made anywhere, the graph, a list, the drawer, the chain as
  // text or the search, shows on the graph; one made outside it is centred.
  // The last epic selected stays the neighbourhood's centre after the
  // selection clears.
  app.onSelect = (id, centre) => {
    if (id) G.centre = id;
    if (G.mode === "neighbourhood") {
      draw();
      return;
    }
    const P = G.prepared;
    if (!P) return;
    applySelection(P);
    drawLegend();
    render();
    if (centre && id && P.index[id] !== undefined && G.vs) flyTo(P.nodes[P.index[id]]);
  };

  // ---------- the toolbar and legend ----------
  function drawToolbar(g) {
    card.querySelectorAll("[data-gmode]").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.gmode === G.mode)));
    $("gc-ms-pick").hidden = G.mode !== "scope";
    $("gc-depth-pick").hidden = G.mode !== "neighbourhood";
    const order = [].concat(...((g.graph && g.graph.milestones) || []));
    if ($("gc-ms").dataset.key !== order.join("\u0000")) {
      $("gc-ms").dataset.key = order.join("\u0000");
      $("gc-ms").innerHTML = order.map((m) => "<option>" + escape(m) + "</option>").join("");
    }
    $("gc-ms").value = gateMilestone(g) || "";
    $("gc-depth").value = String(G.depth);
    $("gc-cross").setAttribute("aria-pressed", String(pure.crossShown(G)));
    // The search's placeholder names what it finds in this mode.
    $("gc-q").placeholder = t(G.mode === "milestones" ? tk("graph.searchMilestone") : tk("graph.search"));
    if ($("gc-q").value !== G.q && document.activeElement !== $("gc-q")) $("gc-q").value = G.q;
  }
  function drawLegend() {
    const P = G.prepared;
    if (!P) { $("gc-legend").innerHTML = ""; return; }
    const c = G.colors;
    const rgb = (a) => "rgb(" + a.join(",") + ")";
    // One row of marks, each counting the nodes drawn with it (conditions
    // included, so they differ from the plan figures above), then one row
    // of edge toggles; since 1.17.1 no heading says so, the counts sit on
    // the marks themselves.
    const marks = MARKS.filter(([k]) => P.counts.marks[k]).map(([k]) =>
      '<button type="button" data-lmark="' + k + '" aria-pressed="' + !G.hiddenMarks.has(k) + '"><i style="background:' + rgb(c.mark[k]) + '"></i>' + escape(app.markWord(k)) + " <b>" + escape(app.tr().num(P.counts.marks[k])) + "</b></button>").join("");
    const ecol = { tree: c.tree, cross: c.muted, back: c.back, critical: c.crit, any: c.accent };
    const edges = EDGE_CLASSES.filter((k) => P.counts.edges[k]).map((k) =>
      '<button type="button" data-ledge="' + k + '" aria-pressed="' + (k === "cross" ? pure.crossShown(G) : !G.hiddenEdges.has(k)) + '"><i class="ln" style="background:' + rgb(ecol[k]) + (k === "cross" ? ";opacity:.45" : "") + '"></i>' + escape(t(EDGE_TEXT[k])) + " <b>" + escape(app.tr().num(P.counts.edges[k])) + "</b></button>").join("");
    // The chain's counts say how many lie within two steps, the part drawn
    // full, and a toggle draws the whole chain full.
    const sel = P.sel >= 0 ? '<span><i class="ln" style="background:' + rgb(c.anc) + '"></i>' + escape(t("graph.waitsOn")) + " <b>" + escape(t("graph.near", { n: P.anc.size, near: pure.withinTwo(P.anc) })) +
      '</b></span><span><i class="ln" style="background:' + rgb(c.desc) + '"></i>' + escape(t("graph.waitedOn")) + " <b>" + escape(t("graph.near", { n: P.desc.size, near: pure.withinTwo(P.desc) })) + "</b></span>" +
      '<button type="button" data-lwhole aria-pressed="' + G.whole + '">' + escape(t("graph.whole")) + "</button>" : "";
    $("gc-legend").innerHTML = marks + '<span class="lhead"></span>' + edges + sel +
      '<span class="help">' + escape((P.L.view === "milestones" ? t("graph.help.milestones") : t("graph.help.epics")) + " " + t("graph.help.input")) + "</span>";
  }
  $("gc-legend").addEventListener("click", (ev) => {
    const m = ev.target.closest("[data-lmark]");
    const e = ev.target.closest("[data-ledge]");
    if (m) {
      const k = m.dataset.lmark;
      if (G.hiddenMarks.has(k)) G.hiddenMarks.delete(k); else G.hiddenMarks.add(k);
    } else if (e) {
      const k = e.dataset.ledge;
      if (k === "cross") {
        toggleCross();
        return;
      }
      if (G.hiddenEdges.has(k)) G.hiddenEdges.delete(k); else G.hiddenEdges.add(k);
    } else if (ev.target.closest("[data-lwhole]")) {
      G.whole = !G.whole;
    } else return;
    drawLegend();
    drawToolbar(app.data("epic-export") || {});
    render();
  });
  card.addEventListener("click", (ev) => {
    const b = ev.target.closest("[data-gmode]");
    if (b) setPlace({ mode: b.dataset.gmode });
  });
  $("gc-ms").addEventListener("change", (ev) => setPlace({ mode: "scope", milestone: ev.target.value }));
  $("gc-depth").addEventListener("change", (ev) => setPlace({ depth: Number(ev.target.value) }));
  // toggleCross shows or hides the plain cross edges in every mode, drawn
  // until hidden, the hidden state kept in the address (cross=0).
  function toggleCross() {
    G.showCross = !G.showCross;
    app.syncURL();
    drawToolbar(app.data("epic-export") || {});
    drawLegend();
    render();
  }
  $("gc-cross").addEventListener("click", toggleCross);

  // ---------- the legend and keys card ----------
  // One card holds the legend and the keys: open on the page, collapsed in
  // full screen until the viewer chooses (tpPure.legendOpen), L or its
  // header toggling it, each place's choice kept in this browser.
  const isFull = () => card.classList.contains("fs");
  function storedLegend() {
    try { return localStorage.getItem(pure.legendKey(isFull())); } catch (e) { return null; }
  }
  function showLegend(open) {
    $("gc-legendcard").classList.toggle("closed", !open);
    $("gc-lg").setAttribute("aria-expanded", String(open));
  }
  function toggleLegend() {
    const open = $("gc-legendcard").classList.contains("closed");
    try { localStorage.setItem(pure.legendKey(isFull()), open ? "1" : "0"); } catch (e) { /* the choice lasts this load */ }
    showLegend(open);
  }
  $("gc-lg").addEventListener("click", toggleLegend);
  // Entering or leaving full screen reads that place's choice and starts or
  // stops hearing the pinch off the canvas. The canvas's own size, read
  // once the window has grown or shrunk, refits a graph still at its fit
  // when the box changed on the screen (tpPure.resizeRefits); a page zoom
  // leaves the graph to zoom with the page.
  document.addEventListener("tp-fs", () => {
    showLegend(pure.legendOpen(storedLegend(), isFull()));
    listenOffCanvas(isFull());
  });
  let boxSize = null;
  new ResizeObserver(() => {
    const box = $("gc-canvas");
    const next = { w: box.clientWidth, h: box.clientHeight, dpr: window.devicePixelRatio || 1 };
    const refit = pure.resizeRefits(boxSize, next);
    boxSize = next;
    if (refit && G.prepared && G.deck && G.atFit) fit();
  }).observe($("gc-canvas"));
  // The full-screen pill keeps its words whole while it fits: measured with
  // them whole, it shortens them (.tight) only when it overflows its room,
  // beside an open drawer or on a narrow screen (tpPure.pillTight).
  const pill = card.querySelector(".toolbar");
  function fitPill() {
    pill.classList.remove("tight");
    if (isFull()) pill.classList.toggle("tight", pure.pillTight(pill.scrollWidth, pill.clientWidth));
  }
  new ResizeObserver(fitPill).observe(pill);
  document.addEventListener("tp-fs", fitPill);
  showLegend(pure.legendOpen(storedLegend(), isFull()));
  $("gc-fit").addEventListener("click", () => { if (G.prepared) fit(); });
  // The canvas's own + − Fit, bottom right, in the page and in full screen.
  $("gc-zfit").addEventListener("click", () => { if (G.prepared) fit(); });
  $("gc-zin").addEventListener("click", () => { if (G.vs) zoomBy(ZOOM_STEP); });
  $("gc-zout").addEventListener("click", () => { if (G.vs) zoomBy(-ZOOM_STEP); });
  // The page's header switches the theme, or the system's setting changes
  // under Auto (tp-theme); the card reads its colours again and bumps its
  // theme version, which every layer's update triggers hold, so deck.gl
  // draws each colour anew.
  function retheme() {
    G.themeVer++;
    readColors();
    drawToolbar(app.data("epic-export") || {});
    drawLegend();
    render();
  }
  document.addEventListener("tp-theme", retheme);

  // ---------- the search and the keys ----------
  // exportMatches is the search over the export's epics, by id or title,
  // for when nothing is laid out: Neighbourhood before an epic is picked,
  // or before the renderer loads.
  function exportMatches() {
    const q = app.fold(G.q.trim());
    if (!q) return [];
    const g = app.data("epic-export") || {};
    return ((g.graph && g.graph.epics) || []).filter((e) => app.fold(e.id).includes(q) || app.fold(e.title || "").includes(q)).map((e) => e.id);
  }
  function showMatches() {
    const P = G.prepared;
    if (!P) {
      $("gc-n").textContent = G.q.trim() ? t("graph.matches", { n: exportMatches().length }) : "";
      return;
    }
    const hits = matches(P);
    $("gc-n").textContent = hits ? t("graph.matches", { n: hits.size }) : countText();
    // In full screen the count sits at the toolbar's end.
    $("gc-nfs").textContent = $("gc-n").textContent;
  }
  $("gc-q").addEventListener("input", (ev) => {
    G.q = ev.target.value;
    G.matchIdx = -1;
    app.syncURL();
    showMatches();
    render();
  });
  // Enter selects the next match and centres it, Shift+Enter the previous,
  // focus staying here so Enter steps on; Esc clears the search.
  $("gc-q").addEventListener("keydown", (ev) => {
    const P = G.prepared;
    if (ev.key === "Escape") {
      if (!G.q) return;
      ev.preventDefault();
      ev.stopPropagation();
      ev.target.value = "";
      G.q = "";
      app.syncURL();
      showMatches();
      render();
      return;
    }
    if (ev.key !== "Enter") return;
    if (!P) {
      const ids = exportMatches();
      if (!ids.length) return;
      ev.preventDefault();
      G.matchIdx = pure.stepMatch(ids.length, G.matchIdx, ev.shiftKey);
      app.selectEpic(ids[G.matchIdx], true, true);
      return;
    }
    const hits = [...(matches(P) || [])];
    if (!hits.length) return;
    ev.preventDefault();
    G.matchIdx = pure.stepMatch(hits.length, G.matchIdx, ev.shiftKey);
    const n = P.nodes[hits[G.matchIdx]];
    flyTo(n);
    if (P.L.view !== "milestones" && n.id !== app.selected()) app.selectEpic(n.id, false, true);
  });
  // "/" reaches the graph's search whenever the Epics tab shows the card,
  // laid out or not, ahead of the page's own "/" for the Specs filter, so
  // it listens in the capture phase; a hidden tab's input is never focused.
  const typing = (t) => t && (t.tagName === "INPUT" || t.tagName === "SELECT" || t.tagName === "TEXTAREA" || t.isContentEditable);
  const cardShown = () => !card.hidden && !$("view-epics").hidden;
  document.addEventListener("keydown", (ev) => {
    if (ev.key !== "/" || ev.metaKey || ev.ctrlKey || ev.altKey || typing(ev.target) || !cardShown() || card.classList.contains("nogl")) return;
    ev.preventDefault();
    ev.stopImmediatePropagation();
    $("gc-q").focus();
    $("gc-q").select();
  }, true);
  // On the focused canvas, or anywhere in full screen, the arrows pan and +
  // and − zoom; F fits and K toggles cross edges whenever the card shows;
  // Esc clears a selection the drawer does not hold.
  const KEY_PAN = { ArrowLeft: [-PAN_PX, 0], ArrowRight: [PAN_PX, 0], ArrowUp: [0, -PAN_PX], ArrowDown: [0, PAN_PX] };
  document.addEventListener("keydown", (ev) => {
    if (!cardShown() || !G.prepared || !G.vs || typing(ev.target) || ev.metaKey || ev.ctrlKey || ev.altKey) return;
    const onCanvas = ev.target === $("gc-canvas") || card.classList.contains("fs");
    if (onCanvas && KEY_PAN[ev.key]) {
      ev.preventDefault();
      pan(...KEY_PAN[ev.key]);
    } else if (onCanvas && (ev.key === "+" || ev.key === "=")) {
      ev.preventDefault();
      zoomBy(ZOOM_STEP);
    } else if (onCanvas && (ev.key === "-" || ev.key === "_")) {
      ev.preventDefault();
      zoomBy(-ZOOM_STEP);
    } else if (ev.key === "f" || ev.key === "F") fit();
    else if (ev.key === "k" || ev.key === "K") toggleCross();
    else if (ev.key === "l" || ev.key === "L") toggleLegend();
    else if (ev.key === "Escape" && app.selected() && $("drawer").hidden) app.selectEpic(null);
  });
  function countText() {
    return G.prepared ? countLine(G.prepared) : "";
  }

  // ---------- the panel ----------
  // clearCanvas empties the card: no layout prepared, no legend or count,
  // no layer drawn, and 0 recorded as drawn for the mode.
  function clearCanvas() {
    G.prepared = null;
    G.built = null;
    drawLegend();
    $("gc-n").textContent = "";
    G.drawing = { mode: G.mode, nodes: 0, edges: 0 };
    recordDrawn();
    if (G.deck) G.deck.setProps({ layers: [] });
  }
  function draw() {
    const g = app.data("epic-export");
    card.hidden = !g;
    if (!g) return;
    if (!G.colors) readColors();
    if (G.msgKey) sayKey(G.msgKey);
    drawToolbar(g);
    if (!G.visible || G.deckState !== "ready") return;
    const key = query(g);
    if (!key) {
      clearCanvas();
      sayKey(tk("graph.pickEpic"));
      return;
    }
    if (key !== G.key || G.seen !== g) {
      G.seen = g;
      fetchLayout(key);
    }
    if (G.error) {
      clearCanvas();
      say(G.error.error);
      return;
    }
    if (!G.data || G.dataKey !== key) return;
    if (G.built !== G.data || G.builtFor !== g) {
      const keep = G.prepared && G.prepared.L.view === G.data.view && G.fitted === key;
      G.prepared = prepare(G.data, g);
      G.built = G.data;
      G.builtFor = g;
      applySelection(G.prepared);
      ensureDeck();
      say("");
      if (!keep) {
        // The first view is a fit, so full screen fits it again; a
        // neighbourhood's, centred on its epic, is not.
        G.vs = fitView();
        G.atFit = true;
        G.fitted = key;
        if (G.data.view === "neighbourhood") {
          const c = G.prepared.nodes.find((n) => n.centre);
          if (c) {
            G.vs = { ...G.vs, target: [c.x, c.y + c.h / 2, 0], zoom: Math.max(G.vs.zoom, NEAR_ZOOM) };
            G.atFit = false;
          }
        }
      }
    } else applySelection(G.prepared);
    showMatches();
    drawLegend();
    render();
  }
  app.panel(["epic-export", "summary"], draw);
  // api lets a scripted browser drive the camera for screenshots and checks.
  G.api = { goTo, fitView, draw, render, flyTo, pan, zoomBy };
})();
