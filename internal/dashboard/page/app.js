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
