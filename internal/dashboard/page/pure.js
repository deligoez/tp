// tp dashboard's pure functions: geometry and grouping that read their
// arguments alone and touch no document, so the tests run them under node.
// The page loads this file before app.js and reads them as tpPure.
(function (root) {
  "use strict";
  const P = {};

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
  // v, n groups before its Show all, worded by t: its key, its title ("no
  // milestone" when it has none), whether it is expanded, the groups shown
  // (none when folded) and its Show all, null when folded or when n groups
  // hold it.
  P.waitSection = function (s, v, n, t) {
    const key = s.milestone;
    const name = key || t("wait.noMilestone");
    const expanded = !v.folded.has(key);
    const all = v.all.has(key);
    const more = expanded && s.groups.length > n ? { expanded: all, text: all ? t("wait.showLargest", { n }) : t("wait.showAll", { n: s.groups.length, name }) } : null;
    return { key, title: t("wait.blockersIn", { name }), expanded, groups: !expanded ? [] : all ? s.groups : s.groups.slice(0, n), more };
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
  // count, naming the condition epics between them, worded by t.
  P.epicTotals = function (epics, t) {
    const conds = (epics || []).filter((e) => e.condition).length;
    const all = (epics || []).length;
    return t("totals.sentence", { epics: t("count.epics", { n: all }), work: t("count.workEpics", { n: all - conds }), conds: t("count.conditionEpics", { n: conds }) });
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

  // mdInline draws a line's code spans, bold, italic and http(s) links over
  // its escaped text; a code span's text is left as it is.
  P.mdInline = function (s) {
    return escText(s).split(/(`[^`]+`)/).map((part, i) => {
      if (i % 2) return "<code>" + part.slice(1, -1) + "</code>";
      return part
        .replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>')
        .replace(/\*\*([^*]+)\*\*/g, "<b>$1</b>")
        .replace(/(^|[^\w*])\*([^*\s](?:[^*]*[^*\s])?)\*(?!\w)/g, "$1<i>$2</i>");
    }).join("");
  };

  const MD_FENCE = /^\s*(```|~~~)/;
  const MD_ITEM = /^\s*(?:[-*+]|\d+[.)])\s+/;
  const mdTable = (lines) => {
    const rows = lines.filter((l) => !/^\|?\s*:?-{2,}/.test(l.trim())).map((l) => l.trim().replace(/^\||\|$/g, "").split("|").map((c) => c.trim()));
    return '<div class="scroll"><table>' + rows.map((r, i) => "<tr>" + r.map((c) => (i ? "<td>" : "<th>") + P.mdInline(c) + (i ? "</td>" : "</th>")).join("") + "</tr>").join("") + "</table></div>";
  };
  const mdList = (lines) => {
    const items = [];
    lines.forEach((l) => { if (MD_ITEM.test(l) || !items.length) items.push(l.replace(MD_ITEM, "")); else items[items.length - 1] += " " + l.trim(); });
    const tag = /^\s*\d/.test(lines[0]) ? "ol" : "ul";
    return "<" + tag + ">" + items.map((x) => "<li>" + P.mdInline(x) + "</li>").join("") + "</" + tag + ">";
  };

  // mdBlock draws one paragraph by what its first line is: a fence, a table,
  // a list, a heading, or prose.
  P.mdBlock = function (text) {
    const lines = String(text).split("\n");
    if (MD_FENCE.test(lines[0])) return "<pre>" + escText(lines.slice(1, MD_FENCE.test(lines[lines.length - 1]) && lines.length > 1 ? -1 : undefined).join("\n")) + "</pre>";
    if (lines.every((l) => l.trim().startsWith("|"))) return mdTable(lines);
    if (MD_ITEM.test(lines[0])) return mdList(lines);
    if (/^#{1,6}\s/.test(lines[0])) return '<h3 class="sub">' + P.mdInline(lines[0].replace(/^#+\s*/, "")) + "</h3>" + (lines.length > 1 ? P.mdBlock(lines.slice(1).join("\n")) : "");
    return "<p>" + P.mdInline(lines.join(" ")) + "</p>";
  };

  // markdown draws a text of paragraphs, each ended by a blank line outside
  // a fence, through mdBlock.
  P.markdown = function (text) {
    const blocks = [];
    let cur = [];
    let fenced = false;
    String(text).split("\n").forEach((l) => {
      if (MD_FENCE.test(l)) fenced = !fenced;
      if (!fenced && !l.trim()) {
        if (cur.length) blocks.push(cur.join("\n"));
        cur = [];
      } else cur.push(l);
    });
    if (cur.length) blocks.push(cur.join("\n"));
    return blocks.map(P.mdBlock).join("");
  };
  // thresholdOf is a milestone's threshold text, the one place the card and
  // the drawer read it: the export's m.gate, else gate, the graph's gate.
  P.thresholdOf = function (m, gate) {
    return String((m.gate && m.gate.threshold) || (gate && gate.threshold) || "").trim();
  };

  // thresholdLine is the threshold as a milestone card words it, by t: the
  // text of a written threshold, else its state's label from labels.
  P.thresholdLine = function (m, gate, labels, t) {
    const th = (m.state || {}).threshold || "absent";
    const text = P.thresholdOf(m, gate);
    return th === "written" && text ? t("threshold.line", { text }) : labels[th] || th;
  };

  // milestoneHTML is a milestone's detail in the drawer, worded by t: its
  // gate's state, its threshold and purpose in full, which its card cuts to
  // one line, and dc, how many of its epics are described, every text
  // escaped.
  P.milestoneHTML = function (m, gate, dc, labels, t) {
    const s = m.state || {};
    const th = s.threshold || "absent";
    const text = P.thresholdOf(m, gate);
    const state = m.open ? t("ms.open") : t("ms.closed");
    const waits = (m.waiting_on || []).length ? t("ms.gateWaits", { state, on: m.waiting_on.join(", ") }) : state;
    const fields = [
      [t("ms.field.gate"), t("ms.gateLine", { state: waits, done: s.members_done || 0, n: s.members || 0 })],
      [t("ms.field.threshold"), (labels[th] || th) + (text ? ": " + text : "")],
      [t("ms.field.purpose"), String((gate || {}).purpose || "").trim() || t("ms.noPurpose")],
      [t("ms.field.described"), t("ms.described", { done: dc.described, n: dc.all })],
    ];
    return "<h3>" + escText(m.milestone) + "</h3><dl>" + fields.map(([k, v]) => "<dt>" + escText(k) + "</dt><dd>" + escText(v) + "</dd>").join("") + "</dl>";
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
  // reads it, worded by t.
  P.closureDescribed = function (nodes, described, t) {
    return t("graph.closureDescribed", { done: nodes.filter((n) => described.has(n.id)).length, n: nodes.length });
  };

  // edgeDrawn says whether the graph card draws an edge l ({ role, critical,
  // any }) under view ({ showCross, hidden: the legend's hidden classes,
  // own: whether an edge is on the selection's chain, if any }). An edge of
  // the critical path is drawn whatever its class, so a view with cross
  // edges hidden never shows the path with holes; only hiding the critical
  // path itself hides it. Any other edge is hidden with its class or its
  // any-of group, and a cross edge waits for showCross unless it is on the
  // selection's own chain.
  P.edgeDrawn = function (l, view) {
    if (l.critical) return !view.hidden.includes("critical");
    if (view.hidden.includes(l.role) || (l.any && view.hidden.includes("any"))) return false;
    return l.role !== "cross" || view.showCross || (!!view.own && view.own(l));
  };

  // edgeLong says whether an edge whose ends lie rows layers and cols
  // columns apart, either way, is long: three layers or eight columns. The
  // card draws a long edge faded and under the short ones, since a few long
  // curves across the plan are what makes it hardest to read.
  P.edgeLong = function (rows, cols) {
    return Math.abs(rows) >= 3 || Math.abs(cols) >= 8;
  };

  // edgeLayers splits the edges edgeDrawn keeps into the card's layers,
  // bottom to top: plain cross edges, any-of edges, long tree and back
  // edges, short ones, and the critical path; drawn is how many it kept,
  // each in one layer, the count the card records.
  P.edgeLayers = function (links, view) {
    const out = { cross: [], any: [], long: [], tree: [], crit: [], drawn: 0 };
    links.forEach((l) => {
      if (!P.edgeDrawn(l, view)) return;
      out.drawn++;
      if (l.critical) out.crit.push(l);
      else if (l.any) out.any.push(l);
      else if (l.role === "cross") out.cross.push(l);
      else if (l.long) out.long.push(l);
      else out.tree.push(l);
    });
    return out;
  };

  // GRAPH_MODES are the graph card's modes, the first its default.
  P.GRAPH_MODES = ["plan", "milestones", "scope", "neighbourhood"];

  // crossShown says whether the graph card draws plain cross edges in s
  // ({ showCross }): in every mode, until K hides them (1.17.1; they were
  // hidden by default in the epic modes before).
  P.crossShown = function (s) {
    return s.showCross !== false;
  };

  // stepMatch is the match Enter moves to among n, from idx (-1 for none
  // yet): the next, wrapping to the first, or with back the previous,
  // wrapping to the last; -1 when there is none.
  P.stepMatch = function (n, idx, back) {
    if (n <= 0) return -1;
    if (back) return idx <= 0 ? n - 1 : idx - 1;
    return (idx + 1) % n;
  };

  // pickTheme is the page's theme: the stored choice when it is light or
  // dark, else null, the system's setting.
  P.pickTheme = function (stored) {
    return stored === "light" || stored === "dark" ? stored : null;
  };

  // themeChoice is which of the header's Auto, Light and Dark reads as
  // chosen for the stored value: light or dark as stored, else auto.
  P.themeChoice = function (stored) {
    return P.pickTheme(stored) || "auto";
  };

  // themeStore is what choosing c stores: light or dark, and null for
  // auto, which clears the stored value so the system's setting rules.
  P.themeStore = function (c) {
    return P.pickTheme(c);
  };

  // placeLabels places band labels in the order given, each box { x, y, w,
  // h, sw, alts } its left edge, top, full width, height, width with the
  // name alone, and further places to try ({ x, y }, the strip above the
  // band). At each place in turn it takes the full label, else the name
  // alone, where neither a label already placed nor one of obstacles (the
  // nodes' boxes) lies under it: { kind, x, y }, else null, the label left
  // out rather than drawn over a node.
  P.placeLabels = function (boxes, obstacles) {
    const placed = (obstacles || []).slice();
    const free = (x, y, w, h) => !placed.some((p) => x < p.x + p.w && p.x < x + w && y < p.y + p.h && p.y < y + h);
    return boxes.map((b) => {
      for (const at of [{ x: b.x, y: b.y }].concat(b.alts || [])) {
        for (const [kind, w] of [["full", b.w], ["short", b.sw]]) {
          if (free(at.x, at.y, w, b.h)) {
            placed.push({ x: at.x, y: at.y, w, h: b.h });
            return { kind, x: at.x, y: at.y };
          }
        }
      }
      return null;
    });
  };

  // withinTwo counts the nodes of a chain, hops mapped to steps, at most
  // two steps from the selection.
  P.withinTwo = function (hops) {
    let n = 0;
    hops.forEach((d) => { if (d <= 2) n++; });
    return n;
  };

  // graphParams is the graph card's place s ({ mode, milestone, depth,
  // showCross, q }) as address pairs, each only when it is not the default:
  // gv the mode, gm the gate's milestone in Gate closure, gd the
  // neighbourhood's depth, cross=0 the cross edges hidden and q the search.
  // The packing is the API's alone; the page draws rows.
  P.graphParams = function (s) {
    const out = [];
    if (s.mode !== "plan") out.push(["gv", s.mode]);
    if (s.mode === "scope" && s.milestone) out.push(["gm", s.milestone]);
    if (s.depth !== 2) out.push(["gd", String(s.depth)]);
    if (!P.crossShown(s)) out.push(["cross", "0"]);
    if (s.q) out.push(["q", s.q]);
    return out;
  };

  // graphPlace reads the graph card's place back through get (an address
  // key to its value or null), a value outside its set as the default.
  P.graphPlace = function (get) {
    const mode = get("gv");
    const depth = get("gd") || "";
    return {
      mode: P.GRAPH_MODES.includes(mode) ? mode : "plan",
      milestone: get("gm") || null,
      depth: /^[1-4]$/.test(depth) ? Number(depth) : 2,
      showCross: get("cross") !== "0",
      q: get("q") || "",
    };
  };

  // hops is every node next reaches from start, breadth first, mapped to
  // its number of steps; start itself is not in it.
  P.hops = function (start, next) {
    const seen = new Map();
    let front = [start];
    for (let d = 1; front.length; d++) {
      const reached = [];
      front.forEach((v) => next(v).forEach((w) => {
        if (w !== start && !seen.has(w)) { seen.set(w, d); reached.push(w); }
      }));
      front = reached;
    }
    return seen;
  };

  // chainAlpha is the opacity a node or edge of a selection's chain is drawn
  // at, d steps from the selection: full within two, fading after, so a
  // selection with hundreds of descendants keeps its near chain legible;
  // full at any distance when whole asks for the whole chain.
  P.chainAlpha = function (d, whole) {
    if (whole) return 255;
    return d <= 2 ? 255 : d === 3 ? 170 : 70;
  };

  // relTime says how long ago ms milliseconds is, in its largest whole unit
  // up to hours, days past two of them, as t's locale writes it; under ten
  // seconds, or a clock gone backwards, is just now.
  P.relTime = function (ms, t) {
    const s = Math.floor(ms / 1000);
    if (!(s >= 10)) return t("time.justNow");
    const rel = new Intl.RelativeTimeFormat(t.locale, { numeric: "always", style: "short" });
    if (s < 60) return rel.format(-s, "second");
    if (s < 3600) return rel.format(-Math.floor(s / 60), "minute");
    if (s < 172800) return rel.format(-Math.floor(s / 3600), "hour");
    return rel.format(-Math.floor(s / 86400), "day");
  };

  // weekRange words an ISO week, "2026-W48", as its Monday-to-Sunday dates
  // the locale writes (23–29 Kas, Nov 23 – 29); anything else reads as
  // itself.
  P.weekRange = function (week, locale) {
    const m = /^(\d{4})-W(\d{2})$/.exec(String(week || ""));
    if (!m) return String(week || "");
    const jan4 = Date.UTC(Number(m[1]), 0, 4);
    const dow = new Date(jan4).getUTCDay() || 7;
    const monday = jan4 - (dow - 1) * 86400000 + (Number(m[2]) - 1) * 7 * 86400000;
    const fmt = new Intl.DateTimeFormat(locale, { day: "numeric", month: "short", timeZone: "UTC" });
    return fmt.formatRange(new Date(monday), new Date(monday + 6 * 86400000));
  };

  // wheelAction is what a wheel event does, with no guess at whether a
  // trackpad or a mouse sent it, by where it lands: on the canvas, Ctrl or
  // ⌘ held, which is also how Chrome and Firefox report a pinch, zooms, and
  // the plain wheel pans in full screen and scrolls the page outside it. In
  // full screen a zoom over the rest of the card, its floating toolbar and
  // legend, zooms the graph, and one anywhere else is cancelled ("block"),
  // so a pinch never zooms the page behind a graph that fills the screen.
  // Everything else is the page's own.
  P.wheelAction = function (ev, fullscreen, place) {
    const zoomKey = ev.ctrlKey || ev.metaKey;
    if (place === "canvas") return zoomKey ? "zoom" : fullscreen ? "pan" : "page";
    if (!fullscreen || !zoomKey) return "page";
    return place === "card" ? "zoom" : "block";
  };

  // gestureAction is what Safari's pinch, its gesture events, does by where
  // it lands: zoom the graph on the canvas, and in full screen anywhere over
  // the card; elsewhere in full screen it is cancelled, and outside full
  // screen a pinch off the canvas zooms the page as Safari does.
  P.gestureAction = function (fullscreen, place) {
    if (place === "canvas" || (fullscreen && place === "card")) return "zoom";
    return fullscreen ? "block" : "page";
  };

  // resizeRefits says whether a change of the canvas box's size, from prev
  // to next, each {w, h} in CSS pixels at devicePixelRatio dpr, refits a
  // graph sitting at its fit: only when the box changed on the screen, in
  // device pixels, beyond a CSS pixel's rounding. A page zoom shrinks the
  // box in CSS pixels by the factor it raises dpr, so it does not refit and
  // the graph zooms with the page; refitting kept the drawing's size while
  // the labels in pixels grew. The first size always fits.
  P.resizeRefits = function (prev, next) {
    if (!prev) return true;
    const tol = Math.max(prev.dpr, next.dpr);
    return Math.abs(prev.w * prev.dpr - next.w * next.dpr) > tol || Math.abs(prev.h * prev.dpr - next.h * next.dpr) > tol;
  };

  // cameraOf keeps of a view state only the camera the page drives: target,
  // one zoom and its bounds. deck.gl's controller hands back zoomX and zoomY
  // too, and OrthographicView projects positions by those while it sizes
  // text by zoom, so a later zoom that changed zoom alone grew the labels and
  // left the drawing where it was.
  P.cameraOf = function (vs) {
    const z = Array.isArray(vs.zoom) ? vs.zoom[0] : vs.zoom;
    const t = vs.target || [0, 0, 0];
    return { target: [t[0], t[1], t[2] || 0], zoom: z, minZoom: -8, maxZoom: 3 };
  };

  // pillTight says whether the full-screen toolbar, measured with its words
  // whole (scrollWidth against clientWidth), overflows the room it has; only
  // then do the mode switch and the cross-edge toggle shorten their words.
  P.pillTight = function (scrollWidth, clientWidth) {
    return scrollWidth > clientWidth + 1;
  };

  // wheelDelta is a wheel event's [x, y] in pixels: its deltaMode counts
  // pixels, lines of 16 pixels, or pages of height pixels.
  P.wheelDelta = function (ev, height) {
    const k = ev.deltaMode === 1 ? 16 : ev.deltaMode === 2 ? height : 1;
    return [(ev.deltaX || 0) * k, (ev.deltaY || 0) * k];
  };

  // ZOOM_PER_PX is how far one pixel of wheel moves the zoom, a power of
  // two: 1/(100 ln 2), so Chrome's pinch, whose deltas undercount the
  // fingers' spread, tracks them one to one and a mouse's notch of 100
  // zooms by 1.41×, as the 1.17.1 audit measured. MAX_ZOOM_STEP keeps any
  // one event within that, well inside 2×.
  const ZOOM_PER_PX = 1 / (100 * Math.LN2);
  const MAX_ZOOM_STEP = 0.5;
  const limitStep = (dz) => Math.max(-MAX_ZOOM_STEP, Math.min(MAX_ZOOM_STEP, dz));

  // wheelZoom is the zoom step for a wheel's dy pixels, up zooming in.
  P.wheelZoom = function (dy) {
    return limitStep(-dy * ZOOM_PER_PX);
  };

  // pinchZoom is the zoom step for Safari's pinch, whose events carry the
  // scale since the gesture began: the log of its change from prev, limited
  // as a wheel's step is; 0 for a scale that is not positive.
  P.pinchZoom = function (prev, scale) {
    if (!(prev > 0) || !(scale > 0)) return 0;
    return limitStep(Math.log2(scale / prev));
  };

  // detailLevel is how much a graph node draws at a label size of px
  // pixels on screen: "far", no text, below 9; "near", its whole id, from 9.
  P.detailLevel = function (px) {
    return px < 9 ? "far" : "near";
  };

  // wrapId is an id as lines of at most per characters: a break after the
  // last ".", "-" or ":" in the line's second half, else at the line's end.
  // Nothing is cut.
  P.wrapId = function (id, per) {
    const lines = [];
    let rest = id;
    while (rest.length > per) {
      let cut = per;
      for (let i = per; i >= Math.ceil(per / 2); i--) {
        if (".-:".includes(rest[i - 1])) {
          cut = i;
          break;
        }
      }
      lines.push(rest.slice(0, cut));
      rest = rest.slice(cut);
    }
    lines.push(rest);
    return lines;
  };

  // fitId is an id whole in a box that holds two lines of per characters at
  // its font: the line width, from per up, whose wrap (wrapId) draws at the
  // largest fit, a fraction of the font no larger than 1, per over the
  // longest line, or 2 over the line count, so the text fills the same box
  // as large as it can and is never cut.
  P.fitId = function (id, per) {
    let best = null;
    for (let w = per; w <= Math.max(per, id.length); w++) {
      const lines = P.wrapId(id, w);
      const longest = Math.max(...lines.map((l) => l.length));
      const fit = Math.round(Math.min(1, per / longest, 2 / lines.length) * 10000) / 10000;
      if (!best || fit > best.fit) best = { lines, fit };
      if (fit === 1) break;
    }
    return best;
  };

  // glyphsOf is every character the graph's text layers draw, the set
  // deck.gl builds its font atlas from: a character left out is not drawn.
  P.glyphsOf = function (nodes, bands) {
    const chars = new Set();
    nodes.forEach((n) => { for (const ch of n.label + n.title + (n.text || "")) chars.add(ch); });
    bands.forEach((b) => { for (const ch of b.name + " ·0123456789") chars.add(ch); });
    return [...chars].join("");
  };

  // fitFloor is the furthest a fit zooms out, as a power of two: where a
  // label of fs units (11 for an epic) is 9 pixels on screen for a gate
  // closure, a neighbourhood or any layout under 200 nodes, so a small
  // graph never fits to blocks; a larger plan fits whole however far out.
  P.fitFloor = function (n, view, fs) {
    if (view === "scope" || view === "neighbourhood" || n < 200) return Math.log2(9 / (fs || 11));
    return -Infinity;
  };

  // luminance is a colour's relative luminance, [r, g, b] in 0..255, as
  // WCAG computes it.
  const luminance = (c) => {
    const ch = c.map((v) => {
      const s = v / 255;
      return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
    });
    return 0.2126 * ch[0] + 0.7152 * ch[1] + 0.0722 * ch[2];
  };

  // contrast is the WCAG contrast ratio of two colours, 1 to 21.
  P.contrast = function (a, b) {
    const x = luminance(a);
    const y = luminance(b);
    return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05);
  };

  // deltaE is the CIE76 colour difference of two colours: their distance
  // in CIELAB (D65), 0 for the same colour, about 100 black to white.
  P.deltaE = function (a, b) {
    const lab = (c) => {
      const f = c.map((v) => {
        const s = v / 255;
        return s <= 0.04045 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
      });
      const g = (x) => (x > 0.008856 ? Math.cbrt(x) : 7.787 * x + 16 / 116);
      const x = g((f[0] * 0.4124 + f[1] * 0.3576 + f[2] * 0.1805) / 0.95047);
      const y = g(f[0] * 0.2126 + f[1] * 0.7152 + f[2] * 0.0722);
      const z = g((f[0] * 0.0193 + f[1] * 0.1192 + f[2] * 0.9505) / 1.08883);
      return [116 * y - 16, 500 * (x - y), 200 * (y - z)];
    };
    const p = lab(a);
    const q = lab(b);
    return Math.hypot(p[0] - q[0], p[1] - q[1], p[2] - q[2]);
  };

  // mixColour is a + (b − a) × t, rounded.
  const mixColour = (a, b, t) => a.map((x, i) => Math.round(x + (b[i] - x) * t));

  // nudgeContrast moves c toward white on a dark bg, toward black on a
  // light one, in steps of a twentieth, until it reads at min against bg;
  // a colour that already reads is returned as it is.
  P.nudgeContrast = function (c, bg, min) {
    const to = luminance(bg) < 0.18 ? [255, 255, 255] : [0, 0, 0];
    let out = c;
    for (let step = 1; P.contrast(out, bg) < min && step <= 20; step++) out = mixColour(c, to, step / 20);
    return out;
  };

  // GRAPH_MARKS are a graph node's marks in legend order, each with the
  // colour token it is drawn in; FILL_SHARE is how much of its mark a
  // node's fill takes over the card, more for a ready or held epic so they
  // stand out, 0.32 for the rest.
  P.GRAPH_MARKS = [
    ["done", "--done"], ["in_progress", "--g-inprog"], ["specced", "--g-specced"], ["claimed", "--g-claimed"],
    ["ready", "--ready"], ["blocked", "--g-blocked"], ["held", "--held"], ["waiting", "--g-waiting"], ["moot", "--moot"],
  ];
  const FILL_SHARE = { ready: 0.5, held: 0.5, waiting: 0.55, moot: 0.55 };
  // foldAt is a list as the rows shown and the rows folded away: its first
  // n, the rest folded, unless only one row would be folded, which shows.
  P.foldAt = function (list, n) {
    return list.length <= n + 1 ? [list, []] : [list.slice(0, n), list.slice(n)];
  };
  // GRAPH_TOKENS are every token graphPalette reads.
  P.GRAPH_TOKENS = ["--sunk", "--surface", "--ink", "--muted", "--line", "--accent", "--crit", "--g-back", "--g-anc", "--g-desc", "--color-paper-3"]
    .concat(P.GRAPH_MARKS.map((m) => m[1]));

  // parseColour reads "#rrggbb" or "rgb(r, g, b)" as [r, g, b], grey for
  // anything else.
  const parseColour = (v) => {
    const s = String(v || "").trim();
    if (/^#[0-9a-f]{6}$/i.test(s)) return [1, 3, 5].map((i) => parseInt(s.slice(i, i + 2), 16));
    const m = /rgba?\(([^)]+)\)/.exec(s);
    return m ? m[1].split(",").slice(0, 3).map((x) => Math.round(Number(x))) : [128, 128, 128];
  };

  // graphPalette is the graph's colours from the page's tokens (name to
  // value): every status hue and edge colour nudged until it reads at 3:1
  // on the canvas, each node fill eased toward the card until the ink
  // reads on it at 4.5:1, tree edges in the ink and the canvas's own
  // colours as they are.
  P.graphPalette = function (tok) {
    const rgb = (name) => parseColour(tok[name]);
    const page = rgb("--sunk");
    const surface = rgb("--surface");
    const ink = rgb("--ink");
    const line = (name) => P.nudgeContrast(rgb(name), page, 3);
    const mark = {};
    const fill = {};
    P.GRAPH_MARKS.forEach(([k, name]) => {
      mark[k] = line(name);
      const f = mixColour(surface, mark[k], FILL_SHARE[k] || 0.32);
      let eased = f;
      for (let step = 1; P.contrast(ink, eased) < 4.5 && step <= 20; step++) eased = mixColour(f, surface, step / 20);
      fill[k] = eased;
    });
    return {
      dark: luminance(page) < 0.18, page, surface, ink, tree: ink, band: rgb("--color-paper-3"), line: rgb("--line"),
      muted: line("--muted"), accent: line("--accent"), crit: line("--crit"), back: line("--g-back"), anc: line("--g-anc"), desc: line("--g-desc"),
      mark, fill,
    };
  };

  // legendOpen says whether the graph's legend and keys card is open, from
  // the choice stored for this place ("1" open, "0" collapsed): without
  // one it is open on the page and collapsed in full screen, where it would
  // cover the graph.
  P.legendOpen = function (stored, fullscreen) {
    if (stored === "1") return true;
    if (stored === "0") return false;
    return !fullscreen;
  };

  // legendKey is where the legend card's choice is kept, one key for the
  // page and one for full screen.
  P.legendKey = function (fullscreen) {
    return fullscreen ? "tp-dashboard-legend-fs" : "tp-dashboard-legend";
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

  // addressURL is the address syncURL writes for the page's place, hash:
  // the fragment alone, relative, so it keeps the page's path, query and
  // origin. Prefixing location.pathname would turn a "//" path into a
  // protocol-relative URL naming another origin, which history refuses.
  P.addressURL = function (hash) {
    return "#" + hash;
  };

  // streamStep is what the page does with its event stream when the tab's
  // visibility changes: "close" a stream a hidden tab holds, so hidden tabs
  // keep none of the browser's six connections per host, "open" one for a
  // visible tab that has none, null otherwise.
  P.streamStep = function (hidden, open) {
    if (hidden && open) return "close";
    if (!hidden && !open) return "open";
    return null;
  };

  // LANGS are the page's languages, the first its fallback.
  P.LANGS = ["en", "tr"];

  // pickLang is the page's language: the stored choice when it is one of
  // LANGS, else Turkish for a browser language starting tr, else English.
  P.pickLang = function (stored, nav) {
    if (P.LANGS.includes(stored)) return stored;
    return /^tr(?:$|[-_])/i.test(String(nav || "")) ? "tr" : "en";
  };

  // PLURAL picks a text's form for a count: English has one and other;
  // Turkish has one form, since a noun after a number stays singular.
  const PLURAL = { en: (n) => (n === 1 ? "one" : "other"), tr: () => "other" };

  // translator is t(key, params) over dict, one language's texts by key, for
  // locale: the key's text with each {name} slot filled from params, a
  // number formatted for the locale; a text given as { one, other } takes
  // the form params.n asks for. A key dict lacks reads as the key. t.html
  // escapes the values it fills, keeping the text's own markup; t.num writes
  // a number, t.pct a share as a percent and t.dur minutes as minutes or
  // hours, each as the locale writes it.
  P.translator = function (dict, locale) {
    const nf = new Intl.NumberFormat(locale);
    const plural = PLURAL[locale] || PLURAL.en;
    const pick = (key, params) => {
      const v = dict[key];
      if (v === undefined) return key;
      if (typeof v === "string") return v;
      if (Array.isArray(v)) return v.join(" ");
      return v[plural(params && params.n)] || v.other;
    };
    const fill = (text, params, wrap) => text.replace(/\{(\w+)\}/g, (slot, name) => {
      if (!params || !(name in params)) return slot;
      const v = params[name];
      return wrap(typeof v === "number" ? nf.format(v) : String(v));
    });
    const t = (key, params) => fill(pick(key, params), params, String);
    t.html = (key, params) => fill(pick(key, params), params, escText);
    // t.list words a key given as an array of strings as its bullets, each
    // markup as t.html makes it; any other key is a list of one.
    t.list = (key, params) => (Array.isArray(dict[key]) ? dict[key] : [pick(key, params)]).map((s) => fill(s, params, escText));
    t.locale = locale;
    t.num = (n) => nf.format(n);
    t.pct = (share) => new Intl.NumberFormat(locale, { style: "percent" }).format(share);
    t.dur = (minutes) => (minutes < 60
      ? new Intl.NumberFormat(locale, { style: "unit", unit: "minute", unitDisplay: "short" }).format(minutes)
      : new Intl.NumberFormat(locale, { style: "unit", unit: "hour", unitDisplay: "short", maximumFractionDigits: 1 }).format(minutes / 60));
    return t;
  };

  if (typeof module === "object" && module.exports) module.exports = P;
  else root.tpPure = P;
})(this);
