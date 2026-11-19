// The Tasks view's Graph: the Steps panel drawn with deck.gl. Its layout is
// Steps' own: the Steps markup (tpApp.stepsMarkup) is laid out unseen at the
// panel's width and measured (tpApp.stepsBoxes), and every card, header and
// line of text is drawn where Steps put it, in Steps' colours. One world unit
// is one CSS pixel of that layout, so zoom 0 is the panel itself, its text at
// its own size. Cards, text, borders and edges are all sized in world units:
// one camera zoom (tpPure.zoomCamera) scales them alike, and pinch, the wheel
// with Ctrl or ⌘, Safari's gesture and the + and − keys all take that path.
//
// The canvas is as tall as the window and sticks to it while a spacer as tall
// as the drawing scrolls past: the page's own scroll, a wheel, two fingers or
// a phone swipe, is the vertical pan, and a drag pans across. Edges are the
// epic map's curves, drawn under the cards and faint at rest; hovering or
// picking a task draws its whole chain strong and dims the rest. The canvas
// has no focusable node: the list beside it, visually hidden until it takes
// focus, is the keyboard's and the screen reader's way in. deck.gl is
// graph.js's, loaded once (tpApp.deck).
(function () {
  "use strict";
  const app = window.tpApp;
  const pure = window.tpPure;
  if (!app || !pure || !app.deck) return;
  const t = (key, params) => app.tr()(key, params);
  // Text draws while 14 pixels of it read at 9 or more on screen, as the
  // epic map's labels do (tpPure.detailLevel); smaller, a card names itself
  // in the tooltip.
  const ZOOM_STEP = 0.5;
  const GESTURE_IDLE_MS = 500;
  const CURVE_STEPS = 12;
  const MARK = { done: "✓", wip: "◐", waits_on_epic: "‖" };
  const SANS = "Inter, system-ui, sans-serif";
  const MONO = "ui-monospace, SFMono-Regular, Menlo, monospace";
  const TG = {
    host: null, spacer: null, box: null, msg: null, deck: null, state: "idle", d: null, sel: null, hover: null,
    S: null, key: null, z: 0, tx: 0, ty: 0, ch: 0, themeVer: 0, pinch: 1, gesture: false, frame: 0,
  };
  window.tpTaskGraph = TG;

  // ---------- the host: built once, moved into each fresh slot ----------
  // The Tasks view redraws its HTML on every change; the host keeps the
  // canvas, so deck.gl's context survives each redraw.
  function host() {
    if (TG.host) return TG.host;
    const h = document.createElement("div");
    h.className = "tgraph";
    h.innerHTML = '<div class="tgspacer"><div class="tgcanvas" aria-hidden="true"><p class="gmsg"></p><div class="gzoom" role="group">' +
      '<button type="button" class="key" data-tz="in">+</button><button type="button" class="key" data-tz="out">−</button>' +
      '<button type="button" class="key" data-tz="fit"></button></div></div></div>';
    TG.host = h;
    TG.spacer = h.querySelector(".tgspacer");
    TG.box = h.querySelector(".tgcanvas");
    TG.msg = h.querySelector(".gmsg");
    h.addEventListener("click", (ev) => {
      const b = ev.target.closest("[data-tz]");
      if (!b || !TG.S) return;
      if (b.dataset.tz === "fit") fit();
      else zoomAt(b.dataset.tz === "in" ? ZOOM_STEP : -ZOOM_STEP, 0, 0);
    });
    TG.box.addEventListener("wheel", onWheel, { passive: false, capture: true });
    ["gesturestart", "gesturechange", "gestureend"].forEach((type) => TG.box.addEventListener(type, onGesture));
    new ResizeObserver(() => { if (live() && TG.host.clientWidth !== TG.S.w) draw(); }).observe(h);
    window.addEventListener("scroll", () => {
      if (!live() || TG.frame) return;
      TG.frame = requestAnimationFrame(() => {
        TG.frame = 0;
        sync();
        render();
      });
    }, { passive: true });
    return h;
  }
  const live = () => TG.state === "ready" && TG.S && TG.host && TG.host.isConnected;
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
    // A pick made off the canvas, a drawer link or the list, is brought into
    // view once the drawer it opens is laid out; one made on it already is.
    const moved = !!sel && sel !== TG.sel && !TG.picked;
    TG.picked = false;
    TG.sel = sel || null;
    if (moved) requestAnimationFrame(() => reveal(sel));
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
  // A theme change (tp-theme, as graph.js hears it) measures the colours
  // again.
  document.addEventListener("tp-theme", () => {
    TG.themeVer++;
    if (live()) draw();
  });

  // ---------- the scene: Steps laid out unseen, then measured ----------
  // shape is a card's outline: a box with corner radii [tl, tr, br, bl],
  // clockwise from its top edge's left end.
  function shape(x, y, w, h, r) {
    const out = [];
    const arc = (cx, cy, rad, a0) => {
      if (rad <= 0) {
        out.push([cx, cy]);
        return;
      }
      for (let i = 0; i <= 4; i++) {
        const a = a0 + (i / 4) * (Math.PI / 2);
        out.push([cx + rad * Math.cos(a), cy + rad * Math.sin(a)]);
      }
    };
    arc(x + w - r[1], y + r[1], r[1], -Math.PI / 2);
    arc(x + w - r[2], y + h - r[2], r[2], 0);
    arc(x + r[3], y + h - r[3], r[3], Math.PI / 2);
    arc(x + r[0], y + r[0], r[0], Math.PI);
    return out;
  }
  // lines is an element's text as the browser broke it into lines.
  function lines(el) {
    const node = el.firstChild;
    const text = el.textContent;
    if (!node || node.nodeType !== 3) return [text];
    const range = document.createRange();
    const out = [];
    let start = 0;
    let top = null;
    for (let i = 0; i < text.length; i++) {
      range.setStart(node, i);
      range.setEnd(node, i + 1);
      const rs = range.getClientRects();
      if (!rs.length) continue;
      if (top === null) top = rs[0].top;
      else if (rs[0].top > top + 2) {
        out.push(text.slice(start, i).trimEnd());
        start = i;
        top = rs[0].top;
      }
    }
    out.push(text.slice(start).trim());
    return out;
  }
  // font is what an element's text needs on the canvas: its size, weight,
  // line height as a share of the size, and where its first line's letters
  // start inside its box.
  function font(el) {
    const cs = getComputedStyle(el);
    const size = parseFloat(cs.fontSize);
    const lh = parseFloat(cs.lineHeight) || size * 1.4;
    return { size, weight: Number(cs.fontWeight) || 400, lh: lh / size, pad: (lh - size) / 2, color: cs.color };
  }
  // measure lays Steps out for spec d at the slot's width and reads every
  // card, header and line of text off it, with the colours it was drawn in.
  function measure(d) {
    const m = document.createElement("div");
    m.className = "stages tgmeasure";
    m.inert = true;
    m.innerHTML = app.stepsMarkup(d);
    const svg = m.querySelector("svg");
    if (svg) svg.remove();
    TG.host.appendChild(m);
    const { R, rect } = app.stepsBoxes(m);
    const css = getComputedStyle(document.documentElement);
    const panel = pure.parseColour(css.getPropertyValue("--surface"));
    const rgb = (v, under) => pure.parseColour(v, under || panel);
    const C = {
      panel, ink: rgb(css.getPropertyValue("--ink")), muted: rgb(css.getPropertyValue("--muted")), wire: rgb(css.getPropertyValue("--line-strong")),
      faint: rgb(css.getPropertyValue("--line")), strong: rgb(css.getPropertyValue("--fill-strong")), sel: rgb(css.getPropertyValue("--stroke-selected")),
    };
    const texts = [];
    const cards = Array.from(m.querySelectorAll(".tcard")).map((el) => {
      const b = R[el.dataset.t];
      const cs = getComputedStyle(el);
      const bg = rgb(cs.backgroundColor);
      const fillWeak = cs.backgroundImage && cs.backgroundImage !== "none" ? rgb(css.getPropertyValue("--fill-weak"), bg) : bg;
      const card = {
        id: el.dataset.t, state: el.dataset.s, ...b, secDim: el.classList.contains("dim"),
        stripe: rgb(cs.borderLeftColor), border: rgb(cs.borderTopColor), bg: fillWeak, dashed: cs.borderTopStyle === "dashed",
        radius: parseFloat(cs.borderTopLeftRadius) || 8, left: parseFloat(cs.borderLeftWidth) || 4, edge: parseFloat(cs.borderTopWidth) || 1,
      };
      const title = el.querySelector(".tt > span:last-child");
      const ic = el.querySelector(".tt .ic");
      const add = (part, node, text) => {
        const f = font(node);
        const r = rect(node);
        texts.push({ card: card.id, part, text, x: r.x, y: r.y + f.pad, size: f.size, weight: f.weight, lh: f.lh, color: rgb(f.color, card.bg) });
      };
      add("title", title, lines(title).join("\n"));
      add("id", el.querySelector(".tid"), el.querySelector(".tid").textContent);
      add("meta", el.querySelector(".tm"), el.querySelector(".tm").textContent);
      if (ic && MARK[card.state]) {
        const r = rect(ic);
        texts.push({ card: card.id, part: "mark", text: MARK[card.state], x: r.x + r.w / 2, y: r.y + r.h / 2, size: r.h + 2, weight: 700, lh: 1, color: card.stripe });
      }
      return card;
    });
    m.querySelectorAll(".step-h > *").forEach((el) => {
      const f = font(el);
      const r = rect(el);
      texts.push({ card: null, part: "head", text: el.textContent, x: r.x, y: r.y + f.pad, size: f.size, weight: f.weight, lh: f.lh, color: rgb(f.color) });
    });
    const w = TG.host.clientWidth;
    const h = m.scrollHeight;
    m.remove();
    const at = {};
    cards.forEach((c) => { at[c.id] = c; });
    const edges = [];
    (d.tasks || []).forEach((x) => (x.depends_on || []).forEach((dep) => {
      const s = at[dep];
      const e = at[x.id];
      if (!s || !e) return;
      edges.push({ from: dep, to: x.id, path: pure.vcurve(s.x + s.w / 2, s.y + s.h, e.x + e.w / 2, e.y, CURVE_STEPS) });
    }));
    const chars = new Set(" ");
    texts.forEach((x) => { for (const ch of x.text) chars.add(ch); });
    cards.forEach((c) => {
      c.outer = shape(c.x, c.y, c.w, c.h, [c.radius, c.radius, c.radius, c.radius]);
      const ri = Math.max(0, c.radius - c.edge);
      c.body = shape(c.x + c.left, c.y + c.edge, c.w - c.left - c.edge, c.h - 2 * c.edge, [Math.max(0, c.radius - c.left), ri, ri, Math.max(0, c.radius - c.left)]);
      // the border Steps draws round the top, right and bottom, as one line
      const o = c.edge / 2;
      const loop = shape(c.x + c.left - o, c.y + o, c.w - c.left, c.h - c.edge, [0, c.radius - o, c.radius - o, 0]);
      c.rim = loop.slice(0, loop.length - 1);
      c.ring = shape(c.x - 1, c.y - 1, c.w + 2, c.h + 2, [c.radius + 1, c.radius + 1, c.radius + 1, c.radius + 1]);
      c.ring.push(c.ring[0]);
    });
    const byId = {};
    (d.tasks || []).forEach((x) => { byId[x.id] = x; });
    return { w, h, cards, at, texts, edges, C, byId, charSet: [...chars] };
  }

  // ---------- the camera: one zoom, the page's scroll as the vertical ----------
  // sync reads the camera's vertical from the page's scroll: the canvas
  // sticks in a spacer as tall as the drawing at this zoom, and shows the
  // part of it the window is over.
  function sync() {
    const S = TG.S;
    const s = Math.pow(2, TG.z);
    const total = Math.max(1, Math.round(S.h * s));
    TG.spacer.style.height = total + "px";
    TG.ch = Math.min(total, window.innerHeight);
    TG.box.style.height = TG.ch + "px";
    const off = Math.max(0, Math.min(total - TG.ch, -TG.spacer.getBoundingClientRect().top));
    TG.ty = (off + TG.ch / 2) / s;
    const cw = TG.host.clientWidth;
    TG.tx = S.w * s <= cw ? S.w / 2 : Math.max(cw / 2 / s, Math.min(S.w - cw / 2 / s, TG.tx));
  }
  // place puts the camera at vs, as tpPure.cameraOf leaves it: its zoom and
  // its across at once, its vertical by scrolling the page.
  function place(vs) {
    const v = pure.cameraOf(vs);
    TG.z = Math.max(-3, Math.min(2, v.zoom));
    TG.tx = v.target[0];
    const s = Math.pow(2, TG.z);
    TG.spacer.style.height = Math.max(1, Math.round(TG.S.h * s)) + "px";
    const ch = Math.min(TG.S.h * s, window.innerHeight);
    const want = v.target[1] * s - ch / 2;
    const top = TG.spacer.getBoundingClientRect().top + window.scrollY;
    window.scrollTo(window.scrollX, top + want);
    sync();
    render();
  }
  const camera = () => ({ target: [TG.tx, TG.ty, 0], zoom: TG.z });
  // reveal brings card id into view (tpPure.revealShift): the page scrolls
  // it between the window's top and, on a phone, the drawer's sheet, and the
  // camera moves across to it clear of the drawer beside a wide screen. At a
  // zoom where Steps fits the panel's width the across stays centred (sync),
  // so a card under the side drawer there is shown by the page alone.
  const REVEAL_PAD = 16;
  function reveal(id) {
    const c = TG.S && TG.S.at[id];
    if (!c || !live()) return;
    const s = Math.pow(2, TG.z);
    const drawer = document.getElementById("drawer");
    const d = drawer.getBoundingClientRect();
    const sheet = !drawer.hidden && d.top > window.innerHeight / 3;
    const side = !drawer.hidden && !sheet && d.left > window.innerWidth / 3;
    const top = TG.spacer.getBoundingClientRect().top + c.y * s;
    const dy = pure.revealShift(top, top + c.h * s, 0, sheet ? d.top : window.innerHeight, REVEAL_PAD);
    if (dy) window.scrollBy(0, dy);
    const box = TG.host.getBoundingClientRect();
    const left = box.left + box.width / 2 + (c.x - TG.tx) * s;
    const dx = pure.revealShift(left, left + c.w * s, box.left, side ? Math.min(box.right, d.left) : box.right, REVEAL_PAD);
    TG.tx += dx / s;
    sync();
    render();
  }
  // zoomAt zooms by dz about (px, py) from the canvas's centre
  // (tpPure.zoomCamera), the one path every zoom takes.
  function zoomAt(dz, px, py) {
    place(pure.zoomCamera(camera(), dz, px, py));
  }
  // fit shows the panel as Steps draws it, zoom 0 across its width, keeping
  // the top of what is in view.
  function fit() {
    zoomAt(-TG.z, 0, -TG.ch / 2);
    TG.tx = TG.S.w / 2;
    sync();
    render();
  }
  // The wheel and the pinch, as the epic map takes them (tpPure.wheelAction,
  // tpPure.gestureAction): Ctrl or ⌘ with the wheel, Chrome's and Firefox's
  // pinch, zooms about the pointer; a plain wheel or two-finger scroll is the
  // page's, and so the graph's vertical pan.
  function pointer(ev) {
    const r = TG.box.getBoundingClientRect();
    return [ev.clientX - r.left - r.width / 2, ev.clientY - r.top - r.height / 2];
  }
  function onWheel(ev) {
    if (!live()) return;
    if (!ev.ctrlKey && !ev.metaKey) TG.gesture = false;
    if (pure.wheelAction(ev, false, "canvas") !== "zoom") return;
    ev.preventDefault();
    // A pinch Safari reports as a gesture zooms there alone.
    if (TG.gesture) return;
    zoomAt(pure.wheelZoom(pure.wheelDelta(ev, TG.box.clientHeight)[1]), ...pointer(ev));
  }
  // Safari's pinch: its gesture events carry the scale since it began, and
  // each step zooms by the change, so the page itself never zooms.
  function onGesture(ev) {
    if (pure.gestureAction(false, "canvas") !== "zoom") return;
    ev.preventDefault();
    if (ev.type === "gesturestart") {
      TG.pinch = 1;
      TG.gesture = true;
      return;
    }
    if (ev.type === "gestureend") {
      TG.gesture = false;
      return;
    }
    if (!live()) return;
    zoomAt(pure.pinchZoom(TG.pinch, ev.scale), ...pointer(ev));
    TG.pinch = ev.scale;
    clearTimeout(TG.gestureTimer);
    TG.gestureTimer = setTimeout(() => { TG.gesture = false; }, GESTURE_IDLE_MS);
  }

  // ---------- the drawing ----------
  // focusSet is the hovered or else the picked task with its whole chain
  // (tpPure.taskChain), null when neither.
  function focusSet() {
    const id = TG.hover || TG.sel;
    if (!id || !TG.S.at[id]) return null;
    const ch = pure.taskChain(TG.d.tasks, id);
    return new Set([id, ...ch.up, ...ch.down]);
  }
  function render() {
    const S = TG.S;
    if (!TG.deck || !S) return;
    const { PolygonLayer, PathLayer, TextLayer } = window.deck;
    const vs = camera();
    const showText = pure.detailLevel(pure.onScreen(vs, 14)) === "near";
    const f = focusSet();
    const C = S.C;
    const v = [TG.hover, TG.sel, TG.key, TG.themeVer];
    // A card outside the focused chain, or outside the section the reader
    // picked (Steps' .dim), fades toward the panel as Steps fades it.
    const fade = (c) => (f && !f.has(c.id)) || c.secDim;
    const tone = (rgb, c) => [...(fade(c) ? rgb.map((x, i) => Math.round(x + (C.panel[i] - x) * 0.6)) : rgb), 255];
    const on = (e) => f && f.has(e.from) && f.has(e.to);
    const dashes = window.deck.PathStyleExtension ? [new window.deck.PathStyleExtension({ dash: true })] : [];
    const fontSettings = { sdf: true, fontSize: 64, buffer: 6, radius: 12, cutoff: 0.22 };
    const text = (id, data, family) => new TextLayer({
      id, data, visible: showText, characterSet: S.charSet, fontSettings, fontFamily: family, sizeUnits: "common",
      getPosition: (x) => [x.x, x.y], getText: (x) => x.text, getSize: (x) => x.size, lineHeight: data.length ? data[0].lh : 1.4,
      getTextAnchor: (x) => (x.part === "mark" ? "middle" : "start"), getAlignmentBaseline: (x) => (x.part === "mark" ? "center" : "top"),
      fontWeight: data.length ? data[0].weight : 400,
      getColor: (x) => (x.card && fade(S.at[x.card]) ? [...x.color, 110] : [...x.color, 255]), updateTriggers: { getColor: v },
    });
    const part = (p) => S.texts.filter((x) => x.part === p);
    const heads = part("head");
    const layers = [
      new PathLayer({
        id: "edges", data: f ? S.edges.filter((e) => !on(e)) : S.edges, getPath: (e) => e.path, widthUnits: "common", getWidth: 1.2,
        getColor: [...(f ? C.faint : C.wire), f ? 255 : 170], capRounded: true, jointRounded: true, updateTriggers: { getColor: v },
      }),
      new PathLayer({
        id: "edges-on", data: f ? S.edges.filter(on) : [], getPath: (e) => e.path, widthUnits: "common", getWidth: 2,
        getColor: [...C.strong, 255], capRounded: true, jointRounded: true,
      }),
      new PolygonLayer({
        id: "cards", data: S.cards, getPolygon: (c) => c.outer, filled: true, stroked: false, pickable: true,
        getFillColor: (c) => tone(c.stripe, c), updateTriggers: { getFillColor: v },
      }),
      new PolygonLayer({
        id: "bodies", data: S.cards, getPolygon: (c) => c.body, filled: true, stroked: false,
        getFillColor: (c) => tone(c.bg, c), updateTriggers: { getFillColor: v },
      }),
      new PathLayer({
        id: "rims", data: S.cards, getPath: (c) => c.rim, widthUnits: "common", getWidth: (c) => c.edge,
        getColor: (c) => tone(c.border, c), getDashArray: (c) => (c.dashed ? [3, 3] : [0, 0]), dashJustified: true, extensions: dashes,
        updateTriggers: { getColor: v },
      }),
      new PathLayer({
        id: "ring", data: TG.sel && S.at[TG.sel] ? [S.at[TG.sel]] : [], getPath: (c) => c.ring, widthUnits: "common", getWidth: 2,
        getColor: [...C.sel, 255],
      }),
      text("titles", part("title"), SANS),
      text("ids", part("id"), MONO),
      text("metas", part("meta"), SANS),
      text("marks", part("mark"), SANS),
      text("heads-b", heads.filter((x) => x.weight >= 600), SANS),
      text("heads", heads.filter((x) => x.weight < 600), SANS),
    ];
    TG.box.dataset.drawnCards = String(S.cards.length);
    TG.box.dataset.zoom = String(Math.round(TG.z * 100) / 100);
    TG.box.dataset.text = showText ? "on" : "off";
    if (TG.sel) TG.box.dataset.selected = TG.sel;
    else delete TG.box.dataset.selected;
    TG.deck.setProps({ layers, viewState: { ...vs, minZoom: -3, maxZoom: 2 } });
  }

  function tooltip({ object, layer }) {
    if (!object || !layer || layer.id !== "cards" || TG.box.dataset.text === "on") return null;
    const x = TG.S.byId[object.id] || { id: object.id };
    const esc = (s) => String(s).replace(/[&<>"']/g, (ch) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[ch]);
    const C = TG.S.C;
    return {
      html: "<b>" + esc(x.title || x.id) + '</b><br><span style="font-family:var(--mono)">' + esc(x.id) + "</span> · " + esc(app.taskWord(x.state)),
      style: { background: "rgb(" + C.ink + ")", color: "rgb(" + C.panel + ")", borderRadius: "var(--r-2)", maxWidth: "320px", font: "var(--type-tiny)", padding: "var(--sp-2) var(--sp-3)" },
    };
  }
  function ensureDeck() {
    if (TG.deck) return;
    TG.deck = new window.deck.Deck({
      parent: TG.box,
      views: new window.deck.OrthographicView({ id: "tg", flipY: true }),
      // A vertical swipe is the page's scroll (touch-action pan-y), so a
      // phone scrolls past and through the graph; across, and the pinch,
      // are the graph's.
      touchAction: "pan-y",
      controller: { scrollZoom: false, dragRotate: false, doubleClickZoom: true, touchZoom: true, keyboard: false, inertia: false },
      viewState: { ...camera(), minZoom: -3, maxZoom: 2 },
      // Every move the controller makes, a drag, a pinch or a double click,
      // goes through the one camera (place), its vertical into the page's
      // scroll.
      onViewStateChange: ({ viewState }) => place(viewState),
      onHover: (info) => {
        const id = info.object && info.layer && info.layer.id === "cards" ? info.object.id : null;
        if (id === TG.hover) return;
        TG.hover = id;
        render();
      },
      onClick: (info) => {
        TG.picked = true;
        if (info.object && info.layer && info.layer.id === "cards") app.pickTask(info.object.id);
        else if (TG.sel) app.pickTask(TG.sel);
        TG.picked = false;
      },
      getCursor: ({ isDragging, isHovering }) => (isDragging ? "grabbing" : isHovering ? "pointer" : "grab"),
      getTooltip: tooltip,
      onLoad: () => TG.box.querySelectorAll("canvas").forEach((cv) => { cv.tabIndex = -1; }),
    });
  }

  // draw measures Steps again when the tasks, their words, the theme or the
  // panel's width changed, and renders.
  function draw() {
    const tasks = TG.d.tasks || [];
    const w = TG.host.clientWidth;
    const key = w + "|" + TG.themeVer + "|" + t("tasks.step", { n: 1 }) + JSON.stringify(tasks.map((x) => [x.id, x.state, x.title, x.depends_on, x.closed_at]));
    if (key !== TG.key) {
      const first = !TG.S;
      TG.S = measure(TG.d);
      TG.key = key;
      if (first) {
        TG.z = 0;
        TG.tx = TG.S.w / 2;
      }
    }
    ensureDeck();
    sync();
    render();
  }
})();
