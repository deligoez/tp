// The graph card (prototype): the plan, the milestones, a gate's closure
// and an epic's neighbourhood, laid out by tp at /api/layout as a forest
// packed into columns (layers top to bottom, tree edges solid, cross edges
// hidden until asked for, back edges violet) and drawn with deck.gl. The
// page computes no place: every row and column comes from tp. deck.gl is
// vendored under vendor/ and loaded only when the card first scrolls into
// view; a browser without WebGL2 gets a plain message instead.
(function () {
  "use strict";
  const app = window.tpApp;
  if (!app) return;
  const DECK_SRC = "vendor/deckgl-9.3.12.min.js";
  const $ = (id) => document.getElementById(id);
  const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);

  // World geometry, one unit a screen pixel at zoom 0: an epic is a box of
  // BOX.w by BOX.h, one column COL wide, one layer ROW tall.
  const GEO = {
    epic: { w: 150, h: 24, col: 172, row: 40, fs: 11 },
    milestone: { w: 200, h: 54, col: 240, row: 110, fs: 22 },
  };
  // Below this on-screen glyph height ids are not drawn: boxes are blocks.
  const MIN_TEXT_PX = 7;
  const CURVE = 10;
  // The marks in legend order, each with the colour token it is drawn in.
  const MARKS = [
    ["done", "done", "--done"], ["in_progress", "in progress", "--g-inprog"], ["specced", "specced", "--g-specced"],
    ["claimed", "claimed", "--g-claimed"], ["ready", "can start", "--ready"], ["blocked", "blocked", "--blocked"],
    ["held", "held by a condition", "--held"], ["waiting", "milestone closed", "--waiting"], ["moot", "moot", "--moot"],
  ];
  const EDGE_CLASSES = [["tree", "tree edges"], ["cross", "cross edges"], ["back", "back edges"], ["critical", "critical path"], ["any", "one of an any-of group"]];

  const G = {
    mode: "plan", milestone: null, depth: 2, pack: "rows", showCross: false, q: "",
    hiddenMarks: new Set(), hiddenEdges: new Set(),
    data: null, key: null, dataKey: null, error: null, seq: 0, seen: null,
    deck: null, deckState: "idle", visible: false, built: null, prepared: null,
    vs: null, far: null, fitted: null, matchIdx: -1, colors: null, loadMs: null,
  };
  window.tpGraph = G;

  // ---------- colours, read from the page's tokens ----------
  function token(name) {
    const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    if (/^#[0-9a-f]{6}$/i.test(v)) return [1, 3, 5].map((i) => parseInt(v.slice(i, i + 2), 16));
    const m = /rgba?\(([^)]+)\)/.exec(v);
    return m ? m[1].split(",").slice(0, 3).map((x) => Math.round(Number(x))) : [128, 128, 128];
  }
  function readColors() {
    const c = { mark: {} };
    MARKS.forEach(([k, , t]) => { c.mark[k] = token(t); });
    Object.assign(c, {
      page: token("--sunk"), surface: token("--surface"), ink: token("--ink"), muted: token("--muted"), line: token("--line"),
      accent: token("--accent"), crit: token("--crit"), back: token("--g-back"), anc: token("--g-anc"), desc: token("--g-desc"),
      band: token("--color-paper-3"),
    });
    const dark = (0.2126 * c.page[0] + 0.7152 * c.page[1] + 0.0722 * c.page[2]) < 128;
    c.dark = dark;
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
  function say(text) {
    const m = $("gc-msg");
    m.textContent = text;
    m.hidden = !text;
  }
  function loadDeck() {
    if (G.deckState !== "idle") return;
    if (!hasWebGL2() || new URLSearchParams(location.search).has("nogl")) {
      G.deckState = "nogl";
      say("This graph is drawn with WebGL2, which this browser does not offer. Everything else on the page works without it; the drawer and the lists above still show each epic's dependencies.");
      return;
    }
    G.deckState = "loading";
    say("Loading the graph renderer.");
    const t0 = performance.now();
    const s = document.createElement("script");
    s.src = DECK_SRC;
    s.onload = () => {
      G.loadMs = Math.round(performance.now() - t0);
      G.deckState = "ready";
      draw();
    };
    s.onerror = () => {
      G.deckState = "failed";
      say("The graph renderer did not load. Reload the page; if it stays, check that tp dashboard is still running.");
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
    if (G.pack !== "rows") p.set("pack", G.pack);
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
      body = { error: "the layout could not be fetched: " + err.message, hint: "check that tp dashboard is still running" };
    }
    if (seq !== G.seq) return;
    if (body && typeof body.error === "string") Object.assign(G, { data: null, dataKey: null, error: body });
    else Object.assign(G, { data: body, dataKey: key, error: null });
    draw();
  }

  // ---------- preparing the drawing from a layout ----------
  function prepare(L, g) {
    const geo = L.view === "milestones" ? GEO.milestone : GEO.epic;
    const epics = {};
    (g.epics || []).forEach((e) => { epics[e.id] = e; });
    const reports = {};
    (g.milestones || []).forEach((m) => { reports[m.milestone] = m; });
    const titles = {};
    ((g.graph && g.graph.epics) || []).forEach((e) => { titles[e.id] = e; });
    const nodes = L.nodes.map((n, i) => {
      const x = n.col * geo.col;
      const y = n.layer * geo.row;
      let mark = null;
      let hold = null;
      let label = n.id;
      let ms = n.milestone || null;
      let title = "";
      let cond = false;
      if (L.view === "milestones") {
        mark = n.open ? (n.epics && n.done === n.epics ? "done" : "ready") : "waiting";
        label = n.id + "  " + n.done + "/" + n.epics;
        title = (n.open ? "open" : "closed") + ", " + n.done + " of " + n.epics + " work epics done, " + n.inbound + " epic edges in";
      } else {
        const e = epics[n.id] || { id: n.id, status: "planned" };
        ms = e.milestone || ms;
        const m = app.markOf(e, reports[e.milestone]);
        mark = m.mark;
        hold = m.hold;
        title = (titles[n.id] && titles[n.id].title) || e.title || "";
        cond = !!e.condition;
      }
      const w = geo.w;
      const h = geo.h;
      const poly = cond
        ? [[x - w / 2 + 8, y], [x + w / 2 - 8, y], [x + w / 2, y + h / 2], [x + w / 2 - 8, y + h], [x - w / 2 + 8, y + h], [x - w / 2, y + h / 2]]
        : [[x - w / 2, y], [x + w / 2, y], [x + w / 2, y + h], [x - w / 2, y + h]];
      return { i, id: n.id, x, y, w, h, poly, mark, hold, label, title, ms, cond, centre: !!n.centre, deferrable: !!n.deferrable, layer: n.layer };
    });
    const index = {};
    nodes.forEach((n) => { index[n.id] = n.i; });
    const links = [];
    L.edges.forEach((e) => {
      const s = index[e.from];
      const d = index[e.to];
      if (s === undefined || d === undefined) return;
      links.push({ s, d, role: e.role || "cross", critical: !!e.critical, any: !!e.any_of, group: e.any_of || null });
    });
    const preds = nodes.map(() => []);
    const succs = nodes.map(() => []);
    links.forEach((l, k) => { succs[l.s].push(k); preds[l.d].push(k); });
    links.forEach((l) => { l.path = curve(nodes[l.s], nodes[l.d], l.role === "back"); });
    // Band regions: each band's rows, over the columns its nodes take.
    const bands = (L.bands || []).map((b, k) => {
      const names = [b.milestone].concat(b.with || []);
      const inBand = nodes.filter((n) => names.includes(n.ms));
      if (!inBand.length) return null;
      const pad = 14;
      const x0 = Math.min(...inBand.map((n) => n.x - n.w / 2)) - pad;
      const x1 = Math.max(...inBand.map((n) => n.x + n.w / 2)) + pad;
      const y0 = b.from_layer * geo.row - pad - 16;
      const y1 = b.to_layer * geo.row + geo.h + pad;
      return { k, name: names.join(" + "), poly: [[x0, y0], [x1, y0], [x1, y1], [x0, y1]], x0, y0, count: inBand.length };
    }).filter(Boolean);
    const chars = new Set();
    nodes.forEach((n) => { for (const ch of n.label) chars.add(ch); for (const ch of n.title) chars.add(ch); });
    bands.forEach((b) => { for (const ch of b.name + " ·0123456789") chars.add(ch); });
    const xs = nodes.map((n) => n.x);
    const ys = nodes.map((n) => n.y);
    const bx = bands.map((b) => b.x0);
    const by = bands.map((b) => b.y0);
    const bounds = nodes.length ? {
      x0: Math.min(...xs, ...bx) - geo.w / 2 - 20, x1: Math.max(...xs) + geo.w / 2 + 40,
      y0: Math.min(...ys, ...by) - 30, y1: Math.max(...ys) + geo.h + 30,
    } : { x0: 0, x1: 1, y0: 0, y1: 1 };
    const counts = { marks: {}, edges: {} };
    nodes.forEach((n) => { counts.marks[n.mark] = (counts.marks[n.mark] || 0) + 1; });
    links.forEach((l) => {
      counts.edges[l.role] = (counts.edges[l.role] || 0) + 1;
      if (l.critical) counts.edges.critical = (counts.edges.critical || 0) + 1;
      if (l.any) counts.edges.any = (counts.edges.any || 0) + 1;
    });
    return { L, geo, nodes, links, preds, succs, bands, bounds, counts, index, charSet: [...chars].join(""), sel: -1, anc: null, desc: null, ver: 0 };
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
  function walk(P, start, edges, pick) {
    const seen = new Set();
    const stack = [start];
    while (stack.length) {
      const v = stack.pop();
      for (const k of edges[v]) {
        const w = pick(P.links[k]);
        if (!seen.has(w)) { seen.add(w); stack.push(w); }
      }
    }
    return seen;
  }
  function applySelection(P) {
    const id = app.selected();
    const i = id && P.index[id] !== undefined ? P.index[id] : -1;
    if (i === P.sel) return;
    P.sel = i;
    P.anc = i < 0 ? null : walk(P, i, P.preds, (l) => l.s);
    P.desc = i < 0 ? null : walk(P, i, P.succs, (l) => l.d);
    P.ver++;
  }
  const inChain = (P, i) => P.sel < 0 || i === P.sel || P.anc.has(i) || P.desc.has(i);
  function matches(P) {
    const q = app.fold(G.q.trim());
    if (!q) return null;
    const hit = new Set();
    P.nodes.forEach((n) => { if (app.fold(n.id).includes(q) || app.fold(n.title).includes(q)) hit.add(n.i); });
    return hit;
  }

  // ---------- drawing ----------
  function nodeFill(P, hits) {
    const c = G.colors;
    return (n) => {
      const base = c.mark[n.mark] || c.muted;
      let a = 255;
      if (G.hiddenMarks.has(n.mark)) a = 28;
      else if (P.sel >= 0 && !inChain(P, n.i)) a = 40;
      else if (hits && !hits.has(n.i)) a = 50;
      const fill = G.far ? base : mix(c.surface, base, n.mark === "waiting" || n.mark === "moot" ? 0.55 : 0.32);
      return [...fill, a];
    };
  }
  function nodeLine(P, hits) {
    const c = G.colors;
    return (n) => {
      if (n.i === P.sel) return [...c.ink, 255];
      if (P.sel >= 0) {
        if (P.anc.has(n.i)) return [...c.anc, 255];
        if (P.desc.has(n.i)) return [...c.desc, 255];
        return [...c.line, 60];
      }
      if (hits && hits.has(n.i)) return [...c.accent, 255];
      if (n.hold) return [...c.mark[n.hold], 255];
      if (n.centre) return [...c.accent, 255];
      return [...(c.mark[n.mark] || c.muted), 230];
    };
  }
  function nodeLineWidth(P, hits) {
    return (n) => {
      if (n.i === P.sel) return 3;
      if (P.sel >= 0 && (P.anc.has(n.i) || P.desc.has(n.i))) return 2;
      if ((hits && hits.has(n.i)) || n.hold || n.centre) return 2;
      return G.far ? 0 : 1;
    };
  }
  function edgeVisible(l) {
    if (G.hiddenEdges.has(l.role)) return false;
    if (l.critical && G.hiddenEdges.has("critical")) return false;
    if (l.any && G.hiddenEdges.has("any")) return false;
    return true;
  }
  function onChain(P, l) {
    if (P.sel < 0) return false;
    const { s, d } = l;
    return s === P.sel || d === P.sel || (P.anc.has(s) && (P.anc.has(d) || d === P.sel)) || (P.desc.has(d) && (P.desc.has(s) || s === P.sel));
  }
  function edgeColor(P) {
    const c = G.colors;
    return (l) => {
      if (!edgeVisible(l)) return [0, 0, 0, 0];
      const base = l.role === "back" ? c.back : l.critical ? c.crit : c.muted;
      if (P.sel >= 0) {
        if (onChain(P, l)) {
          const hue = l.critical ? c.crit : l.role === "back" ? c.back : P.anc.has(l.s) || l.d === P.sel ? c.anc : c.desc;
          return [...hue, 255];
        }
        return [...base, l.role === "cross" && !G.showCross ? 0 : 14];
      }
      if (l.role === "cross") return [...base, G.showCross ? (G.far ? 40 : 70) : 0];
      if (l.critical) return [...base, 230];
      return [...base, G.far ? 130 : 190];
    };
  }
  function edgeWidth(P) {
    return (l) => {
      if (P.sel >= 0 && onChain(P, l)) return l.s === P.sel || l.d === P.sel ? 2.6 : 1.8;
      if (l.critical) return 2.2;
      return l.role === "cross" ? 0.8 : 1.3;
    };
  }

  function render() {
    const P = G.prepared;
    if (!G.deck || !P) return;
    const { OrthographicView, PolygonLayer, PathLayer, TextLayer } = window.deck;
    void OrthographicView;
    const c = G.colors;
    const scale = Math.pow(2, G.vs.zoom);
    G.far = P.geo.fs * scale < MIN_TEXT_PX;
    const hits = matches(P);
    const v = [P.ver, G.q, G.far, G.showCross, [...G.hiddenMarks].join(), [...G.hiddenEdges].join(), c.dark];
    const fontSettings = { sdf: true, fontSize: 64, buffer: 6, radius: 12, cutoff: 0.22 };
    const dash = window.deck.PathStyleExtension ? [new window.deck.PathStyleExtension({ dash: true })] : [];
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
      edgeLayer("cross", P.links.filter((l) => l.role === "cross" && !l.critical && !l.any)),
      edgeLayer("any", P.links.filter((l) => l.any && !l.critical)),
      edgeLayer("tree", P.links.filter((l) => l.role !== "cross" && !l.critical && !l.any)),
      edgeLayer("crit", P.links.filter((l) => l.critical)),
      new PolygonLayer({
        id: "nodes", data: P.nodes, getPolygon: (n) => n.poly, stroked: true, filled: true, pickable: true, autoHighlight: true,
        highlightColor: [...c.accent, 70], getFillColor: nodeFill(P, hits), getLineColor: nodeLine(P, hits), getLineWidth: nodeLineWidth(P, hits),
        lineWidthUnits: "pixels", updateTriggers: { getFillColor: v, getLineColor: v, getLineWidth: v },
      }),
      new TextLayer({
        id: "ids", data: P.nodes, visible: !G.far, getPosition: (n) => [n.x, n.y + n.h / 2], getText: (n) => n.label,
        getSize: P.geo.fs, sizeUnits: "common", getTextAnchor: "middle", getAlignmentBaseline: "center",
        getColor: (n) => [...c.ink, (P.sel >= 0 && !inChain(P, n.i)) || (hits && !hits.has(n.i)) || G.hiddenMarks.has(n.mark) ? 70 : 255],
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
    G.deck.setProps({ layers, viewState: G.vs });
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
  // Ctrl or ⌘ with the wheel, a pinch, or the plain wheel in full screen
  // zooms about the pointer; otherwise the wheel scrolls the page.
  $("gc-canvas").addEventListener("wheel", (ev) => {
    if (!G.vs || !G.prepared) return;
    const fs = card.classList.contains("fs");
    if (!fs && !ev.ctrlKey && !ev.metaKey) return;
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

  function ensureDeck() {
    if (G.deck) return;
    say("");
    G.deck = new window.deck.Deck({
      parent: $("gc-canvas"),
      views: new window.deck.OrthographicView({ id: "ortho", flipY: true }),
      controller: { scrollZoom: false, dragRotate: false, doubleClickZoom: true, touchZoom: true, inertia: 200 },
      viewState: G.vs,
      onViewStateChange: ({ viewState }) => {
        G.vs = viewState;
        render();
      },
      onClick: (info) => {
        if (info.object && info.layer && info.layer.id === "nodes") pick(info.object);
        else if (app.selected()) app.selectEpic(null);
      },
      getCursor: ({ isHovering, isDragging }) => (isDragging ? "grabbing" : isHovering ? "pointer" : "grab"),
      getTooltip: ({ object, layer }) => {
        if (!object || !layer || layer.id !== "nodes") return null;
        const n = object;
        const mk = { mark: n.mark, hold: n.hold };
        const lines = G.prepared.L.view === "milestones"
          ? '<b>' + esc(n.id) + "</b><br>" + esc(n.title)
          : '<span style="opacity:.7;font-family:ui-monospace,monospace;font-size:11px">' + esc(n.id) + (n.ms ? " · " + esc(n.ms) : "") + (n.cond ? " · condition" : "") + "</span><br>" +
            esc(n.title) + "<br>" + esc(app.markText(mk)) + (n.deferrable ? ", deferrable" : "");
        const c = G.colors;
        return { html: lines, style: { background: "rgb(" + c.ink + ")", color: "rgb(" + c.surface + ")", borderRadius: "6px", maxWidth: "320px", fontSize: "13px", padding: "6px 10px", lineHeight: "1.4" } };
      },
    });
  }

  // pick answers a click on a node: a milestone opens its gate's closure, an
  // epic is selected (or cleared when it is the selected one), and in the
  // neighbourhood the view re-centres on it.
  function pick(n) {
    if (G.prepared.L.view === "milestones") {
      G.mode = "scope";
      G.milestone = n.id;
      draw();
      return;
    }
    app.selectEpic(n.id === app.selected() ? null : n.id);
  }
  app.onSelect = () => {
    if (G.mode === "neighbourhood") draw();
    else if (G.prepared) {
      applySelection(G.prepared);
      drawLegend();
      render();
    }
  };

  // ---------- the toolbar and legend ----------
  function drawToolbar(g) {
    card.querySelectorAll("[data-gmode]").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.gmode === G.mode)));
    $("gc-ms-pick").hidden = G.mode !== "scope";
    $("gc-depth-pick").hidden = G.mode !== "neighbourhood";
    const order = [].concat(...((g.graph && g.graph.milestones) || []));
    if ($("gc-ms").dataset.key !== order.join("\u0000")) {
      $("gc-ms").dataset.key = order.join("\u0000");
      $("gc-ms").innerHTML = order.map((m) => "<option>" + esc(m) + "</option>").join("");
    }
    $("gc-ms").value = gateMilestone(g) || "";
    $("gc-depth").value = String(G.depth);
    $("gc-cross").setAttribute("aria-pressed", String(G.showCross));
    $("gc-pack").textContent = "Packing: " + G.pack;
    $("gc-theme").textContent = G.colors && G.colors.dark ? "Light" : "Dark";
  }
  function drawLegend() {
    const P = G.prepared;
    if (!P) { $("gc-legend").innerHTML = ""; return; }
    const c = G.colors;
    const rgb = (a) => "rgb(" + a.join(",") + ")";
    const marks = MARKS.filter(([k]) => P.counts.marks[k]).map(([k, label]) =>
      '<button type="button" data-lmark="' + k + '" aria-pressed="' + !G.hiddenMarks.has(k) + '"><i style="background:' + rgb(c.mark[k]) + '"></i>' + esc(label) + " <b>" + P.counts.marks[k] + "</b></button>").join("");
    const ecol = { tree: c.muted, cross: c.muted, back: c.back, critical: c.crit, any: c.accent };
    const edges = EDGE_CLASSES.filter(([k]) => P.counts.edges[k]).map(([k, label]) =>
      '<button type="button" data-ledge="' + k + '" aria-pressed="' + (k === "cross" ? G.showCross : !G.hiddenEdges.has(k)) + '"><i class="ln" style="background:' + rgb(ecol[k]) + (k === "cross" ? ";opacity:.45" : "") + '"></i>' + esc(label) + " <b>" + P.counts.edges[k] + "</b></button>").join("");
    const sel = P.sel >= 0 ? '<span><i class="ln" style="background:' + rgb(c.anc) + '"></i>what it waits on <b>' + P.anc.size + '</b></span><span><i class="ln" style="background:' + rgb(c.desc) + '"></i>what waits on it <b>' + P.desc.size + "</b></span>" : "";
    $("gc-legend").innerHTML = marks + '<span class="lhead"></span>' + edges + sel +
      '<span class="help">Click an epic for its chain, again or Esc to clear. / search, Enter the next match · F fit · K cross edges. Drag pans; Ctrl or ⌘ with the wheel, or a pinch, zooms; in full screen the wheel alone zooms.' +
      (G.loadMs !== null ? " Renderer loaded in " + G.loadMs + " ms." : "") + "</span>";
  }
  $("gc-legend").addEventListener("click", (ev) => {
    const m = ev.target.closest("[data-lmark]");
    const e = ev.target.closest("[data-ledge]");
    if (m) {
      const k = m.dataset.lmark;
      if (G.hiddenMarks.has(k)) G.hiddenMarks.delete(k); else G.hiddenMarks.add(k);
    } else if (e) {
      const k = e.dataset.ledge;
      if (k === "cross") G.showCross = !G.showCross;
      else if (G.hiddenEdges.has(k)) G.hiddenEdges.delete(k); else G.hiddenEdges.add(k);
    } else return;
    drawLegend();
    drawToolbar(app.data("epic-export") || {});
    render();
  });
  card.addEventListener("click", (ev) => {
    const b = ev.target.closest("[data-gmode]");
    if (!b) return;
    G.mode = b.dataset.gmode;
    draw();
  });
  $("gc-ms").addEventListener("change", (ev) => { G.mode = "scope"; G.milestone = ev.target.value; draw(); });
  $("gc-depth").addEventListener("change", (ev) => { G.depth = Number(ev.target.value); draw(); });
  $("gc-cross").addEventListener("click", () => { G.showCross = !G.showCross; drawToolbar(app.data("epic-export") || {}); drawLegend(); render(); });
  $("gc-pack").addEventListener("click", () => { G.pack = G.pack === "rows" ? "tree" : "rows"; G.fitted = null; draw(); });
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
  $("gc-q").addEventListener("input", (ev) => {
    G.q = ev.target.value;
    G.matchIdx = -1;
    const P = G.prepared;
    const hits = P && matches(P);
    $("gc-n").textContent = hits ? hits.size + (hits.size === 1 ? " match" : " matches") : countText();
    render();
  });
  $("gc-q").addEventListener("keydown", (ev) => {
    const P = G.prepared;
    if (ev.key !== "Enter" || !P) return;
    const hits = [...(matches(P) || [])];
    if (!hits.length) return;
    ev.preventDefault();
    G.matchIdx = (G.matchIdx + 1) % hits.length;
    const n = P.nodes[hits[G.matchIdx]];
    flyTo(n);
    if (P.L.view !== "milestones" && n.id !== app.selected()) app.selectEpic(n.id);
  });
  document.addEventListener("keydown", (ev) => {
    if (!G.visible || !G.prepared || card.hidden) return;
    const t = ev.target;
    if (t && (t.tagName === "INPUT" || t.tagName === "SELECT" || t.tagName === "TEXTAREA")) {
      if (ev.key === "Escape" && t.id === "gc-q") { t.value = ""; G.q = ""; render(); }
      return;
    }
    if (ev.key === "/") { ev.preventDefault(); $("gc-q").focus(); }
    else if (ev.key === "f" || ev.key === "F") goTo(fitView());
    else if (ev.key === "k" || ev.key === "K") $("gc-cross").click();
    else if (ev.key === "Escape" && app.selected() && $("drawer").hidden) app.selectEpic(null);
  });
  function countText() {
    const P = G.prepared;
    if (!P) return "";
    const L = P.L;
    const what = L.view === "milestones" ? "milestones" : "epics";
    const extra = L.view === "scope" ? " in " + L.milestone + "'s closure" : L.view === "neighbourhood" ? " within " + L.depth + " steps of " + L.epic : "";
    return P.nodes.length + " " + what + ", " + P.links.length + " edges, " + L.layers + " layers × " + L.columns + " columns" + extra;
  }

  // ---------- the panel ----------
  function draw() {
    const g = app.data("epic-export");
    card.hidden = !g;
    if (!g) return;
    if (!G.colors) readColors();
    drawToolbar(g);
    if (!G.visible) return;
    if (G.deckState !== "ready") return;
    const key = query(g);
    if (!key) {
      G.prepared = null;
      drawLegend();
      $("gc-n").textContent = "";
      if (G.deck) G.deck.setProps({ layers: [] });
      say("Select an epic, on the graph or in a list above, to see its neighbourhood.");
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
          if (c) G.vs = { ...G.vs, target: [c.x, c.y + c.h / 2, 0], zoom: Math.max(G.vs.zoom, -1) };
        }
      }
    } else applySelection(G.prepared);
    $("gc-n").textContent = G.q ? $("gc-n").textContent : countText();
    drawLegend();
    render();
  }
  app.panel(["epic-export"], draw);
  // api lets a scripted browser drive the camera for screenshots and checks.
  G.api = { goTo, fitView, draw, render, flyTo };
})();
