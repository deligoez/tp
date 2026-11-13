// tp dashboard's page (spec/1.8.0.md §2, §3). It draws what tp pushes and
// computes no figure a tp command reports: every count and state below
// arrives computed in a payload.
(function () {
  "use strict";

  // pure.js loads before this script and defines tpPure; without it the page
  // says so in its banner instead of failing on the first call and staying
  // blank.
  if (!window.tpPure) {
    document.getElementById("banners").innerHTML = '<div class="banner"><b>pure.js did not load</b><span>Reload the page; if it stays, check that tp dashboard is still running.</span></div>';
    return;
  }

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
  // descHTML shows an epic's description as escaped preformatted text: it is
  // markdown, and the page renders no markup from it.
  const descHTML = (s) => '<div><h4>Description</h4><pre class="ddesc">' + esc(s) + "</pre></div>";

  // live holds, per event, the last good payload and the error pushed after
  // it, if any; graph records whether tp pushed the epic export at all.
  const live = {};
  EVENTS.forEach((name) => { live[name] = { data: null, error: null }; });
  const state = { graph: false, tab: null, spec: null, wantTab: null };

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
    if (name === "export-list" && state.tab === null) {
      setTab(state.wantTab || (state.graph ? "epics" : "specs"));
      address.landed = true;
      const now = seenNow();
      lastLook.counts = sinceCounts(readSeen(), now);
      storeSeen(now);
    }
    pushedAt = new Date();
    showPushed();
    render();
  }

  // The header says how long ago tp last pushed, kept current by a timer,
  // with the clock time in its title.
  let pushedAt = null;
  function showPushed() {
    if (!pushedAt) return;
    $("pushed").textContent = "Updated " + tpPure.relTime(Date.now() - pushedAt.getTime());
    $("pushed").title = pushedAt.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
  }
  setInterval(showPushed, 5000);

  function setTab(tab) {
    state.tab = tab;
    document.body.dataset.tab = tab;
    $("loading").hidden = true;
    TABS.forEach((t) => {
      $("tab-" + t).setAttribute("aria-selected", String(t === tab));
      $("view-" + t).hidden = t !== tab;
    });
    syncURL();
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

  // ---------- since you last looked (§17) ----------
  // This browser's memory of the last state it saw: each file's done tasks
  // and recorded review and audit rounds from tp export --list, each epic's
  // status and each slice's round from tp epic export. Each load compares
  // its first state with the one the previous load stored, then stores its
  // own; tp stores nothing, and a browser that refuses storage shows no strip.
  const SEEN_KEY = "tp-dashboard-seen";
  const lastLook = { counts: null };
  function seenNow() {
    const files = {};
    ((data("export-list") || {}).files || []).forEach((f) => {
      files[f.path] = { done: (f.tasks || {}).done || 0, rounds: ((f.review || {}).round || 0) + ((f.audit || {}).round || 0) };
    });
    const g = data("epic-export") || {};
    const epics = {};
    (g.epics || []).forEach((e) => { epics[e.id] = e.status; });
    const slices = {};
    (g.slices || []).forEach((s) => { slices[s.dir] = s.round; });
    return { at: Date.now(), files, epics, slices };
  }
  function readSeen() {
    try { return JSON.parse(localStorage.getItem(SEEN_KEY)); } catch (e) { return null; }
  }
  function storeSeen(seen) {
    try { localStorage.setItem(SEEN_KEY, JSON.stringify(seen)); } catch (e) { /* the strip is lost, nothing else */ }
  }
  // sinceCounts counts what moved from prev to now, never which: tasks
  // closed and rounds recorded (a file or slice prev did not hold counts
  // from zero), and epics both hold whose status differs. Null without prev.
  function sinceCounts(prev, now) {
    if (!prev || typeof prev !== "object" || !prev.files) return null;
    const grew = (was, is) => Math.max(0, is - (was || 0));
    let tasks = 0;
    let rounds = 0;
    Object.keys(now.files).forEach((p) => {
      const was = prev.files[p] || {};
      tasks += grew(was.done, now.files[p].done);
      rounds += grew(was.rounds, now.files[p].rounds);
    });
    Object.keys(now.slices).forEach((d) => { rounds += grew((prev.slices || {})[d], now.slices[d]); });
    const epics = Object.keys(now.epics).filter((id) => prev.epics && id in prev.epics && prev.epics[id] !== now.epics[id]).length;
    return { at: prev.at, tasks, rounds, epics };
  }

  panel(["export-list"], () => {
    const c = lastLook.counts;
    const box = $("since");
    box.hidden = !c || !(c.tasks || c.rounds || c.epics);
    if (box.hidden) return;
    const mins = Math.round((Date.now() - c.at) / 60000);
    const ago = !(mins >= 1) ? "moments ago" : mins < 60 ? mins + " min ago" : mins < 2880 ? Math.round(mins / 60) + " h ago" : Math.round(mins / 1440) + " days ago";
    box.innerHTML = "<b>Since you last looked</b><span class=\"ago\">" + esc(ago) + "</span><ul>" +
      [[c.tasks, "task closed", "tasks closed"], [c.rounds, "round recorded", "rounds recorded"], [c.epics, "epic changed stage", "epics changed stage"]]
        .filter((x) => x[0]).map((x) => "<li>" + esc(plural(x[0], x[1], x[2])) + "</li>").join("") + "</ul>" + info("since", "since you last looked");
  });

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
  // spec phases, which count task files when the repository has no graph. A
  // stage filters the map to the epics tp epic export reports in it; a phase
  // opens Specs at that phase.
  panel(["summary"], () => {
    const sum = data("summary");
    if (!sum || !sum.band) {
      $("band").innerHTML = "";
      return;
    }
    const band = sum.band;
    const cell = (attr, k, label, n, pressed) => '<button type="button" class="pcell" ' + attr + '="' + esc(k) + '"' +
      (pressed === undefined ? "" : ' aria-pressed="' + pressed + '"') + '><b class="num">' + esc(n) + '</b><span class="pl">' + esc(label) + "</span></button>";
    const lane = (title, cells) => '<div class="plane"><span class="plab">' + esc(title) + '</span><div class="pcells">' + cells + "</div></div>";
    const byEpics = band.phases_of === "epics";
    const stages = band.stages || {};
    const phases = band.phases || {};
    $("band").innerHTML = (byEpics ? lane("Epics", STAGES.map(([k, label]) => cell("data-stage", k, label, stages[k] || 0, mapView.stage === k)).join("")) : "") +
      lane(byEpics ? "Epics by spec phase" : "Task files by phase", PHASES.map(([k, label]) => cell("data-phase", k, label, phases[k] || 0)).join(""));
  });
  $("band").addEventListener("click", (ev) => {
    const st = ev.target.closest("[data-stage]");
    const ph = ev.target.closest("[data-phase]");
    if (st) {
      mapView.stage = mapView.stage === st.dataset.stage ? null : st.dataset.stage;
      render();
      syncURL();
      if (mapView.stage) $("mapcard").scrollIntoView({ block: "nearest" });
    } else if (ph) {
      specsView.phase = ph.dataset.phase;
      setTab("specs");
      drawSpecs();
    }
  });

  // Milestone state (§6): one entry per milestone in track order, read from
  // the export's milestones: open or closed with what it waits on, its gate's
  // members done of members (deferrable members in neither), and its
  // threshold's state.
  const THRESHOLD_LABEL = {"written": "threshold written", "absent": "no threshold", "placeholder": "placeholder threshold", "met": "threshold met"};
  panel(["epic-export"], () => {
    const g = data("epic-export");
    const box = $("msstate");
    const ms = (state.graph && g && g.milestones) || [];
    box.hidden = !ms.length;
    if (box.hidden) return;
    const described = tpPure.describedCounts((g.graph && g.graph.epics) || []);
    box.innerHTML = '<span class="plab">Milestones</span><div class="mscells">' + ms.map((m) => {
      const s = m.state || {};
      const waits = m.waiting_on && m.waiting_on.length ? "waits on " + m.waiting_on.join(", ") : "";
      const th = s.threshold || "absent";
      const purpose = ((((g.graph || {}).gates || {})[m.milestone]) || {}).purpose;
      const dc = described[m.milestone] || { described: 0, all: 0 };
      return '<div class="ms" data-open="' + !!m.open + '"><b>' + esc(m.milestone) + '</b><span class="st" data-s="' + (m.open ? "done" : "stale") + '">' +
        (m.open ? "open" : "closed") + "</span>" + (waits ? '<span class="sm muted">' + esc(waits) + "</span>" : "") +
        '<span class="sm">gate ' + esc(s.members_done || 0) + " of " + esc(s.members || 0) + ' done</span><span class="sm th" data-th="' + esc(th) + '">' +
        esc(th === "written" && m.gate && m.gate.threshold ? "threshold: " + m.gate.threshold : THRESHOLD_LABEL[th] || th) + "</span>" +
        (purpose && purpose.trim() ? '<span class="sm purpose">purpose: ' + esc(purpose.trim()) + "</span>" : "") +
        '<span class="sm muted">' + esc(dc.described + " of " + plural(dc.all, "epic", "epics") + " described") + "</span></div>";
    }).join("") + "</div>";
  });

  // Lifecycle over time (§17): the summary's series, one band per stage after
  // planned, stacked from done up, and the unknown band for epics whose stage
  // time tp did not record, drawn hatched so it never reads as a stage.
  const CFD_BANDS = [["done", "done", "done"], ["in_progress", "active", "in progress"], ["specced", "ready", "specced"], ["claimed", "held", "claimed"], ["unknown", "waiting", "time not recorded"]];
  const HATCH = '<pattern id="cfd-hatch" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)"><rect width="6" height="6" fill="var(--waiting)"/><path d="M0 0V6" stroke="var(--muted)" stroke-width="1.4"/></pattern>';
  const cfdFill = (k, v) => (k === "unknown" ? "url(#cfd-hatch)" : "var(--" + v + ")");
  panel(["summary"], () => {
    const sum = data("summary");
    $("cfdcard").hidden = !state.graph || !sum;
    if ($("cfdcard").hidden) return;
    const days = sum.series || [];
    if (!days.length) {
      $("cfd").innerHTML = none("tp has recorded no claim, binding, task start or done time yet.");
      return;
    }
    // Each day is a step of its own width: the counts hold for the whole day
    // and change at its edge, so nothing is drawn between two recorded days.
    const W = Math.max(480, $("cfd").clientWidth || 960);
    const H = 180, L = 40, R = 8, T = 8, B = 24;
    const top = Math.max(1, ...days.map((d) => CFD_BANDS.reduce((a, [k]) => a + (d[k] || 0), 0)));
    const step = (W - L - R) / days.length;
    const edges = (i) => [L + i * step, L + (i + 1) * step];
    const y = (v) => H - B - (v / top) * (H - B - T);
    const line = (vals) => vals.map((v, i) => edges(i).map((ex) => ex + "," + y(v)).join(" "));
    let base = days.map(() => 0);
    let areas = "";
    CFD_BANDS.forEach(([k, v, l]) => {
      const up = days.map((d, i) => base[i] + (d[k] || 0));
      areas += '<polygon points="' + line(up).join(" ") + " " + line(base).reverse().map((p) => p.split(" ").reverse().join(" ")).join(" ") +
        '" fill="' + cfdFill(k, v) + '" opacity=".8"><title>' + esc(l) + "</title></polygon>";
      base = up;
    });
    const first = days[0].date;
    const lastDay = days[days.length - 1];
    const axis = '<line class="ax" x1="' + L + '" x2="' + (W - R) + '" y1="' + (H - B) + '" y2="' + (H - B) + '"/>' +
      '<text x="' + (L - 6) + '" y="' + (T + 10) + '" text-anchor="end">' + top + '</text><text x="' + (L - 6) + '" y="' + (H - B) + '" text-anchor="end">0</text>' +
      '<text x="' + L + '" y="' + (H - 6) + '">' + esc(first) + "</text>" +
      (days.length > 1 ? '<text x="' + (W - R) + '" y="' + (H - 6) + '" text-anchor="end">' + esc(lastDay.date) + "</text>" : "");
    const today = CFD_BANDS.map(([k, , l]) => (lastDay[k] || 0) + " " + l).join(", ");
    const swatch = (k, v) => '<svg viewBox="0 0 14 14" aria-hidden="true"><rect width="14" height="14" rx="3" fill="' + cfdFill(k, v) + '"/></svg>';
    $("cfd").innerHTML = '<svg class="cfd" viewBox="0 0 ' + W + " " + H + '" role="img" aria-label="Work epics by stage per day from ' + esc(first) + " to " + esc(lastDay.date) +
      "; on " + esc(lastDay.date) + ": " + esc(today) + '"><defs>' + HATCH + "</defs>" + areas + axis + "</svg>" +
      '<div class="cfdkey">' + CFD_BANDS.map(([k, v, l]) => "<span>" + swatch(k, v) + esc(l) + ", " + esc(lastDay[k] || 0) + " on " + esc(lastDay.date) + "</span>").join("") +
      "<span>planned, " + esc(lastDay.planned || 0) + " on " + esc(lastDay.date) + ", left out so the moving stages stay readable</span></div>";
  });

  // What each panel and figure lists and where it comes from (§4).
  const INFO = {
    band: "Work epics per stage, and per spec phase at the earliest phase of their bound specs; with no graph the phases count task files. From the summary's band. Click a stage to show only its epics on the map, again to clear; click a phase to open Specs at it. Milestones: each one open or closed by its opening order with what it waits on, its gate's members done out of members (deferrable members in neither count), and its threshold as written, absent, a placeholder or met, from the export's milestones.",
    share: "Work epics done over all work epics; condition epics are in no figure. From the summary's plan, the share tp epic status reports.",
    ready: "Work epics that can start now: open milestone, every dependency done, no bound spec, no claim. In tp epic ready's order, most downstream first.",
    critical: "Not-done epics on the longest dependency chain of the whole graph: tp epic status's top-level critical_path.",
    blocked: "Work epics waiting on a dependency that is not done. Held epics are the blocked ones a condition holds.",
    held: "Blocked work epics a condition epic, or a group of conditions, holds: they wait on an operator decision, not on work.",
    operator: "Decisions only the operator takes, each with the command that settles it: actionable conditions (tp epic done), any-of groups of conditions to choose one of, gates whose written threshold awaits --met, placeholder thresholds to rewrite (tp epic set), slices at their cap that can take one more round (tp epic review --grant-round), and the escalations stopping a spec bound to an epic. From the summary's plan, the export's milestones[].operator and slices, and the task file list's escalations.",
    loops: "Epic review slices not done, less those a re-review alone settles. What each needs differs: an open or unreviewed slice a round, a stale one with a blocking finding open a round that reads it, one with a round in flight that round recorded, not a new one, one at its cap its findings dispositioned, and one stale at its cap the operator's grant, so it is in the operator queue too. After the plus, the slices a re-review alone settles: a graph edit changed their units since their last round, they would otherwise be done, and they are under their cap. From the summary's open_loops and stale_loops.",
    lint: "The error and warning counts tp epic lint prints for the graph, from the export's lint.",
    throughput: "Work epics done per ISO week across every milestone, from their done time. From the summary's plan.",
    waiting: "What is blocked, and by what. The blocked work epics of open milestones, grouped under each dependency they wait on directly, an any-of group as one: an epic waiting on three blockers is in three groups, so the group sizes do not add up to the total. Each group names its blocker, what the blocker is doing (startable, claimed, an operator decision, or waiting itself) and how many epics wait on it directly, largest first. The blocked epics of milestones not yet open are one counted row, and deferrable gate members are left out, so the parts add up to the blocked epics figure. From the summary's waiting list and the export's blocked_by and blocked_by_any_of.",
    most: "Not-done work epics other work epics depend on, transitively, most dependents first.",
    since: "What changed since this browser last saw the page: tasks closed and review, audit and plan review rounds recorded, and epics whose stage changed, counted and never named. This browser keeps the last state it saw and compares it on each load; tp stores nothing for it, and a private window or cleared storage shows no strip.",
    cfd: "Work epics per stage per day, from the times tp records: claimed at the claim, specced at the earliest binding time of the bound specs, in progress at the earliest start of their tasks, done at the done record or the audit that released the last bound spec. An epic that reached a stage at a time tp did not record (a spec bound before tp recorded binding times) is counted as time not recorded, never on a guessed day. An epic's planning time is not recorded, so an epic added to the graph later counts as planned from the chart's first day. Planned epics are left out of the bands. From the summary's series.",
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
    // The plan's queue, plus the bound specs' escalations the task file list
    // carries (§7), so the figure counts the rows the list draws.
    const queue = f.operator_queue + escalationRows().length;
    $("kpis").innerHTML = [
      kpi("share", Math.round(f.share_done * 100) + "%<small> " + esc(f.done) + " of " + esc(f.work) + "</small>", "share of work epics done"),
      kpi("ready", esc(f.ready), "ready epics"),
      kpi("critical", esc(f.critical_path), "epics on the critical path", "crit"),
      kpi("blocked", esc(f.blocked), "blocked epics", f.blocked ? "warn" : ""),
      kpi("held", esc(f.held), "held epics", f.held ? "op" : ""),
      kpi("operator", esc(queue), "operator decisions waiting", queue ? "op" : ""),
      kpi("loops", esc(f.open_loops - (f.stale_loops || 0)) + (f.stale_loops ? "<small> + " + esc(f.stale_loops) + " stale</small>" : ""), "review loops needing a round"),
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

  // row draws one list row; an epic's row names it with a link that
  // selects it.
  const row = (id, title, meta, why, epic) => '<div class="row"><span class="id">' + (epic ? epicLink(id) : esc(id)) + '</span><span class="t">' + esc(title) +
    '</span><span class="m">' + esc(meta || "") + "</span>" + (why ? '<span class="why">' + why + "</span>" : "") + "</div>";
  const none = (text) => '<p class="empty">' + esc(text) + "</p>";
  const titleOf = (v, id) => (v.epics[id] ? v.epics[id].title : "");
  const dependents = (v, id) => {
    const d = v.epics[id] && v.epics[id].downstream;
    return d ? plural(d.dependents, "epic waits", "epics wait") + " on it" : "";
  };

  // escalationRows are the escalations tp export --list names on the task
  // files bound to an epic, one row each (§7).
  const escalationRows = () => ((data("export-list") || {}).files || []).filter((f) => f.epic).flatMap((f) =>
    (f.escalations || []).map((b) => row(f.epic, f.title || f.path, b.code, "<code>" + esc(f.path) + "</code>: " + esc(b.message))));

  // operatorRows is §7's list, each row with the command that settles it:
  // the actionable conditions, the any-of groups as their choices, the gates
  // awaiting --met, each placeholder threshold once as its rewrite, the
  // slices that may take --grant-round, and the bound specs' escalations.
  function operatorRows(v) {
    const done = (id) => "<code>tp epic done " + esc(id) + ' --evidence "&lt;text or URL&gt;"</code>';
    const ops = v.plan.operator_conditions.map((id) => row(id, titleOf(v, id), "condition", done(id)));
    (v.plan.operator_groups || []).forEach((c) => ops.push(row(c.epic, titleOf(v, c.epic), "choose one of " + plural(c.any_of.length, "condition", "conditions"),
      c.any_of.map(done).join(" or "))));
    (v.g.milestones || []).forEach((m) => {
      const o = m.operator || {};
      (o.gates || []).forEach((gate) => ops.push(row(gate.milestone, gate.threshold, "gate to measure", "<code>tp epic gate " + esc(gate.milestone) + ' --met --evidence "&lt;text or URL&gt;"</code>')));
      (o.threshold_rewrites || []).forEach((r) => ops.push(row(r.milestone, r.threshold, "threshold to write", "<code>" + esc(r.command) + "</code>")));
    });
    (v.g.slices || []).filter((s) => s.grantable).forEach((s) => ops.push(row("slice", sliceLabel(s), roundText(s),
      "stale at its cap: <code>tp epic review " + esc(sliceLabel(s)) + " --grant-round</code>")));
    return ops.concat(escalationRows());
  }

  // Waiting epics (dashboard-ux.md §2) answers what is blocked and by what:
  // the waiting epics of open milestones grouped under each direct blocker
  // (tpPure.waitingGroups), the largest groups first and the rest behind
  // "show all"; those of milestones not yet open in one counted row. Its
  // total is the KPI's blocked epics, the parts named.
  const WAIT_SHOWN = 6;
  const BLOCKER_LABEL = { startable: "startable", claimed: "claimed", decision: "operator decision", waiting: "waiting" };
  const waitView = { all: false };
  function waitingHTML(v) {
    const open = {};
    (v.g.milestones || []).forEach((m) => { open[m.milestone] = !!m.open; });
    const w = tpPure.waitingGroups(v.plan.waiting, v.epics, v.plan.ready, open);
    const blocked = v.plan.figures.blocked;
    if (!blocked) return none("No epic waits on another.");
    const deferred = blocked - v.plan.waiting.length;
    const byMs = {};
    w.closed.forEach((id) => { const m = (v.epics[id] || {}).milestone; byMs[m] = (byMs[m] || 0) + 1; });
    const parts = [plural(w.open, "in an open milestone", "in open milestones") + ", grouped below under what each waits on directly, so an epic waiting on two blockers is in two groups"];
    if (w.closed.length) parts.push(w.closed.length + " in milestones not yet open");
    if (deferred) parts.push(plural(deferred, "deferrable gate member", "deferrable gate members") + " left out");
    const shown = waitView.all ? w.groups : w.groups.slice(0, WAIT_SHOWN);
    const group = (gr, i) => '<div class="wgroup" data-wg="' + i + '"><div class="wh">' + (gr.blocker ? epicLink(gr.blocker) : '<span class="muted">one of</span> ' + gr.any_of.map(epicLink).join(", ")) +
      '<span class="bst" data-b="' + esc(gr.state) + '">' + esc(BLOCKER_LABEL[gr.state]) + '</span><span class="wn num" title="epics of open milestones that wait on it directly">' + esc(plural(gr.epics.length, "waits", "wait")) + " on it here</span></div>" +
      '<div class="wids">' + gr.epics.map(epicLink).join(" ") + "</div></div>";
    return '<p class="wsum"><b class="num">' + esc(plural(blocked, "blocked epic", "blocked epics")) + "</b>: " + esc(parts.join("; ")) + ".</p>" +
      shown.map(group).join("") +
      (w.groups.length > WAIT_SHOWN ? '<button type="button" class="chip wall" data-wall aria-expanded="' + waitView.all + '">' +
        (waitView.all ? "Show the largest " + WAIT_SHOWN : "Show all " + w.groups.length + " groups") + "</button>" : "") +
      (w.closed.length ? '<div class="wgroup wclosed"><div class="wh"><span>' + esc(plural(w.closed.length, "epic waits", "epics wait")) +
        ' in milestones not yet open</span></div><div class="wids muted">' + esc(Object.keys(byMs).map((m) => m + " " + byMs[m]).join(", ")) + "</div></div>" : "");
  }
  $("waitcard").addEventListener("click", (ev) => {
    if (!ev.target.closest("[data-wall]")) return;
    waitView.all = !waitView.all;
    render();
  });

  // The now lists (§4): what can start, what waits on the operator, what
  // waits on other work, and what the most work waits on.
  panel(["summary", "epic-export"], () => {
    const v = planView();
    $("now").hidden = !v;
    $("waitcard").hidden = !v;
    if (!v) return;
    $("now-ready").innerHTML = v.plan.ready.map((id) => row(id, titleOf(v, id), dependents(v, id), "", true)).join("") || none("No epic can start now.");
    $("now-operator").innerHTML = operatorRows(v).join("") || none("Nothing waits on the operator.");
    $("now-waiting").innerHTML = waitingHTML(v);
    $("now-most").innerHTML = v.plan.most_waited_on.slice(0, 10).map((id) => row(id, titleOf(v, id), dependents(v, id), "", true)).join("") || none("No epic is waited on.");
  });

  INFO.slices = "One row per epic review slice (tp epic review): its round with the cap and the grants spent, its state, the units changed since its last recorded round when it is stale, its open findings by role, and the findings of each recorded round as a curve. From the export's slices and the summary's per-round counts.";

  // sliceLabel names a slice as tp epic review takes it.
  function sliceLabel(s) {
    const k = s.key;
    if (!k) return s.dir;
    if (k.kind === "epics") return "--epics " + (k.ids || []).join(",") + (k.name ? " --name " + k.name : "");
    return "--" + k.kind + " " + k.value;
  }

  // sliceTitle names a slice for reading: a set by its name, or as unnamed,
  // and its size; any other slice as tp epic review takes it.
  function sliceTitle(s) {
    const k = s.key;
    if (!k || k.kind !== "epics") return sliceLabel(s);
    return (k.name ? "set " + k.name : "unnamed set") + ", " + plural((k.ids || []).length, "epic", "epics");
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
        const on = s.dir === hist.dir;
        return '<tr class="' + (on ? "on" : "") + '"><td class="k"><button type="button" class="slink" data-slice="' + esc(s.dir) + '" aria-pressed="' + on +
          '" title="Show this slice\'s graph history">' + esc(sliceTitle(s)) + "</button>" +
          (s.key && s.key.kind === "epics" ? '<small class="scmd">' + esc(sliceLabel(s)) + "</small>" : "") + '</td><td data-l="Round">' + esc(roundText(s)) + '</td><td data-l="State"><span class="st" data-s="' + esc(s.state) + '"' + stale + ">" +
          esc(s.state.replace("_", " ")) + '</span></td><td class="n" data-l="Open findings by role">' + open + '</td><td data-l="Findings per round">' + curve(sum ? sum.rounds[s.dir] : null) + "</td></tr>";
      }).join("") + "</tbody></table>";
  });

  // ---------- a slice's graph history (§11) ----------
  // One slice at a time, from tp epic review <slice> --history: its rounds in
  // order with their findings by role and what changed since the round
  // before, then what changed since its last round. Two chosen rounds are
  // fetched with from and to, their difference drawn beside the earlier
  // round's findings. Fetched when the slice is shown and again on every push.
  const hist = { dir: null, pair: null, data: null, pairData: null, error: null, seq: 0, seen: null };
  INFO.history = "A slice's rounds in order, from tp epic review <slice> --history: each round's findings by role, and what changed in the slice's units since the round before, read from the two rounds' snapshots. Now is what changed since the last round. Pick two rounds to see their difference beside the earlier round's findings. A change made and undone between two rounds leaves no trace, and which write made a change is not recorded.";

  const selectedSlice = () => (((data("epic-export") || {}).slices) || []).find((s) => s.dir === hist.dir) || null;
  const csvField = (id) => (/[",\s]/.test(id) ? '"' + id.replace(/"/g, '""') + '"' : id);

  // sliceQuery names the slice as /api/history takes it, or null for a slice
  // whose key tp does not know yet.
  function sliceQuery(s) {
    const k = s && s.key;
    if (!k) return null;
    const p = new URLSearchParams();
    if (k.kind === "epics") {
      p.set("epics", (k.ids || []).map(csvField).join(","));
      if (k.name) p.set("name", k.name);
    } else {
      p.set(k.kind, k.value);
    }
    return p;
  }

  async function getHistory(p) {
    try {
      const res = await fetch("/api/history?" + p.toString(), { cache: "no-store" });
      return await res.json();
    } catch (err) {
      return { error: "tp epic review --history could not be fetched: " + err.message, hint: "check that tp dashboard is still running" };
    }
  }

  // shownPair is the two rounds compared: the chosen ones, else the last two.
  function shownPair() {
    const n = hist.data && hist.data.rounds ? hist.data.rounds.length : 0;
    if (hist.pair && hist.pair[1] <= n) return hist.pair;
    return n >= 2 ? [n - 1, n] : null;
  }

  async function fetchHistory() {
    const s = selectedSlice();
    const q = sliceQuery(s);
    const seq = ++hist.seq;
    if (!q) {
      Object.assign(hist, { data: null, pairData: null, error: s ? { error: "tp does not know this slice's key yet", hint: "its next tp epic review emission records it" } : null });
      drawHistory();
      return;
    }
    const full = await getHistory(q);
    if (seq !== hist.seq) return;
    if (isError(full)) {
      hist.error = full;
      drawHistory();
      return;
    }
    Object.assign(hist, { data: full, error: null });
    const pair = shownPair();
    let pairData = null;
    if (pair) {
      const pq = new URLSearchParams(q);
      pq.set("from", pair[0]);
      pq.set("to", pair[1]);
      pairData = await getHistory(pq);
      if (seq !== hist.seq) return;
    }
    hist.pairData = pairData;
    drawHistory();
  }

  // diffParts says what a difference holds, one phrase per kind of change.
  function diffParts(d) {
    if (!d) return [];
    const parts = [];
    const n = (list, one, many) => { if (list && list.length) parts.push(plural(list.length, one, many)); };
    n(d.units_added, "unit added", "units added");
    n(d.units_removed, "unit removed", "units removed");
    n(d.edges_added, "edge added", "edges added");
    n(d.edges_removed, "edge removed", "edges removed");
    n(d.whys_changed, "why rewritten", "whys rewritten");
    n(d.fields_changed, "field changed", "fields changed");
    if (d.hashes_only) n(d.units_changed, "unit changed", "units changed");
    return parts;
  }
  const diffLine = (d) => (d ? diffParts(d).join(", ") || "no change" : "the first round shown");

  function findingsLine(byRole) {
    const roles = Object.keys(byRole || {}).sort();
    return roles.length ? roles.map((r) => esc(r) + " " + esc(byRole[r].length)).join(", ") : '<span class="muted">clean</span>';
  }

  // diffDetail draws every change of a difference: units, edges, whys with
  // both texts, and fields with both values.
  function diffDetail(d) {
    if (!d) return none("No difference.");
    const out = [];
    const code = (s) => "<code>" + esc(s) + "</code>";
    const list = (title, items) => { if (items.length) out.push('<div class="dpart"><h4>' + esc(title) + "</h4>" + items.join("") + "</div>"); };
    const units = (xs) => (xs || []).map((u) => '<div class="dl">' + code(u) + "</div>");
    list("Units added", units(d.units_added));
    list("Units removed", units(d.units_removed));
    list("Edges added", (d.edges_added || []).map((e) => '<div class="dl ins">' + code(e.unit) + " → " + code(e.to) + (e.why ? '<span class="dwhy">' + esc(e.why) + "</span>" : "") + "</div>"));
    list("Edges removed", (d.edges_removed || []).map((e) => '<div class="dl del">' + code(e.unit) + " → " + code(e.to) + (e.why ? '<span class="dwhy">' + esc(e.why) + "</span>" : "") + "</div>"));
    list("Whys rewritten", (d.whys_changed || []).map((w) => '<div class="dl">' + code(w.unit) + " → " + code(w.to) +
      '<div class="was">' + esc(w.before) + '</div><div class="now">' + esc(w.after) + "</div></div>"));
    list("Fields changed", (d.fields_changed || []).map((f) => '<div class="dl">' + code(f.unit) + " " + esc(f.field) +
      (f.before ? '<div class="was">' + esc(f.before) + "</div>" : "") + (f.after ? '<div class="now">' + esc(f.after) + "</div>" : "") + "</div>"));
    if (d.hashes_only) list("Units changed (their blocks were not kept, so only which changed is known)", units(d.units_changed));
    return out.join("") || none("Nothing changed between these two snapshots.");
  }

  // findingsList draws a round's findings under their role, each with its
  // severity, place and disposition.
  function findingsList(byRole) {
    const roles = Object.keys(byRole || {}).sort();
    if (!roles.length) return none("This round found nothing.");
    return roles.map((r) => '<div class="dpart"><h4>' + esc(r) + ' <span class="muted num">' + byRole[r].length + "</span></h4>" +
      byRole[r].map((f) => {
        const res = f.resolved && f.resolved.status ? '<span class="sm muted">' + esc(f.resolved.status) + "</span>" : "";
        return '<div class="fnd"><span class="sev" data-sev="' + esc(f.severity || "") + '">' + esc(f.severity || "") + "</span><code>" + esc(f.location || "") + "</code>" + res +
          "<p>" + esc(f.finding || "") + "</p></div>";
      }).join("") + "</div>").join("");
  }

  function drawHistory() {
    const box = $("slice-history");
    const s = selectedSlice();
    box.hidden = !s;
    if (!s) return;
    const head = '<div class="hhead"><h3>Graph history of <code>' + esc(sliceLabel(s)) + "</code></h3>" + info("history", "the graph history") +
      '<span class="spacer"></span><button type="button" class="chip" data-hclose>Close</button></div>';
    if (hist.error) {
      box.innerHTML = head + '<div class="banner"><b>The history could not be derived</b><span>' + esc(hist.error.error) + "</span>" +
        (hist.error.hint ? '<span class="hint">' + esc(hist.error.hint) + "</span>" : "") + "</div>";
      return;
    }
    const h = hist.data;
    if (!h) {
      box.innerHTML = head + none("Reading the history from tp.");
      return;
    }
    const rounds = h.rounds || [];
    if (!rounds.length) {
      box.innerHTML = head + none("This slice has recorded no round yet, so it has no history.");
      return;
    }
    const pair = shownPair();
    const rows = rounds.map((r) => '<tr class="' + (pair && pair.includes(r.round) ? "on" : "") + '"><td>Round ' + esc(r.round) + "</td><td>" + findingsLine(r.findings_by_role) +
      "</td><td>" + esc(r.diff_from ? "since round " + r.diff_from + ": " + diffLine(r.diff) : diffLine(null)) + "</td></tr>").join("") +
      (h.current ? '<tr><td>Now</td><td class="muted">not reviewed</td><td>' + esc("since round " + h.current.from_round + ": " + diffLine(h.current.diff)) + "</td></tr>" : "");
    const opts = (sel) => rounds.map((r) => '<option value="' + r.round + '"' + (r.round === sel ? " selected" : "") + ">round " + r.round + "</option>").join("");
    let compare = none("Two rounds are needed to compare; this slice has recorded one.");
    if (pair) {
      const pd = hist.pairData;
      const pick = '<div class="hpick"><label>Compare <select data-hr="0" aria-label="Earlier round">' + opts(pair[0]) + "</select></label>" +
        '<label>with <select data-hr="1" aria-label="Later round">' + opts(pair[1]) + "</select></label></div>";
      let body = none("Reading the two rounds from tp.");
      if (isError(pd)) body = '<div class="banner"><b>The two rounds could not be compared</b><span>' + esc(pd.error) + "</span></div>";
      else if (pd && pd.rounds && pd.rounds.length === 2) {
        const [a, b] = pd.rounds;
        body = '<div class="hcmp"><div><h4 class="hsub">Round ' + esc(a.round) + "'s findings</h4>" + findingsList(a.findings_by_role) + "</div>" +
          '<div><h4 class="hsub">From round ' + esc(a.round) + " to round " + esc(b.round) + '<span class="muted sm"> ' + esc(diffLine(b.diff)) + "</span></h4>" + diffDetail(b.diff) + "</div></div>";
      }
      compare = pick + body;
    }
    box.innerHTML = head + '<div class="scroll"><table class="loops hrounds"><thead><tr><th>Round</th><th>Findings by role</th><th>What changed in the slice</th></tr></thead><tbody>' +
      rows + "</tbody></table></div>" + compare;
  }

  // The history follows tp: a picked slice is fetched, and fetched again on
  // every push while it is shown.
  panel(["epic-export", "summary"], () => {
    const pushed = [data("epic-export"), data("summary")];
    if (!hist.dir || state.tab === null) {
      drawHistory();
      return;
    }
    const seen = hist.seen;
    if (!seen || seen.dir !== hist.dir || seen.p[0] !== pushed[0] || seen.p[1] !== pushed[1]) {
      hist.seen = { dir: hist.dir, p: pushed };
      fetchHistory();
    }
    drawHistory();
  });

  function showSlice(dir) {
    hist.dir = hist.dir === dir ? null : dir;
    Object.assign(hist, { pair: null, data: null, pairData: null, error: null, seen: null });
    render();
    syncURL();
    if (hist.dir) $("slice-history").scrollIntoView({ block: "nearest" });
  }
  $("plan-review").addEventListener("click", (ev) => {
    const b = ev.target.closest("[data-slice]");
    if (b) showSlice(b.dataset.slice);
    else if (ev.target.closest("[data-hclose]")) showSlice(hist.dir);
  });
  $("plan-review").addEventListener("change", (ev) => {
    const sel = ev.target.closest("[data-hr]");
    if (!sel) return;
    const pair = (shownPair() || [1, 2]).slice();
    pair[Number(sel.dataset.hr)] = Number(sel.value);
    if (pair[0] === pair[1]) return drawHistory();
    hist.pair = pair.sort((x, y) => x - y);
    hist.pairData = null;
    syncURL();
    fetchHistory();
  });

  // ---------- the plan map (§9) ----------
  // tp lays the map out: every epic's layout names its lane, the lane's kind,
  // its milestone's column and its row inside the milestone. The page turns
  // those into coordinates and places nothing by any rule of its own.
  const MAP = { boxW: 178, rowH: 18, head: 34, padB: 12, colGap: 40, slotGap: 14, laneGap: 22, left: 92, top: 14 };
  const LANE_LABEL = { main: "Main", parallel: "Parallel", triggered: "Triggered" };
  const MARK_LABEL = { done: "done", ready: "can start", active: "claimed or under way", blocked: "blocked", held: "held by a condition", waiting: "milestone closed", moot: "moot" };
  const MARKS = ["done", "ready", "active", "blocked", "held", "waiting", "moot"];
  const mapView = { selected: null, focus: null, edges: false, nb: false, tracks: new Set(), geo: null, deps: {}, dependents: {}, byId: {}, q: "", stage: null };
  // NB_DIR names the two directions the neighbours filter marks an edge with:
  // in, from an epic the selected one depends on; out, to an epic depending
  // on it.
  const NB_DIR = {"in": "Incoming", "out": "Outgoing"};

  // FOLD maps the letters the epic search folds before Unicode decomposition:
  // the Turkish dotless and dotted i, which decomposition and lower-casing do
  // not bring to a plain i, and the Turkish letters with a cedilla, breve or
  // diaeresis, spelled out so a test can read them.
  const FOLD = {"ı": "i", "İ": "i", "ş": "s", "Ş": "s", "ğ": "g", "Ğ": "g", "ç": "c", "Ç": "c", "ö": "o", "Ö": "o", "ü": "u", "Ü": "u"};
  const FOLD_RE = new RegExp("[" + Object.keys(FOLD).join("") + "]", "g");
  // fold reduces s to what the epic search compares: case and diacritics
  // gone, so isik matches IŞIK and ışık alike.
  const fold = (s) => String(s).replace(FOLD_RE, (c) => FOLD[c]).normalize("NFD").replace(/\p{M}/gu, "").toLowerCase();
  // searchHit reports whether the epic's id or title holds the folded query.
  const searchHit = (e, q) => !q || fold(e.id).includes(q) || fold(e.title || "").includes(q);
  INFO.map = "Lanes from the opening order: the main lane is the longest chain of milestone openings, parallel lanes the other chains, the triggered lane the milestones that open on a trigger. A milestone's column is its opening depth. tp computes every place (the export's layout); the page only draws it. Colour and shape both carry an epic's state; a diamond is a condition. The orange line is the critical path. The bar on a box's right edge is its gate's members done. Click an epic for its chain; All edges draws every edge. Direct only keeps the selected epic, the epics it depends on and the epics that depend on it, marks each edge in or out, and shows an edge's why on hover. The search matches ids and titles ignoring case and accents, Turkish ı and İ included.";

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
          deps[e.id].push({ id: x, why: d.why, group: d.any_of || null });
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

  // neighboursOf is the selected epic, the epics it depends on and the epics
  // that depend on it, one step each way.
  function neighboursOf(id) {
    const seen = new Set([id]);
    (mapView.deps[id] || []).forEach((d) => seen.add(d.id));
    (mapView.dependents[id] || []).forEach((y) => seen.add(y));
    return seen;
  }

  // nbLabel writes an edge's direction at its middle; an edge inside one
  // milestone's column bulges right over the ids, so its label goes left.
  function nbLabel(p, q, dir) {
    const same = Math.abs(q.x - p.x) < 1;
    return '<text class="nbdir ' + dir + '" x="' + (same ? p.x - 9 : (p.x + q.x) / 2) + '" y="' + ((p.y + q.y) / 2 + (same ? 3 : -3)) + '" text-anchor="' + (same ? "end" : "middle") + '">' + dir + "</text>";
  }

  // neighbourEdges draws the selected epic's edges, each marked in or out and
  // carrying its why, its two ends and its any-of group for the hover tip.
  function neighbourEdges(id, pos) {
    const out = [];
    const draw = (from, to, d, dir) => {
      const p = pos[from];
      const q = pos[to];
      if (!p || !q) return;
      const path = edgePath(p, q);
      out.push('<g class="nbedge" data-dir="' + dir + '" data-from="' + esc(from) + '" data-to="' + esc(to) + '" data-why="' + esc(d.why || "") + '"' +
        (d.group ? ' data-group="' + esc(d.group.join(", ")) + '"' : "") + '><path class="hit" d="' + path + '"/>' +
        '<path class="edge ' + dir + '" d="' + path + '"' + (d.group ? ' stroke-dasharray="3 2"' : "") + "/>" +
        nbLabel(p, q, dir) + "</g>");
    };
    (mapView.deps[id] || []).forEach((d) => draw(d.id, id, d, "in"));
    new Set(mapView.dependents[id] || []).forEach((y) => (mapView.deps[y] || []).filter((d) => d.id === id).forEach((d) => draw(id, y, d, "out")));
    return out;
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
    mapView.tracks = new Set([...mapView.tracks].filter((i) => i < tracks.length));
    box.innerHTML = tracks.length > 1 ? tracks.map((t, i) => '<button class="chip" type="button" data-track="' + i + '" aria-pressed="' + mapView.tracks.has(i) + '" title="' +
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
    const nb = sel && mapView.nb;
    const chain = sel ? (nb ? neighboursOf(sel) : chainOf(sel)) : null;
    const trackOf = {};
    ((g.graph && g.graph.milestones) || []).forEach((t, i) => t.forEach((m) => { trackOf[m] = i; }));
    const q = fold(mapView.q.trim());
    const inStage = (e) => !mapView.stage || e.status === mapView.stage;
    const shown = (e) => (!mapView.tracks.size || mapView.tracks.has(trackOf[e.milestone])) && (!chain || chain.has(e.id)) && searchHit(e, q) && inStage(e);
    const stageNote = mapView.stage ? plural(geo.placed.filter(inStage).length, "epic", "epics") + " " + (STAGES.find(([k]) => k === mapView.stage) || ["", mapView.stage])[1].toLowerCase() : "";
    $("map-n").textContent = [q ? plural(geo.placed.filter((e) => searchHit(e, q) && inStage(e)).length, "match", "matches") : "", stageNote].filter(Boolean).join(", ");
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
    if (!nb && (mapView.edges || sel)) {
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
    // One epic is in the tab order at a time: the one focus last moved to,
    // else the selected one, else the first.
    if (!geo.pos[mapView.focus]) mapView.focus = sel || (geo.placed[0] || {}).id || null;
    const hadFocus = $("mapsvg").contains(document.activeElement);
    const described = describedIds(g);
    const stops = tpPure.tabStops(geo.placed.map((e) => e.id), mapView.focus);
    geo.placed.forEach((e, i) => {
      const p = geo.pos[e.id];
      const s = markState(e, geo.byMs[e.milestone]);
      out.push('<g class="node' + (shown(e) ? "" : " dim") + '" data-id="' + esc(e.id) + '" role="button" tabindex="' + stops[i] +
        '" aria-label="' + esc(e.id + ": " + e.title + ", " + MARK_LABEL[s]) + '"' + (e.id === sel ? ' aria-pressed="true"' : "") + ">" +
        (e.id === sel ? '<circle cx="' + p.x + '" cy="' + p.y + '" r="10" fill="none" stroke="var(--accent)" stroke-width="2.2"/>' : "") +
        mark(e.condition, s, p.x, p.y, 4.8) + '<text class="eid" x="' + (p.x + 12) + '" y="' + (p.y + 4) + '">' + esc(clip(e.id, 19)) + "</text>" +
        descMark(described.has(e.id), p.x + 12, p.y + 4, clip(e.id, 19).length) +
        '<rect class="rowhit" x="' + (p.x - 9) + '" y="' + (p.y - MAP.rowH / 2) + '" width="' + (MAP.boxW - 24) + '" height="' + MAP.rowH + '" fill="transparent"/></g>');
    });
    // The neighbour edges sit above the epics so their labels read; the hit
    // test below still gives an epic's mark and id to the epic.
    if (nb) out.push("<g>" + neighbourEdges(sel, geo.pos).join("") + "</g>");
    $("mapsvg").innerHTML = out.join("");
    if (hadFocus) focusNode(mapView.focus);
    mapPort.bounds(geo.W, geo.H);
    syncURL();
  }
  const clip = (s, n) => (s.length > n ? s.slice(0, n - 1) + "…" : s);
  panel(["epic-export"], drawMap);
  // describedIds is the set of the graph's epics with a description.
  const describedIds = (g) => new Set(((g.graph && g.graph.epics) || []).filter((e) => String(e.description || "").trim()).map((e) => e.id));
  // descMark draws the mark of a described epic after its id label of n
  // characters at x, y; nothing for an epic with no description.
  const descMark = (on, x, y, n) => (on ? '<text class="dmark" x="' + (x + n * 6.7 + 4) + '" y="' + y + '"><title>has a description</title>¶</text>' : "");

  // focusNode puts the map's tab stop on id and focuses it.
  function focusNode(id) {
    const svg = $("mapsvg");
    const node = id && [...svg.querySelectorAll(".node")].find((n) => n.dataset.id === id);
    if (!node) return;
    svg.querySelectorAll('.node[tabindex="0"]').forEach((n) => n.setAttribute("tabindex", "-1"));
    node.setAttribute("tabindex", "0");
    mapView.focus = id;
    node.focus({ preventScroll: true });
  }
  // The arrows move the map's focus to the nearest epic that way
  // (tpPure.roveNext), taking the view along; Enter or Space selects it.
  $("mapsvg").addEventListener("keydown", (ev) => {
    const node = ev.target.closest && ev.target.closest(".node");
    if (!node) return;
    if (ev.key === "Enter" || ev.key === " ") {
      ev.preventDefault();
      selectEpic(node.dataset.id);
      return;
    }
    const next = tpPure.roveNext(mapView.geo.pos, node.dataset.id, ev.key);
    if (next === node.dataset.id) return;
    ev.preventDefault();
    focusNode(next);
    mapPort.reveal(mapView.geo.pos[next]);
  });

  // makePort pans and zooms the drawing svg in the box el: drag or one
  // finger to pan, Ctrl or ⌘ with the wheel, a pinch, or the plain wheel
  // while hooks.card fills the screen, to zoom; otherwise a plain wheel
  // scrolls the page. hooks.first gives a drawing's first view, hooks.hover
  // and hooks.click answer the pointer, and hooks.card holds the zoom keys.
  function makePort(el, svg, hooks) {
    let v = null;
    let size = { W: 1, H: 1 };
    let drag = null;
    let moved = false;
    const apply = () => { if (v) svg.setAttribute("viewBox", v.x + " " + v.y + " " + v.w + " " + v.h); };
    // fit shows the drawing's first view (hooks.first).
    function fit() {
      const r = el.getBoundingClientRect();
      if (!r.width || !r.height) return;
      v = hooks.first(size.W, size.H, r.width, r.height);
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
      if (!tpPure.wheelZooms(ev, hooks.card.classList.contains("fs")) || !v) return;
      ev.preventDefault();
      const p = toSvg(ev.clientX, ev.clientY);
      zoomAt(Math.exp(Math.max(-80, Math.min(80, ev.deltaY)) * 0.01), p.x, p.y);
    }, { passive: false });
    // Touch: one finger pans like a drag, two pinch (tpPure.pinchView); the
    // fingers down are kept by pointer id, in the order they came down.
    const fingers = new Map();
    let pinch = null;
    const inBox = (ev) => {
      const r = el.getBoundingClientRect();
      return { x: ev.clientX - r.left, y: ev.clientY - r.top };
    };
    el.addEventListener("pointerdown", (ev) => {
      if (!v) return;
      if (ev.pointerType === "touch") fingers.set(ev.pointerId, inBox(ev));
      const act = tpPure.pointerAction(ev.pointerType === "touch" ? fingers.size : 0, ev);
      if (act === "none") return;
      if (act === "pinch") {
        const [a, b] = [...fingers.values()];
        pinch = { a, b, v: { ...v } };
        drag = null;
        moved = true;
        return;
      }
      drag = { x: ev.clientX, y: ev.clientY, v: { ...v } };
      moved = false;
    });
    el.addEventListener("pointermove", (ev) => {
      if (fingers.has(ev.pointerId)) fingers.set(ev.pointerId, inBox(ev));
      if (pinch && fingers.size >= 2) {
        const r = el.getBoundingClientRect();
        const [a, b] = [...fingers.values()];
        v = tpPure.pinchView(pinch.v, pinch.a, pinch.b, a, b, r.width, r.height);
        apply();
        return;
      }
      if (drag && (moved || Math.abs(ev.clientX - drag.x) + Math.abs(ev.clientY - drag.y) > 3)) {
        if (!moved) { moved = true; el.classList.add("drag"); el.setPointerCapture(ev.pointerId); }
        const r = el.getBoundingClientRect();
        v = { ...drag.v, x: drag.v.x - (ev.clientX - drag.x) / r.width * drag.v.w, y: drag.v.y - (ev.clientY - drag.y) / r.height * drag.v.h };
        apply();
        $("tip").hidden = true;
        return;
      }
      if (ev.pointerType !== "touch") hooks.hover(ev);
    });
    const lift = (ev) => {
      fingers.delete(ev.pointerId);
      if (fingers.size < 2) pinch = null;
      el.classList.remove("drag");
      const wasDrag = !drag || moved;
      drag = null;
      return wasDrag;
    };
    el.addEventListener("pointerup", (ev) => {
      const pinched = !!pinch;
      if (!lift(ev) && !pinched) hooks.click(ev);
    });
    el.addEventListener("pointercancel", lift);
    el.addEventListener("pointerleave", () => { $("tip").hidden = true; });
    hooks.card.querySelectorAll("[data-z]").forEach((b) => b.addEventListener("click", () => {
      const z = b.dataset.z;
      if (z === "fit" || !v) fit();
      else zoomAt(z === "in" ? 1 / 1.5 : 1.5, v.x + v.w / 2, v.y + v.h / 2);
    }));
    // A resize, as a drawer opening beside the page, keeps the scale: the
    // view widens or narrows with the box instead of shrinking the drawing.
    let boxW = 0;
    new ResizeObserver(() => {
      const r = el.getBoundingClientRect();
      if (!r.width || !r.height) return;
      if (!v) fit();
      else {
        const w = boxW ? v.w * r.width / boxW : v.w;
        v = { ...v, w, h: w * r.height / r.width };
        apply();
      }
      boxW = r.width;
    }).observe(el);
    return {
      // centre takes the view to the drawing point p (tpPure.centreView),
      // once the layout settles, as an opening drawer narrows the map.
      centre(p) {
        requestAnimationFrame(() => {
          const r = el.getBoundingClientRect();
          if (!v || !p || !r.width) return;
          v = tpPure.centreView({ ...v, h: v.w * r.height / r.width }, p, r.width);
          apply();
        });
      },
      // reveal centres p only when it lies outside the view.
      reveal(p) {
        if (v && p && (p.x < v.x || p.x > v.x + v.w || p.y < v.y || p.y > v.y + v.h)) this.centre(p);
      },
      // bounds takes the drawing's size; the first drawing, and any when
      // refit is set, is fitted, later ones keep the reader's view.
      bounds(W, H, refit) {
        const first = size.W === 1 && size.H === 1;
        size = { W, H };
        if (first || refit || !v) fit();
        else apply();
      },
    };
  }
  const mapPort = makePort($("map"), $("mapsvg"), { card: $("mapcard"), first: tpPure.initialView, hover: (ev) => mapHover(ev), click: (ev) => mapClick(ev) });

  // Full screen (dashboard-ux.md §2): a card's button asks the Fullscreen
  // API for the page and marks the card to fill it, so the drawer still opens
  // beside it; Esc, which the browser handles, leaves it. Where the API is
  // missing the card fills the window alone and Esc unmarks it. While a card
  // fills the screen the plain wheel zooms it.
  function setFullScreen(card) {
    document.querySelectorAll(".card.fs").forEach((c) => { if (c !== card) c.classList.remove("fs"); });
    if (card) card.classList.add("fs");
    document.body.classList.toggle("mapfs", !!card);
    document.querySelectorAll("[data-fs]").forEach((b) => {
      const on = !!card && b.dataset.fs === card.id;
      b.setAttribute("aria-pressed", String(on));
      b.textContent = on ? "Exit full screen" : "Full screen";
    });
  }
  document.addEventListener("click", (ev) => {
    const b = ev.target.closest && ev.target.closest("[data-fs]");
    if (!b) return;
    const card = $(b.dataset.fs);
    if (card.classList.contains("fs")) {
      if (document.fullscreenElement) document.exitFullscreen();
      setFullScreen(null);
      return;
    }
    setFullScreen(card);
    const root = document.documentElement;
    if (root.requestFullscreen && !document.fullscreenElement) root.requestFullscreen().catch(() => { /* the card fills the window alone */ });
  });
  document.addEventListener("fullscreenchange", () => { if (!document.fullscreenElement) setFullScreen(null); });
  document.addEventListener("keydown", (ev) => { if (ev.key === "Escape" && !document.fullscreenElement && document.body.classList.contains("mapfs") && $("drawer").hidden) setFullScreen(null); });

  // mapHit reads everything under the pointer: an epic's mark or id wins,
  // then a neighbour edge, then the rest of an epic's row, so an edge drawn
  // across a row is still found and an epic under an edge still answers.
  function mapHit(ev) {
    const els = document.elementsFromPoint(ev.clientX, ev.clientY).filter((el) => el.closest && el.closest("#mapsvg"));
    const nodeOf = (el) => el.closest("#mapsvg .node");
    const mark = els.find((el) => nodeOf(el) && !el.classList.contains("rowhit"));
    if (mark) return { node: nodeOf(mark) };
    const edge = els.map((el) => el.closest("#mapsvg .nbedge")).find(Boolean);
    if (edge) return { edge };
    const row = els.find(nodeOf);
    return { node: row ? nodeOf(row) : null };
  }
  // edgeTip is a neighbour edge's hover text: its direction, its two ends and
  // the why the graph gives for it.
  function edgeTip(d) {
    return '<span class="tid">' + esc(NB_DIR[d.dir]) + ": " + esc(d.to) + " depends on " + esc(d.from) + "</span><br>" +
      (d.why ? esc(d.why) : '<span class="tid">no why recorded</span>') + (d.group ? "<br>one of {" + esc(d.group) + "}" : "");
  }
  function mapHover(ev) {
    const { node: n, edge } = mapHit(ev);
    const tip = $("tip");
    const e = n && mapView.byId[n.dataset.id];
    if (!e && !edge) { tip.hidden = true; return; }
    if (edge) tip.innerHTML = edgeTip(edge.dataset);
    else {
      const s = markState(e, mapView.geo.byMs[e.milestone]);
      tip.innerHTML = '<span class="tid">' + esc(e.id) + ", milestone " + esc(e.milestone) + (e.condition ? ", condition" : "") + "</span><br>" + esc(e.title) +
        "<br>" + esc(MARK_LABEL[s]) + ", " + esc(e.status.replace("_", " "));
    }
    tip.hidden = false;
    tip.style.left = Math.min(ev.clientX + 14, window.innerWidth - 310) + "px";
    tip.style.top = (ev.clientY + 14) + "px";
  }
  function mapClick(ev) {
    const { node: n, edge } = mapHit(ev);
    if (edge) return;
    selectEpic(n ? n.dataset.id : null);
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
  // The search dims what does not match and centres its first match; Enter
  // selects that match and brings the map into view.
  const firstMatch = () => {
    const q = fold(mapView.q.trim());
    return q && mapView.geo ? mapView.geo.placed.find((e) => searchHit(e, q)) || null : null;
  };
  $("map-q").addEventListener("input", (ev) => {
    mapView.q = ev.target.value;
    drawMap();
    const hit = firstMatch();
    if (hit) mapPort.centre(mapView.geo.pos[hit.id]);
  });
  $("map-q").addEventListener("keydown", (ev) => {
    const hit = ev.key === "Enter" && firstMatch();
    if (!hit) return;
    ev.preventDefault();
    mapView.selected = null;
    selectEpic(hit.id, true);
    $("mapcard").scrollIntoView({ block: "nearest" });
  });
  $("f-edges").addEventListener("click", () => {
    mapView.edges = !mapView.edges;
    $("f-edges").setAttribute("aria-pressed", String(mapView.edges));
    drawMap();
  });
  $("f-nb").addEventListener("click", () => {
    mapView.nb = !mapView.nb;
    $("f-nb").setAttribute("aria-pressed", String(mapView.nb));
    drawMap();
  });
  $("maplegend").innerHTML = MARKS.map((s) => '<span><svg viewBox="0 0 15 15" aria-hidden="true">' + mark(false, s, 7.5, 7.5, 4.6) + "</svg>" + esc(MARK_LABEL[s]) + "</span>").join("") +
    '<span><svg viewBox="0 0 15 15" aria-hidden="true">' + mark(true, "ready", 7.5, 7.5, 4.4) + "</svg>condition</span>" +
    '<span><svg viewBox="0 0 15 15" aria-hidden="true"><path d="M1 7.5H14" stroke="var(--crit)" stroke-width="2.6"/></svg>critical path</span>' +
    '<span><svg viewBox="0 0 15 15" aria-hidden="true"><path d="M1 7.5H14" stroke="var(--accent)" stroke-dasharray="3 2"/></svg>one of a group</span>' +
    '<span><b class="dmark" aria-hidden="true">¶</b>has a description</span>' +
    '<span><svg viewBox="0 0 15 15" aria-hidden="true"><path class="edge in" d="M1 7.5H14"/></svg>in: what it depends on</span>' +
    '<span><svg viewBox="0 0 15 15" aria-hidden="true"><path class="edge out" d="M1 7.5H14"/></svg>out: what depends on it</span>' +
    '<span class="help">Click an epic for its chain, again to clear; Direct only narrows it to one step each way. Drag to pan; Ctrl or ⌘ with the wheel, or a pinch, zooms; in full screen the wheel alone zooms and Esc leaves.</span>';

  // ---------- the selected epic: its chain, its drawer (§4, §11) ----------
  // Selecting an epic on the map, in a now list or in the drawer draws its
  // chain with every edge's why and opens its drawer: its fields, what it
  // depends on and what depends on it with the whys, downstream, its bound
  // specs, its own stepper and its graph history, fetched from
  // /api/epic-history when it opens and again on every push.
  const epicHist = { id: null, data: null, error: null, seq: 0 };
  INFO.matrix = "Row depends on column: each number counts the dependency edges from the row milestone's epics to the column's, each member of an any-of group one, and its colour says how many of those dependencies are done. The diagonal is the milestone's own share of work epics done. From the summary's matrix and the export's milestones.";
  INFO.drawer = "One epic from tp epic export and the graph: its fields, what it depends on and what depends on it with the whys, downstream (the not-done work epics that wait on it), the specs bound to it with their phase and loops, its stepper (claimed, with who claimed it, or skipped when it was bound unclaimed; bound; done), and its graph history from tp epic review --history --of-epic.";
  const ESTEPS = [["claimed", "Claimed"], ["bound", "Bound"], ["done", "Done"]];
  const PHASE_NAME = Object.fromEntries(PHASES);

  // selectEpic selects id, or clears the selection when id is the selected
  // epic or null; centre takes the map to it, as a selection made outside
  // the map does.
  // opener is what held focus when a selection opens the drawer, read before
  // the redraws below replace it; drawDrawer takes it.
  let opener = null;
  function selectEpic(id, centre) {
    if ($("drawer").hidden) opener = { el: document.activeElement, key: focusKey(document.activeElement) };
    mapView.selected = id && id !== mapView.selected ? id : null;
    // A selection shows its neighbourhood unless a gate's closure is shown.
    if (mapView.selected && gview.view === "milestones") gview.view = null;
    drawMap();
    drawSelected();
    if (centre && mapView.selected && mapView.geo) mapPort.centre(mapView.geo.pos[mapView.selected]);
  }

  const graphEpic = (g, id) => ((g.graph && g.graph.epics) || []).find((e) => e.id === id) || null;
  const epicLink = (id) => '<button type="button" class="link" data-epic="' + esc(id) + '">' + esc(id) + "</button>";
  const stamp = (iso) => (iso ? String(iso).replace("T", " ").replace(/\..*$|Z$/, "").slice(0, 16) + " UTC" : "");

  // chainEdges walks the selected epic's edges one way, breadth first: up,
  // to what it depends on, or down, to what depends on it.
  function chainEdges(id, up) {
    const seen = new Set([id]);
    const out = [];
    let front = [id];
    while (front.length) {
      const next = [];
      front.forEach((x) => {
        const edges = up ? (mapView.deps[x] || []).map((d) => ({ from: x, to: d.id, why: d.why, group: d.group }))
          : [...new Set(mapView.dependents[x] || [])].flatMap((y) => (mapView.deps[y] || []).filter((d) => d.id === x).map((d) => ({ from: y, to: x, why: d.why, group: d.group })));
        edges.forEach((e) => {
          out.push(e);
          const n = up ? e.to : e.from;
          if (!seen.has(n)) { seen.add(n); next.push(n); }
        });
      });
      front = next;
    }
    return out;
  }

  function edgeList(edges, empty) {
    if (!edges.length) return none(empty);
    return '<ul class="elist">' + edges.map((e) => "<li>" + epicLink(e.from) + ' <span class="muted">depends on</span> ' + epicLink(e.to) +
      (e.group ? ' <span class="esub">one of {' + esc(e.group.join(", ")) + "}</span>" : "") +
      '<span class="dwhy">' + (e.why ? esc(e.why) : "no why recorded") + "</span></li>").join("") + "</ul>";
  }

  // drawChain draws the selected epic's chain as text, the collapsible view
  // under the dependency views.
  function drawChain(g) {
    $("chaincard").hidden = !g;
    if (!g) return;
    const id = mapView.selected;
    $("chain").innerHTML = id && mapView.deps ? '<p class="note">' + esc(id) + ": " + esc((mapView.byId[id] || {}).title || "") + "</p>" +
      '<div class="two-col"><div><h3 class="hsub">What it waits on</h3>' + edgeList(chainEdges(id, true), "It depends on no epic.") + "</div>" +
      '<div><h3 class="hsub">What waits on it</h3>' + edgeList(chainEdges(id, false), "No epic depends on it.") + "</div></div>"
      : none("Select an epic on the map, or in a list above, to see its chain.");
  }

  // ---------- the dependency views (dashboard-ux.md §2) ----------
  // tp lays each view out at /api/layout: the milestones and how they open,
  // a gate's closure banded by milestone, or the selected epic's
  // neighbourhood. The page turns layers and orders into points
  // (tpPure.placeLayout) and draws them; it computes nothing about the graph.
  const GVIEWS = ["milestones", "scope", "neighbourhood"];
  const GVIEW_LABEL = { milestones: "the milestones", scope: "a gate's closure", neighbourhood: "an epic's neighbourhood" };
  const GDIMS = {
    milestones: { colW: 240, rowH: 84, left: 16, top: 12, head: 0, inset: 0, box: 196 },
    scope: { colW: 200, rowH: 26, left: 16, top: 12, head: 30, inset: 12 },
    neighbourhood: { colW: 210, rowH: 28, left: 16, top: 12, head: 0, inset: 12 },
  };
  const gview = { view: null, milestone: null, depth: 2, data: null, dataKey: null, error: null, key: null, drawn: null, seen: null, seq: 0, pos: {}, focus: null };
  INFO.chain = "The plan's dependencies in three views tp lays out, each read left to right, what a node depends on to its left. Milestones: each milestone with its work epics done of all, the dependency edges into it from other milestones' epics, and an edge from each milestone it opens after; click one for its gate's closure. Gate closure: what the milestone's gate pulls in (tp epic scope), banded by milestone, its critical path in orange, the alternatives of an any-of group drawn as one dashed fork. Neighbourhood: the selected epic in the middle, what it depends on to the left and what depends on it to the right, to the depth picked; clicking an epic re-centres on it. Selecting an epic highlights its edges and dims the rest. An edge that skips layers is a plain curve. The chain as text lists every edge with its why.";

  // gviewNow is the view shown: the one picked, else the selected epic's
  // neighbourhood, else the milestones.
  const gviewNow = () => gview.view || (mapView.selected ? "neighbourhood" : "milestones");

  // gviewMilestone is the closure's milestone: the one picked, else the
  // first open milestone whose work is not all done, else the first.
  function gviewMilestone(g) {
    const ms = g.milestones || [];
    const pick = ms.find((m) => m.milestone === gview.milestone) || ms.find((m) => m.open && m.share_done < 1) || ms[0];
    return pick ? pick.milestone : null;
  }

  // layoutQuery is the view's /api/layout query, or null while the view
  // needs a selection it does not have.
  function layoutQuery(g) {
    const view = gviewNow();
    const p = new URLSearchParams({ view });
    if (view === "scope") {
      const m = gviewMilestone(g);
      if (!m) return null;
      p.set("milestone", m);
    } else if (view === "neighbourhood") {
      if (!mapView.selected) return null;
      p.set("epic", mapView.selected);
      p.set("depth", String(gview.depth));
    }
    return p;
  }

  // fetchLayout fetches key's layout, clearing the error of the key before;
  // what it holds is drawn only once dataKey says it came for key.
  async function fetchLayout(key) {
    const seq = ++gview.seq;
    Object.assign(gview, { key, error: null });
    let body;
    try {
      const res = await fetch("/api/layout?" + key, { cache: "no-store" });
      body = await res.json();
    } catch (err) {
      body = { error: "the layout could not be fetched: " + err.message, hint: "check that tp dashboard is still running" };
    }
    if (seq !== gview.seq) return;
    Object.assign(gview, isError(body) ? { data: null, dataKey: null, error: body } : { data: body, dataKey: key, error: null });
    drawGraph();
  }

  // graphPanel fetches the view when its query or the graph changed, then
  // draws what it holds.
  function graphPanel() {
    const g = data("epic-export");
    if (!state.graph || !g) return drawGraph();
    const q = layoutQuery(g);
    const key = q ? q.toString() : null;
    if (key && (key !== gview.key || gview.seen !== g)) {
      gview.seen = g;
      fetchLayout(key);
    }
    if (!key) Object.assign(gview, { key: null, data: null, dataKey: null, error: null });
    drawGraph();
  }
  panel(["epic-export"], graphPanel);

  // gedgePath draws one edge from the right of its dependency to the left of
  // its dependent; an any-of member's edge ends at its group's fork.
  function gedgePath(p, q, view, fork) {
    const d = GDIMS[view];
    const a = view === "milestones" ? { x: p.x + d.box, y: p.y } : { x: p.x + d.colW - d.inset - 40, y: p.y };
    const b = view === "milestones" ? { x: q.x, y: q.y } : { x: q.x - (fork ? 26 : 8), y: q.y };
    return edgePath(a, b);
  }

  // gnodeHTML draws one node: a milestone as a box with its counts, an epic
  // as its mark and id.
  function gnodeHTML(n, p, view, ctx, stop) {
    const cls = "gnode" + (ctx.near && !ctx.near.has(n.id) ? " dim" : "") + (n.centre ? " centre" : "");
    const tab = ' role="button" tabindex="' + stop + '"';
    if (view === "milestones") {
      const d = GDIMS.milestones;
      const y = p.y - (d.rowH - 16) / 2;
      return '<g class="' + cls + '" data-id="' + esc(n.id) + '"' + tab + ' aria-label="' + esc(n.id + ": " + (n.open ? "open" : "closed") + ", " + n.done + " of " + n.epics + " work epics done, " + n.inbound + " edges in") + '">' +
        '<rect class="msbox' + (n.open ? "" : " closed") + '" x="' + p.x + '" y="' + y + '" width="' + d.box + '" height="' + (d.rowH - 16) + '" rx="7"/>' +
        '<text class="msname" x="' + (p.x + 12) + '" y="' + (y + 21) + '">' + esc(n.id) + '</text><text class="msshare" x="' + (p.x + d.box - 12) + '" y="' + (y + 21) + '" text-anchor="end">' + (n.open ? "open" : "closed") + "</text>" +
        '<text class="gsub" x="' + (p.x + 12) + '" y="' + (y + 40) + '">' + esc(n.done + " of " + plural(n.epics, "epic", "epics") + " done") + "</text>" +
        '<text class="gsub" x="' + (p.x + 12) + '" y="' + (y + 56) + '">' + esc(plural(n.inbound, "edge", "edges") + " in") + "</text></g>";
    }
    const e = ctx.epics[n.id] || { id: n.id, status: "planned" };
    const s = markState(e, { report: ctx.ms[e.milestone] });
    return '<g class="' + cls + '" data-id="' + esc(n.id) + '"' + tab + ' aria-label="' + esc(n.id + ": " + (e.title || "") + ", " + MARK_LABEL[s]) + '">' +
      (n.id === mapView.selected ? '<circle cx="' + p.x + '" cy="' + p.y + '" r="10" fill="none" stroke="var(--accent)" stroke-width="2.2"/>' : "") +
      mark(e.condition, s, p.x, p.y, 4.8) + '<text class="eid" x="' + (p.x + 12) + '" y="' + (p.y + 4) + '">' + esc(clip(n.id, 22)) + "</text>" +
      descMark(ctx.described.has(n.id), p.x + 12, p.y + 4, clip(n.id, 22).length) +
      '<rect class="rowhit" x="' + (p.x - 9) + '" y="' + (p.y - 11) + '" width="' + (GDIMS[view].colW - 40) + '" height="22" fill="transparent"/></g>';
  }

  // gedgesHTML draws the edges: the critical path's in orange, an any-of
  // group's members dashed into one fork before their dependent, and with an
  // epic selected its edges highlighted and the rest dimmed.
  function gedgesHTML(L, geo, hl) {
    const out = [];
    const forks = new Set();
    L.edges.forEach((e) => {
      const p = geo.pos[e.from];
      const q = geo.pos[e.to];
      if (!p || !q) return;
      const mine = hl && (e.from === hl || e.to === hl);
      const cls = "gedge" + (e.critical ? " crit" : "") + (e.any_of ? " any" : "") + (hl ? (mine ? " hl" : " dimmed") : "");
      out.push('<path class="' + cls + '" d="' + gedgePath(p, q, L.view, !!e.any_of) + '"/>');
      const fork = e.any_of && e.to + "\u0000" + e.any_of.join("\u0000");
      if (fork && !forks.has(fork)) {
        forks.add(fork);
        out.push('<path class="gedge any fork' + (hl ? (mine ? " hl" : " dimmed") : "") + '" d="M' + (q.x - 26) + " " + q.y + "H" + (q.x - 8) + '"/><circle class="gfork" cx="' + (q.x - 26) + '" cy="' + q.y + '" r="2.5"/>');
      }
    });
    return out.join("");
  }

  // drawGraph draws the view gview holds, refitting it when it is another
  // view than the one drawn last.
  function drawGraph() {
    const g = data("epic-export");
    const card = $("chaincard");
    card.hidden = !state.graph || !g;
    if (card.hidden) return;
    const view = gviewNow();
    card.querySelectorAll("[data-gv]").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.gv === view)));
    $("gm-pick").hidden = view !== "scope";
    $("gd-pick").hidden = view !== "neighbourhood";
    const order = [].concat(...((g.graph && g.graph.milestones) || []));
    if ($("gm").dataset.key !== order.join("\u0000")) {
      $("gm").dataset.key = order.join("\u0000");
      $("gm").innerHTML = order.map((m) => "<option>" + esc(m) + "</option>").join("");
    }
    $("gm").value = gviewMilestone(g) || "";
    $("gd").value = String(gview.depth);
    const L = gview.data;
    const say = (text) => { $("gsvg").innerHTML = ""; $("g-n").textContent = text; };
    if (!layoutQuery(g)) return say("Select an epic on the map or in a list to see its neighbourhood.");
    const ready = tpPure.layoutReady(gview);
    if (ready === "error") return say(gview.error.error);
    if (ready !== "draw") return say("Reading " + GVIEW_LABEL[view] + " from tp.");
    const geo = tpPure.placeLayout(L, GDIMS[view]);
    gview.pos = geo.pos;
    if (!geo.pos[gview.focus]) gview.focus = (L.nodes.find((n) => n.centre) || L.nodes[0] || {}).id || null;
    const epics = {};
    (g.epics || []).forEach((e) => { epics[e.id] = e; });
    const ms = {};
    (g.milestones || []).forEach((m) => { ms[m.milestone] = m; });
    const hl = view !== "milestones" && geo.pos[mapView.selected] ? mapView.selected : null;
    const near = hl ? new Set([hl].concat(L.edges.filter((e) => e.from === hl || e.to === hl).flatMap((e) => [e.from, e.to]))) : null;
    const hadFocus = $("gsvg").contains(document.activeElement);
    const top = GDIMS[view].top;
    $("gsvg").innerHTML = (L.bands || []).length ? geo.bands.map((b, i) => '<rect class="gband' + (i % 2 ? " odd" : "") + '" x="' + (b.x + 2) + '" y="' + top + '" width="' + (b.w - 8) + '" height="' + (geo.H - 2 * top) + '" rx="7"/>' +
      '<text class="msname" x="' + (b.x + 12) + '" y="' + (top + 20) + '">' + esc(b.name) + "</text>").join("") : "";
    const ctx = { epics, ms, near, described: describedIds(g) };
    const stops = tpPure.tabStops(L.nodes.map((n) => n.id), gview.focus);
    $("gsvg").innerHTML += "<g>" + gedgesHTML(L, geo, hl) + "</g>" + L.nodes.map((n, i) => gnodeHTML(n, geo.pos[n.id], view, ctx, stops[i])).join("");
    $("g-n").textContent = plural(L.nodes.length, view === "milestones" ? "milestone" : "epic", view === "milestones" ? "milestones" : "epics") +
      (L.view === "scope" ? " in " + L.milestone + "'s closure" : L.view === "neighbourhood" ? " within " + plural(L.depth, "step", "steps") + " of " + L.epic : "");
    if (hadFocus) gfocus(gview.focus);
    const key = gview.key;
    gport.bounds(geo.W, geo.H, gview.drawn !== key);
    if (gview.drawn !== key && view === "neighbourhood") gport.centre(geo.pos[L.epic]);
    else if (hl && (gview.drawn !== key || gview.hl !== hl)) gport.reveal(geo.pos[hl]);
    gview.drawn = key;
    gview.hl = hl;
  }

  // gnodeAt is the dependency view's node under the pointer.
  const gnodeAt = (ev) => {
    const el = document.elementFromPoint(ev.clientX, ev.clientY);
    return el && el.closest ? el.closest("#gsvg .gnode") : null;
  };
  // gpick answers a node picked by click or key: a milestone opens its
  // gate's closure; an epic is selected, its neighbourhood re-centred on it.
  function gpick(id) {
    if (gviewNow() === "milestones") {
      Object.assign(gview, { view: "scope", milestone: id });
      syncURL();
      graphPanel();
      return;
    }
    if (mapView.selected !== id) {
      mapView.selected = null;
      selectEpic(id, true);
    }
  }
  function gfocus(id) {
    const node = id && [...$("gsvg").querySelectorAll(".gnode")].find((n) => n.dataset.id === id);
    if (!node) return;
    $("gsvg").querySelectorAll('.gnode[tabindex="0"]').forEach((n) => n.setAttribute("tabindex", "-1"));
    node.setAttribute("tabindex", "0");
    gview.focus = id;
    node.focus({ preventScroll: true });
  }
  const gport = makePort($("gmap"), $("gsvg"), {
    card: $("chaincard"),
    first: tpPure.containView,
    hover(ev) {
      const n = gnodeAt(ev);
      const tip = $("tip");
      const L = gview.data;
      const node = n && L ? L.nodes.find((x) => x.id === n.dataset.id) : null;
      if (!node) { tip.hidden = true; return; }
      const e = ((data("epic-export") || {}).epics || []).find((x) => x.id === node.id);
      tip.innerHTML = L.view === "milestones" ? '<span class="tid">' + esc(node.id) + "</span><br>" + esc((node.open ? "open" : "closed") + ", " + node.done + " of " + node.epics + " work epics done") +
        "<br>" + esc(plural(node.inbound, "dependency edge", "dependency edges") + " from other milestones' epics") :
        '<span class="tid">' + esc(node.id) + ", milestone " + esc(e ? e.milestone : node.milestone || "") + "</span><br>" + esc(e ? e.title : "") +
        [...new Set(L.edges.filter((x) => x.from === node.id && x.any_of).map((x) => x.any_of.join(", ")))].map((g) => "<br>one of {" + esc(g) + "}").join("") +
        (node.deferrable ? "<br>deferrable" : "");
      tip.hidden = false;
      tip.style.left = Math.min(ev.clientX + 14, window.innerWidth - 310) + "px";
      tip.style.top = (ev.clientY + 14) + "px";
    },
    click(ev) {
      const n = gnodeAt(ev);
      if (n) gpick(n.dataset.id);
    },
  });
  $("gsvg").addEventListener("keydown", (ev) => {
    const node = ev.target.closest && ev.target.closest(".gnode");
    if (!node) return;
    if (ev.key === "Enter" || ev.key === " ") {
      ev.preventDefault();
      gpick(node.dataset.id);
      return;
    }
    const next = tpPure.roveNext(gview.pos, node.dataset.id, ev.key);
    if (next === node.dataset.id) return;
    ev.preventDefault();
    gfocus(next);
    gport.reveal(gview.pos[next]);
  });
  $("chaincard").addEventListener("click", (ev) => {
    const b = ev.target.closest("[data-gv]");
    if (!b) return;
    gview.view = b.dataset.gv;
    syncURL();
    graphPanel();
  });
  $("gm").addEventListener("change", (ev) => { Object.assign(gview, { view: "scope", milestone: ev.target.value }); syncURL(); graphPanel(); });
  $("gd").addEventListener("change", (ev) => { gview.depth = Number(ev.target.value); syncURL(); graphPanel(); });
  $("glegend").innerHTML = '<span><svg viewBox="0 0 15 15" aria-hidden="true"><path d="M1 7.5H14" stroke="var(--crit)" stroke-width="2.6"/></svg>critical path</span>' +
    '<span><svg viewBox="0 0 15 15" aria-hidden="true"><path d="M1 3L9 7.5M1 12L9 7.5M9 7.5H14" stroke="var(--accent)" stroke-dasharray="2 1.5" fill="none"/></svg>one of an any-of group</span>' +
    '<span><svg viewBox="0 0 15 15" aria-hidden="true"><path d="M1 7.5H14" stroke="var(--accent)" stroke-width="2"/></svg>the selected epic\'s edges</span>' +
    '<span class="help">Milestones: click one for its gate closure. Gate closure: banded by milestone; click an epic to select it. Neighbourhood: what it depends on to the left, what depends on it to the right; click an epic to re-centre on it. Drag or one finger pans; Ctrl or ⌘ with the wheel, or a pinch, zooms.</span>';

  // drawMatrix draws the summary's milestone matrix in track order.
  function drawMatrix(g, sum) {
    const card = $("matrixcard");
    card.hidden = !g || !sum || !sum.matrix;
    if (card.hidden) return;
    const order = [].concat(...((g.graph && g.graph.milestones) || []));
    const rep = {};
    (g.milestones || []).forEach((m) => { rep[m.milestone] = m; });
    const cell = {};
    sum.matrix.forEach((c) => { cell[c.from + "\u0000" + c.to] = c; });
    const tint = (v, p) => "background:color-mix(in srgb, var(--" + v + ") " + p + "%, var(--surface))";
    const mst = (m) => { const r = rep[m] || {}; return r.share_done === 1 ? "done" : r.open ? "active" : "waiting"; };
    const head = (m) => ' style="' + tint(mst(m), 24) + '" title="' + esc(m + ": " + ((rep[m] || {}).open === false ? "closed" : "open") + ", " + Math.round(((rep[m] || {}).share_done || 0) * 100) + "% of work epics done") + '"';
    if (!order.length) {
      $("matrix").innerHTML = none("The graph names no milestone.");
      return;
    }
    $("matrix").innerHTML = '<table class="matrix"><tr><th></th>' + order.map((m) => "<th" + head(m) + ">" + esc(m) + "</th>").join("") + "</tr>" +
      order.map((m) => '<tr><th class="r"' + head(m) + ">" + esc(m) + "</th>" + order.map((o) => {
        if (m === o) return '<td class="diag" style="' + tint(mst(m), 40) + '" title="' + esc(m + ": " + Math.round(((rep[m] || {}).share_done || 0) * 100) + "% of work epics done") + '">' + Math.round(((rep[m] || {}).share_done || 0) * 100) + "</td>";
        const c = cell[m + "\u0000" + o];
        if (!c) return "<td></td>";
        const v = c.done === c.edges ? "done" : c.done ? "active" : (rep[m] || {}).open ? "blocked" : "waiting";
        return '<td style="' + tint(v, Math.min(58, 26 + c.edges * 3)) + '" title="' + esc(m + " depends on " + o + ": " + plural(c.edges, "edge", "edges") + ", " + c.done + " done") + '">' + c.edges + "</td>";
      }).join("") + "</tr>").join("") + "</table>" +
      '<div class="mkey">' + [["done", "every dependency done"], ["active", "some done"], ["blocked", "none done, row open"], ["waiting", "none done, row closed"]].map(([v, t]) => '<span><i style="' + tint(v, 50) + '"></i>' + esc(t) + "</span>").join("") + "<span>diagonal: the milestone's own % done</span></div>";
  }

  // epicStepper is the epic's own stepper: claimed (who claimed it, or
  // skipped when it was bound or done unclaimed), bound, done.
  function epicStepper(e, ge) {
    const claim = ge && ge.claimed;
    const bound = (e.files || []).filter((f) => f.bound_at).map((f) => f.bound_at).sort()[0] || null;
    const reached = { claimed: !!claim || ["specced", "in_progress", "done"].includes(e.status), bound: (e.files || []).length > 0, done: e.status === "done" };
    const sub = {
      claimed: claim ? "by " + claim.by + (claim.at ? ", " + stamp(claim.at) : "") : reached.claimed ? "skipped" : "not claimed",
      bound: reached.bound ? plural(e.files.length, "spec", "specs") + (bound ? ", " + stamp(bound) : "") : "no spec yet",
      done: reached.done ? (e.done_at ? stamp(e.done_at) : "done") : reached.bound ? "when its specs release" : "not yet",
    };
    const cur = ESTEPS.findIndex(([k]) => !reached[k]);
    return '<ol class="stepper e3" aria-label="Epic lifecycle">' + ESTEPS.map(([k, label], i) => {
      let st = reached[k] ? "done" : i === cur ? "cur" : "todo";
      if (k === "claimed" && st === "done" && !claim) st = "skip";
      const mk = st === "done" ? CHECK : st === "skip" ? "–" : String(i + 1);
      return '<li class="' + st + '"' + (st === "cur" ? ' aria-current="step"' : "") + '><span class="sn">' + mk + '</span><span class="sl">' + label + '</span><span class="sd">' + esc(sub[k]) + "</span></li>";
    }).join("") + "</ol>";
  }

  // epicHistoryHTML draws the --of-epic payload: per slice, each recorded
  // round's edges of the epic with their whys, as that round's snapshot kept
  // them.
  function epicHistoryHTML(id) {
    if (epicHist.id !== id || (!epicHist.data && !epicHist.error)) return none("Reading its graph history.");
    if (epicHist.error) return '<div class="banner"><b>Its graph history could not be read</b><span>' + esc(epicHist.error.error) + "</span>" + (epicHist.error.hint ? '<span class="hint">' + esc(epicHist.error.hint) + "</span>" : "") + "</div>";
    const h = epicHist.data;
    const known = (x) => (((data("epic-export") || {}).epics) || []).some((e) => e.id === x);
    if (!(h.slices || []).length) return none(h.message || "It is in no recorded slice, so it has no graph history.");
    return h.slices.map((s) => '<div class="ehs"><h5><code>' + esc(sliceLabel({ key: s.key, dir: s.state_dir })) + "</code></h5><ol class=\"ehr\">" + (s.rounds || []).map((r) => "<li><b>Round " + esc(r.round) + "</b>" +
      (r.hashes_only ? '<span class="muted sm">this round kept unit hashes only: no edge or why text was kept</span>'
        : (r.edges || []).length ? '<ul class="elist">' + r.edges.map((x) => "<li>depends on " + (known(x.to) ? epicLink(x.to) : esc(x.to)) + '<span class="dwhy">' + (x.why ? esc(x.why) : "no why recorded") + "</span></li>").join("") + "</ul>"
          : '<span class="muted sm">no dependency</span>') + "</li>").join("") + "</ol></div>").join("");
  }

  async function fetchEpicHistory(id) {
    const seq = ++epicHist.seq;
    let body;
    try {
      const res = await fetch("/api/epic-history?epic=" + encodeURIComponent(id), { cache: "no-store" });
      body = await res.json();
    } catch (err) {
      body = { error: "tp epic review --history --of-epic could not be fetched: " + err.message, hint: "check that tp dashboard is still running" };
    }
    if (seq !== epicHist.seq) return;
    Object.assign(epicHist, isError(body) ? { id, error: body } : { id, data: body, error: null });
    drawDrawer();
  }

  function fileLine(f) {
    const t = f.tasks || {};
    const loop = (name, l) => (l && l.round ? name + " round " + l.round + " " + (LOOP_LABEL[l.state] || l.state) : "");
    return '<li><button type="button" class="link" data-spec="' + esc(f.path) + '">' + esc(f.path) + '</button><small>' + esc([PHASE_NAME[f.phase] || f.phase,
      plural((t.done || 0), "task", "tasks") + " done of " + ((t.open || 0) + (t.wip || 0) + (t.done || 0)), loop("ground", f.ground), loop("review", f.review), loop("audit", f.audit)].filter(Boolean).join(", ")) + "</small></li>";
  }

  // focusKey names a focusable element so a redraw's replacement is found
  // again: a map epic, a dependency-view node, a waiting group's link by its
  // group, or a list link by its box; null for anything else.
  const FOCUSABLE = "#mapsvg .node, #gsvg .gnode, [data-epic]";
  function focusKey(el) {
    if (!el || !el.closest) return null;
    const node = el.closest("#mapsvg .node, #gsvg .gnode");
    if (node) return (node.closest("#mapsvg") ? "map:" : "graph:") + node.dataset.id;
    if (!el.dataset || !el.dataset.epic) return null;
    const wg = el.closest("[data-wg]");
    if (wg) return "wait:" + wg.dataset.wg + ":" + el.dataset.epic;
    const box = el.closest("[id]");
    return box ? "link:" + box.id + ":" + el.dataset.epic : null;
  }

  // drawerReturn is what held focus when the drawer opened: the element and
  // its key, to find it again when a redraw replaced it. Null while the
  // drawer is closed.
  let drawerReturn = null;
  function returnFocus() {
    const r = drawerReturn;
    drawerReturn = null;
    const els = [...document.querySelectorAll(FOCUSABLE)];
    const key = tpPure.focusAfterClose({ connected: !!r.el && r.el.isConnected && r.el !== document.body, keys: r.keys }, els.map(focusKey));
    if (key === "same") r.el.focus({ preventScroll: true });
    else if (key && key.startsWith("map:")) focusNode(key.slice(4));
    else if (key && key.startsWith("graph:")) gfocus(key.slice(6));
    else if (key) els.find((x) => focusKey(x) === key).focus({ preventScroll: true });
  }

  function drawDrawer() {
    const g = data("epic-export");
    const id = mapView.selected;
    const e = g && id ? (g.epics || []).find((x) => x.id === id) : null;
    const open = !!e && state.tab === "epics";
    $("drawer").hidden = !open;
    document.body.classList.toggle("dopen", open);
    // Opening takes focus to the close button; closing gives it back.
    if (open && !drawerReturn) {
      const from = opener || { el: document.activeElement, key: focusKey(document.activeElement) };
      drawerReturn = { el: from.el, keys: [from.key].filter(Boolean) };
      opener = null;
      $("dclose").focus({ preventScroll: true });
    }
    if (!open && drawerReturn) returnFocus();
    if (!open) return;
    const ge = graphEpic(g, id) || {};
    const ms = (g.milestones || []).find((m) => m.milestone === e.milestone) || {};
    const crit = ((g.critical_path && g.critical_path.by_count) || []).includes(id);
    const down = e.downstream;
    const deps = (ge.depends_on || []).map((d) => {
      const members = d.any_of || [d.id];
      return "<li>" + (d.any_of ? '<span class="muted">one of</span> ' : "") + members.map((x) => {
        const xe = (g.epics || []).find((y) => y.id === x);
        return epicLink(x) + (xe ? ' <span class="esub">(' + esc((STAGE_LABEL[xe.status] || xe.status).toLowerCase()) + ")</span>" : "");
      }).join(", ") + '<span class="dwhy">' + (d.why ? esc(d.why) : "no why recorded") + "</span></li>";
    }).join("");
    const needs = [...new Set(mapView.dependents[id] || [])].map((y) => {
      const d = (mapView.deps[y] || []).find((x) => x.id === id) || {};
      return "<li>" + epicLink(y) + (d.group ? ' <span class="esub">one of {' + esc(d.group.join(", ")) + "}</span>" : "") + '<span class="dwhy">' + (d.why ? esc(d.why) : "no why recorded") + "</span></li>";
    }).join("");
    const waits = (e.blocked_by || []).concat((e.blocked_by_any_of || []).map((x) => "one of {" + x.join(", ") + "}"));
    const fields = [
      ["Milestone", e.milestone + ", " + (ms.open === false ? "closed" : "open") + (ms.state ? ", gate " + ms.state.members_done + " of " + ms.state.members + " members done" : "")],
      ["Status", (STAGE_LABEL[e.status] || e.status) + (waits.length ? ", waits on " + waits.join(", ") : "")],
      ["Downstream", down ? plural(down.dependents, "work epic waits", "work epics wait") + " on it, " + down.dependents_open + " in open milestones; longest chain " + down.longest_chain : "nothing waits on it"],
      ge.estimate_minutes ? ["Estimate", minutes(ge.estimate_minutes)] : null,
      (ge.tags || []).length ? ["Tags", ge.tags.join(", ")] : null,
      ge.acceptance ? ["Acceptance", ge.acceptance] : null,
      ge.done ? ["Done record", stamp(ge.done.at) + (ge.done.evidence ? ": " + ge.done.evidence : "")] : null,
    ].filter(Boolean);
    $("dbody").innerHTML = '<div><div class="dmeta"><code>' + esc(id) + "</code><span>" + (e.condition ? "condition" : "work epic") + "</span>" + (crit ? '<span class="critpill">on the critical path</span>' : "") + "</div><h3>" + esc(e.title) + "</h3></div>" +
      (e.condition ? '<p class="impact">A condition: it takes no spec, and the operator records it with <code>tp epic done ' + esc(id) + ' --evidence</code>.</p>' : epicStepper(e, ge.claimed !== undefined ? ge : null)) +
      "<dl>" + fields.map(([k, v]) => "<dt>" + esc(k) + "</dt><dd>" + esc(v) + "</dd>").join("") + "</dl>" +
      (String(ge.description || "").trim() ? descHTML(ge.description) : "") +
      (e.condition ? "" : "<div><h4>Specs bound to it</h4>" + ((e.files || []).length ? '<ul class="elist">' + e.files.map(fileLine).join("") + "</ul>" : none("None yet: tp init <spec> --epic " + id + " binds one.")) + "</div>") +
      "<div><h4>Depends on</h4>" + (deps ? '<ul class="elist">' + deps + "</ul>" : none("Nothing.")) + "</div>" +
      "<div><h4>Depended on by</h4>" + (needs ? '<ul class="elist">' + needs + "</ul>" : none("Nothing.")) + "</div>" +
      "<div><h4>Graph history " + info("ehistory", "its graph history") + "</h4>" + epicHistoryHTML(id) + "</div>";
  }
  INFO.ehistory = "How this epic's edges and whys read in each recorded round of every epic review slice whose snapshot holds it, from tp epic review --history --of-epic. A round recorded before snapshots kept their text holds hashes only, so it shows no edge or why.";

  // drawSelected draws what follows the selection: the chain, the drawer and
  // its history, fetched again on every push while the drawer is open.
  function drawSelected() {
    const g = data("epic-export");
    drawChain(state.graph && g ? g : null);
    graphPanel();
    drawMatrix(state.graph ? g : null, data("summary"));
    if (mapView.selected && epicHist.id !== mapView.selected) Object.assign(epicHist, { id: mapView.selected, data: null, error: null });
    drawDrawer();
    if (mapView.selected && state.tab === "epics" && g) fetchEpicHistory(mapView.selected);
  }
  panel(["epic-export", "summary"], drawSelected);

  $("dclose").addEventListener("click", () => selectEpic(null));
  document.addEventListener("keydown", (ev) => { if (ev.key === "Escape" && !$("drawer").hidden && $("pop").hidden) selectEpic(null); });
  document.addEventListener("click", (ev) => {
    const a = ev.target.closest && ev.target.closest("[data-epic]");
    if (a) {
      mapView.selected = null;
      selectEpic(a.dataset.epic, true);
      return;
    }
    const s = ev.target.closest && ev.target.closest("#dbody [data-spec]");
    if (s) openSpec(s.dataset.spec);
  });

  // ---------- the Specs tab (§12) ----------
  // One row per task file, as tp export --list lists them: already in the
  // loop's order, the rows an escalation stops first within a phase. The page
  // groups them under their phase and keeps that order.
  const specsView = { phase: "all", q: "" };
  const STAGE_LABEL = Object.fromEntries(STAGES);
  const LOOP_LABEL = { none: "none", open: "open", stale: "stale", cap_reached: "cap reached", done: "done" };
  const PAUSE = '<svg class="ic" viewBox="0 0 16 16" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M6 4v8M10 4v8"/></svg>';
  const waitsOp = (f) => !!(f.escalations && f.escalations.length);
  INFO.specs = "Every task file git sees, one row each, from tp export --list: grouped by the phase tp resume reports, in the loop's order (review, decompose, implement, audit, release), the ones an operator decision stops first. Operator decisions shows only those. Click a row to open that spec; press / anywhere to filter.";

  // specMatches is the filter: a case-insensitive substring of the title, the
  // path or the epic.
  function specMatches(f, q) {
    return !q || [f.title, f.path, f.epic].some((s) => s && s.toLowerCase().includes(q));
  }

  function loopCell(label, l) {
    const v = l && l.state && l.state !== "none"
      ? '<span class="st" data-s="' + esc(l.state) + '">round ' + esc(l.round) + ", " + esc(LOOP_LABEL[l.state] || l.state) + "</span>"
      : '<span class="muted">none</span>';
    return '<span class="lc"><span>' + label + "</span>" + v + "</span>";
  }

  function specRow(f) {
    const ops = f.escalations || [];
    const t = f.tasks || {};
    const total = (t.open || 0) + (t.wip || 0) + (t.done || 0);
    const tasks = total
      ? '<span class="pm"><span class="meter" aria-hidden="true"><i style="width:' + Math.round((t.done || 0) / total * 100) + '%"></i></span><span class="num">' +
        esc(t.done || 0) + "/" + esc(total) + "</span></span>" + (t.wip ? '<span class="sm muted">' + esc(t.wip) + " in progress</span>" : "")
      : '<span class="muted">none yet</span>';
    const epic = f.epic
      ? '<code>' + esc(f.epic) + "</code>" + (f.epic_state ? '<span class="sm muted">' + esc(STAGE_LABEL[f.epic_state] || f.epic_state) + "</span>" : "")
      : '<span class="muted">no epic</span>';
    const next = ops.map((b) => '<span class="stat" title="' + esc(b.message) + '">' + PAUSE + esc(b.code) + "</span>").join("") +
      (f.error ? '<span class="snext err">' + esc(f.error) + "</span>" : '<code class="snext">' + esc(f.next_step || "") + "</code>");
    return '<tr class="srow' + (f.path === state.spec ? " on" : "") + '" tabindex="0" data-path="' + esc(f.path) + '">' +
      '<td class="sname"><span class="pt">' + (ops.length ? '<span class="mk" title="Waits on an operator decision">' + PAUSE + "</span>" : "") +
      "<span>" + esc(f.title || f.path) + "</span></span><code>" + esc(f.path) + "</code></td>" +
      '<td class="sepic">' + epic + "</td><td>" + tasks + '</td><td class="sloop">' + loopCell("review", f.review) + loopCell("audit", f.audit) +
      '</td><td class="snextc">' + next + "</td></tr>";
  }

  function drawSpecs() {
    const list = data("export-list");
    const files = (list && list.files) || [];
    const groups = PHASES.concat([["", "Unreadable"]]);
    const chip = (v, label, n) => '<button type="button" class="chip" data-phase="' + v + '" aria-pressed="' + (specsView.phase === v) + '">' +
      esc(label) + ' <span class="num muted">' + n + "</span></button>";
    $("phasechips").innerHTML = chip("all", "All", files.length) + chip("op", "Operator decisions", files.filter(waitsOp).length) +
      PHASES.map(([p, label]) => chip(p, label, files.filter((f) => f.phase === p).length)).join("");
    const q = specsView.q.trim().toLowerCase();
    const ph = specsView.phase;
    const focused = document.activeElement && document.activeElement.closest && document.activeElement.closest("#specs-body .srow");
    let n = 0;
    const body = groups.map(([p, label]) => {
      if (ph !== "all" && ph !== "op" && ph !== p) return "";
      const rows = files.filter((f) => (f.phase || "") === p && (ph !== "op" || waitsOp(f)) && specMatches(f, q));
      n += rows.length;
      return rows.length ? '<tbody><tr class="sgroup"><th colspan="5" scope="rowgroup">' + esc(label) + ' <span class="num muted">' + rows.length + "</span></th></tr>" +
        rows.map(specRow).join("") + "</tbody>" : "";
    }).join("");
    $("specs-body").innerHTML = n
      ? '<table class="stable"><thead><tr><th scope="col">Spec</th><th scope="col">Epic</th><th scope="col">Tasks</th><th scope="col">Loops</th><th scope="col">Next step</th></tr></thead>' + body + "</table>"
      : none(list ? "No task file matches" + (q ? " “" + specsView.q.trim() + "”" : "") + " here." : "");
    $("specs-n").textContent = list ? n : "";
    if (focused) {
      const again = $("specs-body").querySelector('.srow[data-path="' + CSS.escape(focused.dataset.path) + '"]');
      if (again) again.focus();
    }
    syncURL();
  }
  panel(["export-list"], drawSpecs);

  // ---------- the Spec tab (§13) ----------
  // One task file at a time, from tp export of that file: fetched when it is
  // opened, and again each time tp pushes a changed list.
  const work = { path: null, data: null, error: null, view: null, task: null, sec: null, tview: "steps", seq: 0 };
  const VIEWS = [["read", "Read the spec"], ["tasks", "Tasks"], ["history", "History"]];
  const STEPS = ["ground", "review", "decompose", "implement", "audit", "release"];
  const CHECK = '<svg class="ic" viewBox="0 0 16 16" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M3.5 8.5l3 3 6-7"/></svg>';
  const capital = (s) => s[0].toUpperCase() + s.slice(1);
  const listRow = (path) => ((data("export-list") || {}).files || []).find((x) => x.path === path) || null;
  const escalations = (d) => ((d.resume && d.resume.blockers) || []).filter((b) => b.class === "escalate");
  INFO.loop = "The loop tp runs a spec through: ground its claims, review, decompose into tasks, implement, audit, release. The marked step is the phase tp resume reports; a pause mark there is a decision only the operator takes. Each step's line comes from tp export's loops and tasks.";

  // A spec in review or decompose opens on its text, one with tasks on them.
  const defaultView = (d) => ((d.tasks || []).length && !["review", "decompose"].includes(d.resume.phase) ? "tasks" : "read");

  // openSpec takes one task file to the Spec tab.
  function openSpec(path) {
    if (work.path !== path) Object.assign(work, { path, data: null, error: null, view: null, task: null, sec: null });
    state.spec = path;
    setTab("work");
    drawSpecs();
    drawWork();
    fetchSpec();
  }

  async function fetchSpec() {
    const path = work.path;
    const seq = ++work.seq;
    let body;
    try {
      const res = await fetch("/api/export?file=" + encodeURIComponent(path), { cache: "no-store" });
      body = await res.json();
    } catch (err) {
      body = { error: "tp export --file " + path + " could not be fetched: " + err.message, hint: "check that tp dashboard is still running" };
    }
    if (seq !== work.seq) return;
    if (isError(body)) {
      work.error = body;
    } else {
      work.data = body;
      work.error = null;
      if (work.view === null) {
        work.view = defaultView(body);
        address.quiet = true;
      }
    }
    drawWork();
  }

  // stepLines says, per step, what happened in it or when it comes.
  function stepLines(d, f) {
    const L = d.loops || {};
    const phase = d.resume.phase;
    const past = (step) => STEPS.indexOf(phase) > STEPS.indexOf(step);
    const last = (l) => (l && l.rounds && l.rounds.length ? l.rounds[l.rounds.length - 1] : null);
    const loopLine = (l, r, step) => {
      if (l.streak >= l.clean_rounds_needed) return "converged in round " + r.round;
      if (past(step)) return "ended in round " + r.round;
      return "round " + r.round + (l.cap ? " of " + l.cap : "") + ", " + l.streak + " of " + l.clean_rounds_needed + " clean";
    };
    const g = last(L.ground);
    const r = last(L.review);
    const a = last(L.audit);
    const t = (f && f.tasks) || { open: 0, wip: 0, done: 0 };
    const n = t.open + t.wip + t.done;
    return {
      ground: g ? (g.fails ? "round " + g.round + ": " + plural(g.fails, "FAIL", "FAILs") + " to repair" : "claims checked in round " + g.round) : "not run",
      review: r ? loopLine(L.review, r, "review") : phase === "review" ? "round 1 next" : "no round recorded",
      decompose: n ? plural(n, "task", "tasks") + " written" : phase === "decompose" ? "writing the tasks" : "after review",
      implement: n ? t.done + " of " + n + " done" : "after decompose",
      audit: a ? loopLine(L.audit, a, "audit") : phase === "audit" ? "round 1 next" : "after implement",
      release: phase === "release" ? "released" : "after audit",
    };
  }

  // stepper marks the phase tp resume reports; ground, which tp resume never
  // reports, is skipped when it has no round.
  function stepper(d, f) {
    const lines = stepLines(d, f);
    const cur = d.resume.phase === "release" ? STEPS.length : STEPS.indexOf(d.resume.phase);
    const paused = escalations(d).length > 0;
    return '<ol class="stepper">' + STEPS.map((p, i) => {
      let st = i < cur ? "done" : i === cur ? "cur" : "todo";
      if (p === "ground" && st === "done" && !(d.loops.ground.rounds || []).length) st = "skip";
      const mark = st === "done" ? CHECK : st === "cur" && paused ? PAUSE : String(i + 1);
      return '<li class="' + st + (st === "cur" && paused ? " op" : "") + '"' + (st === "cur" ? ' aria-current="step"' : "") + '><span class="sn">' + mark +
        '</span><span class="sl">' + capital(p) + '</span><span class="sd">' + esc(lines[p]) + "</span></li>";
    }).join("") + "</ol>";
  }

  // impactText says what releasing the spec does to the plan, from the
  // summary's release_impact.
  function impactText(d) {
    if (d.resume.phase === "release") return "Released.";
    if (!d.epic) return "Bound to no epic, so releasing it moves nothing in the plan.";
    const sum = data("summary");
    const imp = sum && sum.release_impact && sum.release_impact[d.path];
    if (!imp) return "";
    const code = (x) => "<code>" + esc(x) + "</code>";
    const parts = [];
    if (imp.frees.length) parts.push("frees " + imp.frees.map(code).join(", "));
    imp.gates.forEach((gt) => parts.push("brings milestone " + code(gt.milestone) + "'s gate from " + esc(gt.members_done) + " to " + esc(gt.members_done_after) + " of " + esc(gt.members) + " members done"));
    if (!parts.length) return "Releasing it alone frees no epic and moves no gate.";
    return "Releasing it completes epic " + code(d.epic) + ", which " + parts.join(" and ") + ".";
  }

  function specHead(d, f) {
    const na = d.resume.next_action || {};
    const cmd = na.command || na.brief_command;
    const epic = d.epic
      ? '<span class="sh-epic">epic <code>' + esc(d.epic) + "</code>" + (f && f.epic_state ? " " + esc(STAGE_LABEL[f.epic_state] || f.epic_state) : "") + "</span>"
      : '<span class="sh-epic">bound to no epic</span>';
    return '<div class="sh-row"><h1>' + esc((f && f.title) || d.spec) + "</h1>" + epic + "</div>" +
      '<div class="loophead"><h2>The loop</h2>' + info("loop", "the loop") + "</div>" + stepper(d, f) +
      '<p class="impact">' + impactText(d) + "</p>" +
      escalations(d).map((b) => '<div class="opnote"><span class="mk">' + PAUSE + "</span><div><b>" + esc(b.code) + "</b> waits on the operator: " + esc(b.message) + "</div></div>").join("") +
      '<div class="cmdline"><span class="lab">Next, as <code>tp resume</code> says</span>' +
      (cmd ? "<code>" + esc(cmd) + '</code><button type="button" class="key" data-copy="' + esc(cmd) + '">Copy</button>' : "") +
      '<span class="sum">' + esc(na.summary || "") + "</span></div>";
  }

  function drawWork() {
    $("work-none").hidden = !!work.path;
    $("work-body").hidden = !work.path;
    $("c-work").textContent = work.path || "";
    if (!work.path) {
      syncURL();
      return;
    }
    const d = work.data;
    const f = listRow(work.path);
    const e = work.error;
    const banner = e ? '<div class="banner"><b>tp export could not read ' + esc(work.path) + "</b><span>" + esc(e.error) + "</span>" +
      (e.hint ? '<span class="hint">' + esc(e.hint) + "</span>" : "") + (d ? '<span class="kept">Showing the last good state.</span>' : "") + "</div>" : "";
    const crumb = '<div class="crumb"><button type="button" class="link" id="crumb-specs">Specs</button><span aria-hidden="true">/</span><code>' + esc(work.path) + "</code></div>";
    $("spechead").innerHTML = crumb + banner + (d ? specHead(d, f) : e ? "" : none("Reading " + work.path + " with tp export."));
    const counts = {
      read: d ? plural((d.sections || []).filter((s) => s.anchor !== "§0").length, "section", "sections") : "",
      tasks: f && f.tasks && f.tasks.open + f.tasks.wip + f.tasks.done ? f.tasks.done + " of " + (f.tasks.open + f.tasks.wip + f.tasks.done) + " done" : "none yet",
      history: d ? plural(["ground", "review", "audit"].reduce((s, k) => s + ((d.loops[k] && d.loops[k].rounds) || []).length, 0), "round", "rounds") : "",
    };
    $("subnav").innerHTML = d ? VIEWS.map(([v, label]) => '<button type="button" role="tab" data-view="' + v + '" aria-controls="sv-' + v + '" aria-selected="' + (work.view === v) + '">' +
      esc(label) + '<span class="muted num">' + esc(counts[v]) + "</span></button>").join("") : "";
    VIEWS.forEach(([v]) => { $("sv-" + v).hidden = !d || work.view !== v; });
    if (d && work.view === "read") drawRead(d);
    if (d && work.view === "tasks") drawTasks(d);
    if (d && work.view === "history") drawSpecHistory(d);
    syncURL();
  }

  // ---------- the Spec tab's Read view (§13) ----------
  // The spec's text from tp export's sections, each section and paragraph
  // under the anchor tp gave it; a section's task count opens Tasks with the
  // tasks written from it picked out.
  INFO.reader = "The spec as the file reads now, in its numbered sections, from tp export. Every section and paragraph carries a stable anchor (§3 ¶2; hover a paragraph to see it) that findings and feedback can point at. A section's task count opens Tasks with the tasks written from it picked out.";

  // inline draws a paragraph's code spans and bold over its escaped text.
  const inline = (s) => esc(s).replace(/`([^`]+)`/g, "<code>$1</code>").replace(/\*\*([^*]+)\*\*/g, "<b>$1</b>");

  // tableHTML draws a paragraph of table rows, its first row the head and
  // separator rows left out.
  function tableHTML(lines) {
    const rows = lines.filter((l) => !/^\|?\s*:?-{2,}/.test(l.trim())).map((l) => l.trim().replace(/^\||\|$/g, "").split("|").map((c) => c.trim()));
    return '<div class="scroll"><table>' + rows.map((r, i) => "<tr>" + r.map((c) => (i ? "<td>" : "<th>") + inline(c) + (i ? "</td>" : "</th>")).join("") + "</tr>").join("") + "</table></div>";
  }

  // listHTML draws a paragraph of list items, a line with no marker
  // continuing the item before it.
  function listHTML(lines) {
    const mark = /^\s*(?:[-*+]|\d+[.)])\s+/;
    const items = [];
    lines.forEach((l) => { if (mark.test(l) || !items.length) items.push(l.replace(mark, "")); else items[items.length - 1] += " " + l.trim(); });
    const tag = /^\s*\d/.test(lines[0]) ? "ol" : "ul";
    return "<" + tag + ">" + items.map((x) => "<li>" + inline(x) + "</li>").join("") + "</" + tag + ">";
  }

  // blockHTML draws one paragraph of the spec by what its first line is: a
  // fence, a table, a list, a heading that opens no section, or prose.
  function blockHTML(text) {
    const lines = String(text).split("\n");
    if (/^\s*(```|~~~)/.test(lines[0])) return "<pre>" + esc(lines.slice(1, /^\s*(```|~~~)/.test(lines[lines.length - 1]) && lines.length > 1 ? -1 : undefined).join("\n")) + "</pre>";
    if (lines.every((l) => l.trim().startsWith("|"))) return tableHTML(lines);
    if (/^\s*(?:[-*+]|\d+[.)])\s+/.test(lines[0])) return listHTML(lines);
    if (/^#{1,6}\s/.test(lines[0])) return '<h3 class="sub">' + inline(lines[0].replace(/^#+\s*/, "")) + "</h3>" + (lines.length > 1 ? blockHTML(lines.slice(1).join("\n")) : "");
    return "<p>" + inline(lines.join(" ")) + "</p>";
  }

  // secKey is a section's anchor without its sign: the address's sec value.
  const secKey = (sec) => String(sec.anchor).replace(/^§/, "");
  const secTitle = (sec) => sec.heading.replace(/^\d+(?:\.\d+)*\.?\s+/, "");

  // tasksBySection counts, per section key, the tasks written from it.
  function tasksBySection(d) {
    const by = {};
    (d.tasks || []).forEach((t) => new Set((t.source_sections || []).map((s) => sectionOf(d, s)).filter(Boolean).map(secKey)).forEach((k) => { (by[k] = by[k] || []).push(t.id); }));
    return by;
  }

  function drawRead(d) {
    const secs = d.sections || [];
    const pre = secs.find((s) => secKey(s) === "0");
    const numbered = secs.filter((s) => s !== pre);
    const by = tasksBySection(d);
    const reviews = ((d.loops.review && d.loops.review.rounds) || []).length;
    const blk = (k, p) => '<div class="blk" id="p-' + esc(k) + "-" + esc(p.anchor.split("¶")[1] || "") + '"><span class="anc">' + esc(p.anchor) + "</span>" + blockHTML(p.text) + "</div>";
    const section = (s) => {
      const k = secKey(s);
      const n = (by[k] || []).length;
      return '<section class="sec" id="sec-' + esc(k) + '"><h2 class="sth"><span class="n">' + esc(s.anchor) + "</span><span>" + esc(secTitle(s)) + "</span>" +
        (n ? '<button type="button" class="tc" data-sec="' + esc(k) + '" aria-label="' + esc("Show the " + plural(n, "task", "tasks") + " written from " + s.anchor) + '">' + plural(n, "task", "tasks") + "</button>" : "") + "</h2>" +
        (s.paragraphs || []).map((p) => blk(k, p)).join("") + "</section>";
    };
    $("sv-read").innerHTML = '<div class="doc"><div class="rmeta"><span>' + (reviews ? "reviewed in " + plural(reviews, "round", "rounds") : "no review round yet") + "</span><span>" +
      ((d.tasks || []).length ? "each section names the tasks written from it" : "tasks are written from these sections after review") + "</span>" + info("reader", "the spec reader") + "</div>" +
      "<h1>" + esc((pre && pre.heading) || d.spec) + "</h1>" + (pre && pre.paragraphs ? pre.paragraphs.map((p) => blk("0", p)).join("") : "") +
      (numbered.length ? '<nav class="toc" aria-label="Sections">' + numbered.map((s) => '<button type="button" data-goto="' + esc(secKey(s)) + '">' + esc(s.anchor + " " + secTitle(s)) + "</button>").join("") + "</nav>" : "") +
      (numbered.map(section).join("") || none("The spec has no numbered section.")) + "</div>";
  }

  // gotoSection shows the Read view at one section.
  function gotoSection(k) {
    work.view = "read";
    drawWork();
    const el = $("sec-" + k);
    if (!el) return;
    el.scrollIntoView({ block: "start" });
    el.classList.add("flash");
    setTimeout(() => el.classList.remove("flash"), 1400);
  }

  $("sv-read").addEventListener("click", (ev) => {
    const c = ev.target.closest("[data-sec]");
    const g = ev.target.closest("[data-goto]");
    if (c) {
      Object.assign(work, { sec: c.dataset.sec, view: "tasks", task: null });
      drawWork();
      window.scrollTo(0, $("subnav").offsetTop);
    } else if (g) {
      gotoSection(g.dataset.goto);
    }
  });

  // ---------- the Spec tab's History view (§13) ----------
  // From tp export's loops: every recorded round on a time axis, the round
  // table per loop, and each review round's snapshot difference from the
  // round before beside that round's findings.
  const LOOPS = [["ground", "Ground"], ["review", "Review"], ["audit", "Audit"]];
  INFO.spechistory = "This spec's recorded rounds from tp export: each loop's rounds by the time they were recorded (a ground round records no time, so its file's time stands in), the round table with each round's findings (ground: FAILs) and whether it is clean, with the loop's cap and its current run of clean rounds, and for each review round what changed in the spec since the round before, beside that round's findings.";

  // roundAxis draws each loop's rounds as dots on one time axis, a clean
  // round filled.
  function roundAxis(L) {
    const all = [];
    LOOPS.forEach(([k]) => ((L[k] && L[k].rounds) || []).forEach((r) => all.push(Date.parse(r.recorded_at))));
    const times = all.filter((x) => !isNaN(x));
    if (!times.length) return "";
    const lo = Math.min(...times);
    const span = Math.max(1, Math.max(...times) - lo);
    const x = (iso) => 90 + ((Date.parse(iso) - lo) / span) * 490;
    const day = (ms) => new Date(ms).toISOString().slice(0, 10);
    const lanes = LOOPS.map(([k, label], i) => {
      const y = 16 + i * 26;
      return '<text x="0" y="' + (y + 4) + '">' + label + '</text><line class="ax" x1="90" x2="580" y1="' + y + '" y2="' + y + '"/>' +
        ((L[k] && L[k].rounds) || []).filter((r) => !isNaN(Date.parse(r.recorded_at))).map((r) => {
          const n = r.fails !== undefined ? plural(r.fails, "FAIL", "FAILs") : plural(r.findings || 0, "finding", "findings");
          return '<circle class="' + (r.clean ? "clean" : "dirty") + '" cx="' + x(r.recorded_at) + '" cy="' + y + '" r="5"><title>' +
            esc(label + " round " + r.round + ", " + r.recorded_at.replace("T", " ").replace("Z", " UTC") + ": " + n + (r.clean ? ", clean" : "")) + "</title></circle>";
        }).join("");
    }).join("");
    return '<svg class="raxis" viewBox="0 0 600 ' + (LOOPS.length * 26 + 22) + '" role="img" aria-label="Rounds over time">' + lanes +
      '<text x="90" y="' + (LOOPS.length * 26 + 18) + '">' + day(lo) + '</text><text x="580" y="' + (LOOPS.length * 26 + 18) + '" text-anchor="end">' + day(lo + span) + "</text></svg>";
  }

  // roundTable lists each loop's rounds under a line naming its cap and its
  // current run of clean rounds against the run it needs.
  function roundTable(L) {
    const body = LOOPS.map(([k, label]) => {
      const l = L[k];
      if (!l || !(l.rounds || []).length) return "";
      const head = '<tr class="lgroup"><th colspan="4">' + label + ' <span class="muted">' + esc((l.cap ? "cap " + l.cap : "uncapped") + ", " + l.streak + " of " + l.clean_rounds_needed + " clean in a row") + "</span></th></tr>";
      return head + l.rounds.map((r) => "<tr><td>Round " + esc(r.round) + "</td><td>" + esc(r.recorded_at.replace("T", " ").replace("Z", " UTC")) + '</td><td class="n">' +
        esc(r.fails !== undefined ? plural(r.fails, "FAIL", "FAILs") : plural(r.findings || 0, "finding", "findings")) + "</td><td>" + (r.clean ? CHECK + " clean" : '<span class="muted">not clean</span>') + "</td></tr>").join("");
    }).join("");
    return '<div class="scroll"><table class="loops"><thead><tr><th>Round</th><th>Recorded</th><th>Findings</th><th>Clean</th></tr></thead><tbody>' + body + "</tbody></table></div>";
  }

  // snapshotDiff draws the sections a round's snapshot changed, added or
  // removed against the round before's.
  function snapshotDiff(d) {
    if (!d) return none("The first round: there is no round before to compare with.");
    const changed = (d.changed || []).map((s) => '<div class="dl ins"><code>' + esc(s.heading) + '</code> <span class="muted sm">' + esc(s.status) + "</span>" +
      (s.content && s.content.trim() ? '<div class="now sectext">' + esc(s.content.trim()) + "</div>" : "") + "</div>");
    const removed = (d.removed || []).map((s) => '<div class="dl del"><code>' + esc(s.heading) + "</code></div>");
    return (changed.length ? '<div class="dpart"><h4>Changed or added</h4>' + changed.join("") + "</div>" : "") +
      (removed.length ? '<div class="dpart"><h4>Removed</h4>' + removed.join("") + "</div>" : "") || none("Nothing changed in the spec since the round before.");
  }

  function drawSpecHistory(d) {
    const L = d.loops || {};
    const review = ((L.review && L.review.rounds) || []).slice().reverse();
    const pairs = review.map((r) => {
      const byRole = {};
      (r.finding_rows || []).forEach((f) => { (byRole[f.role || "unattributed"] = byRole[f.role || "unattributed"] || []).push({ severity: f.severity, location: f.location, finding: f.finding, resolved: f.disposition ? { status: f.disposition } : null }); });
      return '<div class="rpair"><h3 class="hsub">Review round ' + esc(r.round) + ' <span class="muted sm">' + esc(plural(r.findings || 0, "finding", "findings") + (r.clean ? ", clean" : "")) + "</span></h3>" +
        '<div class="hcmp"><div><h4 class="hsub">What changed' + (r.round > 1 ? " since round " + esc(r.round - 1) : "") + "</h4>" + snapshotDiff(r.snapshot_diff) + "</div>" +
        '<div><h4 class="hsub">Round ' + esc(r.round) + "'s findings</h4>" + findingsList(byRole) + "</div></div></div>";
    }).join("");
    const any = LOOPS.some(([k]) => L[k] && (L[k].rounds || []).length);
    $("sv-history").innerHTML = '<div class="hhead"><h2>History</h2>' + info("spechistory", "the spec's history") + "</div>" +
      (any ? roundAxis(L) + roundTable(L) + pairs : none("This spec has recorded no round yet."));
  }

  // ---------- the Spec tab's Tasks view (§13) ----------
  // BOARD is the Board's columns: key, label and the task states tp export
  // reports that each one holds. Ready and waiting hold the open tasks, so the
  // columns count what tp status counts as open, wip and done.
  const BOARD = [["ready", "Ready", ["ready"]], ["wip", "In progress", ["wip"]], ["waiting", "Waiting", ["waits_on_task", "waits_on_epic"]], ["done", "Done", ["done"]]];
  const TASK_LABEL = { ready: "ready", wip: "in progress", waits_on_task: "waits on a task", waits_on_epic: "waits on its epic", done: "done" };
  const TASK_COLOR = { ready: "ready", wip: "active", waits_on_task: "moot", waits_on_epic: "held", done: "done" };
  const WIP_MARK = '<svg class="ic" viewBox="0 0 16 16" aria-hidden="true"><circle cx="8" cy="8" r="5.5" fill="none" stroke="currentColor" stroke-width="1.6"/><path d="M8 2.5a5.5 5.5 0 0 1 0 11z" fill="currentColor"/></svg>';
  const TASK_MARK = { done: CHECK, wip: WIP_MARK, waits_on_epic: PAUSE, waits_on_task: "", ready: "" };
  INFO.tasks = "This spec's tasks from tp export, two ways. Steps is the dependency tree: a task sits one step below the last task it waits on, so step 1 needs no other task, and lines join a task to what it waits on. Board is the same tasks by state: ready, in progress, waiting (on a task, or a pause mark when its epic is not ready, so tp next starts none), done; ready and waiting together are tp status's open count. Click a task for its detail, again to show progress.";
  INFO.progress = "From the tasks tp export reports: their states, the estimates of the tasks not done, how long each task in progress has run against its estimate, and the closing times as a running count.";

  // since says how long ago an RFC 3339 time was, in the largest whole unit.
  function since(iso) {
    const m = Math.max(0, Math.round((Date.now() - Date.parse(iso)) / 60000));
    return m < 60 ? m + " min" : m < 2880 ? Math.round(m / 60) + " h" : Math.round(m / 1440) + " d";
  }
  const minutes = (m) => (m < 60 ? m + " min" : Math.round(m / 6) / 10 + " h");

  // taskIndex holds this file's tasks by id and, for each, the tasks that
  // depend on it.
  function taskIndex(d) {
    const by = {};
    const dependents = {};
    (d.tasks || []).forEach((t) => { by[t.id] = t; });
    (d.tasks || []).forEach((t) => (t.depends_on || []).forEach((x) => { if (by[x]) (dependents[x] = dependents[x] || []).push(t.id); }));
    return { by, dependents };
  }

  // stepsOf puts each task one step below the last task it waits on.
  function stepsOf(d, ix) {
    const depth = {};
    const of = (id, trail) => {
      if (depth[id] !== undefined) return depth[id];
      if (trail.has(id)) return 0;
      trail.add(id);
      const ds = (ix.by[id].depends_on || []).filter((x) => ix.by[x]);
      depth[id] = ds.length ? 1 + Math.max(...ds.map((x) => of(x, trail))) : 0;
      return depth[id];
    };
    const steps = [];
    (d.tasks || []).forEach((t) => { const k = of(t.id, new Set()); (steps[k] = steps[k] || []).push(t); });
    return steps;
  }

  function taskMeta(t, ix) {
    if (t.state === "done") return t.closed_at ? "closed " + since(t.closed_at) + " ago" : "done";
    if (t.state === "wip") return t.started_at ? "started " + since(t.started_at) + " ago" : "in progress";
    if (t.state === "waits_on_task" || t.state === "waits_on_epic") return "after " + (t.waits_on || []).join(", ");
    const n = (ix.dependents[t.id] || []).length;
    return n ? "unblocks " + n : "unblocks nothing";
  }

  function taskCard(t, ix) {
    const dim = work.secIds && !work.secIds.has(t.id);
    return '<button type="button" class="tcard' + (dim ? " dim" : "") + '" data-s="' + esc(t.state) + '" data-t="' + esc(t.id) + '" aria-pressed="' + (t.id === work.task) + '">' +
      '<span class="tt">' + (TASK_MARK[t.state] || "") + "<span>" + esc(t.title) + '</span></span><code class="tid">' + esc(t.id) +
      '</code><span class="tm">' + esc(taskMeta(t, ix)) + "</span></button>";
  }

  function stepsHTML(d, ix) {
    return '<svg class="wires" id="wires" aria-hidden="true"></svg>' + stepsOf(d, ix).map((g, i) => '<div class="step"><div class="step-h"><b>Step ' + (i + 1) +
      "</b><span>" + (i ? "after step " + i : "needs no other task") + '</span></div><div class="cards">' + g.map((t) => taskCard(t, ix)).join("") + "</div></div>").join("");
  }

  function boardHTML(d, ix) {
    return BOARD.map(([key, label, states]) => {
      const ts = (d.tasks || []).filter((t) => states.includes(t.state));
      return '<section class="bcol" data-col="' + key + '" aria-label="' + esc(label) + '"><div class="step-h"><b>' + esc(label) + '</b><span class="num">' + ts.length + "</span></div>" +
        '<div class="bcards">' + (ts.map((t) => taskCard(t, ix)).join("") || none("None")) + "</div></section>";
    }).join("");
  }

  // drawWires joins each task in Steps to what it waits on, the picked task's
  // own links drawn strong.
  function drawWires(d) {
    const svg = $("wires");
    const box = $("stages");
    if (!svg || !box) return;
    const br = box.getBoundingClientRect();
    const W = box.scrollWidth;
    const H = box.scrollHeight;
    svg.setAttribute("viewBox", "0 0 " + W + " " + H);
    svg.setAttribute("width", W);
    svg.setAttribute("height", H);
    const R = {};
    box.querySelectorAll(".tcard").forEach((el) => {
      const r = el.getBoundingClientRect();
      R[el.dataset.t] = { x: r.left - br.left + box.scrollLeft, y: r.top - br.top + box.scrollTop, w: r.width, h: r.height };
    });
    const out = [];
    (d.tasks || []).forEach((t) => (t.depends_on || []).forEach((x) => {
      const a = R[x];
      const b = R[t.id];
      if (!a || !b) return;
      const on = work.task && (work.task === x || work.task === t.id);
      const p = { x: a.x + a.w / 2, y: a.y + a.h };
      const q = { x: b.x + b.w / 2, y: b.y };
      const dy = Math.max(12, (q.y - p.y) / 2);
      out.push('<path class="' + (on ? "on" : "") + '" d="M' + p.x + " " + p.y + "C" + p.x + " " + (p.y + dy) + " " + q.x + " " + (q.y - dy) + " " + q.x + " " + q.y + '"/>');
    }));
    svg.innerHTML = out.join("");
  }

  // closedOverTime draws the running count of closed tasks per day, from the
  // first close to today, against the dashed line where every task is done.
  function closedOverTime(tasks) {
    const closed = tasks.filter((t) => t.state === "done" && t.closed_at).map((t) => Date.parse(t.closed_at)).sort((a, b) => a - b);
    if (!closed.length) return none("No task closed yet.");
    const day = 86400000;
    const first = Math.floor(closed[0] / day);
    const days = Math.max(1, Math.floor(Date.now() / day) - first);
    const W = 320;
    const H = 72;
    const pts = [];
    for (let i = 0; i <= days; i++) pts.push(closed.filter((c) => Math.floor(c / day) <= first + i).length);
    const xy = pts.map((v, i) => (i / days * W).toFixed(1) + "," + (H - 4 - v / tasks.length * (H - 8)).toFixed(1)).join(" ");
    const date = (t) => new Date(t).toISOString().slice(0, 10);
    return '<svg class="burn" viewBox="0 0 ' + W + " " + H + '" preserveAspectRatio="none" role="img" aria-label="' + esc(closed.length + " of " + tasks.length + " tasks closed since " + date(closed[0])) + '">' +
      '<line x1="0" x2="' + W + '" y1="4" y2="4" class="goal"/><polyline points="' + xy + '" class="line"/></svg>' +
      '<div class="burnx"><span>' + esc(date(closed[0])) + "</span><span>all " + tasks.length + " done at the dashed line</span><span>today</span></div>";
  }

  // running says how long a task in progress has run against its estimate.
  function running(t) {
    if (!t.started_at) return "in progress";
    const ran = Math.max(0, Math.round((Date.now() - Date.parse(t.started_at)) / 60000));
    const over = t.estimate_minutes && ran > t.estimate_minutes;
    return '<span class="' + (over ? "over" : "") + '">running ' + esc(minutes(ran)) + (t.estimate_minutes ? ", estimate " + esc(minutes(t.estimate_minutes)) : ", no estimate") + "</span>";
  }

  function progressHTML(d) {
    const tasks = d.tasks || [];
    if (!tasks.length) {
      return '<div class="pv"><h3>No tasks yet</h3>' + none(d.resume.phase === "decompose" ? "Review is done; decomposing writes the tasks." : "The spec is still in review; its tasks are written once review is done.") + "</div>";
    }
    const count = (states) => tasks.filter((t) => states.includes(t.state)).length;
    const seg = BOARD.map(([key, label, states]) => [key, label.toLowerCase(), count(states)]);
    const color = { ready: "ready", wip: "active", waiting: "moot", done: "done" };
    const order = ["done", "wip", "ready", "waiting"].map((k) => seg.find((s) => s[0] === k));
    const left = tasks.filter((t) => t.state !== "done");
    const est = left.reduce((s, t) => s + (t.estimate_minutes || 0), 0);
    const noEst = left.filter((t) => !t.estimate_minutes).length;
    const wip = tasks.filter((t) => t.state === "wip").map((t) => '<li><button class="link" type="button" data-t="' + esc(t.id) + '">' + esc(t.title) + "</button><small>" + running(t) + "</small></li>").join("");
    return '<div class="pv"><div class="pv-h"><h3><span class="num">' + count(["done"]) + "</span> of " + tasks.length + " tasks done</h3>" + info("progress", "progress") + "</div>" +
      '<div class="pbar" role="img" aria-label="' + esc(order.map((s) => s[2] + " " + s[1]).join(", ")) + '">' + order.map((s) => (s[2] ? '<i style="flex:' + s[2] + ";background:var(--" + color[s[0]] + ')"></i>' : "")).join("") + "</div>" +
      '<div class="pkey">' + order.map((s) => '<span><i style="background:var(--' + color[s[0]] + ')"></i><b class="num">' + s[2] + "</b>&nbsp;" + esc(s[1]) + "</span>").join("") + "</div>" +
      '<p class="note">' + (left.length ? "About " + esc(minutes(est)) + " of estimated work left" + (noEst ? ", " + plural(noEst, "task", "tasks") + " without an estimate" : "") + "." : "Every task is done.") + "</p>" +
      (wip ? '<div><h4>In progress</h4><ul class="wiplist">' + wip + "</ul></div>" : "") +
      "<div><h4>Closed over time</h4>" + closedOverTime(tasks) + "</div>" + '<p class="note">Click a task for its detail.</p></div>';
  }

  // sectionOf finds the export's section for one of a task's source_sections.
  const sectionOf = (d, src) => (d.sections || []).find((s) => s.heading === String(src).replace(/^#+\s*/, "")) || null;

  function taskDetail(d, t, ix) {
    const link = (id) => '<button class="link" type="button" data-t="' + esc(id) + '">' + esc(id) + "</button>";
    const deps = (t.depends_on || []).map((x) => {
      const u = ix.by[x];
      const ok = !u || u.state === "done";
      return '<li><span class="mk" style="color:var(--' + (ok ? "done" : "blocked") + ')">' + (ok ? "✓" : "○") + "</span><span>" + (u ? link(x) + "<small>" + esc(u.title) + "</small>" : "<code>" + esc(x) + "</code>") + "</span></li>";
    }).join("") || '<li class="empty">No dependencies</li>';
    const needs = (ix.dependents[t.id] || []).map((x) => '<li><span class="mk">→</span><span>' + link(x) + "<small>" + esc(TASK_LABEL[ix.by[x].state] || ix.by[x].state) + "</small></span></li>").join("") ||
      '<li class="empty">Nothing waits on it</li>';
    const shas = t.commit_shas || (t.commit_sha ? [t.commit_sha] : []);
    let time;
    if (t.state === "done") {
      time = "<dt>Closed</dt><dd>" + (t.closed_at ? esc(since(t.closed_at)) + " ago" : "done") + (shas.length ? ", " + (shas.length > 1 ? "commits " : "commit ") + shas.map((s) => "<code>" + esc(s) + "</code>").join(" ") : "") + "</dd>" +
        '<dt>Evidence</dt><dd class="ev">' + esc(t.closed_reason || "none recorded") + "</dd>";
    } else if (t.state === "wip") {
      time = "<dt>Progress</dt><dd>" + running(t) + "</dd>";
    } else {
      time = "<dt>Estimate</dt><dd>" + (t.estimate_minutes ? esc(minutes(t.estimate_minutes)) : "none") + "</dd>" + (t.waits_on && t.waits_on.length ? "<dt>Waits on</dt><dd>" + esc(t.waits_on.join(", ")) + "</dd>" : "");
    }
    const cmd = t.state === "ready" ? "tp claim " + t.id : t.state === "wip" ? "tp done " + t.id + ' "<evidence>"' : "";
    const acc = String(t.acceptance || "").split("\n").map((l) => l.replace(/^- /, "").trim()).filter(Boolean);
    const src = (t.source_sections || []).map((s) => {
      const sec = sectionOf(d, s);
      const p = sec && sec.paragraphs && sec.paragraphs[0];
      return '<figure class="excerpt"><figcaption>' + (sec ? '<button type="button" class="link" data-goto="' + esc(secKey(sec)) + '">' + esc(sec.anchor + " " + secTitle(sec)) + "</button>" : esc(String(s).replace(/^#+\s*/, ""))) + "</figcaption>" +
        (p ? "<blockquote>" + esc(p.text) + "</blockquote>" : "") + "</figure>";
    }).join("");
    return '<div class="td-head"><div><span class="stat" style="color:var(--' + (TASK_COLOR[t.state] || "muted") + ')">' + (TASK_MARK[t.state] || "") + esc(TASK_LABEL[t.state] || t.state) + "</span>" +
      "<h3>" + esc(t.title) + '</h3><code class="tid">' + esc(t.id) + '</code></div><button type="button" class="key" data-tclose aria-label="Close the task detail and show progress">×</button></div>' +
      "<dl>" + time + "</dl>" +
      (cmd ? '<div class="cmdline"><code>' + esc(cmd) + '</code><button type="button" class="key" data-copy="' + esc(cmd) + '">Copy</button></div>' : "") +
      (t.state === "waits_on_epic" ? '<p class="opnote">Its epic <b>' + esc(d.epic || "") + "</b> waits on a dependency, so <code>tp next</code> starts none of this file's tasks yet.</p>" : "") +
      '<div><h4>Acceptance</h4><ul class="deps">' + acc.map((a) => '<li><span class="mk">–</span><span>' + esc(a) + "</span></li>").join("") + "</ul></div>" +
      '<div class="two-col"><div><h4>Depends on</h4><ul class="deps">' + deps + '</ul></div><div><h4>Needed by</h4><ul class="deps">' + needs + "</ul></div></div>" +
      (src ? "<div><h4>From the spec</h4>" + src + "</div>" : "");
  }

  function drawTasks(d) {
    const ix = taskIndex(d);
    if (work.task && !ix.by[work.task]) work.task = null;
    const focused = document.activeElement && document.activeElement.closest && document.activeElement.closest("#sv-tasks [data-t]");
    const tasks = d.tasks || [];
    const tview = (v, label) => '<button type="button" data-tview="' + v + '" aria-pressed="' + (work.tview === v) + '">' + label + "</button>";
    const sec = work.sec !== null ? (d.sections || []).find((s) => secKey(s) === work.sec) : null;
    if (!sec) work.sec = null;
    work.secIds = sec ? new Set(tasksBySection(d)[work.sec] || []) : null;
    const chip = sec ? '<span class="secchip">' + esc(plural(work.secIds.size, "task", "tasks")) + ' from <button type="button" class="link" data-goto="' + esc(work.sec) + '">' + esc(sec.anchor + " " + secTitle(sec)) + "</button>" +
      '<button type="button" class="key" data-secclear aria-label="Show every task">×</button></span>' : "";
    $("sv-tasks").innerHTML = '<section class="panel tpanel"><div class="toolbar"><h2>Tasks ' + info("tasks", "the tasks") + "</h2>" + chip + '<span class="spacer"></span>' +
      '<div class="seg" role="group" aria-label="Task view">' + tview("steps", "Steps") + tview("board", "Board") + "</div></div>" +
      '<div class="stages' + (work.tview === "board" ? " board" : "") + '" id="stages">' + (tasks.length ? (work.tview === "board" ? boardHTML(d, ix) : stepsHTML(d, ix)) : none("No tasks yet.")) + "</div>" +
      '<div class="legend">' + ["ready", "wip", "waits_on_task", "waits_on_epic", "done"].map((s) => '<span><i class="lg" data-s="' + s + '"></i>' + esc(TASK_LABEL[s]) + "</span>").join("") + "</div></section>" +
      '<aside class="panel tside" id="tside" aria-label="Task detail and progress">' + (work.task ? taskDetail(d, ix.by[work.task], ix) : progressHTML(d)) + "</aside>";
    if (work.tview === "steps") drawWires(d);
    if (focused) {
      const again = $("sv-tasks").querySelector('.tcard[data-t="' + CSS.escape(focused.dataset.t) + '"]');
      if (again) again.focus();
    }
  }

  // copyKey copies a command and says so on the key for a moment.
  function copyKey(b) {
    const done = (text) => { b.textContent = text; setTimeout(() => { b.textContent = "Copy"; }, 1200); };
    try {
      navigator.clipboard.writeText(b.dataset.copy).then(() => done("Copied"), () => done("Copy failed"));
    } catch (err) {
      done("Copy failed");
    }
  }

  $("sv-tasks").addEventListener("click", (ev) => {
    const v = ev.target.closest("[data-tview]");
    const t = ev.target.closest("[data-t]");
    const c = ev.target.closest("[data-copy]");
    const g = ev.target.closest("[data-goto]");
    if (c) {
      copyKey(c);
      return;
    }
    if (g) {
      gotoSection(g.dataset.goto);
      return;
    }
    if (ev.target.closest("[data-secclear]")) work.sec = null;
    else if (v) work.tview = v.dataset.tview;
    else if (ev.target.closest("[data-tclose]")) work.task = null;
    else if (t) work.task = t.classList.contains("tcard") && work.task === t.dataset.t ? null : t.dataset.t;
    else return;
    drawWork();
  });
  window.addEventListener("resize", () => { if (work.data && work.view === "tasks" && work.tview === "steps") drawWires(work.data); });

  // The open spec follows tp: a pushed list re-reads its export, and a pushed
  // summary redraws its release impact.
  let seenList = null;
  panel(["export-list", "summary"], () => {
    const l = data("export-list");
    if (l !== seenList) {
      seenList = l;
      if (work.path) fetchSpec();
    }
    drawWork();
  });
  $("subnav").addEventListener("click", (ev) => {
    const b = ev.target.closest("[data-view]");
    if (!b) return;
    work.view = b.dataset.view;
    drawWork();
  });
  $("spechead").addEventListener("click", (ev) => {
    if (ev.target.closest("#crumb-specs")) {
      setTab("specs");
      return;
    }
    const b = ev.target.closest("[data-copy]");
    if (b) copyKey(b);
  });
  function openSpecs() {
    setTab("specs");
    $("specs-q").focus();
    $("specs-q").select();
  }
  $("specs-q").addEventListener("input", (ev) => { specsView.q = ev.target.value; drawSpecs(); });
  $("specs-q").addEventListener("keydown", (ev) => {
    if (ev.key === "Enter") {
      const r = $("specs-body").querySelector(".srow");
      if (r) openSpec(r.dataset.path);
    } else if (ev.key === "ArrowDown") {
      const r = $("specs-body").querySelector(".srow");
      if (r) { ev.preventDefault(); r.focus(); }
    }
  });
  $("phasechips").addEventListener("click", (ev) => {
    const b = ev.target.closest("[data-phase]");
    if (!b) return;
    specsView.phase = b.dataset.phase;
    drawSpecs();
  });
  $("specs-body").addEventListener("click", (ev) => {
    const r = ev.target.closest(".srow");
    if (r) openSpec(r.dataset.path);
  });
  $("specs-body").addEventListener("keydown", (ev) => {
    const r = ev.target.closest(".srow");
    if (!r) return;
    if (ev.key === "Enter") {
      openSpec(r.dataset.path);
      return;
    }
    if (ev.key !== "ArrowDown" && ev.key !== "ArrowUp") return;
    ev.preventDefault();
    const rows = [...$("specs-body").querySelectorAll(".srow")];
    const i = rows.indexOf(r) + (ev.key === "ArrowDown" ? 1 : -1);
    if (i < 0) $("specs-q").focus();
    else if (rows[i]) rows[i].focus();
  });
  // "/" reaches the filter from anywhere but a field being typed in.
  document.addEventListener("keydown", (ev) => {
    const tag = (ev.target.tagName || "").toLowerCase();
    if (ev.key !== "/" || ev.metaKey || ev.ctrlKey || ev.altKey || tag === "input" || tag === "textarea" || ev.target.isContentEditable) return;
    ev.preventDefault();
    openSpecs();
  });

  TABS.forEach((t) => { $("tab-" + t).addEventListener("click", () => setTab(t)); });

  // ---------- the address (§14) ----------
  // The page's place lives in the address's fragment, so back and forward and
  // a copied link land where the operator was. URL_KEYS names every key: the
  // tab, the spec, its view, the picked task, the selected epic, the slice
  // whose graph history is shown and its two compared rounds, and the active
  // filters (the map's tracks, all edges, neighbours and search; the Specs
  // tab's phase and filter; the Tasks view's Steps or Board and the section
  // whose tasks it picks out, sec, a section anchor without its sign). A change of
  // place (tab, spec, view, task, epic, slice, rounds) adds a history entry;
  // a filter replaces the current one.
  const URL_KEYS = ["tab", "spec", "view", "task", "epic", "tracks", "edges", "nb", "q", "phase", "sq", "tview", "slice", "hr", "stage", "sec", "gv", "gm", "gd"];
  const PLACE_KEYS = ["tab", "spec", "view", "task", "epic", "slice", "hr", "gv", "gm"];
  const address = { landed: false, quiet: false, applying: false };

  function placeParams() {
    const p = new URLSearchParams();
    const put = (k, v) => { if (v) p.set(k, v); };
    put("tab", state.tab);
    put("spec", work.path);
    if (work.path) {
      put("view", work.view);
      put("task", work.task);
      put("sec", work.sec);
      if (work.tview !== "steps") put("tview", work.tview);
    }
    put("epic", mapView.selected);
    put("tracks", [...mapView.tracks].sort((a, b) => a - b).join(","));
    if (mapView.edges) put("edges", "1");
    if (mapView.nb) put("nb", "1");
    put("q", mapView.q);
    put("stage", mapView.stage);
    if (specsView.phase !== "all") put("phase", specsView.phase);
    put("sq", specsView.q);
    put("slice", hist.dir);
    if (hist.dir && hist.pair) put("hr", hist.pair.join("-"));
    put("gv", gview.view);
    if (gview.view === "scope") put("gm", gview.milestone);
    if (gview.depth !== 2) put("gd", String(gview.depth));
    return p;
  }

  // syncURL writes the page's place into the address: a new history entry
  // when the place changed after the page first landed, else in place.
  function syncURL() {
    if (address.applying || state.tab === null) return;
    const next = placeParams();
    const cur = new URLSearchParams(location.hash.slice(1));
    const quiet = address.quiet;
    address.quiet = false;
    if (next.toString() === cur.toString()) return;
    const moved = PLACE_KEYS.some((k) => (next.get(k) || "") !== (cur.get(k) || ""));
    const url = location.pathname + location.search + "#" + next.toString();
    if (moved && address.landed && !quiet) history.pushState(null, "", url);
    else history.replaceState(null, "", url);
  }

  // readURL takes the page's place from the address; a spec it names that is
  // not the open one is opened and fetched.
  function readURL() {
    const p = new URLSearchParams(location.hash.slice(1));
    const tab = p.get("tab");
    state.wantTab = TABS.includes(tab) ? tab : null;
    const spec = p.get("spec");
    if ((spec || null) !== work.path) {
      Object.assign(work, { path: spec || null, data: null, error: null });
      state.spec = work.path;
      if (work.path && state.tab !== null) fetchSpec();
    }
    const view = p.get("view");
    work.view = VIEWS.some(([v]) => v === view) ? view : work.data ? defaultView(work.data) : null;
    work.task = p.get("task");
    work.sec = /^\d+(?:\.\d+)*$/.test(p.get("sec") || "") ? p.get("sec") : null;
    work.tview = p.get("tview") === "board" ? "board" : "steps";
    mapView.selected = p.get("epic");
    mapView.tracks = new Set((p.get("tracks") || "").split(",").filter((x) => /^\d+$/.test(x)).map(Number));
    mapView.edges = p.get("edges") === "1";
    mapView.nb = p.get("nb") === "1";
    mapView.q = p.get("q") || "";
    const stage = p.get("stage");
    mapView.stage = STAGES.some(([k]) => k === stage) ? stage : null;
    const phase = p.get("phase");
    specsView.phase = phase === "op" || PHASES.some(([x]) => x === phase) ? phase : "all";
    specsView.q = p.get("sq") || "";
    const slice = p.get("slice") || null;
    const hr = /^(\d+)-(\d+)$/.exec(p.get("hr") || "");
    const pair = hr && Number(hr[1]) >= 1 && Number(hr[1]) < Number(hr[2]) ? [Number(hr[1]), Number(hr[2])] : null;
    if (slice !== hist.dir || String(pair) !== String(hist.pair)) {
      Object.assign(hist, { dir: slice, pair, data: slice === hist.dir ? hist.data : null, pairData: null, error: null, seen: null });
    }
    gview.view = GVIEWS.includes(p.get("gv")) ? p.get("gv") : null;
    gview.milestone = p.get("gm") || null;
    gview.depth = /^[1-4]$/.test(p.get("gd") || "") ? Number(p.get("gd")) : 2;
    $("map-q").value = mapView.q;
    $("specs-q").value = specsView.q;
    $("f-edges").setAttribute("aria-pressed", String(mapView.edges));
    $("f-nb").setAttribute("aria-pressed", String(mapView.nb));
    $("trackchips").querySelectorAll("[data-track]").forEach((b) => b.setAttribute("aria-pressed", String(mapView.tracks.has(Number(b.dataset.track)))));
  }

  readURL();
  window.addEventListener("popstate", () => {
    address.applying = true;
    readURL();
    if (state.tab !== null) setTab(state.wantTab || (state.graph ? "epics" : "specs"));
    render();
    address.applying = false;
  });

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
