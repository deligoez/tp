// The graph card (spec/backlog/dashboard-graph.md): the plan, the
// milestones, a gate's closure and an epic's neighbourhood, laid out by tp at
// /api/layout as a forest packed into columns (layers top to bottom, tree
// edges solid, cross edges hidden until asked for, back edges violet, the
// critical path always drawn) and drawn with deck.gl. The page computes no
// place: every row and column comes from tp. deck.gl is vendored under
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
  // countLine is the toolbar's count line: what the card drew and, for a
  // gate closure or a neighbourhood, what it is drawn from.
  function countLine(P, described) {
    const L = P.L;
    const nodes = L.view === "milestones" ? t("graph.countMilestones", { n: P.nodes.length }) : t("count.epics", { n: P.nodes.length });
    const line = t("graph.count", { nodes, edges: P.links.length, layers: L.layers, cols: L.columns });
    if (L.view === "scope") return line + " " + t("graph.inClosure", { ms: L.milestone }) + "; " + pure.closureDescribed(L.nodes, described, app.tr());
    if (L.view === "neighbourhood") return line + " " + t("graph.within", { depth: L.depth, epic: L.epic });
    return line;
  }

  // World geometry, one unit a screen pixel at zoom 0: an epic is a box of
  // w by h, one column col wide, one layer row tall.
  const GEO = {
    epic: { w: 150, h: 24, col: 172, row: 40, fs: 11 },
    milestone: { w: 200, h: 54, col: 240, row: 110, fs: 22 },
  };
  // Below this on-screen glyph height ids are not drawn: boxes are blocks.
  const MIN_TEXT_PX = 7;
  // The neighbourhood opens no further out than this, where an epic's
  // 11-unit id reads at its full 11 pixels.
  const NEAR_ZOOM = 0;
  const CURVE = 10;
  // One arrow key pans this many screen pixels; + and − zoom by ZOOM_STEP.
  const PAN_PX = 90;
  const ZOOM_STEP = 0.5;
  // The marks in legend order, each with the colour token it is drawn in;
  // a ready or held epic's box is filled stronger, so they stand out.
  const MARKS = [
    ["done", "--done"], ["in_progress", "--g-inprog"], ["specced", "--g-specced"], ["claimed", "--g-claimed"],
    ["ready", "--ready"], ["blocked", "--g-blocked"], ["held", "--held"], ["waiting", "--waiting"], ["moot", "--moot"],
  ];
  const STRONG = { ready: 0.5, held: 0.5, waiting: 0.55, moot: 0.55 };
  const EDGE_CLASSES = ["tree", "cross", "back", "critical", "any"];

  const G = {
    mode: "plan", milestone: null, depth: 2, showCross: false, q: "",
    hiddenMarks: new Set(), hiddenEdges: new Set(),
    data: null, key: null, dataKey: null, error: null, seq: 0, seen: null,
    deck: null, deckState: "idle", visible: false, built: null, builtFor: null, prepared: null,
    vs: null, far: null, fitted: null, matchIdx: -1, colors: null, loadMs: null, firstDrawMs: null,
    sets: null, setsKey: null, drawing: null, msgKey: tk("graph.notYet"),
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
  function token(name) {
    const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    if (/^#[0-9a-f]{6}$/i.test(v)) return [1, 3, 5].map((i) => parseInt(v.slice(i, i + 2), 16));
    const m = /rgba?\(([^)]+)\)/.exec(v);
    return m ? m[1].split(",").slice(0, 3).map((x) => Math.round(Number(x))) : [128, 128, 128];
  }
  function readColors() {
    const c = { mark: {} };
    MARKS.forEach(([k, t]) => { c.mark[k] = token(t); });
    Object.assign(c, {
      page: token("--sunk"), surface: token("--surface"), ink: token("--ink"), muted: token("--muted"), line: token("--line"),
      accent: token("--accent"), crit: token("--crit"), back: token("--g-back"), anc: token("--g-anc"), desc: token("--g-desc"),
      band: token("--color-paper-3"),
    });
    c.dark = (0.2126 * c.page[0] + 0.7152 * c.page[1] + 0.0722 * c.page[2]) < 128;
    G.colors = c;
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
  function query(g) {
    const p = new URLSearchParams({ view: G.mode });
    if (G.mode === "scope") {
      const m = gateMilestone(g);
      if (!m) return null;
      p.set("milestone", m);
    } else if (G.mode === "neighbourhood") {
      const sel = app.selected();
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
      centre: !!n.centre, deferrable: !!n.deferrable, layer: n.layer, status: null };
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
      links.push({ s, d, role: e.role || "cross", critical: !!e.critical, any: !!e.any_of });
    });
    const preds = nodes.map(() => []);
    const succs = nodes.map(() => []);
    links.forEach((l, k) => { succs[l.s].push(k); preds[l.d].push(k); });
    links.forEach((l) => { l.path = curve(nodes[l.s], nodes[l.d], l.role === "back"); });
    const bands = bandRegions(L, nodes, geo);
    const chars = new Set();
    nodes.forEach((n) => { for (const ch of n.label + n.title) chars.add(ch); });
    bands.forEach((b) => { for (const ch of b.name + " ·0123456789") chars.add(ch); });
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
    return { L, geo, nodes, links, preds, succs, bands, bounds, counts, index, described, charSet: [...chars].join(""), sel: -1, anc: null, desc: null, ver: 0 };
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
    if (P.sel >= 0) return inChain(P, n.i) ? pure.chainAlpha(stepsOf(P, n.i)) : 40;
    if (hits && !hits.has(n.i)) return 50;
    return staged(n) ? 255 : 50;
  }
  function nodeFill(P, hits) {
    const c = G.colors;
    return (n) => {
      const base = c.mark[n.mark] || c.muted;
      const fill = G.far ? base : mix(c.surface, base, STRONG[n.mark] || 0.32);
      return [...fill, nodeAlpha(P, hits, n)];
    };
  }
  function nodeLine(P, hits) {
    const c = G.colors;
    return (n) => {
      if (n.i === P.sel) return [...c.ink, 255];
      if (P.sel >= 0) {
        const a = inChain(P, n.i) ? pure.chainAlpha(stepsOf(P, n.i)) : 60;
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
  function nodeLineWidth(P, hits) {
    return (n) => {
      if (n.i === P.sel) return 3;
      if (P.sel >= 0 && (P.anc.has(n.i) || P.desc.has(n.i))) return stepsOf(P, n.i) <= 2 ? 2 : 1;
      if ((hits && hits.has(n.i)) || n.hold || n.centre) return 2;
      return G.far ? 0 : 1;
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
  function edgeColor(P) {
    const c = G.colors;
    return (l) => {
      const base = l.role === "back" ? c.back : l.critical ? c.crit : c.muted;
      if (P.sel >= 0) {
        if (!onChain(P, l)) return [...base, 14];
        const hue = l.critical ? c.crit : l.role === "back" ? c.back : P.anc.has(l.s) ? c.anc : c.desc;
        return [...hue, pure.chainAlpha(Math.max(stepsOf(P, l.s), stepsOf(P, l.d)))];
      }
      if (l.critical) return [...base, 235];
      if (l.role === "cross") return [...base, G.far ? 40 : 70];
      return [...base, G.far ? 130 : 190];
    };
  }
  function edgeWidth(P) {
    return (l) => {
      if (P.sel >= 0 && onChain(P, l)) return l.s === P.sel || l.d === P.sel ? 2.6 : 1.8;
      if (l.critical) return 2.4;
      return l.role === "cross" ? 0.8 : 1.3;
    };
  }
  // edgeSets is the four edge layers' data (tpPure.edgeLayers), kept while
  // the drawing and the toggles that choose them stay the same.
  function edgeSets(P) {
    const key = P.ver + "|" + G.showCross + "|" + [...G.hiddenEdges].join();
    if (G.setsKey !== key || G.sets === null || G.sets.of !== P) {
      G.sets = { ...pure.edgeLayers(P.links, { showCross: G.showCross, hidden: [...G.hiddenEdges] }), of: P };
      G.setsKey = key;
    }
    return G.sets;
  }

  function render() {
    const P = G.prepared;
    if (!G.deck || !P) return;
    const { PolygonLayer, PathLayer, TextLayer } = window.deck;
    const c = G.colors;
    const scale = Math.pow(2, G.vs.zoom);
    G.far = P.geo.fs * scale < MIN_TEXT_PX;
    const hits = matches(P);
    const v = [P.ver, G.q, G.far, G.showCross, [...G.hiddenMarks].join(), [...G.hiddenEdges].join(), c.dark, app.stage()];
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
        id: "bands", data: P.bands, getPolygon: (b) => b.poly, stroked: true, filled: true,
        getFillColor: (b) => [...mix(c.page, c.band, b.k % 2 ? 0.55 : 0.9), 255],
        getLineColor: [...c.line, 255], getLineWidth: 1, lineWidthUnits: "pixels", updateTriggers: { getFillColor: v },
      }),
      edgeLayer("cross", sets.cross),
      edgeLayer("any", sets.any),
      edgeLayer("tree", sets.tree),
      edgeLayer("crit", sets.crit),
      new PolygonLayer({
        id: "nodes", data: P.nodes, getPolygon: (n) => n.poly, stroked: true, filled: true, pickable: true, autoHighlight: true,
        highlightColor: [...c.accent, 70], getFillColor: nodeFill(P, hits), getLineColor: nodeLine(P, hits), getLineWidth: nodeLineWidth(P, hits),
        lineWidthUnits: "pixels", updateTriggers: { getFillColor: v, getLineColor: v, getLineWidth: v },
      }),
      new TextLayer({
        id: "ids", data: P.nodes, visible: !G.far, getPosition: (n) => [n.x, n.y + n.h / 2], getText: (n) => n.label,
        getSize: P.geo.fs, sizeUnits: "common", getTextAnchor: "middle", getAlignmentBaseline: "center",
        getColor: (n) => [...c.ink, nodeAlpha(P, hits, n) >= 170 ? 255 : 70],
        fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace", characterSet: P.charSet, fontSettings,
        updateTriggers: { getColor: v },
      }),
      new TextLayer({
        id: "band-names", data: P.bands, getPosition: (b) => [b.x0 + 2, b.y0], getText: (b) => b.name + " · " + b.count,
        getPixelOffset: [0, -3], getSize: 13, sizeUnits: "pixels", getTextAnchor: "start", getAlignmentBaseline: "bottom", getColor: [...c.ink, 230],
        fontFamily: "system-ui, sans-serif", fontWeight: 700, characterSet: P.charSet, fontSettings,
        outlineWidth: 3, outlineColor: [...c.page, 255], updateTriggers: { getColor: v, outlineColor: v },
      }),
      new TextLayer({
        id: "sel-title", data: sel && sel.title ? [sel] : [], getPosition: (n) => [n.x + n.w / 2 + 8, n.y + n.h / 2], getText: (n) => n.title,
        getSize: 14, sizeUnits: "pixels", getTextAnchor: "start", getAlignmentBaseline: "center", getColor: [...c.ink, 255],
        fontFamily: "system-ui, sans-serif", fontWeight: 600, characterSet: P.charSet, fontSettings,
        outlineWidth: 4, outlineColor: [...c.page, 255], updateTriggers: { getColor: v, outlineColor: v },
      }),
    ];
    // What this frame holds, recorded on the card once deck.gl drew it.
    G.drawing = { mode: P.L.view, nodes: P.nodes.length, edges: sets.drawn };
    G.deck.setProps({ layers, viewState: G.vs });
  }
  // recordDrawn writes what the last frame drew onto the card, so the audit
  // reads per mode that the canvas holds the layout's nodes and edges, and
  // the first frame's time from the card asking for the renderer: loading
  // it, fetching the layout, preparing it and drawing.
  function recordDrawn() {
    const d = G.drawing;
    if (!d) return;
    card.dataset.drawnMode = d.mode;
    card.dataset.drawnNodes = String(d.nodes);
    card.dataset.drawnEdges = String(d.edges);
    if (G.firstDrawMs === null && d.nodes) {
      G.firstDrawMs = Math.round(performance.now() - G.loadFrom);
      card.dataset.firstDrawMs = String(G.firstDrawMs);
      card.dataset.rendererMs = String(G.loadMs);
    }
  }

  // ---------- camera ----------
  function fitView() {
    const P = G.prepared;
    const box = $("gc-canvas");
    const w = Math.max(1, box.clientWidth);
    const h = Math.max(1, box.clientHeight);
    const b = P.bounds;
    // A margin of 24 pixels a side and the band names above the top band;
    // a small graph is shown at its size, never magnified.
    const zoom = Math.min(0, Math.log2(Math.min((w - 48) / (b.x1 - b.x0), (h - 64) / (b.y1 - b.y0))));
    const shift = 10 / Math.pow(2, zoom);
    return { target: [(b.x0 + b.x1) / 2, (b.y0 + b.y1) / 2 - shift, 0], zoom, minZoom: -8, maxZoom: 3 };
  }
  function goTo(vs) {
    G.vs = vs;
    render();
  }
  function flyTo(n) {
    goTo({ ...G.vs, target: [n.x, n.y + n.h / 2, 0], zoom: Math.max(G.vs.zoom, 0) });
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
  // Ctrl or ⌘ with the wheel, a pinch, or the plain wheel in full screen
  // zooms about the pointer (tpPure.wheelZooms); otherwise the wheel
  // scrolls the page.
  $("gc-canvas").addEventListener("wheel", (ev) => {
    if (!G.vs || !G.prepared || !pure.wheelZooms(ev, card.classList.contains("fs"))) return;
    ev.preventDefault();
    const box = $("gc-canvas").getBoundingClientRect();
    const px = ev.clientX - box.left - box.width / 2;
    const py = ev.clientY - box.top - box.height / 2;
    const s0 = Math.pow(2, G.vs.zoom);
    const z = Math.min(3, Math.max(-8, G.vs.zoom - ev.deltaY * (ev.ctrlKey ? 0.01 : 0.002)));
    const s1 = Math.pow(2, z);
    const wx = G.vs.target[0] + px / s0;
    const wy = G.vs.target[1] + py / s0;
    goTo({ ...G.vs, zoom: z, target: [wx - px / s1, wy - py / s1, 0] });
  }, { passive: false });

  function tooltip({ object, layer }) {
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
        G.vs = viewState;
        render();
      },
      onAfterRender: recordDrawn,
      onClick: (info) => {
        if (info.object && info.layer && info.layer.id === "nodes") pick(info.object);
        else if (app.selected()) app.selectEpic(null);
      },
      getCursor: ({ isHovering, isDragging }) => (isDragging ? "grabbing" : isHovering ? "pointer" : "grab"),
      getTooltip: tooltip,
    });
  }

  // setPlace changes the card's place and keeps it in the address.
  function setPlace(change) {
    Object.assign(G, change);
    app.syncURL();
    draw();
  }

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
  app.onSelect = (id, centre) => {
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
    $("gc-cross").setAttribute("aria-pressed", String(G.showCross));
    $("gc-theme").textContent = G.colors && G.colors.dark ? t("graph.light") : t("graph.dark");
    if ($("gc-q").value !== G.q && document.activeElement !== $("gc-q")) $("gc-q").value = G.q;
  }
  function drawLegend() {
    const P = G.prepared;
    if (!P) { $("gc-legend").innerHTML = ""; return; }
    const c = G.colors;
    const rgb = (a) => "rgb(" + a.join(",") + ")";
    const marks = MARKS.filter(([k]) => P.counts.marks[k]).map(([k]) =>
      '<button type="button" data-lmark="' + k + '" aria-pressed="' + !G.hiddenMarks.has(k) + '"><i style="background:' + rgb(c.mark[k]) + '"></i>' + escape(app.markWord(k)) + " <b>" + escape(app.tr().num(P.counts.marks[k])) + "</b></button>").join("");
    const ecol = { tree: c.muted, cross: c.muted, back: c.back, critical: c.crit, any: c.accent };
    const edges = EDGE_CLASSES.filter((k) => P.counts.edges[k]).map((k) =>
      '<button type="button" data-ledge="' + k + '" aria-pressed="' + (k === "cross" ? G.showCross : !G.hiddenEdges.has(k)) + '"><i class="ln" style="background:' + rgb(ecol[k]) + (k === "cross" ? ";opacity:.45" : "") + '"></i>' + escape(t(EDGE_TEXT[k])) + " <b>" + escape(app.tr().num(P.counts.edges[k])) + "</b></button>").join("");
    const sel = P.sel >= 0 ? '<span><i class="ln" style="background:' + rgb(c.anc) + '"></i>' + escape(t("graph.waitsOn")) + " <b>" + P.anc.size + '</b></span><span><i class="ln" style="background:' + rgb(c.desc) + '"></i>' + escape(t("graph.waitedOn")) + " <b>" + P.desc.size + "</b></span>" : "";
    $("gc-legend").innerHTML = marks + '<span class="lhead"></span>' + edges + sel +
      '<span class="help">' + escape(t("graph.help")) + (G.loadMs !== null ? " " + escape(t("graph.loadedIn", { ms: G.loadMs })) : "") + "</span>";
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
        G.showCross = !G.showCross;
        app.syncURL();
      } else if (G.hiddenEdges.has(k)) G.hiddenEdges.delete(k); else G.hiddenEdges.add(k);
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
  $("gc-cross").addEventListener("click", () => {
    G.showCross = !G.showCross;
    app.syncURL();
    drawToolbar(app.data("epic-export") || {});
    drawLegend();
    render();
  });
  $("gc-fit").addEventListener("click", () => { if (G.prepared) goTo(fitView()); });
  $("gc-theme").addEventListener("click", () => {
    document.documentElement.dataset.theme = G.colors && G.colors.dark ? "light" : "dark";
    retheme();
  });
  function retheme() {
    readColors();
    drawToolbar(app.data("epic-export") || {});
    drawLegend();
    render();
  }
  window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", retheme);

  // ---------- the search and the keys ----------
  function showMatches() {
    const P = G.prepared;
    const hits = P && matches(P);
    $("gc-n").textContent = hits ? t("graph.matches", { n: hits.size }) : countText();
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
    if (ev.key !== "Enter" || !P) return;
    const hits = [...(matches(P) || [])];
    if (!hits.length) return;
    ev.preventDefault();
    G.matchIdx = pure.stepMatch(hits.length, G.matchIdx, ev.shiftKey);
    const n = P.nodes[hits[G.matchIdx]];
    flyTo(n);
    if (P.L.view !== "milestones" && n.id !== app.selected()) app.selectEpic(n.id, false, true);
  });
  // "/" reaches the graph's search while the card is on screen, ahead of
  // the page's own "/" for the Specs filter, so it listens in the capture
  // phase.
  const typing = (t) => t && (t.tagName === "INPUT" || t.tagName === "SELECT" || t.tagName === "TEXTAREA" || t.isContentEditable);
  const cardShown = () => G.visible && !card.hidden && !!G.prepared && !$("view-epics").hidden;
  document.addEventListener("keydown", (ev) => {
    if (ev.key !== "/" || ev.metaKey || ev.ctrlKey || ev.altKey || typing(ev.target) || !cardShown()) return;
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
    if (!cardShown() || typing(ev.target) || ev.metaKey || ev.ctrlKey || ev.altKey) return;
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
    } else if (ev.key === "f" || ev.key === "F") goTo(fitView());
    else if (ev.key === "k" || ev.key === "K") $("gc-cross").click();
    else if (ev.key === "Escape" && app.selected() && $("drawer").hidden) app.selectEpic(null);
  });
  function countText() {
    return G.prepared ? countLine(G.prepared, G.prepared.described) : "";
  }

  // ---------- the panel ----------
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
      G.prepared = null;
      drawLegend();
      $("gc-n").textContent = "";
      G.drawing = { mode: G.mode, nodes: 0, edges: 0 };
      recordDrawn();
      if (G.deck) G.deck.setProps({ layers: [] });
      sayKey(tk("graph.pickEpic"));
      return;
    }
    if (key !== G.key || G.seen !== g) {
      G.seen = g;
      fetchLayout(key);
    }
    if (G.error) { say(G.error.error); return; }
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
        G.vs = fitView();
        G.fitted = key;
        if (G.data.view === "neighbourhood") {
          const c = G.prepared.nodes.find((n) => n.centre);
          if (c) G.vs = { ...G.vs, target: [c.x, c.y + c.h / 2, 0], zoom: Math.max(G.vs.zoom, NEAR_ZOOM) };
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
