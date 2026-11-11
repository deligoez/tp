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
