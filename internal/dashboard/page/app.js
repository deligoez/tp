// tp dashboard's page (spec/1.8.0.md §2, §3). It draws what tp pushes and
// computes no figure a tp command reports: every count and state below
// arrives computed in a payload.
(function () {
  "use strict";

  // The event stream and the payloads it pushes, by event name (§2).
  const STREAM = "/api/events";
  const EVENTS = ["epic-export", "export-list", "summary"];
  const SOURCE_LABEL = {
    "epic-export": "The epic graph (tp epic export)",
    "export-list": "The task file list (tp export --list)",
    "summary": "The dashboard summary",
  };

  // The three tabs, in the lifecycle's order (§3).
  const TABS = ["epics", "specs", "work"];
  const STAGES = [["planned", "Planned"], ["claimed", "Claimed"], ["specced", "Specced"], ["in_progress", "In progress"], ["done", "Done"]];
  const PHASES = [["review", "Review"], ["decompose", "Decompose"], ["implement", "Implement"], ["audit", "Audit"], ["release", "Release"]];

  const $ = (id) => document.getElementById(id);
  const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
  const plural = (n, one, many) => n + " " + (n === 1 ? one : many);

  // live holds, per event, the last good payload and the error pushed after
  // it, if any; graph records whether tp pushed the epic export at all.
  const live = {};
  EVENTS.forEach((name) => { live[name] = { data: null, error: null }; });
  const state = { graph: false, tab: null };

  // panels draw the tabs: each names the payloads it reads and draws from
  // their last good state. A later view registers its own with panel().
  const panels = [];
  function panel(sources, draw) { panels.push({ sources, draw }); }
  const data = (name) => live[name].data;

  // isError tells tp's error, as the stream pushes it, from a payload.
  function isError(p) {
    return p !== null && typeof p === "object" && typeof p.error === "string" && "hint" in p;
  }

  function receive(name, payload) {
    const entry = live[name];
    if (isError(payload)) {
      entry.error = payload;
    } else {
      entry.data = payload;
      entry.error = null;
    }
    if (name === "epic-export") state.graph = true;
    // The server replays the current state in derivation order, the graph's
    // export before the list, so the list's arrival settles where to open.
    if (name === "export-list" && state.tab === null) setTab(state.graph ? "epics" : "specs");
    $("pushed").textContent = "Updated " + new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
    render();
  }

  function setTab(tab) {
    state.tab = tab;
    document.body.dataset.tab = tab;
    $("loading").hidden = true;
    TABS.forEach((t) => {
      $("tab-" + t).setAttribute("aria-selected", String(t === tab));
      $("view-" + t).hidden = t !== tab;
    });
  }

  function render() {
    renderBanners();
    panels.forEach((p) => p.draw());
    document.querySelectorAll("[data-src]").forEach((el) => {
      const stale = el.dataset.src.split(" ").some((name) => live[name] && live[name].error && live[name].data);
      el.classList.toggle("stale", stale);
      el.setAttribute("aria-disabled", String(stale));
    });
  }

  // renderBanners shows each failed derivation's error and hint, and says
  // when the page keeps the last good state under it.
  function renderBanners() {
    $("banners").innerHTML = EVENTS.filter((name) => live[name].error).map((name) => {
      const e = live[name].error;
      const kept = live[name].data ? '<span class="kept">Showing the last good state, greyed, until tp derives it again.</span>' : "";
      return '<div class="banner"><b>' + esc(SOURCE_LABEL[name]) + " could not be derived</b><span>" + esc(e.error) + "</span>" +
        (e.hint ? '<span class="hint">' + esc(e.hint) + "</span>" : "") + kept + "</div>";
    }).join("");
  }

  // The tab labels name what each tab holds, with its unit.
  panel(["epic-export", "export-list"], () => {
    const g = data("epic-export");
    $("c-epics").textContent = g && Array.isArray(g.epics) ? plural(g.epics.length, "epic", "epics") : state.tab && !state.graph ? "no graph" : "";
    const list = data("export-list");
    $("c-specs").textContent = list && Array.isArray(list.files) ? plural(list.files.length, "task file", "task files") : "";
  });

  // With no graph the Epics tab says so and names the command that starts one.
  panel(["epic-export"], () => { $("epics-none").hidden = state.graph || state.tab === null; });

  // The lifecycle band (§6): epic stages when the phases count epics, and the
  // spec phases, which count task files when the repository has no graph.
  panel(["summary"], () => {
    const sum = data("summary");
    if (!sum || !sum.band) {
      $("band").innerHTML = "";
      return;
    }
    const band = sum.band;
    const cell = (label, n) => '<div class="pcell"><b class="num">' + esc(n) + '</b><span class="pl">' + esc(label) + "</span></div>";
    const lane = (title, axis, counts) => '<div class="plane"><span class="plab">' + esc(title) + '</span><div class="pcells">' +
      axis.map(([k, label]) => cell(label, counts[k] || 0)).join("") + "</div></div>";
    const byEpics = band.phases_of === "epics";
    $("band").innerHTML = (byEpics ? lane("Epics", STAGES, band.stages || {}) : "") +
      lane(byEpics ? "Epics by spec phase" : "Task files by phase", PHASES, band.phases || {});
  });

  // What each panel and figure lists and where it comes from (§4).
  const INFO = {
    band: "Work epics per stage, and per spec phase at the earliest phase of their bound specs; with no graph the phases count task files. From the summary's band.",
    share: "Work epics done over all work epics; condition epics are in no figure. From the summary's plan, the share tp epic status reports.",
    ready: "Work epics that can start now: open milestone, every dependency done, no bound spec, no claim. In tp epic ready's order, most downstream first.",
    critical: "Not-done epics on the longest dependency chain of the whole graph: tp epic status's top-level critical_path.",
    blocked: "Work epics waiting on a dependency that is not done. Held epics are the blocked ones a condition holds.",
    held: "Blocked work epics a condition epic, or a group of conditions, holds: they wait on an operator decision, not on work.",
    operator: "Decisions only the operator takes: actionable conditions (tp epic done), gates whose written threshold awaits --met, and placeholder thresholds to rewrite. From the summary's plan and the export's milestones[].operator.",
    loops: "Epic review slices whose loop is not done: open, stale, at its cap or with a round in flight. From the export's slices.",
    lint: "The error and warning counts tp epic lint prints for the graph, from the export's lint.",
    throughput: "Work epics done per ISO week across every milestone, from their done time. From the summary's plan.",
    waiting: "Work epics waiting on a dependency that is not done, each with what it waits on; deferrable gate members are left out.",
    most: "Not-done work epics other work epics depend on, transitively, most dependents first.",
  };

  // The ⓘ buttons open one note at a time beside themselves.
  let openInfo = null;
  function closeInfo() {
    if (openInfo) openInfo.setAttribute("aria-expanded", "false");
    openInfo = null;
    $("pop").hidden = true;
  }
  document.addEventListener("click", (ev) => {
    const btn = ev.target.closest && ev.target.closest(".info");
    if (!btn) {
      if (!(ev.target.closest && ev.target.closest("#pop"))) closeInfo();
      return;
    }
    if (openInfo === btn) {
      closeInfo();
      return;
    }
    closeInfo();
    const pop = $("pop");
    pop.textContent = INFO[btn.dataset.info] || "";
    pop.hidden = false;
    const r = btn.getBoundingClientRect();
    pop.style.top = (r.bottom + window.scrollY + 6) + "px";
    pop.style.left = Math.max(16, Math.min(r.left + window.scrollX - 8, document.documentElement.clientWidth - pop.offsetWidth - 16)) + "px";
    btn.setAttribute("aria-expanded", "true");
    openInfo = btn;
  });
  document.addEventListener("keydown", (ev) => { if (ev.key === "Escape") closeInfo(); });

  const info = (key, label) => '<button class="info" type="button" data-info="' + key + '" aria-label="About ' + esc(label) + '" aria-expanded="false">i</button>';
  const kpi = (key, value, label, cls) => '<div class="card kpi' + (cls ? " " + cls : "") + '">' + info(key, label) +
    '<span class="v">' + value + '</span><span class="l">' + esc(label) + "</span></div>";

  // The plan as the page reads it: the summary's plan and the export's
  // epics by id, or null while either is missing.
  function planView() {
    const sum = data("summary");
    const g = data("epic-export");
    if (!state.graph || !sum || !sum.plan || !g) return null;
    const epics = {};
    (g.epics || []).forEach((e) => { epics[e.id] = e; });
    return { plan: sum.plan, g, epics };
  }

  // The KPI strip opens the Epics tab (§4); every figure arrives computed.
  panel(["summary", "epic-export"], () => {
    const v = planView();
    $("kpis").hidden = !v;
    if (!v) return;
    const f = v.plan.figures;
    const lint = v.g.lint || { errors: 0, warnings: 0 };
    const weeks = f.throughput || [];
    const last = weeks.length ? weeks[weeks.length - 1] : null;
    $("kpis").innerHTML = [
      kpi("share", Math.round(f.share_done * 100) + "%<small> " + esc(f.done) + " of " + esc(f.work) + "</small>", "share of work epics done"),
      kpi("ready", esc(f.ready), "ready epics"),
      kpi("critical", esc(f.critical_path), "epics on the critical path", "crit"),
      kpi("blocked", esc(f.blocked), "blocked epics", f.blocked ? "warn" : ""),
      kpi("held", esc(f.held), "held epics", f.held ? "op" : ""),
      kpi("operator", esc(f.operator_queue), "operator decisions waiting", f.operator_queue ? "op" : ""),
      kpi("loops", esc(f.open_loops), "open review loops"),
      kpi("lint", esc(lint.errors) + "<small> / " + esc(lint.warnings) + "</small>", "lint errors / warnings", lint.errors ? "warn" : ""),
      kpi("throughput", last ? esc(last.done) + "<small> in " + esc(last.week) + "</small>" : "0", "epics done per week").replace("</div>", spark(weeks) + "</div>"),
    ].join("");
  });

  // spark draws the weekly throughput as bars, scaled to the busiest week.
  function spark(weeks) {
    if (!weeks.length) return "";
    const top = Math.max(...weeks.map((w) => w.done));
    const w = 100 / weeks.length;
    return '<svg viewBox="0 0 100 32" preserveAspectRatio="none" role="img" aria-label="Epics done per week">' + weeks.map((x, i) => {
      const h = Math.max(2, (x.done / top) * 30);
      return '<rect x="' + (i * w + w * 0.15) + '" y="' + (32 - h) + '" width="' + w * 0.7 + '" height="' + h + '"><title>' + esc(x.week + ": " + x.done) + "</title></rect>";
    }).join("") + "</svg>";
  }

  const row = (id, title, meta, why) => '<div class="row"><span class="id">' + esc(id) + '</span><span class="t">' + esc(title) +
    '</span><span class="m">' + esc(meta || "") + "</span>" + (why ? '<span class="why">' + why + "</span>" : "") + "</div>";
  const none = (text) => '<p class="empty">' + esc(text) + "</p>";
  const titleOf = (v, id) => (v.epics[id] ? v.epics[id].title : "");
  const dependents = (v, id) => {
    const d = v.epics[id] && v.epics[id].downstream;
    return d ? plural(d.dependents, "epic waits", "epics wait") + " on it" : "";
  };

  // The now lists (§4): what can start, what waits on the operator, what
  // waits on other work, and what the most work waits on.
  panel(["summary", "epic-export"], () => {
    const v = planView();
    $("now").hidden = !v;
    if (!v) return;
    $("now-ready").innerHTML = v.plan.ready.map((id) => row(id, titleOf(v, id), dependents(v, id))).join("") || none("No epic can start now.");
    const ops = v.plan.operator_conditions.map((id) => row(id, titleOf(v, id), "condition", "<code>tp epic done " + esc(id) + ' --evidence "&lt;text or URL&gt;"</code>'));
    (v.g.milestones || []).forEach((m) => {
      const o = m.operator || {};
      (o.gates || []).forEach((gate) => ops.push(row(gate.milestone, gate.threshold, "gate to measure", "<code>tp epic gate " + esc(gate.milestone) + ' --met --evidence "&lt;text or URL&gt;"</code>')));
      (o.threshold_rewrites || []).forEach((r) => ops.push(row(r.milestone, r.threshold, "threshold to write", "<code>" + esc(r.command) + "</code>")));
    });
    $("now-operator").innerHTML = ops.join("") || none("Nothing waits on the operator.");
    $("now-waiting").innerHTML = v.plan.waiting.map((id) => {
      const on = (v.epics[id] && v.epics[id].blocked_by) || [];
      return row(id, titleOf(v, id), v.epics[id] ? v.epics[id].milestone : "", "waits on " + on.map((x) => "<code>" + esc(x) + "</code>").join(", "));
    }).join("") || none("No epic waits on another.");
    $("now-most").innerHTML = v.plan.most_waited_on.slice(0, 10).map((id) => row(id, titleOf(v, id), dependents(v, id))).join("") || none("No epic is waited on.");
  });

  INFO.slices = "One row per epic review slice (tp epic review): its round with the cap and the grants spent, its state, the units changed since its last recorded round when it is stale, its open findings by role, and the findings of each recorded round as a curve. From the export's slices and the summary's per-round counts.";

  // sliceLabel names a slice as tp epic review takes it.
  function sliceLabel(s) {
    const k = s.key;
    if (!k) return s.dir;
    if (k.kind === "epics") return "--epics " + (k.ids || []).join(",") + (k.name ? " --name " + k.name : "");
    return "--" + k.kind + " " + k.value;
  }

  // roundText says the round against the cap and the grants spent.
  function roundText(s) {
    let t = "round " + s.round;
    if (s.max_rounds !== null && s.max_rounds !== undefined) t += ", cap " + s.max_rounds;
    if (s.grants_spent) t += " + " + plural(s.grants_spent, "grant", "grants");
    return t;
  }

  // curve draws each recorded round's finding count, so a converging slice
  // reads as a falling line.
  function curve(rounds) {
    if (!rounds || !rounds.length) return "";
    const top = Math.max(1, ...rounds.map((r) => r.total));
    const step = rounds.length > 1 ? 88 / (rounds.length - 1) : 0;
    const pts = rounds.map((r, i) => [4 + i * step, 21 - (r.total / top) * 18]);
    const label = rounds.map((r) => "round " + r.round + ": " + r.total).join(", ");
    return '<svg class="curve" viewBox="0 0 96 24" role="img" aria-label="Findings per round: ' + esc(label) + '"><title>' + esc(label) + "</title>" +
      (pts.length > 1 ? '<polyline points="' + pts.map((p) => p.join(",")).join(" ") + '"/>' : "") +
      pts.map((p) => '<circle cx="' + p[0] + '" cy="' + p[1] + '" r="2"/>').join("") + "</svg>";
  }

  // Plan review (§5): one row per slice. It follows the KPI strip while no
  // epic has a bound spec, a planning-stage project's work (§3), and closes
  // the tab in §4's order once one does.
  panel(["epic-export", "summary"], () => {
    const g = data("epic-export");
    const sum = data("summary");
    const box = $("plan-review");
    box.hidden = !state.graph || !g;
    if (box.hidden) return;
    const bound = (g.epics || []).some((e) => e.files && e.files.length);
    if (bound) $("view-epics").appendChild(box);
    else $("kpis").after(box);
    const slices = g.slices || [];
    if (!slices.length) {
      $("slices").innerHTML = '<p class="empty">No slice of the plan has been reviewed: <code>tp epic review --milestone &lt;m&gt;</code> starts one.</p>';
      return;
    }
    $("slices").innerHTML = '<table class="loops"><thead><tr><th>Slice</th><th>Round</th><th>State</th><th>Open findings by role</th><th>Findings per round</th></tr></thead><tbody>' +
      slices.map((s) => {
        const stale = s.state === "stale" && s.stale_units && s.stale_units.length ? ' title="Changed since round ' + esc(s.round) + ": " + esc(s.stale_units.join(", ")) + '"' : "";
        const open = Object.keys(s.open_by_role || {}).sort().map((r) => esc(r) + " " + esc(s.open_by_role[r])).join(", ") || "none";
        return '<tr><td class="k">' + esc(sliceLabel(s)) + "</td><td>" + esc(roundText(s)) + '</td><td><span class="st" data-s="' + esc(s.state) + '"' + stale + ">" +
          esc(s.state.replace("_", " ")) + '</span></td><td class="n">' + open + "</td><td>" + curve(sum ? sum.rounds[s.dir] : null) + "</td></tr>";
      }).join("") + "</tbody></table>";
  });

  // ---------- the plan map (§9) ----------
  // tp lays the map out: every epic's layout names its lane, the lane's kind,
  // its milestone's column and its row inside the milestone. The page turns
  // those into coordinates and places nothing by any rule of its own.
  const MAP = { boxW: 178, rowH: 18, head: 34, padB: 12, colGap: 40, slotGap: 14, laneGap: 22, left: 92, top: 14 };
  const LANE_LABEL = { main: "Main", parallel: "Parallel", triggered: "Triggered" };
  const MARK_LABEL = { done: "done", ready: "can start", active: "claimed or under way", blocked: "blocked", held: "held by a condition", waiting: "milestone closed", moot: "moot" };
  const MARKS = ["done", "ready", "active", "blocked", "held", "waiting", "moot"];
  const mapView = { selected: null, edges: false, tracks: new Set(), geo: null, deps: {}, dependents: {}, byId: {} };
  INFO.map = "Lanes from the opening order: the main lane is the longest chain of milestone openings, parallel lanes the other chains, the triggered lane the milestones that open on a trigger. A milestone's column is its opening depth. tp computes every place (the export's layout); the page only draws it. Colour and shape both carry an epic's state; a diamond is a condition. The orange line is the critical path. The bar on a box's right edge is its gate's members done. Click an epic for its chain; All edges draws every edge.";

  // mapGeometry turns the export's layout into boxes and points. Milestones
  // sharing a lane and a column (the triggered lane's) sit side by side in
  // graph-file order.
  function mapGeometry(g) {
    const placed = (g.epics || []).filter((e) => e.layout);
    const order = [].concat(...((g.graph && g.graph.milestones) || []));
    const open = {};
    (g.milestones || []).forEach((m) => { open[m.milestone] = m; });
    const byMs = {};
    placed.forEach((e) => {
      const l = e.layout;
      const m = byMs[e.milestone] || (byMs[e.milestone] = { id: e.milestone, lane: l.lane, kind: l.lane_kind, column: l.column, rows: 0, report: open[e.milestone] || null });
      m.rows = Math.max(m.rows, l.row + 1);
    });
    const ms = Object.values(byMs).sort((a, b) => order.indexOf(a.id) - order.indexOf(b.id));
    const taken = {};
    const lanes = [];
    let cols = 0;
    ms.forEach((m) => {
      const k = m.lane + "/" + m.column;
      m.slot = taken[k] = taken[k] === undefined ? 0 : taken[k] + 1;
      cols = Math.max(cols, m.column + 1);
      const lane = lanes[m.lane] || (lanes[m.lane] = { kind: m.kind, rows: 0 });
      lane.rows = Math.max(lane.rows, m.rows);
    });
    const colX = [];
    for (let c = 0; c < cols; c++) colX[c] = MAP.left + c * (MAP.boxW + MAP.colGap);
    let x = MAP.left + cols * (MAP.boxW + MAP.colGap);
    let y = MAP.top;
    lanes.forEach((lane) => {
      lane.y = y;
      lane.h = MAP.head + lane.rows * MAP.rowH + MAP.padB;
      y += lane.h + MAP.laneGap;
    });
    // Only the triggered lane holds milestones of one column; they run on to
    // the right inside their own lane, so they widen no other lane's column.
    ms.forEach((m) => {
      m.x = colX[m.column] + m.slot * (MAP.boxW + MAP.slotGap);
      m.y = lanes[m.lane].y;
      m.h = lanes[m.lane].h;
      x = Math.max(x, m.x + MAP.boxW + MAP.colGap);
    });
    const pos = {};
    placed.forEach((e) => {
      const m = byMs[e.milestone];
      pos[e.id] = { x: m.x + 16, y: m.y + MAP.head + e.layout.row * MAP.rowH + MAP.rowH / 2 };
    });
    return { ms, byMs, lanes, pos, placed, W: x, H: y };
  }

  // markState reads an epic's mark from what the export says of it.
  function markState(e, ms) {
    if (e.status === "done") return "done";
    if (e.moot) return "moot";
    if (e.held) return "held";
    if (e.blocked) return "blocked";
    if (ms && ms.report && !ms.report.open) return "waiting";
    if (e.status !== "planned") return "active";
    return "ready";
  }

  // mark draws one state: a circle for a work epic, a diamond for a
  // condition, and a shape inside for every state, so colour is never alone.
  function mark(cond, s, x, y, r) {
    const c = "var(--" + s + ")";
    const shape = (fill, stroke, dash) => (cond
      ? '<path d="M' + x + " " + (y - r - 1) + "L" + (x + r + 1) + " " + y + "L" + x + " " + (y + r + 1) + "L" + (x - r - 1) + " " + y + 'Z"'
      : '<circle cx="' + x + '" cy="' + y + '" r="' + r + '"') +
      ' fill="' + fill + '" stroke="' + stroke + '" stroke-width="1.5"' + (dash ? ' stroke-dasharray="' + dash + '"' : "") + "/>";
    switch (s) {
      case "done": return shape(c, c);
      case "ready": return '<circle cx="' + x + '" cy="' + y + '" r="' + (r + 2.8) + '" fill="none" stroke="' + c + '" stroke-width="1" opacity=".55"/>' + shape(c, c);
      case "active": return shape("var(--surface)", c) + '<circle cx="' + x + '" cy="' + y + '" r="' + (r - 2.3) + '" fill="' + c + '"/>';
      case "blocked": return shape(c, c) + '<path d="M' + (x - r + 1.6) + " " + (y + r - 1.6) + "L" + (x + r - 1.6) + " " + (y - r + 1.6) + '" stroke="var(--surface)" stroke-width="1.6"/>';
      case "held": return shape(c, c) + '<rect x="' + (x - 2.3) + '" y="' + (y - 2.5) + '" width="1.5" height="5" fill="var(--surface)"/><rect x="' + (x + 0.8) + '" y="' + (y - 2.5) + '" width="1.5" height="5" fill="var(--surface)"/>';
      case "moot": return shape("none", "var(--moot)", "1.2 1.6");
      default: return shape("var(--surface)", "var(--waiting)", "2 1.5");
    }
  }

  // The graph's edges, both ways, as stored: an any-of group names each member.
  function mapEdges(g) {
    const deps = {};
    const dependents = {};
    ((g.graph && g.graph.epics) || []).forEach((e) => {
      deps[e.id] = [];
      (e.depends_on || []).forEach((d) => {
        (d.any_of || [d.id]).forEach((x) => {
          deps[e.id].push({ id: x, why: d.why, group: !!d.any_of });
          (dependents[x] = dependents[x] || []).push(e.id);
        });
      });
    });
    mapView.deps = deps;
    mapView.dependents = dependents;
  }

  // chainOf is every epic the selected one depends on, and every epic that
  // depends on it, transitively.
  function chainOf(id) {
    const seen = new Set([id]);
    const walk = (x, next) => (next[x] || []).forEach((y) => {
      const yid = y.id || y;
      if (!seen.has(yid)) { seen.add(yid); walk(yid, next); }
    });
    walk(id, mapView.deps);
    walk(id, mapView.dependents);
    return seen;
  }

  function edgePath(p, q) {
    const dx = Math.max(18, (q.x - p.x) / 2);
    return "M" + p.x + " " + p.y + "C" + (p.x + dx) + " " + p.y + " " + (q.x - dx) + " " + q.y + " " + q.x + " " + q.y;
  }

  // The track chips filter the map; each track is one list of the graph's
  // milestones, named by its first and last.
  function drawTrackChips(g) {
    const tracks = (g.graph && g.graph.milestones) || [];
    const key = JSON.stringify(tracks);
    const box = $("trackchips");
    if (box.dataset.key === key) return;
    box.dataset.key = key;
    mapView.tracks = new Set();
    box.innerHTML = tracks.length > 1 ? tracks.map((t, i) => '<button class="chip" type="button" data-track="' + i + '" aria-pressed="false" title="' +
      esc(t.join(", ")) + '">' + esc(t.length > 1 ? t[0] + "–" + t[t.length - 1] : t[0] || "track " + (i + 1)) + "</button>").join("") : "";
  }

  function drawMap() {
    const g = data("epic-export");
    const card = $("mapcard");
    card.hidden = !state.graph || !g;
    if (card.hidden) return;
    drawTrackChips(g);
    mapEdges(g);
    const geo = mapView.geo = mapGeometry(g);
    mapView.byId = {};
    geo.placed.forEach((e) => { mapView.byId[e.id] = e; });
    if (mapView.selected && !geo.pos[mapView.selected]) mapView.selected = null;
    const sel = mapView.selected;
    const chain = sel ? chainOf(sel) : null;
    const trackOf = {};
    ((g.graph && g.graph.milestones) || []).forEach((t, i) => t.forEach((m) => { trackOf[m] = i; }));
    const shown = (e) => (!mapView.tracks.size || mapView.tracks.has(trackOf[e.milestone])) && (!chain || chain.has(e.id));
    const out = [];
    geo.lanes.forEach((lane) => out.push('<text class="lanelab" x="12" y="' + (lane.y + 22) + '">' + esc(LANE_LABEL[lane.kind] || lane.kind) + "</text>"));
    geo.ms.forEach((m) => {
      const r = m.report || {};
      const st = r.state || {};
      const share = st.members ? st.members_done / st.members : 0;
      const ih = m.h - MAP.head - MAP.padB;
      const label = r.open === false ? "closed" : Math.round((r.share_done || 0) * 100) + "%";
      out.push('<g><rect class="msbox' + (r.open === false ? " closed" : "") + '" x="' + m.x + '" y="' + m.y + '" width="' + MAP.boxW + '" height="' + m.h + '" rx="7"/>' +
        '<text class="msname" x="' + (m.x + 12) + '" y="' + (m.y + 21) + '">' + esc(m.id) + "</text>" +
        '<text class="msshare" x="' + (m.x + MAP.boxW - 15) + '" y="' + (m.y + 21) + '" text-anchor="end">' + esc(label) + "</text>" +
        '<rect x="' + (m.x + MAP.boxW - 7.5) + '" y="' + (m.y + MAP.head) + '" width="4" height="' + ih + '" rx="2" fill="var(--line)"/>' +
        '<rect x="' + (m.x + MAP.boxW - 7.5) + '" y="' + (m.y + MAP.head + ih * (1 - share)) + '" width="4" height="' + ih * share + '" rx="2" fill="var(--' +
        (st.members && st.members_done === st.members ? "done" : r.open ? "active" : "waiting") + ')"><title>' +
        esc(m.id + " gate: " + (st.members_done || 0) + " of " + (st.members || 0) + " members done") + "</title></rect></g>");
    });
    const edges = [];
    if (mapView.edges || sel) {
      Object.keys(mapView.deps).forEach((id) => mapView.deps[id].forEach((d) => {
        const p = geo.pos[d.id];
        const q = geo.pos[id];
        if (!p || !q || (chain && !(chain.has(id) && chain.has(d.id)))) return;
        edges.push('<path class="edge' + (sel ? " sel" : "") + '" d="' + edgePath(p, q) + '"' + (d.group ? ' stroke-dasharray="3 2"' : "") + "/>");
      }));
    }
    out.push("<g>" + edges.join("") + "</g>");
    const crit = ((g.critical_path && g.critical_path.by_count) || []).filter((id) => geo.pos[id]);
    if (crit.length > 1) out.push('<polyline class="crit" points="' + crit.map((id) => geo.pos[id].x + "," + geo.pos[id].y).join(" ") + '"/>');
    geo.placed.forEach((e) => {
      const p = geo.pos[e.id];
      const s = markState(e, geo.byMs[e.milestone]);
      out.push('<g class="node' + (shown(e) ? "" : " dim") + '" data-id="' + esc(e.id) + '">' +
        (e.id === sel ? '<circle cx="' + p.x + '" cy="' + p.y + '" r="10" fill="none" stroke="var(--accent)" stroke-width="2.2"/>' : "") +
        mark(e.condition, s, p.x, p.y, 4.8) + '<text class="eid" x="' + (p.x + 12) + '" y="' + (p.y + 4) + '">' + esc(clip(e.id, 19)) + "</text>" +
        '<rect x="' + (p.x - 9) + '" y="' + (p.y - MAP.rowH / 2) + '" width="' + (MAP.boxW - 24) + '" height="' + MAP.rowH + '" fill="transparent"/></g>');
    });
    $("mapsvg").innerHTML = out.join("");
    mapPort.bounds(geo.W, geo.H);
  }
  const clip = (s, n) => (s.length > n ? s.slice(0, n - 1) + "…" : s);
  panel(["epic-export"], drawMap);

  // mapPort pans and zooms the map: drag to pan, Ctrl or ⌘ with the wheel (a
  // pinch) to zoom; a plain wheel scrolls the page.
  const mapPort = (function () {
    const el = $("map");
    const svg = $("mapsvg");
    let v = null;
    let size = { W: 1, H: 1 };
    let drag = null;
    let moved = false;
    const apply = () => { if (v) svg.setAttribute("viewBox", v.x + " " + v.y + " " + v.w + " " + v.h); };
    function fit() {
      const r = el.getBoundingClientRect();
      if (!r.width || !r.height) return;
      const ar = r.width / r.height;
      const W = size.W + 10;
      const H = size.H + 10;
      v = W / H > ar ? { x: -5, y: -5, w: W, h: W / ar } : { x: -5, y: -5, w: H * ar, h: H };
      apply();
    }
    function zoomAt(f, px, py) {
      if (!v) return;
      const nw = Math.min(Math.max(size.W, size.H) * 3, Math.max(120, v.w * f));
      f = nw / v.w;
      v = { x: px - (px - v.x) * f, y: py - (py - v.y) * f, w: v.w * f, h: v.h * f };
      apply();
    }
    const toSvg = (cx, cy) => {
      const r = el.getBoundingClientRect();
      return { x: v.x + (cx - r.left) / r.width * v.w, y: v.y + (cy - r.top) / r.height * v.h };
    };
    el.addEventListener("wheel", (ev) => {
      if (!(ev.ctrlKey || ev.metaKey) || !v) return;
      ev.preventDefault();
      const p = toSvg(ev.clientX, ev.clientY);
      zoomAt(Math.exp(Math.max(-80, Math.min(80, ev.deltaY)) * 0.01), p.x, p.y);
    }, { passive: false });
    el.addEventListener("pointerdown", (ev) => {
      if (ev.button !== 0 || !v || ev.pointerType === "touch") return;
      drag = { x: ev.clientX, y: ev.clientY, v: { ...v } };
      moved = false;
    });
    el.addEventListener("pointermove", (ev) => {
      if (drag && (moved || Math.abs(ev.clientX - drag.x) + Math.abs(ev.clientY - drag.y) > 3)) {
        if (!moved) { moved = true; el.classList.add("drag"); el.setPointerCapture(ev.pointerId); }
        const r = el.getBoundingClientRect();
        v = { ...drag.v, x: drag.v.x - (ev.clientX - drag.x) / r.width * drag.v.w, y: drag.v.y - (ev.clientY - drag.y) / r.height * drag.v.h };
        apply();
        $("tip").hidden = true;
        return;
      }
      mapHover(ev);
    });
    el.addEventListener("pointerup", (ev) => {
      el.classList.remove("drag");
      const wasDrag = drag && moved;
      drag = null;
      if (!wasDrag) mapClick(ev);
    });
    el.addEventListener("pointerleave", () => { $("tip").hidden = true; });
    $("mapcard").querySelectorAll("[data-z]").forEach((b) => b.addEventListener("click", () => {
      const z = b.dataset.z;
      if (z === "fit" || !v) fit();
      else zoomAt(z === "in" ? 1 / 1.5 : 1.5, v.x + v.w / 2, v.y + v.h / 2);
    }));
    new ResizeObserver(() => {
      const r = el.getBoundingClientRect();
      if (!r.width || !r.height) return;
      if (!v) fit();
      else { v.h = v.w * r.height / r.width; apply(); }
    }).observe(el);
    return {
      // bounds takes the drawing's size; the first drawing is fitted, later
      // ones keep the reader's view.
      bounds(W, H) {
        const first = size.W === 1 && size.H === 1;
        size = { W, H };
        if (first || !v) fit();
        else apply();
      },
    };
  })();

  const nodeAt = (ev) => {
    const el = document.elementFromPoint(ev.clientX, ev.clientY);
    return el && el.closest ? el.closest("#mapsvg .node") : null;
  };
  function mapHover(ev) {
    const n = nodeAt(ev);
    const tip = $("tip");
    const e = n && mapView.byId[n.dataset.id];
    if (!e) { tip.hidden = true; return; }
    const s = markState(e, mapView.geo.byMs[e.milestone]);
    tip.innerHTML = '<span class="tid">' + esc(e.id) + ", milestone " + esc(e.milestone) + (e.condition ? ", condition" : "") + "</span><br>" + esc(e.title) +
      "<br>" + esc(MARK_LABEL[s]) + ", " + esc(e.status.replace("_", " "));
    tip.hidden = false;
    tip.style.left = Math.min(ev.clientX + 14, window.innerWidth - 310) + "px";
    tip.style.top = (ev.clientY + 14) + "px";
  }
  function mapClick(ev) {
    const n = nodeAt(ev);
    const id = n ? n.dataset.id : null;
    mapView.selected = id && id !== mapView.selected ? id : null;
    drawMap();
  }
  $("trackchips").addEventListener("click", (ev) => {
    const b = ev.target.closest("[data-track]");
    if (!b) return;
    const i = Number(b.dataset.track);
    if (mapView.tracks.has(i)) mapView.tracks.delete(i);
    else mapView.tracks.add(i);
    b.setAttribute("aria-pressed", String(mapView.tracks.has(i)));
    drawMap();
  });
  $("f-edges").addEventListener("click", () => {
    mapView.edges = !mapView.edges;
    $("f-edges").setAttribute("aria-pressed", String(mapView.edges));
    drawMap();
  });
  $("maplegend").innerHTML = MARKS.map((s) => '<span><svg viewBox="0 0 15 15" aria-hidden="true">' + mark(false, s, 7.5, 7.5, 4.6) + "</svg>" + esc(MARK_LABEL[s]) + "</span>").join("") +
    '<span><svg viewBox="0 0 15 15" aria-hidden="true">' + mark(true, "ready", 7.5, 7.5, 4.4) + "</svg>condition</span>" +
    '<span><svg viewBox="0 0 15 15" aria-hidden="true"><path d="M1 7.5H14" stroke="var(--crit)" stroke-width="2.6"/></svg>critical path</span>' +
    '<span><svg viewBox="0 0 15 15" aria-hidden="true"><path d="M1 7.5H14" stroke="var(--accent)" stroke-dasharray="3 2"/></svg>one of a group</span>' +
    '<span class="help">Click an epic for its chain, again to clear. Drag to pan; Ctrl or ⌘ with the wheel, or a pinch, zooms.</span>';

  TABS.forEach((t) => { $("tab-" + t).addEventListener("click", () => setTab(t)); });

  // The stream: a page that reconnects gets the current state again (§2).
  const conn = $("conn");
  const source = new EventSource(STREAM);
  source.addEventListener("open", () => { conn.dataset.state = "live"; conn.textContent = "Live"; });
  source.addEventListener("error", () => { conn.dataset.state = "lost"; conn.textContent = "Reconnecting"; });
  EVENTS.forEach((name) => {
    source.addEventListener(name, (ev) => {
      let payload;
      try {
        payload = JSON.parse(ev.data);
      } catch (err) {
        payload = { error: "the " + name + " event is not JSON: " + err.message, hint: "report it: tp pushes JSON only" };
      }
      receive(name, payload);
    });
  });
})();
