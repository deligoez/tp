// The Tasks view's Graph: a spec's tasks drawn with deck.gl like the epic
// map (graph.js), laid out in the Steps panel's steps by tpPure.taskGraphLayout.
// A task's state is on its node: done a muted fill and a check, ready the
// brand's outline (the one saturated mark at rest), in progress the
// in-progress tone, waiting neutral. Edges are thin low-alpha curves that
// leave a node's bottom and enter the next one's top; hovering or selecting a
// task draws its whole chain at full strength and dims the rest. Labels
// show once they draw at DESIGN.md's 14-pixel floor, and the tooltip names a
// task below that. The canvas has no focusable node: the list beside it,
// visually hidden until it takes focus, is the keyboard's and the screen
// reader's way in. deck.gl is graph.js's, loaded once (tpApp.deck).
(function () {
  "use strict";
  const app = window.tpApp;
  const pure = window.tpPure;
  if (!app || !pure || !app.deck) return;
  const t = (key, params) => app.tr()(key, params);
  const GEO = pure.TASK_GEO;
  // MARGIN is the room round the drawing at its fit, in screen pixels.
  const MARGIN = 24;
  // TALL_PX is the tallest canvas a drawing gets so it reads at zoom 0.
  const TALL_PX = 1400;
  // Labels are TITLE_PX and ID_PX world units, a pixel each at zoom 0, and
  // show only while they draw at MIN_PX or more.
  const TITLE_PX = 15;
  const ID_PX = 14;
  const MIN_PX = 14;
  const CURVE_STEPS = 12;
  const RADIUS = 6;
  const MARK = { done: "✓", wip: "●", waits_on_epic: "‖" };
  const TG = { host: null, box: null, msg: null, deck: null, state: "idle", d: null, sel: null, hover: null, P: null, vs: null, c: null, key: null, ro: null, themeVer: 0 };
  window.tpTaskGraph = TG;
  // A theme change (tp-theme, as graph.js hears it) reads the colours again
  // and bumps the version every layer's update triggers hold.
  document.addEventListener("tp-theme", () => {
    TG.themeVer++;
    if (TG.state !== "ready" || !TG.P) return;
    readColors();
    render();
  });

  // ---------- the host: built once, moved into each fresh slot ----------
  // The Tasks view redraws its HTML on every change; the host keeps the
  // canvas, so deck.gl's context survives each redraw.
  function host() {
    if (TG.host) return TG.host;
    const h = document.createElement("div");
    h.className = "tgraph";
    h.innerHTML = '<div class="map tgcanvas" aria-hidden="true"><p class="gmsg"></p><div class="gzoom" role="group">' +
      '<button type="button" class="key" data-tz="in">+</button><button type="button" class="key" data-tz="out">−</button>' +
      '<button type="button" class="key" data-tz="fit"></button></div></div>';
    TG.host = h;
    TG.box = h.querySelector(".tgcanvas");
    TG.msg = h.querySelector(".gmsg");
    h.addEventListener("click", (ev) => {
      const b = ev.target.closest("[data-tz]");
      if (!b || !TG.vs) return;
      if (b.dataset.tz === "fit") fit();
      else zoomAt(b.dataset.tz === "in" ? 0.5 : -0.5);
    });
    // Ctrl or ⌘ with the wheel, and a pinch, zoom about the pointer; the
    // plain wheel scrolls the page.
    TG.box.addEventListener("wheel", (ev) => {
      if (!TG.vs || (!ev.ctrlKey && !ev.metaKey)) return;
      ev.preventDefault();
      const r = TG.box.getBoundingClientRect();
      zoomAt(pure.wheelZoom(pure.wheelDelta(ev, TG.box.clientHeight)[1]), ev.clientX - r.left - r.width / 2, ev.clientY - r.top - r.height / 2);
    }, { passive: false, capture: true });
    TG.ro = new ResizeObserver(() => { if (TG.state === "ready" && TG.host.isConnected) draw(); });
    TG.ro.observe(TG.box);
    return h;
  }
  // words labels the zoom keys in the page's language.
  function words() {
    const keys = TG.host.querySelectorAll("[data-tz]");
    keys[0].setAttribute("aria-label", t("graph.zoomIn"));
    keys[1].setAttribute("aria-label", t("graph.zoomOut"));
    keys[2].textContent = t("graph.fit");
  }
  function say(text) {
    TG.msg.textContent = text;
    TG.msg.hidden = !text;
  }

  // show draws spec d's tasks into the Tasks view's slot, sel picked.
  TG.show = function (d, sel) {
    const slot = document.getElementById("tg-slot");
    if (!slot) return;
    slot.appendChild(host());
    words();
    TG.d = d;
    TG.sel = sel || null;
    if (!app.hasWebGL2() || new URLSearchParams(location.search).has("nogl")) {
      say(t("graph.noGL"));
      return;
    }
    if (TG.state === "idle") {
      TG.state = "loading";
      say(t("graph.loading"));
      app.deck(() => {
        TG.state = "ready";
        say("");
        draw();
      }, () => {
        TG.state = "failed";
        say(t("graph.failed"));
      });
      return;
    }
    if (TG.state === "ready") draw();
  };

  // ---------- colours, read from the page's tokens ----------
  function readColors() {
    const css = getComputedStyle(document.documentElement);
    const raw = (name) => css.getPropertyValue(name).trim();
    const page = pure.parseColour(raw("--sunk"));
    const rgb = (name) => pure.parseColour(raw(name), page);
    const mix = (a, b, k) => a.map((x, i) => Math.round(x + (b[i] - x) * k));
    const surface = rgb("--surface");
    TG.c = {
      page, surface, mix, ink: rgb("--ink"), muted: rgb("--muted"), line: rgb("--line"), strong: rgb("--line-strong"),
      brand: rgb("--accent"), done: rgb("--done"), active: rgb("--active"), held: rgb("--held"),
      doneFill: mix(surface, page, 0.4),
    };
  }

  // ---------- the drawing: nodes, edges, labels ----------
  // roundRect is a node's outline as a polygon with rounded corners.
  function roundRect(x, y, w, h, r) {
    const out = [];
    const arc = (cx, cy, a0) => {
      for (let i = 0; i <= 4; i++) {
        const a = a0 + (i / 4) * (Math.PI / 2);
        out.push([cx + r * Math.cos(a), cy + r * Math.sin(a)]);
      }
    };
    arc(x + w - r, y + r, -Math.PI / 2);
    arc(x + w - r, y + h - r, 0);
    arc(x + r, y + h - r, Math.PI / 2);
    arc(x + r, y + r, Math.PI);
    return out;
  }
  // fitTitle cuts a title to what a node's width holds, with an ellipsis.
  function fitTitle(s) {
    const max = Math.floor((GEO.w - 40) / (TITLE_PX * 0.5));
    return s.length > max ? s.slice(0, max - 1).trimEnd() + "…" : s;
  }
  // prepare lays the tasks out for a canvas w wide and sizes the canvas. A
  // drawing whose columns fit the width at zoom 0 within TALL_PX takes a
  // canvas that tall, so its labels read without zooming (a small spec on a
  // phone); any other is fitted whole into the map's own height.
  function prepare(tasks, w) {
    const inner = w - 2 * MARGIN;
    const wide = pure.taskGraphLayout(tasks, pure.taskFitCols(tasks, inner, Infinity));
    const readable = wide.bounds.x1 <= inner && wide.bounds.y1 + 2 * MARGIN <= TALL_PX;
    const fitH = Math.min(window.innerHeight * 0.78, 820);
    const maxH = readable ? Math.max(fitH, wide.bounds.y1 + 2 * MARGIN) : fitH;
    const L = readable ? wide : pure.taskGraphLayout(tasks, pure.taskFitCols(tasks, inner, maxH - 2 * MARGIN));
    const b = L.bounds;
    const zoom = Math.min(0, Math.log2(Math.min((w - 2 * MARGIN) / Math.max(1, b.x1 - b.x0), (maxH - 2 * MARGIN) / Math.max(1, b.y1 - b.y0))));
    TG.box.style.height = Math.round(Math.max(240, Math.min(maxH, (b.y1 - b.y0) * Math.pow(2, zoom) + 2 * MARGIN))) + "px";
    const by = {};
    tasks.forEach((x) => { by[x.id] = x; });
    const nodes = L.nodes.map((n) => {
      const task = by[n.id];
      return { ...n, task, state: task.state, title: fitTitle(task.title || n.id), poly: roundRect(n.x, n.y, n.w, n.h, RADIUS) };
    });
    const at = {};
    nodes.forEach((n) => { at[n.id] = n; });
    const edges = [];
    tasks.forEach((x) => (x.depends_on || []).forEach((dep) => {
      const s = at[dep];
      const e = at[x.id];
      if (!s || !e) return;
      edges.push({ from: dep, to: x.id, done: s.state === "done" && e.state === "done", path: pure.vcurve(s.x + s.w / 2, s.y + s.h, e.x + e.w / 2, e.y, CURVE_STEPS) });
    }));
    const steps = L.steps.map((s) => ({ y: s.y, text: t("tasks.step", { n: s.n }) }));
    const chars = new Set(" …✓●‖·0123456789");
    nodes.forEach((n) => { for (const ch of n.title + n.id) chars.add(ch); });
    steps.forEach((s) => { for (const ch of s.text) chars.add(ch); });
    tasks.forEach((x) => { for (const ch of x.title || "") chars.add(ch); });
    TG.P = { nodes, edges, steps, at, bounds: b, charSet: [...chars] };
  }

  // focusSet is the hovered or else the picked task with its whole chain
  // (tpPure.taskChain), null when neither.
  function focusSet() {
    const id = TG.hover || TG.sel;
    if (!id || !TG.P.at[id]) return null;
    const ch = pure.taskChain(TG.d.tasks, id);
    return new Set([id, ...ch.up, ...ch.down]);
  }
  const OUTLINE = { ready: "brand", wip: "active", waits_on_epic: "held", waits_on_task: "strong", done: "line" };

  function render() {
    const P = TG.P;
    if (!TG.deck || !P) return;
    const { PolygonLayer, PathLayer, TextLayer } = window.deck;
    const c = TG.c;
    const scale = Math.pow(2, TG.vs.zoom);
    const labels = ID_PX * scale >= MIN_PX - 0.01;
    const f = focusSet();
    const dim = (n) => f && !f.has(n.id);
    const on = (e) => f && f.has(e.from) && f.has(e.to);
    const v = [TG.hover, TG.sel, labels, TG.key, TG.themeVer];
    const fontSettings = { sdf: true, fontSize: 64, buffer: 6, radius: 12, cutoff: 0.22 };
    const fill = (n) => {
      const base = n.state === "done" ? c.doneFill : c.surface;
      return [...(dim(n) ? c.mix(base, c.page, 0.6) : base), 255];
    };
    const outline = (n) => [...c[OUTLINE[n.state] || "strong"], dim(n) ? 70 : 255];
    const outlineW = (n) => (n.state === "ready" || n.state === "wip" ? 2 : 1) + (n.id === TG.sel ? 1 : 0);
    const edgeColor = (e) => {
      if (f) return on(e) ? [...c.ink, 235] : [...c.muted, 22];
      return [...c.muted, e.done ? 40 : 90];
    };
    const sel = TG.sel && P.at[TG.sel] ? P.at[TG.sel] : null;
    const tip = !labels && (TG.hover || TG.sel) ? P.at[TG.hover || TG.sel] : null;
    const text = (id, data, extra) => new TextLayer({
      id, data, characterSet: P.charSet, fontSettings, sizeUnits: "common", getTextAnchor: "start", getAlignmentBaseline: "top",
      updateTriggers: { getColor: v }, ...extra,
    });
    const layers = [
      new PathLayer({
        id: "edges-rest", data: f ? P.edges.filter((e) => !on(e)) : P.edges, getPath: (e) => e.path, getColor: edgeColor, getWidth: 1.25,
        widthUnits: "pixels", capRounded: true, jointRounded: true, updateTriggers: { getColor: v },
      }),
      new PolygonLayer({
        id: "nodes", data: P.nodes, getPolygon: (n) => n.poly, stroked: true, filled: true, pickable: true,
        getFillColor: fill, getLineColor: outline, getLineWidth: outlineW, lineWidthUnits: "pixels",
        updateTriggers: { getFillColor: v, getLineColor: v, getLineWidth: v },
      }),
      // the focused chain's edges draw over the dimmed nodes, so no node
      // cuts the chain a task is read by
      new PathLayer({
        id: "edges-on", data: f ? P.edges.filter(on) : [], getPath: (e) => e.path, getColor: edgeColor, getWidth: 2,
        widthUnits: "pixels", capRounded: true, jointRounded: true, updateTriggers: { getColor: v },
      }),
      // and the chain's own nodes over those edges, so a line never runs
      // across a node it does not enter
      new PolygonLayer({
        id: "nodes-on", data: f ? P.nodes.filter((n) => f.has(n.id)) : [], getPolygon: (n) => n.poly, stroked: true, filled: true,
        getFillColor: fill, getLineColor: outline, getLineWidth: outlineW, lineWidthUnits: "pixels",
        updateTriggers: { getFillColor: v, getLineColor: v, getLineWidth: v },
      }),
      text("marks", P.nodes.filter((n) => MARK[n.state]), {
        visible: labels, getPosition: (n) => [n.x + 12, n.y + 13], getText: (n) => MARK[n.state], getSize: TITLE_PX,
        getColor: (n) => [...c[n.state === "done" ? "done" : n.state === "wip" ? "active" : "held"], dim(n) ? 80 : 255],
        fontFamily: "Inter, system-ui, sans-serif", fontWeight: 700,
      }),
      text("titles", P.nodes, {
        visible: labels, getPosition: (n) => [n.x + 30, n.y + 12], getText: (n) => n.title, getSize: TITLE_PX,
        getColor: (n) => [...(n.state === "done" ? c.muted : c.ink), dim(n) ? 80 : 255],
        fontFamily: "Inter, system-ui, sans-serif", fontWeight: 600,
      }),
      text("ids", P.nodes, {
        visible: labels, getPosition: (n) => [n.x + 30, n.y + 36], getText: (n) => n.id, getSize: ID_PX,
        getColor: (n) => [...c.muted, dim(n) ? 80 : 255], fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
      }),
      text("steps", P.steps, {
        // a step's name sits on its first row's top edge, so it stays in the
        // gap above the step however far the camera pulls back
        // and leaves once that gap is too thin to hold it
        visible: (GEO.head + GEO.sgap) * scale >= MIN_PX + 5,
        getPosition: (s) => [0, s.y + GEO.head], getText: (s) => s.text, getSize: MIN_PX, sizeUnits: "pixels",
        getAlignmentBaseline: "bottom", getPixelOffset: [0, -3],
        getColor: [...c.ink, 255], fontFamily: "Inter, system-ui, sans-serif", fontWeight: 700,
      }),
      // Below the label floor the picked or hovered task still names itself
      // beside its node, at the floor's size.
      text("focus-title", tip ? [tip] : [], {
        getPosition: (n) => [n.x + n.w + 8, n.y + n.h / 2], getText: (n) => n.task.title || n.id, getSize: MIN_PX, sizeUnits: "pixels",
        getAlignmentBaseline: "center", getColor: [...c.ink, 255], fontFamily: "Inter, system-ui, sans-serif", fontWeight: 600,
        background: true, getBackgroundColor: [...c.surface, 245], backgroundPadding: [6, 3, 6, 3], getBorderColor: [...c.line, 255], getBorderWidth: 1,
      }),
    ];
    if (sel) TG.box.dataset.selected = sel.id;
    else delete TG.box.dataset.selected;
    TG.box.dataset.drawnNodes = String(P.nodes.length);
    TG.box.dataset.drawnEdges = String(P.edges.length);
    TG.box.dataset.labels = labels ? "on" : "off";
    TG.deck.setProps({ layers, viewState: TG.vs });
  }

  // ---------- camera ----------
  // fitView shows the whole drawing with MARGIN round it, never magnified
  // past zoom 0; a drawing smaller than the canvas sits at its top left.
  function fitView() {
    const b = TG.P.bounds;
    const w = Math.max(1, TG.box.clientWidth);
    const h = Math.max(1, TG.box.clientHeight);
    const zoom = Math.min(0, Math.log2(Math.min((w - 2 * MARGIN) / Math.max(1, b.x1 - b.x0), (h - 2 * MARGIN) / Math.max(1, b.y1 - b.y0))));
    const s = Math.pow(2, zoom);
    const cx = (b.x1 - b.x0) * s < w - 2 * MARGIN ? b.x0 - MARGIN / s + w / 2 / s : (b.x0 + b.x1) / 2;
    const cy = (b.y1 - b.y0) * s < h - 2 * MARGIN ? b.y0 - MARGIN / s + h / 2 / s : (b.y0 + b.y1) / 2;
    return { target: [cx, cy, 0], zoom, minZoom: -6, maxZoom: 2 };
  }
  function fit() {
    TG.vs = fitView();
    render();
  }
  // zoomAt zooms by dz about the point (px, py) from the canvas's centre.
  function zoomAt(dz, px, py) {
    const vs = TG.vs;
    const z = Math.min(2, Math.max(-6, vs.zoom + dz));
    const s0 = Math.pow(2, vs.zoom);
    const s1 = Math.pow(2, z);
    const wx = vs.target[0] + (px || 0) / s0;
    const wy = vs.target[1] + (py || 0) / s0;
    TG.vs = { ...vs, zoom: z, target: [wx - (px || 0) / s1, wy - (py || 0) / s1, 0] };
    render();
  }

  function tooltip({ object, layer }) {
    if (!object || !layer || layer.id !== "nodes") return null;
    const x = object.task;
    const esc = (s) => String(s).replace(/[&<>"']/g, (ch) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[ch]);
    const deps = (x.depends_on || []).length ? "<br>" + esc(t("task.after", { ids: x.depends_on.join(", ") })) : "";
    const c = TG.c;
    return {
      html: "<b>" + esc(x.title || x.id) + '</b><br><span style="font-family:var(--mono)">' + esc(x.id) + "</span> · " + esc(app.taskWord(x.state)) + deps,
      style: { background: "rgb(" + c.ink + ")", color: "rgb(" + c.surface + ")", borderRadius: "var(--r-2)", maxWidth: "320px", font: "var(--type-tiny)", padding: "var(--sp-2) var(--sp-3)" },
    };
  }
  function ensureDeck() {
    if (TG.deck) return;
    TG.deck = new window.deck.Deck({
      parent: TG.box,
      views: new window.deck.OrthographicView({ id: "tg", flipY: true }),
      controller: { scrollZoom: false, dragRotate: false, doubleClickZoom: true, touchZoom: true, keyboard: false, inertia: 200 },
      viewState: TG.vs,
      onViewStateChange: ({ viewState }) => {
        TG.vs = viewState;
        render();
      },
      onHover: (info) => {
        const id = info.object && info.layer && info.layer.id === "nodes" ? info.object.id : null;
        if (id === TG.hover) return;
        TG.hover = id;
        render();
      },
      onClick: (info) => {
        if (info.object && info.layer && info.layer.id === "nodes") app.pickTask(info.object.id);
        else if (TG.sel) app.pickTask(TG.sel);
      },
      getCursor: ({ isDragging, isHovering }) => (isDragging ? "grabbing" : isHovering ? "pointer" : "grab"),
      getTooltip: tooltip,
      onLoad: () => TG.box.querySelectorAll("canvas").forEach((cv) => { cv.tabIndex = -1; }),
    });
  }

  // draw lays the tasks out again when they or the canvas's width changed,
  // fits the camera then, and renders.
  function draw() {
    const tasks = TG.d.tasks || [];
    const w = Math.max(1, TG.box.clientWidth);
    // The camera fits again only when the shape changes, not on a task's
    // state moving under the live stream.
    const shape = w + "|" + JSON.stringify(tasks.map((x) => [x.id, x.depends_on]));
    const key = shape + "|" + t("tasks.step", { n: 1 }) + JSON.stringify(tasks.map((x) => [x.state, x.title]));
    readColors();
    if (key !== TG.key) {
      prepare(tasks, w);
      TG.key = key;
    }
    ensureDeck();
    if (shape !== TG.shape || !TG.vs) {
      TG.vs = fitView();
      TG.shape = shape;
    }
    render();
  }
})();
