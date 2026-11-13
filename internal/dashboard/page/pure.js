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

  // edgeDrawn says whether the graph card draws an edge l ({ role, critical,
  // any }) under view ({ showCross, hidden: the legend's hidden classes }).
  // An edge of the critical path is drawn whatever its class, so a view
  // with cross edges hidden never shows the path with holes; only hiding
  // the critical path itself hides it. Any other edge is hidden with its
  // class or its any-of group, and a cross edge waits for showCross.
  P.edgeDrawn = function (l, view) {
    if (l.critical) return !view.hidden.includes("critical");
    if (view.hidden.includes(l.role) || (l.any && view.hidden.includes("any"))) return false;
    return l.role !== "cross" || view.showCross;
  };

  // edgeLayers splits the edges edgeDrawn keeps into the card's four
  // layers, bottom to top: plain cross edges, any-of edges, tree and back
  // edges, and the critical path; drawn is how many it kept, each in one
  // layer, the count the card records.
  P.edgeLayers = function (links, view) {
    const out = { cross: [], any: [], tree: [], crit: [], drawn: 0 };
    links.forEach((l) => {
      if (!P.edgeDrawn(l, view)) return;
      out.drawn++;
      if (l.critical) out.crit.push(l);
      else if (l.any) out.any.push(l);
      else if (l.role === "cross") out.cross.push(l);
      else out.tree.push(l);
    });
    return out;
  };

  // GRAPH_MODES are the graph card's modes, the first its default.
  P.GRAPH_MODES = ["plan", "milestones", "scope", "neighbourhood"];

  // graphParams is the graph card's place s ({ mode, milestone, depth, pack,
  // showCross, q }) as address pairs, each only when it is not the default:
  // gv the mode, gm the gate's milestone in Gate closure, gd the
  // neighbourhood's depth, pack, cross and q the search.
  P.graphParams = function (s) {
    const out = [];
    if (s.mode !== "plan") out.push(["gv", s.mode]);
    if (s.mode === "scope" && s.milestone) out.push(["gm", s.milestone]);
    if (s.depth !== 2) out.push(["gd", String(s.depth)]);
    if (s.pack === "tree") out.push(["pack", "tree"]);
    if (s.showCross) out.push(["cross", "1"]);
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
      pack: get("pack") === "tree" ? "tree" : "rows",
      showCross: get("cross") === "1",
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
  // selection with hundreds of descendants keeps its near chain legible.
  P.chainAlpha = function (d) {
    return d <= 2 ? 255 : d === 3 ? 170 : 70;
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

  // wheelZooms says whether a wheel event zooms the graph: with Ctrl or ⌘
  // held, or with the plain wheel while the graph fills the screen.
  P.wheelZooms = function (ev, fullscreen) {
    return !!(ev.ctrlKey || ev.metaKey || fullscreen);
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
  // escapes the values it fills, keeping the text's own markup; t.pct writes
  // a share as a percent and t.dur minutes as minutes or hours, both as the
  // locale writes them.
  P.translator = function (dict, locale) {
    const nf = new Intl.NumberFormat(locale);
    const plural = PLURAL[locale] || PLURAL.en;
    const pick = (key, params) => {
      const v = dict[key];
      if (v === undefined) return key;
      if (typeof v === "string") return v;
      return v[plural(params && params.n)] || v.other;
    };
    const fill = (text, params, wrap) => text.replace(/\{(\w+)\}/g, (slot, name) => {
      if (!params || !(name in params)) return slot;
      const v = params[name];
      return wrap(typeof v === "number" ? nf.format(v) : String(v));
    });
    const t = (key, params) => fill(pick(key, params), params, String);
    t.html = (key, params) => fill(pick(key, params), params, escText);
    t.locale = locale;
    t.pct = (share) => new Intl.NumberFormat(locale, { style: "percent" }).format(share);
    t.dur = (minutes) => (minutes < 60
      ? new Intl.NumberFormat(locale, { style: "unit", unit: "minute", unitDisplay: "short" }).format(minutes)
      : new Intl.NumberFormat(locale, { style: "unit", unit: "hour", unitDisplay: "short", maximumFractionDigits: 1 }).format(minutes / 60));
    return t;
  };

  if (typeof module === "object" && module.exports) module.exports = P;
  else root.tpPure = P;
})(this);
