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

  // ---------- the page's language (1.17.0) ----------
  // Every word the page writes is t(key, params), read from i18n/<lang>.json
  // through tpPure.translator; the API serves state as values and the page
  // words them, so no payload carries a translated word. tk marks a key a
  // table holds, so the key check finds it. The choice is kept in this
  // browser, else read from the browser's language, and set on <html lang>.
  const LANG_KEY = "tp-dashboard-lang";
  const tk = (key) => key;
  // words is a table of keys, each worded in the page's language.
  const words = (table) => Object.fromEntries(Object.entries(table).map(([k, key]) => [k, t(key)]));
  let t = tpPure.translator({}, "en");
  function storedLang() {
    try { return localStorage.getItem(LANG_KEY); } catch (e) { return null; }
  }
  function storeLang(lang) {
    try { localStorage.setItem(LANG_KEY, lang); } catch (e) { /* the choice lasts this load */ }
  }
  // The words are asked for with the page's stamp, as its scripts are, so
  // a browser never reads an older tp's words.
  async function loadLang(lang) {
    const res = await fetch("i18n/" + lang + ".json?v=" + document.documentElement.dataset.v);
    if (!res.ok) throw new Error("i18n/" + lang + ".json: " + res.status);
    t = tpPure.translator(await res.json(), lang);
    document.documentElement.lang = lang;
    applyStatic();
  }
  // CONN_TEXT words the connection's state, which the stream sets.
  const CONN_TEXT = { connecting: tk("conn.connecting"), live: tk("conn.live"), lost: tk("conn.lost"), paused: tk("conn.paused") };
  // applyStatic words the page's own markup: an element's data-i18n is its
  // text, data-i18n-html its markup from the dictionary, and data-i18n-<attr>
  // that attribute.
  function applyStatic() {
    document.querySelectorAll("[data-i18n]").forEach((el) => { el.textContent = t(el.dataset.i18n); });
    document.querySelectorAll("[data-i18n-html]").forEach((el) => { el.innerHTML = t(el.dataset.i18nHtml); });
    ["title", "placeholder", "aria-label", "aria-roledescription"].forEach((attr) => {
      document.querySelectorAll("[data-i18n-" + attr + "]").forEach((el) => el.setAttribute(attr, t(el.getAttribute("data-i18n-" + attr))));
    });
    document.querySelectorAll("[data-lang]").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.lang === t.locale)));
    $("conn").textContent = t(CONN_TEXT[$("conn").dataset.state]);
    themeLabel();
    fullScreenLabels();
  }
  // The header's EN | TR switch loads the language picked, keeps the
  // choice and draws the page again in it.
  document.addEventListener("click", (ev) => {
    const b = ev.target.closest && ev.target.closest("[data-lang]");
    if (!b || b.dataset.lang === t.locale) return;
    storeLang(b.dataset.lang);
    loadLang(b.dataset.lang).then(() => {
      closeInfo();
      showPushed();
      if (state.tab !== null) render();
    }, () => { /* the page stays in the language it has */ });
  });

  // The event stream and the payloads it pushes, by event name (§2).
  const STREAM = "/api/events";
  const EVENTS = ["epic-export", "export-list", "summary"];
  const SOURCE_LABEL = {
    "epic-export": tk("source.epicExport"),
    "export-list": tk("source.exportList"),
    "summary": tk("source.summary"),
  };

  // The three tabs, in the lifecycle's order (§3). A stage and a phase are
  // the values the API serves, each with the key that words it.
  const TABS = ["epics", "specs", "work"];
  const STAGES = [["planned", tk("stage.planned")], ["claimed", tk("stage.claimed")], ["specced", tk("stage.specced")], ["in_progress", tk("stage.in_progress")], ["done", tk("stage.done")]];
  const PHASES = [["review", tk("phase.review")], ["decompose", tk("phase.decompose")], ["implement", tk("phase.implement")], ["audit", tk("phase.audit")], ["release", tk("phase.release")]];
  // word is a served value in the page's language through table, its key
  // for each value, or the value itself when the table does not hold it.
  const word = (table, v) => (table[v] ? t(table[v]) : v);

  const $ = (id) => document.getElementById(id);
  const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
  // descHTML shows an epic's description as escaped preformatted text: it is
  // markdown, and the page renders no markup from it.
  const descHTML = (s) => '<div><h4>' + esc(t("drawer.description")) + '</h4><pre class="ddesc">' + esc(s) + "</pre></div>";

  // ---------- the theme ----------
  // The header's Light/Dark toggle sets <html data-theme>, keeps the choice
  // in this browser and tells the graph card (tp-theme); without a stored
  // choice the system's setting rules. The toggle names the theme it
  // switches to.
  const THEME_KEY = "tp-dashboard-theme";
  function storedTheme() {
    try { return localStorage.getItem(THEME_KEY); } catch (e) { return null; }
  }
  const SYSTEM_DARK = window.matchMedia("(prefers-color-scheme: dark)");
  const darkNow = () => {
    const set = document.documentElement.dataset.theme;
    return set ? set === "dark" : SYSTEM_DARK.matches;
  };
  const themeLabel = () => { $("theme").textContent = t(darkNow() ? tk("theme.light") : tk("theme.dark")); };
  function applyTheme(theme) {
    if (theme) document.documentElement.dataset.theme = theme;
    else delete document.documentElement.dataset.theme;
    themeLabel();
    document.dispatchEvent(new Event("tp-theme"));
  }
  applyTheme(tpPure.pickTheme(storedTheme()));
  SYSTEM_DARK.addEventListener("change", themeLabel);
  $("theme").addEventListener("click", () => {
    const next = tpPure.nextTheme(darkNow());
    try { localStorage.setItem(THEME_KEY, next); } catch (e) { /* the choice lasts this load */ }
    applyTheme(next);
  });

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
    $("pushed").textContent = t("app.updated", { ago: tpPure.relTime(Date.now() - pushedAt.getTime(), t) });
    $("pushed").title = pushedAt.toLocaleTimeString(t.locale, { hour: "2-digit", minute: "2-digit", second: "2-digit" });
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
    const ago = tpPure.relTime(Date.now() - c.at, t);
    box.innerHTML = "<b>" + esc(t("since.title")) + "</b><span class=\"ago\">" + esc(ago) + "</span><ul>" +
      [[c.tasks, tk("since.tasks")], [c.rounds, tk("since.rounds")], [c.epics, tk("since.epics")]]
        .filter((x) => x[0]).map((x) => "<li>" + esc(t(x[1], { n: x[0] })) + "</li>").join("") + "</ul>" + info("since", t("since.title"));
  });

  // renderBanners shows each failed derivation's error and hint, and says
  // when the page keeps the last good state under it.
  function renderBanners() {
    $("banners").innerHTML = EVENTS.filter((name) => live[name].error).map((name) => {
      const e = live[name].error;
      const kept = live[name].data ? '<span class="kept">' + esc(t("banner.kept")) + "</span>" : "";
      return '<div class="banner"><b>' + esc(t("banner.failed", { source: t(SOURCE_LABEL[name]) })) + "</b><span>" + esc(e.error) + "</span>" +
        (e.hint ? '<span class="hint">' + esc(e.hint) + "</span>" : "") + kept + "</div>";
    }).join("");
  }

  // totalsOf is the one totals sentence (tpPure.epicTotals) of the export
  // g, which the tab, the share card and the Lifecycle all show; empty
  // without a graph.
  const totalsOf = (g) => (g && Array.isArray(g.epics) ? tpPure.epicTotals(g.epics, t) : "");

  // The tab labels name what each tab holds, with its unit.
  panel(["epic-export", "export-list"], () => {
    const g = data("epic-export");
    $("c-epics").textContent = g && Array.isArray(g.epics) ? t("count.epics", { n: g.epics.length }) : state.tab && !state.graph ? t("tabs.noGraph") : "";
    // The tab counts every epic, the share and the Lifecycle work epics:
    // one sentence names the conditions between them wherever both show.
    const totals = totalsOf(g);
    $("tab-epics").title = totals;
    $("totals").textContent = totals ? totals + "." : "";
    $("totals").hidden = !totals;
    const list = data("export-list");
    $("c-specs").textContent = list && Array.isArray(list.files) ? t("count.taskFiles", { n: list.files.length }) : "";
  });

  // With no graph the Epics tab says so and names the command that starts one.
  panel(["epic-export"], () => { $("epics-none").hidden = state.graph || state.tab === null; });

  // The lifecycle band (§6): epic stages when the phases count epics, and the
  // spec phases, which count task files when the repository has no graph. A
  // stage dims the graph card's epics outside it, as tp epic export reports
  // them; a phase opens Specs at that phase.
  panel(["summary"], () => {
    const sum = data("summary");
    if (!sum || !sum.band) {
      $("band").innerHTML = "";
      return;
    }
    const band = sum.band;
    const cell = (attr, k, label, n, pressed) => '<button type="button" class="pcell" ' + attr + '="' + esc(k) + '"' +
      (pressed === undefined ? "" : ' aria-pressed="' + pressed + '"') + '><b class="num">' + esc(t.num(n)) + '</b><span class="pl">' + esc(t(label)) + "</span></button>";
    const lane = (title, cells) => '<div class="plane"><span class="plab">' + esc(title) + '</span><div class="pcells">' + cells + "</div></div>";
    const byEpics = band.phases_of === "epics";
    const stages = band.stages || {};
    const phases = band.phases || {};
    $("band").innerHTML = (byEpics ? lane(t("band.lane.epics"), STAGES.map(([k, label]) => cell("data-stage", k, label, stages[k] || 0, mapView.stage === k)).join("")) : "") +
      lane(byEpics ? t("band.lane.epicPhases") : t("band.lane.filePhases"), PHASES.map(([k, label]) => cell("data-phase", k, label, phases[k] || 0)).join(""));
    fitAllRows();
  });
  $("band").addEventListener("click", (ev) => {
    const st = ev.target.closest("[data-stage]");
    const ph = ev.target.closest("[data-phase]");
    if (st) {
      mapView.stage = mapView.stage === st.dataset.stage ? null : st.dataset.stage;
      render();
      syncURL();
      if (mapView.stage) $("graphcard").scrollIntoView({ block: "nearest" });
    } else if (ph) {
      specsView.phase = ph.dataset.phase;
      setTab("specs");
      drawSpecs();
    }
  });

  // Milestone state (§6): one card per milestone in track order, read from
  // the export's milestones: open or closed with what it waits on, its gate's
  // members done of members (deferrable members in neither), and one line
  // (tpPure.msLine), the gate's purpose when it has one, else its threshold,
  // cut to the card's width with the whole text in its title. A card is a
  // button that opens its milestone in the drawer (milestoneHTML), where the
  // threshold and the purpose read in full; msView.detail is the one shown.
  const THRESHOLD_LABEL = {"written": tk("threshold.written"), "absent": tk("threshold.absent"), "placeholder": tk("threshold.placeholder"), "met": tk("threshold.met")};
  const msView = { detail: null };
  const gateOf = (g, id) => (((g.graph || {}).gates || {})[id]) || {};
  panel(["epic-export"], () => {
    const g = data("epic-export");
    const box = $("msstate");
    const ms = (state.graph && g && g.milestones) || [];
    box.hidden = !ms.length;
    if (box.hidden) return;
    const described = tpPure.describedCounts((g.graph && g.graph.epics) || []);
    box.innerHTML = '<span class="plab">' + esc(t("ms.title")) + '</span><div class="mscells">' + ms.map((m) => {
      const s = m.state || {};
      const waits = m.waiting_on && m.waiting_on.length ? t("ms.waitsOn", { on: m.waiting_on.join(", ") }) : "";
      const line = tpPure.msLine(gateOf(g, m.milestone).purpose, tpPure.thresholdLine(m, gateOf(g, m.milestone), words(THRESHOLD_LABEL), t));
      const dc = described[m.milestone] || { described: 0, all: 0 };
      return '<button type="button" class="ms" data-ms="' + esc(m.milestone) + '" data-open="' + !!m.open + '" aria-expanded="false" aria-controls="drawer"><b>' + esc(m.milestone) + '</b><span class="st" data-s="' + (m.open ? "done" : "stale") + '">' +
        esc(m.open ? t("ms.open") : t("ms.closed")) + "</span>" + (waits ? '<span class="sm muted">' + esc(waits) + "</span>" : "") +
        '<span class="sm">' + esc(t("ms.gate", { done: s.members_done || 0, n: s.members || 0 })) + "</span>" +
        '<span class="sm line ' + (line.kind === "purpose" ? 'purpose"' : 'th" data-th="' + esc(s.threshold || "absent") + '"') + ' title="' + esc(line.text) + '">' + esc(line.text) + "</span>" +
        '<span class="sm muted">' + esc(t("ms.described", { done: dc.described, n: dc.all })) + "</span></button>";
    }).join("") + "</div>";
    fitAllRows();
  });
  $("msstate").addEventListener("click", (ev) => {
    const b = ev.target.closest("[data-ms]");
    if (b) openMilestone(b.dataset.ms);
  });

  // Lifecycle over time (§17): the summary's series, one band per stage after
  // planned, stacked from done up, and the unknown band for epics whose stage
  // time tp did not record, drawn hatched so it never reads as a stage.
  const CFD_BANDS = [["done", "done", tk("cfd.band.done")], ["in_progress", "active", tk("cfd.band.in_progress")], ["specced", "ready", tk("cfd.band.specced")], ["claimed", "held", tk("cfd.band.claimed")], ["unknown", "waiting", tk("cfd.band.unknown")]];
  const HATCH = '<pattern id="cfd-hatch" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)"><rect width="6" height="6" fill="var(--waiting)"/><path d="M0 0V6" stroke="var(--muted)" stroke-width="1.4"/></pattern>';
  const cfdFill = (k, v) => (k === "unknown" ? "url(#cfd-hatch)" : "var(--" + v + ")");
  panel(["summary"], () => {
    const sum = data("summary");
    $("cfdcard").hidden = !state.graph || !sum;
    if ($("cfdcard").hidden) return;
    const days = sum.series || [];
    if (!days.length) {
      $("cfd").innerHTML = none(t("cfd.none"));
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
        '" fill="' + cfdFill(k, v) + '" opacity=".8"><title>' + esc(t(l)) + "</title></polygon>";
      base = up;
    });
    const first = days[0].date;
    const lastDay = days[days.length - 1];
    const axis = '<line class="ax" x1="' + L + '" x2="' + (W - R) + '" y1="' + (H - B) + '" y2="' + (H - B) + '"/>' +
      '<text x="' + (L - 6) + '" y="' + (T + 10) + '" text-anchor="end">' + t.num(top) + '</text><text x="' + (L - 6) + '" y="' + (H - B) + '" text-anchor="end">0</text>' +
      '<text x="' + L + '" y="' + (H - 6) + '">' + esc(first) + "</text>" +
      (days.length > 1 ? '<text x="' + (W - R) + '" y="' + (H - 6) + '" text-anchor="end">' + esc(lastDay.date) + "</text>" : "");
    const today = CFD_BANDS.map(([k, , l]) => t("cfd.count", { band: t(l), n: lastDay[k] || 0 })).join(", ");
    const swatch = (k, v) => '<svg viewBox="0 0 14 14" aria-hidden="true"><rect width="14" height="14" rx="3" fill="' + cfdFill(k, v) + '"/></svg>';
    $("cfd").innerHTML = '<svg class="cfd" viewBox="0 0 ' + W + " " + H + '" role="img" aria-label="' + esc(t("cfd.aria", { from: first, to: lastDay.date, today })) + '"><defs>' + HATCH + "</defs>" + areas + axis + "</svg>" +
      '<div class="cfdkey">' + CFD_BANDS.map(([k, v, l]) => "<span>" + swatch(k, v) + esc(t("cfd.key", { band: t(l), n: lastDay[k] || 0, day: lastDay.date })) + "</span>").join("") +
      "<span>" + esc(t("cfd.planned", { n: lastDay.planned || 0, day: lastDay.date })) + "</span></div>";
  });

  // What each panel and figure lists and where it comes from (§4), each
  // help a key.
  const INFO = {
    band: tk("info.band"),
    share: tk("info.share"),
    ready: tk("info.ready"),
    critical: tk("info.critical"),
    blocked: tk("info.blocked"),
    held: tk("info.held"),
    operator: tk("info.operator"),
    loops: tk("info.loops"),
    lint: tk("info.lint"),
    throughput: tk("info.throughput"),
    waiting: tk("info.waiting"),
    most: tk("info.most"),
    since: tk("info.since"),
    cfd: tk("info.cfd"),
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
    // A help is a key, or a function that words a help with live figures.
    const help = INFO[btn.dataset.info];
    pop.textContent = typeof help === "function" ? help() : help ? t(help) : "";
    pop.hidden = false;
    const r = btn.getBoundingClientRect();
    pop.style.top = (r.bottom + window.scrollY + 6) + "px";
    pop.style.left = Math.max(16, Math.min(r.left + window.scrollX - 8, document.documentElement.clientWidth - pop.offsetWidth - 16)) + "px";
    btn.setAttribute("aria-expanded", "true");
    openInfo = btn;
  });
  document.addEventListener("keydown", (ev) => { if (ev.key === "Escape") closeInfo(); });

  // info is the ⓘ button of the help INFO[key], labelled for what it is about.
  const info = (key, label) => '<button class="info" type="button" data-info="' + key + '" aria-label="' + esc(t("about.label", { label })) + '" aria-expanded="false">i</button>';
  const kpi = (key, value, label, cls, title) => '<div class="card kpi' + (cls ? " " + cls : "") + '"' + (title ? ' title="' + esc(title) + '"' : "") + ">" + info(key, label) +
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
    const totals = totalsOf(v.g);
    $("kpis").innerHTML = [
      kpi("share", esc(t.pct(f.share_done)) + "<small> " + esc(t("kpi.shareOf", { done: f.done, n: f.work })) + "</small>", t("kpi.share"), "", totals),
      kpi("ready", esc(t.num(f.ready)), t("kpi.ready")),
      kpi("critical", esc(t.num(f.critical_path)), t("kpi.critical"), "crit"),
      kpi("blocked", esc(t.num(f.blocked)), t("kpi.blocked"), f.blocked ? "warn" : ""),
      kpi("held", esc(t.num(f.held)), t("kpi.held"), f.held ? "op" : ""),
      kpi("operator", esc(t.num(queue)), t("kpi.operator"), queue ? "op" : ""),
      kpi("loops", esc(t.num(f.open_loops - (f.stale_loops || 0))) + (f.stale_loops ? "<small> " + esc(t("kpi.stale", { n: f.stale_loops })) + "</small>" : ""), t("kpi.loops")),
      kpi("lint", esc(t.num(lint.errors)) + "<small> / " + esc(t.num(lint.warnings)) + "</small>", t("kpi.lint"), lint.errors ? "warn" : ""),
      kpi("throughput", last ? esc(t.num(last.done)) + "<small> " + esc(t("kpi.inWeek", { week: last.week })) + "</small>" : "0", t("kpi.throughput")).replace("</div>", spark(weeks) + "</div>"),
    ].join("");
    fitAllRows();
  });

  // fitRows spreads a grid's cards, each at least minCard px wide, over the
  // fewest rows its width allows (tpPure.balancedColumns), as evenly as they
  // divide, every row filling the width (tpPure.cardSpans): nine KPI cards
  // at 1200 px read five on the first row and four stretched across the
  // second. With no width yet, in a hidden tab, the CSS's own columns hold
  // until the grid is laid out and changes size.
  function fitRows(box, minCard) {
    const cards = [...box.children];
    const cols = tpPure.balancedColumns(cards.length, box.clientWidth, minCard, parseFloat(getComputedStyle(box).columnGap) || 0);
    const grid = cols === null ? null : tpPure.cardSpans(cards.length, cols);
    if (!grid) {
      delete box.dataset.cols;
      cards.forEach((c) => { c.style.gridColumn = ""; });
      return;
    }
    // Only what changed is written, so the ResizeObserver this feeds settles.
    if (box.dataset.cols !== String(grid.columns)) {
      box.dataset.cols = String(grid.columns);
      box.style.setProperty("--cols", String(grid.columns));
    }
    cards.forEach((c, i) => {
      const span = "span " + grid.spans[i];
      if (c.style.gridColumn !== span) c.style.gridColumn = span;
    });
  }
  // The grids that spread their cards so, each with its narrowest card: the
  // KPI row, the Lifecycle's stage and phase cells, its milestone cards.
  const ROWS = [["#kpis", 140], ["#band .pcells", 120], ["#msstate .mscells", 170]];
  function fitAllRows() {
    ROWS.forEach(([sel, min]) => document.querySelectorAll(sel).forEach((box) => fitRows(box, min)));
  }
  const rowsSeen = new ResizeObserver(fitAllRows);
  ["kpis", "band", "msstate"].forEach((id) => rowsSeen.observe($(id)));

  // spark draws the weekly throughput as bars, scaled to the busiest week.
  function spark(weeks) {
    if (!weeks.length) return "";
    const top = Math.max(...weeks.map((w) => w.done));
    const w = 100 / weeks.length;
    return '<svg viewBox="0 0 100 32" preserveAspectRatio="none" role="img" aria-label="' + esc(t("kpi.throughput")) + '">' + weeks.map((x, i) => {
      const h = Math.max(2, (x.done / top) * 30);
      return '<rect x="' + (i * w + w * 0.15) + '" y="' + (32 - h) + '" width="' + w * 0.7 + '" height="' + h + '"><title>' + esc(x.week + ": " + t.num(x.done)) + "</title></rect>";
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
    return d ? t("list.dependents", { n: d.dependents }) : "";
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
    const evidence = ' --evidence "&lt;' + esc(t("op.evidence")) + '&gt;"';
    const done = (id) => "<code>tp epic done " + esc(id) + evidence + "</code>";
    const ops = v.plan.operator_conditions.map((id) => row(id, titleOf(v, id), t("op.condition"), done(id)));
    (v.plan.operator_groups || []).forEach((c) => ops.push(row(c.epic, titleOf(v, c.epic), t("op.chooseOne", { n: c.any_of.length }),
      c.any_of.map(done).join(" " + esc(t("op.or")) + " "))));
    (v.g.milestones || []).forEach((m) => {
      const o = m.operator || {};
      (o.gates || []).forEach((gate) => ops.push(row(gate.milestone, gate.threshold, t("op.gate"), "<code>tp epic gate " + esc(gate.milestone) + " --met" + evidence + "</code>")));
      (o.threshold_rewrites || []).forEach((r) => ops.push(row(r.milestone, r.threshold, t("op.threshold"), "<code>" + esc(r.command) + "</code>")));
    });
    (v.g.slices || []).filter((s) => s.grantable).forEach((s) => ops.push(row(t("op.slice"), sliceLabel(s), roundText(s),
      esc(t("op.staleAtCap")) + ": <code>tp epic review " + esc(sliceLabel(s)) + " --grant-round</code>")));
    return ops.concat(escalationRows());
  }

  // Waiting epics (dashboard-ux.md §2) answers what is blocked and by what:
  // the waiting epics of open milestones grouped under each direct blocker
  // (tpPure.waitingGroups), the groups in one section per blocker's
  // milestone in the graph file's order (tpPure.waitingSections), each
  // section folding on its own, its largest groups first and the rest
  // behind its own "show all"; those of milestones not yet open in one
  // counted row. Its total is the KPI's blocked epics, the parts named.
  const WAIT_SHOWN = 3;
  const BLOCKER_LABEL = { startable: tk("blocker.startable"), claimed: tk("blocker.claimed"), specced: tk("blocker.specced"), in_progress: tk("blocker.in_progress"), decision: tk("blocker.decision"), waiting: tk("blocker.waiting") };
  let waitView = { all: new Set(), folded: new Set() };
  // waitGroupHTML draws one group; wg keys it for focus to come back to a
  // link inside it after a redraw.
  function waitGroupHTML(gr, wg) {
    return '<div class="wgroup" data-wg="' + esc(wg) + '"><div class="wh">' + (gr.blocker ? epicLink(gr.blocker) : '<span class="muted">' + esc(t("wait.oneOf")) + "</span> " + gr.any_of.map(epicLink).join(", ")) +
      '<span class="bst" data-b="' + esc(gr.state) + '">' + esc(word(BLOCKER_LABEL, gr.state)) + "</span>" +
      (gr.critical ? '<span class="bcrit">' + esc(t("wait.critical")) + "</span>" : "") +
      '<span class="wn num" title="' + esc(t("wait.hereTitle")) + '">' + esc(t("wait.here", { n: gr.epics.length })) + "</span></div>" +
      '<div class="wids">' + gr.epics.map(epicLink).join(" ") + "</div></div>";
  }
  // waitSectionHTML draws a section as tpPure.waitSection decides it under
  // waitView.
  function waitSectionHTML(s) {
    const w = tpPure.waitSection(s, waitView, WAIT_SHOWN, t);
    return '<section class="wsec"><button type="button" class="wsh" data-wsec="' + esc(w.key) + '" aria-expanded="' + w.expanded + '"><b>' + esc(w.title) + "</b>" +
      '<span class="muted num">' + esc(t("wait.blockers", { n: s.groups.length }) + ", " + t("wait.onThem", { n: s.epics })) + "</span></button>" +
      w.groups.map((gr, i) => waitGroupHTML(gr, w.key + ":" + i)).join("") +
      (w.more ? '<button type="button" class="chip wall" data-wall="' + esc(w.key) + '" aria-expanded="' + w.more.expanded + '">' + esc(w.more.text) + "</button>" : "") + "</section>";
  }
  function waitingHTML(v) {
    const open = {};
    (v.g.milestones || []).forEach((m) => { open[m.milestone] = !!m.open; });
    const w = tpPure.waitingGroups(v.plan.waiting, v.epics, v.plan.ready, open);
    const blocked = v.plan.figures.blocked;
    if (!blocked) return none(t("wait.none"));
    const deferred = blocked - v.plan.waiting.length;
    const byMs = {};
    w.closed.forEach((id) => { const m = (v.epics[id] || {}).milestone; byMs[m] = (byMs[m] || 0) + 1; });
    const parts = [t("wait.partOpen", { n: w.open })];
    if (w.closed.length) parts.push(t("wait.partClosed", { n: w.closed.length }));
    if (deferred) parts.push(t("wait.partDeferred", { n: deferred }));
    const order = [].concat(...((v.g.graph && v.g.graph.milestones) || []));
    const crit = (v.g.critical_path && v.g.critical_path.by_count) || [];
    return '<p class="wsum"><b class="num">' + esc(t("wait.blocked", { n: blocked })) + "</b>: " + esc(parts.join("; ")) + ".</p>" +
      tpPure.waitingSections(w.groups, v.epics, order, crit).map(waitSectionHTML).join("") +
      (w.closed.length ? '<div class="wgroup wclosed"><div class="wh"><span>' + esc(t("wait.closedRow", { n: w.closed.length })) +
        '</span></div><div class="wids muted">' + esc(Object.keys(byMs).map((m) => m + " " + t.num(byMs[m])).join(", ")) + "</div></div>" : "");
  }
  // A section's header folds it and its button shows all of its groups,
  // each section on its own.
  $("waitcard").addEventListener("click", (ev) => {
    const b = ev.target.closest("[data-wall], [data-wsec]");
    if (!b) return;
    const key = focusKey(b);
    waitView = tpPure.flipWait(waitView, b.dataset.wall !== undefined ? "all" : "folded", b.dataset.wall !== undefined ? b.dataset.wall : b.dataset.wsec);
    render();
    refocus(key);
  });

  // The now lists (§4): what can start, what waits on the operator, what
  // waits on other work, and what the most work waits on.
  panel(["summary", "epic-export"], () => {
    const v = planView();
    $("now").hidden = !v;
    $("waitcard").hidden = !v;
    if (!v) return;
    $("now-ready").innerHTML = v.plan.ready.map((id) => row(id, titleOf(v, id), dependents(v, id), "", true)).join("") || none(t("list.noReady"));
    $("now-operator").innerHTML = operatorRows(v).join("") || none(t("list.noOperator"));
    $("now-waiting").innerHTML = waitingHTML(v);
    $("now-most").innerHTML = v.plan.most_waited_on.slice(0, 10).map((id) => row(id, titleOf(v, id), dependents(v, id), "", true)).join("") || none(t("list.noMost"));
  });

  INFO.slices = tk("info.slices");

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
    return (k.name ? t("slice.set", { name: k.name }) : t("slice.unnamed")) + ", " + t("count.epics", { n: (k.ids || []).length });
  }

  // roundText says the round against the cap and the grants spent.
  function roundText(s) {
    const parts = [t("slice.round", { n: s.round })];
    if (s.max_rounds !== null && s.max_rounds !== undefined) parts.push(t("slice.cap", { n: s.max_rounds }));
    return parts.join(", ") + (s.grants_spent ? " + " + t("slice.grants", { n: s.grants_spent }) : "");
  }

  // LOOP_LABEL words a loop's state as tp export and tp epic export serve it.
  const LOOP_LABEL = { none: tk("loop.state.none"), open: tk("loop.state.open"), stale: tk("loop.state.stale"), cap_reached: tk("loop.state.cap_reached"),
    in_flight: tk("loop.state.in_flight"), done: tk("loop.state.done"), complete: tk("loop.state.complete") };

  // curve draws each recorded round's finding count, so a converging slice
  // reads as a falling line.
  function curve(rounds) {
    if (!rounds || !rounds.length) return "";
    const top = Math.max(1, ...rounds.map((r) => r.total));
    const step = rounds.length > 1 ? 88 / (rounds.length - 1) : 0;
    const pts = rounds.map((r, i) => [4 + i * step, 21 - (r.total / top) * 18]);
    const label = rounds.map((r) => t("slice.roundCount", { round: r.round, n: r.total })).join(", ");
    return '<svg class="curve" viewBox="0 0 96 24" role="img" aria-label="' + esc(t("slices.curveAria", { rounds: label })) + '"><title>' + esc(label) + "</title>" +
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
      $("slices").innerHTML = '<p class="empty">' + t.html("slices.none_html") + "</p>";
      return;
    }
    const col = { slice: t("slices.col.slice"), round: t("slices.col.round"), state: t("slices.col.state"), open: t("slices.col.open"), curve: t("slices.col.curve") };
    $("slices").innerHTML = '<table class="loops"><thead><tr><th>' + esc(col.slice) + "</th><th>" + esc(col.round) + "</th><th>" + esc(col.state) + "</th><th>" + esc(col.open) + "</th><th>" + esc(col.curve) + "</th></tr></thead><tbody>" +
      slices.map((s) => {
        const stale = s.state === "stale" && s.stale_units && s.stale_units.length ? ' title="' + esc(t("slices.staleTitle", { round: s.round, units: s.stale_units.join(", ") })) + '"' : "";
        const open = Object.keys(s.open_by_role || {}).sort().map((r) => esc(r) + " " + esc(t.num(s.open_by_role[r]))).join(", ") || esc(t("slices.noOpen"));
        const on = s.dir === hist.dir;
        return '<tr class="' + (on ? "on" : "") + '"><td class="k"><button type="button" class="slink" data-slice="' + esc(s.dir) + '" aria-pressed="' + on +
          '" title="' + esc(t("slices.showHistory")) + '">' + esc(sliceTitle(s)) + "</button>" +
          (s.key && s.key.kind === "epics" ? '<small class="scmd">' + esc(sliceLabel(s)) + "</small>" : "") + '</td><td data-l="' + esc(col.round) + '">' + esc(roundText(s)) + '</td><td data-l="' + esc(col.state) + '"><span class="st" data-s="' + esc(s.state) + '"' + stale + ">" +
          esc(word(LOOP_LABEL, s.state)) + '</span></td><td class="n" data-l="' + esc(col.open) + '">' + open + '</td><td data-l="' + esc(col.curve) + '">' + curve(sum ? sum.rounds[s.dir] : null) + "</td></tr>";
      }).join("") + "</tbody></table>";
  });

  // ---------- a slice's graph history (§11) ----------
  // One slice at a time, from tp epic review <slice> --history: its rounds in
  // order with their findings by role and what changed since the round
  // before, then what changed since its last round. Two chosen rounds are
  // fetched with from and to, their difference drawn beside the earlier
  // round's findings. Fetched when the slice is shown and again on every push.
  const hist = { dir: null, pair: null, data: null, pairData: null, error: null, seq: 0, seen: null };
  INFO.history = tk("info.history");

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
      return { error: t("err.fetch", { what: "tp epic review --history", message: err.message }), hint: t("err.stillRunning") };
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
      Object.assign(hist, { data: null, pairData: null, error: s ? { error: t("hist.noKey"), hint: t("hist.noKeyHint") } : null });
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
    const n = (list, key) => { if (list && list.length) parts.push(t(key, { n: list.length })); };
    n(d.units_added, tk("diff.unitsAdded"));
    n(d.units_removed, tk("diff.unitsRemoved"));
    n(d.edges_added, tk("diff.edgesAdded"));
    n(d.edges_removed, tk("diff.edgesRemoved"));
    n(d.whys_changed, tk("diff.whysChanged"));
    n(d.fields_changed, tk("diff.fieldsChanged"));
    if (d.hashes_only) n(d.units_changed, tk("diff.unitsChanged"));
    return parts;
  }
  const diffLine = (d) => (d ? diffParts(d).join(", ") || t("diff.none") : t("diff.first"));

  function findingsLine(byRole) {
    const roles = Object.keys(byRole || {}).sort();
    return roles.length ? roles.map((r) => esc(r) + " " + esc(t.num(byRole[r].length))).join(", ") : '<span class="muted">' + esc(t("hist.clean")) + "</span>";
  }

  // diffDetail draws every change of a difference: units, edges, whys with
  // both texts, and fields with both values.
  function diffDetail(d) {
    if (!d) return none(t("diff.noDifference"));
    const out = [];
    const code = (s) => "<code>" + esc(s) + "</code>";
    const list = (title, items) => { if (items.length) out.push('<div class="dpart"><h4>' + esc(title) + "</h4>" + items.join("") + "</div>"); };
    const units = (xs) => (xs || []).map((u) => '<div class="dl">' + code(u) + "</div>");
    list(t("diff.head.unitsAdded"), units(d.units_added));
    list(t("diff.head.unitsRemoved"), units(d.units_removed));
    list(t("diff.head.edgesAdded"), (d.edges_added || []).map((e) => '<div class="dl ins">' + code(e.unit) + " → " + code(e.to) + (e.why ? '<span class="dwhy">' + esc(e.why) + "</span>" : "") + "</div>"));
    list(t("diff.head.edgesRemoved"), (d.edges_removed || []).map((e) => '<div class="dl del">' + code(e.unit) + " → " + code(e.to) + (e.why ? '<span class="dwhy">' + esc(e.why) + "</span>" : "") + "</div>"));
    list(t("diff.head.whysChanged"), (d.whys_changed || []).map((w) => '<div class="dl">' + code(w.unit) + " → " + code(w.to) +
      '<div class="was">' + esc(w.before) + '</div><div class="now">' + esc(w.after) + "</div></div>"));
    list(t("diff.head.fieldsChanged"), (d.fields_changed || []).map((f) => '<div class="dl">' + code(f.unit) + " " + esc(f.field) +
      (f.before ? '<div class="was">' + esc(f.before) + "</div>" : "") + (f.after ? '<div class="now">' + esc(f.after) + "</div>" : "") + "</div>"));
    if (d.hashes_only) list(t("diff.head.unitsChanged"), units(d.units_changed));
    return out.join("") || none(t("diff.nothing"));
  }

  // SEVERITY and DISPOSITION word a finding's severity and its resolution's
  // status, as the round files record them.
  const SEVERITY = { critical: tk("sev.critical"), high: tk("sev.high"), medium: tk("sev.medium"), low: tk("sev.low") };
  const DISPOSITION = { fixed: tk("disp.fixed"), wontfix: tk("disp.wontfix"), duplicate: tk("disp.duplicate") };

  // findingsList draws a round's findings under their role, each with its
  // severity, place and disposition.
  function findingsList(byRole) {
    const roles = Object.keys(byRole || {}).sort();
    if (!roles.length) return none(t("hist.nothingFound"));
    return roles.map((r) => '<div class="dpart"><h4>' + esc(r) + ' <span class="muted num">' + esc(t.num(byRole[r].length)) + "</span></h4>" +
      byRole[r].map((f) => {
        const res = f.resolved && f.resolved.status ? '<span class="sm muted">' + esc(word(DISPOSITION, f.resolved.status)) + "</span>" : "";
        return '<div class="fnd"><span class="sev" data-sev="' + esc(f.severity || "") + '">' + esc(word(SEVERITY, f.severity || "")) + "</span><code>" + esc(f.location || "") + "</code>" + res +
          "<p>" + esc(f.finding || "") + "</p></div>";
      }).join("") + "</div>").join("");
  }

  function drawHistory() {
    const box = $("slice-history");
    const s = selectedSlice();
    box.hidden = !s;
    if (!s) return;
    const head = '<div class="hhead"><h3>' + esc(t("hist.title")) + " <code>" + esc(sliceLabel(s)) + "</code></h3>" + info("history", t("hist.about")) +
      '<span class="spacer"></span><button type="button" class="chip" data-hclose>' + esc(t("hist.close")) + "</button></div>";
    if (hist.error) {
      box.innerHTML = head + '<div class="banner"><b>' + esc(t("hist.failed")) + "</b><span>" + esc(hist.error.error) + "</span>" +
        (hist.error.hint ? '<span class="hint">' + esc(hist.error.hint) + "</span>" : "") + "</div>";
      return;
    }
    const h = hist.data;
    if (!h) {
      box.innerHTML = head + none(t("hist.reading"));
      return;
    }
    const rounds = h.rounds || [];
    if (!rounds.length) {
      box.innerHTML = head + none(t("hist.noRound"));
      return;
    }
    const pair = shownPair();
    const since = (from, d) => t("hist.since", { round: from, change: diffLine(d) });
    const rows = rounds.map((r) => '<tr class="' + (pair && pair.includes(r.round) ? "on" : "") + '"><td>' + esc(t("hist.round", { n: r.round })) + "</td><td>" + findingsLine(r.findings_by_role) +
      "</td><td>" + esc(r.diff_from ? since(r.diff_from, r.diff) : diffLine(null)) + "</td></tr>").join("") +
      (h.current ? "<tr><td>" + esc(t("hist.now")) + '</td><td class="muted">' + esc(t("hist.notReviewed")) + "</td><td>" + esc(since(h.current.from_round, h.current.diff)) + "</td></tr>" : "");
    const opts = (sel) => rounds.map((r) => '<option value="' + r.round + '"' + (r.round === sel ? " selected" : "") + ">" + esc(t("slice.round", { n: r.round })) + "</option>").join("");
    let compare = none(t("hist.needTwo"));
    if (pair) {
      const pd = hist.pairData;
      const pick = '<div class="hpick"><label>' + esc(t("hist.compare")) + ' <select data-hr="0" aria-label="' + esc(t("hist.earlier")) + '">' + opts(pair[0]) + "</select></label>" +
        "<label>" + esc(t("hist.with")) + ' <select data-hr="1" aria-label="' + esc(t("hist.later")) + '">' + opts(pair[1]) + "</select></label></div>";
      let body = none(t("hist.readingTwo"));
      if (isError(pd)) body = '<div class="banner"><b>' + esc(t("hist.compareFailed")) + "</b><span>" + esc(pd.error) + "</span></div>";
      else if (pd && pd.rounds && pd.rounds.length === 2) {
        const [a, b] = pd.rounds;
        body = '<div class="hcmp"><div><h4 class="hsub">' + esc(t("hist.roundFindings", { n: a.round })) + "</h4>" + findingsList(a.findings_by_role) + "</div>" +
          '<div><h4 class="hsub">' + esc(t("hist.fromTo", { from: a.round, to: b.round })) + '<span class="muted sm"> ' + esc(diffLine(b.diff)) + "</span></h4>" + diffDetail(b.diff) + "</div></div>";
      }
      compare = pick + body;
    }
    box.innerHTML = head + '<div class="scroll"><table class="loops hrounds"><thead><tr><th>' + esc(t("hist.col.round")) + "</th><th>" + esc(t("hist.col.findings")) + "</th><th>" + esc(t("hist.col.changed")) + "</th></tr></thead><tbody>" +
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

  // ---------- the plan's epics: marks, search, edges ----------
  // The graph card (graph.js) draws the plan; the page keeps what the
  // drawer, the lists and the text chain read: each mark's words, the
  // selection, the lifecycle's stage filter, and the graph's edges both ways.
  const MARK_LABEL = { done: tk("mark.done"), ready: tk("mark.ready"), claimed: tk("mark.claimed"), specced: tk("mark.specced"), in_progress: tk("mark.in_progress"),
    blocked: tk("mark.blocked"), held: tk("mark.held"), waiting: tk("mark.waiting"), moot: tk("mark.moot") };
  const mapView = { selected: null, deps: {}, dependents: {}, stage: null };
  // The graph card's help ends with graph.js's note, a gate closure's
  // described count.
  INFO.graph = () => [t("info.graph"), window.tpApp && window.tpApp.graphNote ? window.tpApp.graphNote() : ""].filter(Boolean).join(" ");

  // FOLD maps the letters the epic search folds before Unicode decomposition:
  // the Turkish dotless and dotted i, which decomposition and lower-casing do
  // not bring to a plain i, and the Turkish letters with a cedilla, breve or
  // diaeresis, spelled out so a test can read them.
  const FOLD = {"ı": "i", "İ": "i", "ş": "s", "Ş": "s", "ğ": "g", "Ğ": "g", "ç": "c", "Ç": "c", "ö": "o", "Ö": "o", "ü": "u", "Ü": "u"};
  const FOLD_RE = new RegExp("[" + Object.keys(FOLD).join("") + "]", "g");
  // fold reduces s to what the epic search compares: case and diacritics
  // gone, so isik matches IŞIK and ışık alike.
  const fold = (s) => String(s).replace(FOLD_RE, (c) => FOLD[c]).normalize("NFD").replace(/\p{M}/gu, "").toLowerCase();
  // markOf reads an epic's mark (tpPure.markState) given its milestone's
  // report from the export.
  const markOf = (e, report) => tpPure.markState(e, !!report && !report.open);

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

  // Full screen (dashboard-ux.md §2): a card's button asks the Fullscreen
  // API for the page and marks the card to fill it, so the drawer still opens
  // beside it; Esc, which the browser handles, leaves it. Where the API is
  // missing the card fills the window alone and Esc unmarks it. While a card
  // fills the screen the plain wheel pans it.
  function setFullScreen(card) {
    document.querySelectorAll(".card.fs").forEach((c) => { if (c !== card) c.classList.remove("fs"); });
    if (card) card.classList.add("fs");
    document.body.classList.toggle("mapfs", !!card);
    document.querySelectorAll("[data-fs]").forEach((b) => b.setAttribute("aria-pressed", String(!!card && b.dataset.fs === card.id)));
    fullScreenLabels();
  }
  // fullScreenLabels words each full-screen button by its pressed state.
  function fullScreenLabels() {
    document.querySelectorAll("[data-fs]").forEach((b) => {
      b.textContent = b.getAttribute("aria-pressed") === "true" ? t("graph.exitFullScreen") : t("graph.fullScreen");
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

  // ---------- the selected epic: its chain, its drawer (§4, §11) ----------
  // Selecting an epic on the graph, in a now list or in the drawer draws its
  // chain with every edge's why and opens its drawer: its fields, what it
  // depends on and what depends on it with the whys, downstream, its bound
  // specs, its own stepper and its graph history, fetched from
  // /api/epic-history when it opens and again on every push.
  const epicHist = { id: null, data: null, error: null, seq: 0 };
  INFO.matrix = tk("info.matrix");
  INFO.drawer = tk("info.drawer");
  const ESTEPS = [["claimed", tk("estep.claimed")], ["bound", tk("estep.bound")], ["done", tk("estep.done")]];
  const PHASE_NAME = Object.fromEntries(PHASES);

  // selectEpic selects id, or clears the selection when id is the selected
  // epic or null; centre asks the graph card to take its view to it, as a
  // selection made outside the card does.
  // opener is what held focus when a selection opens the drawer, read before
  // the redraws below replace it; drawDrawer takes it. keepFocus, set by the
  // graph's search, leaves focus where it is when the drawer opens, so the
  // next Enter there steps to the next match.
  let opener = null;
  let keepFocusOnce = false;
  function selectEpic(id, centre, keepFocus) {
    if ($("drawer").hidden) opener = { el: document.activeElement, key: focusKey(document.activeElement) };
    // An epic takes the drawer over from a milestone shown in it.
    moveDrawer({ type: "epic", id });
    keepFocusOnce = !!keepFocus;
    drawSelected();
    keepFocusOnce = false;
    syncURL();
    if (window.tpApp && window.tpApp.onSelect) window.tpApp.onSelect(mapView.selected, !!centre);
  }

  // openMilestone shows milestone id in the drawer, or closes the drawer
  // when id is the milestone it shows; a selected epic gives way to it.
  function openMilestone(id) {
    if ($("drawer").hidden) opener = { el: document.activeElement, key: focusKey(document.activeElement) };
    const had = mapView.selected;
    moveDrawer({ type: "milestone", id });
    drawSelected();
    syncURL();
    if (had && window.tpApp && window.tpApp.onSelect) window.tpApp.onSelect(mapView.selected, false);
  }

  // The drawer's state is the milestone shown and the epic selected; it
  // moves only by tpPure.drawerNext.
  function drawerState() {
    return { detail: msView.detail, selected: mapView.selected };
  }
  function moveDrawer(act) {
    const next = tpPure.drawerNext(drawerState(), act);
    msView.detail = next.detail;
    mapView.selected = next.selected;
  }

  const graphEpic = (g, id) => ((g.graph && g.graph.epics) || []).find((e) => e.id === id) || null;
  const epicLink = (id) => '<button type="button" class="link" data-epic="' + esc(id) + '">' + esc(id) + "</button>";
  const stamp = (iso) => (iso ? String(iso).replace("T", " ").replace(/\..*$|Z$/, "").slice(0, 16) + " UTC" : "");

  // chainEdges walks the selected epic's edges one way, breadth first: up,
  // to what it depends on, or down, to what depends on it, each edge with
  // its level, 1 for the epic's own.
  function chainEdges(id, up) {
    const seen = new Set([id]);
    const out = [];
    let front = [id];
    for (let level = 1; front.length; level++) {
      const next = [];
      front.forEach((x) => {
        const edges = up ? (mapView.deps[x] || []).map((d) => ({ from: x, to: d.id, why: d.why, group: d.group }))
          : [...new Set(mapView.dependents[x] || [])].flatMap((y) => (mapView.deps[y] || []).filter((d) => d.id === x).map((d) => ({ from: y, to: x, why: d.why, group: d.group })));
        edges.forEach((e) => {
          out.push({ ...e, level });
          const n = up ? e.to : e.from;
          if (!seen.has(n)) { seen.add(n); next.push(n); }
        });
      });
      front = next;
    }
    return out;
  }

  // whyHTML is an edge's why, or that none was recorded.
  const whyHTML = (why) => '<span class="dwhy">' + esc(why || t("chain.noWhy")) + "</span>";
  // groupHTML names the any-of group an edge belongs to.
  const groupHTML = (group) => (group ? ' <span class="esub">' + esc(t("chain.group", { ids: group.join(", ") })) + "</span>" : "");

  function edgeList(edges, empty) {
    if (!edges.length) return none(empty);
    return '<ul class="elist">' + edges.map((e) => "<li>" + epicLink(e.from) + ' <span class="muted">' + esc(t("chain.dependsOn")) + "</span> " + epicLink(e.to) +
      groupHTML(e.group) + whyHTML(e.why) + "</li>").join("") + "</ul>";
  }

  // chainHTML lists one side of a chain: the epic's own edges, then the
  // deeper ones folded into a details element, so a chain of thousands is
  // a handful of tab stops until it is opened.
  function chainHTML(edges, empty) {
    const direct = edges.filter((e) => e.level === 1);
    const deeper = edges.filter((e) => e.level > 1);
    return edgeList(direct, empty) + (deeper.length ? '<details class="deeper"><summary>' + esc(t("chain.deeper", { n: deeper.length })) + "</summary>" + edgeList(deeper, "") + "</details>" : "");
  }

  // drawChain draws the selected epic's chain as text, the collapsible view
  // under the graph card: every edge it waits on and every edge waiting on
  // it, transitively, each with its why, so a keyboard user has the
  // dependency picture the canvas cannot give them.
  function drawChain(g) {
    $("chaintext").hidden = !g;
    if (!g) return;
    const id = mapView.selected;
    $("chain").innerHTML = id && mapView.deps ? '<p class="note">' + esc(id) + ": " + esc((graphEpic(g, id) || {}).title || "") + "</p>" +
      '<div class="two-col"><div><h3 class="hsub">' + esc(t("chain.waitsOn")) + "</h3>" + chainHTML(chainEdges(id, true), t("chain.noDeps")) + "</div>" +
      '<div><h3 class="hsub">' + esc(t("chain.waitedOn")) + "</h3>" + chainHTML(chainEdges(id, false), t("chain.noDependents")) + "</div></div>"
      : none(t("chain.pick"));
  }

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
    const share = (m) => (rep[m] || {}).share_done || 0;
    const head = (m) => ' style="' + tint(mst(m), 24) + '" title="' + esc(t("matrix.head", { ms: m, state: (rep[m] || {}).open === false ? t("ms.closed") : t("ms.open"), share: t.pct(share(m)) })) + '"';
    if (!order.length) {
      $("matrix").innerHTML = none(t("matrix.none"));
      return;
    }
    $("matrix").innerHTML = '<table class="matrix"><tr><th></th>' + order.map((m) => "<th" + head(m) + ">" + esc(m) + "</th>").join("") + "</tr>" +
      order.map((m) => '<tr><th class="r"' + head(m) + ">" + esc(m) + "</th>" + order.map((o) => {
        if (m === o) return '<td class="diag" style="' + tint(mst(m), 40) + '" title="' + esc(t("matrix.diag", { ms: m, share: t.pct(share(m)) })) + '">' + esc(t.num(Math.round(share(m) * 100))) + "</td>";
        const c = cell[m + "\u0000" + o];
        if (!c) return "<td></td>";
        const v = c.done === c.edges ? "done" : c.done ? "active" : (rep[m] || {}).open ? "blocked" : "waiting";
        return '<td style="' + tint(v, Math.min(58, 26 + c.edges * 3)) + '" title="' + esc(t("matrix.cell", { from: m, to: o, n: c.edges, done: c.done })) + '">' + esc(t.num(c.edges)) + "</td>";
      }).join("") + "</tr>").join("") + "</table>" +
      '<div class="mkey">' + [["done", tk("matrix.key.done")], ["active", tk("matrix.key.some")], ["blocked", tk("matrix.key.open")], ["waiting", tk("matrix.key.closed")]]
        .map(([v, key]) => '<span><i style="' + tint(v, 50) + '"></i>' + esc(t(key)) + "</span>").join("") + "<span>" + esc(t("matrix.key.diag")) + "</span></div>";
  }

  // epicStepper is the epic's own stepper: claimed (who claimed it, or
  // skipped when it was bound or done unclaimed), bound, done.
  function epicStepper(e, ge) {
    const claim = ge && ge.claimed;
    const bound = (e.files || []).filter((f) => f.bound_at).map((f) => f.bound_at).sort()[0] || null;
    const reached = { claimed: !!claim || ["specced", "in_progress", "done"].includes(e.status), bound: (e.files || []).length > 0, done: e.status === "done" };
    const at = (iso) => (iso ? ", " + stamp(iso) : "");
    const sub = {
      claimed: claim ? t("estep.by", { who: claim.by }) + at(claim.at) : reached.claimed ? t("estep.skipped") : t("estep.notClaimed"),
      bound: reached.bound ? t("count.specs", { n: e.files.length }) + at(bound) : t("estep.noSpec"),
      done: reached.done ? (e.done_at ? stamp(e.done_at) : t("mark.done")) : reached.bound ? t("estep.whenReleased") : t("estep.notYet"),
    };
    const cur = ESTEPS.findIndex(([k]) => !reached[k]);
    return '<ol class="stepper e3" aria-label="' + esc(t("estep.label")) + '">' + ESTEPS.map(([k, label], i) => {
      let st = reached[k] ? "done" : i === cur ? "cur" : "todo";
      if (k === "claimed" && st === "done" && !claim) st = "skip";
      const mk = st === "done" ? CHECK : st === "skip" ? "–" : String(i + 1);
      return '<li class="' + st + '"' + (st === "cur" ? ' aria-current="step"' : "") + '><span class="sn">' + mk + '</span><span class="sl">' + esc(t(label)) + '</span><span class="sd">' + esc(sub[k]) + "</span></li>";
    }).join("") + "</ol>";
  }

  // epicHistoryHTML draws the --of-epic payload: per slice, each recorded
  // round's edges of the epic with their whys, as that round's snapshot kept
  // them.
  function epicHistoryHTML(id) {
    if (epicHist.id !== id || (!epicHist.data && !epicHist.error)) return none(t("ehist.reading"));
    if (epicHist.error) return '<div class="banner"><b>' + esc(t("ehist.failed")) + "</b><span>" + esc(epicHist.error.error) + "</span>" + (epicHist.error.hint ? '<span class="hint">' + esc(epicHist.error.hint) + "</span>" : "") + "</div>";
    const h = epicHist.data;
    const known = (x) => (((data("epic-export") || {}).epics) || []).some((e) => e.id === x);
    if (!(h.slices || []).length) return none(h.message || t("ehist.none"));
    return h.slices.map((s) => '<div class="ehs"><h5><code>' + esc(sliceLabel({ key: s.key, dir: s.state_dir })) + "</code></h5><ol class=\"ehr\">" + (s.rounds || []).map((r) => "<li><b>" + esc(t("hist.round", { n: r.round })) + "</b>" +
      (r.hashes_only ? '<span class="muted sm">' + esc(t("ehist.hashesOnly")) + "</span>"
        : (r.edges || []).length ? '<ul class="elist">' + r.edges.map((x) => "<li>" + esc(t("chain.dependsOn")) + " " + (known(x.to) ? epicLink(x.to) : esc(x.to)) + whyHTML(x.why) + "</li>").join("") + "</ul>"
          : '<span class="muted sm">' + esc(t("ehist.noDependency")) + "</span>") + "</li>").join("") + "</ol></div>").join("");
  }

  async function fetchEpicHistory(id) {
    const seq = ++epicHist.seq;
    let body;
    try {
      const res = await fetch("/api/epic-history?epic=" + encodeURIComponent(id), { cache: "no-store" });
      body = await res.json();
    } catch (err) {
      body = { error: t("err.fetch", { what: "tp epic review --history --of-epic", message: err.message }), hint: t("err.stillRunning") };
    }
    if (seq !== epicHist.seq) return;
    Object.assign(epicHist, isError(body) ? { id, error: body } : { id, data: body, error: null });
    drawDrawer();
  }

  function fileLine(f) {
    const tasks = f.tasks || {};
    const loop = (key, l) => (l && l.round ? t(key, { n: l.round, state: word(LOOP_LABEL, l.state) }) : "");
    return '<li><button type="button" class="link" data-spec="' + esc(f.path) + '">' + esc(f.path) + "</button><small>" + esc([word(PHASE_NAME, f.phase),
      t("file.tasksDone", { done: tasks.done || 0, n: (tasks.open || 0) + (tasks.wip || 0) + (tasks.done || 0) }), loop(tk("file.ground"), f.ground), loop(tk("file.review"), f.review), loop(tk("file.audit"), f.audit)].filter(Boolean).join(", ")) + "</small></li>";
  }

  // focusKey names a focusable element so a redraw's replacement is found
  // again: a toggle by its data key (tpPure.toggleKey), a waiting group's
  // link by its group, or a list link by its box; null for anything else.
  // The graph card draws on a canvas and has no focusable node.
  const FOCUSABLE = "[data-epic], [data-ms], [data-wsec], [data-wall]";
  function focusKey(el) {
    if (!el || !el.closest) return null;
    const toggle = el.dataset ? tpPure.toggleKey(el.dataset) : null;
    if (toggle) return "toggle:" + toggle;
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
  // refocus puts focus on the element focusKey names key, as after a
  // toggle's redraw replaced the button that held it.
  function refocus(key) {
    const el = key && [...document.querySelectorAll(FOCUSABLE)].find((x) => focusKey(x) === key);
    if (el) el.focus({ preventScroll: true });
  }
  function returnFocus() {
    const r = drawerReturn;
    drawerReturn = null;
    const els = [...document.querySelectorAll(FOCUSABLE)];
    const key = tpPure.focusAfterClose({ connected: !!r.el && r.el.isConnected && r.el !== document.body, keys: r.keys }, els.map(focusKey));
    if (key === "same") r.el.focus({ preventScroll: true });
    else if (key) els.find((x) => focusKey(x) === key).focus({ preventScroll: true });
  }

  function drawDrawer() {
    const g = data("epic-export");
    const id = mapView.selected;
    const e = g && id ? (g.epics || []).find((x) => x.id === id) : null;
    // A milestone a push dropped closes for good, so a later push that
    // brings it back does not reopen the drawer and take focus.
    if (g) moveDrawer({ type: "present", milestones: (g.milestones || []).map((m) => m.milestone) });
    const msd = !e && msView.detail && g ? (g.milestones || []).find((m) => m.milestone === msView.detail) : null;
    const open = (!!e || !!msd) && state.tab === "epics";
    document.querySelectorAll("#msstate [data-ms]").forEach((b) => b.setAttribute("aria-expanded", String(open && tpPure.cardExpanded(drawerState(), b.dataset.ms))));
    $("drawer").hidden = !open;
    document.body.classList.toggle("dopen", open);
    // Opening takes focus to the close button, unless the graph's search
    // opened it; closing gives it back.
    const keepFocus = keepFocusOnce;
    if (open && !drawerReturn) {
      const from = opener || { el: document.activeElement, key: focusKey(document.activeElement) };
      drawerReturn = { el: from.el, keys: [from.key].filter(Boolean) };
      opener = null;
      if (!keepFocus) $("dclose").focus({ preventScroll: true });
    }
    if (!open && drawerReturn) returnFocus();
    if (!open) return;
    $("dkind").textContent = msd ? t("drawer.milestone") : t("drawer.epic");
    $("drawer").setAttribute("aria-label", msd ? t("drawer.msLabel") : t("drawer.epicLabel"));
    $("dclose").setAttribute("aria-label", msd ? t("drawer.msClose") : t("drawer.epicClose"));
    $("dinfo").setAttribute("aria-label", msd ? t("drawer.msAbout") : t("drawer.epicAbout"));
    if (msd) {
      const dc = tpPure.describedCounts((g.graph && g.graph.epics) || [])[msd.milestone] || { described: 0, all: 0 };
      $("dbody").innerHTML = tpPure.milestoneHTML(msd, gateOf(g, msd.milestone), dc, words(THRESHOLD_LABEL), t);
      return;
    }
    const ge = graphEpic(g, id) || {};
    const ms = (g.milestones || []).find((m) => m.milestone === e.milestone) || {};
    const crit = ((g.critical_path && g.critical_path.by_count) || []).includes(id);
    const down = e.downstream;
    const deps = (ge.depends_on || []).map((d) => {
      const members = d.any_of || [d.id];
      return "<li>" + (d.any_of ? '<span class="muted">' + esc(t("wait.oneOf")) + "</span> " : "") + members.map((x) => {
        const xe = (g.epics || []).find((y) => y.id === x);
        return epicLink(x) + (xe ? ' <span class="esub">(' + esc(word(STAGE_LABEL, xe.status).toLocaleLowerCase(t.locale)) + ")</span>" : "");
      }).join(", ") + whyHTML(d.why) + "</li>";
    }).join("");
    const needs = [...new Set(mapView.dependents[id] || [])].map((y) => {
      const d = (mapView.deps[y] || []).find((x) => x.id === id) || {};
      return "<li>" + epicLink(y) + groupHTML(d.group) + whyHTML(d.why) + "</li>";
    }).join("");
    const waits = (e.blocked_by || []).concat((e.blocked_by_any_of || []).map((x) => t("chain.group", { ids: x.join(", ") })));
    const msLine = e.milestone + ", " + (ms.open === false ? t("ms.closed") : t("ms.open"));
    const fields = [
      [t("drawer.f.milestone"), ms.state ? t("drawer.msGate", { ms: msLine, done: ms.state.members_done, n: ms.state.members }) : msLine],
      [t("drawer.f.status"), word(STAGE_LABEL, e.status) + (waits.length ? ", " + t("ms.waitsOn", { on: waits.join(", ") }) : "")],
      [t("drawer.f.downstream"), down ? t("drawer.downstream", { n: down.dependents, open: down.dependents_open, chain: down.longest_chain }) : t("drawer.noDownstream")],
      ge.estimate_minutes ? [t("drawer.f.estimate"), t.dur(ge.estimate_minutes)] : null,
      (ge.tags || []).length ? [t("drawer.f.tags"), ge.tags.join(", ")] : null,
      ge.acceptance ? [t("drawer.f.acceptance"), ge.acceptance] : null,
      ge.done ? [t("drawer.f.done"), stamp(ge.done.at) + (ge.done.evidence ? ": " + ge.done.evidence : "")] : null,
    ].filter(Boolean);
    $("dbody").innerHTML = '<div><div class="dmeta"><code>' + esc(id) + "</code><span>" + esc(e.condition ? t("drawer.condition") : t("drawer.workEpic")) + "</span>" + (crit ? '<span class="critpill">' + esc(t("wait.critical")) + "</span>" : "") + "</div><h3>" + esc(e.title) + "</h3></div>" +
      (e.condition ? '<p class="impact">' + t.html("drawer.conditionNote_html", { id }) + "</p>" : epicStepper(e, ge.claimed !== undefined ? ge : null)) +
      "<dl>" + fields.map(([k, v]) => "<dt>" + esc(k) + "</dt><dd>" + esc(v) + "</dd>").join("") + "</dl>" +
      (String(ge.description || "").trim() ? descHTML(ge.description) : "") +
      (e.condition ? "" : "<div><h4>" + esc(t("drawer.specs")) + "</h4>" + ((e.files || []).length ? '<ul class="elist">' + e.files.map(fileLine).join("") + "</ul>" : none(t("drawer.noSpecs", { id }))) + "</div>") +
      "<div><h4>" + esc(t("drawer.dependsOn")) + "</h4>" + (deps ? '<ul class="elist">' + deps + "</ul>" : none(t("drawer.nothing"))) + "</div>" +
      "<div><h4>" + esc(t("drawer.dependedOnBy")) + "</h4>" + (needs ? '<ul class="elist">' + needs + "</ul>" : none(t("drawer.nothing"))) + "</div>" +
      "<div><h4>" + esc(t("drawer.history")) + " " + info("ehistory", t("drawer.history")) + "</h4>" + epicHistoryHTML(id) + "</div>";
  }
  INFO.ehistory = tk("info.ehistory");

  // drawSelected draws what follows the selection: the chain, the drawer and
  // its history, fetched again on every push while the drawer is open.
  function drawSelected() {
    const g = data("epic-export");
    if (g) mapEdges(g);
    if (mapView.selected && g && !graphEpic(g, mapView.selected)) mapView.selected = null;
    drawChain(state.graph && g ? g : null);
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
  const PAUSE = '<svg class="ic" viewBox="0 0 16 16" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M6 4v8M10 4v8"/></svg>';
  const waitsOp = (f) => !!(f.escalations && f.escalations.length);
  INFO.specs = tk("info.specs");

  // specMatches is the filter: a case-insensitive substring of the title, the
  // path or the epic.
  function specMatches(f, q) {
    return !q || [f.title, f.path, f.epic].some((s) => s && s.toLowerCase().includes(q));
  }

  function loopCell(label, l) {
    const v = l && l.state && l.state !== "none"
      ? '<span class="st" data-s="' + esc(l.state) + '">' + esc(t("slice.round", { n: l.round }) + ", " + word(LOOP_LABEL, l.state)) + "</span>"
      : '<span class="muted">' + esc(t("loop.state.none")) + "</span>";
    return '<span class="lc"><span>' + esc(label) + "</span>" + v + "</span>";
  }

  function specRow(f) {
    const ops = f.escalations || [];
    const counts = f.tasks || {};
    const total = (counts.open || 0) + (counts.wip || 0) + (counts.done || 0);
    const tasks = total
      ? '<span class="pm"><span class="meter" aria-hidden="true"><i style="width:' + Math.round((counts.done || 0) / total * 100) + '%"></i></span><span class="num">' +
        esc(t.num(counts.done || 0)) + "/" + esc(t.num(total)) + "</span></span>" + (counts.wip ? '<span class="sm muted">' + esc(t("specs.wip", { n: counts.wip })) + "</span>" : "")
      : '<span class="muted">' + esc(t("specs.noTasks")) + "</span>";
    const epic = f.epic
      ? '<code>' + esc(f.epic) + "</code>" + (f.epic_state ? '<span class="sm muted">' + esc(word(STAGE_LABEL, f.epic_state)) + "</span>" : "")
      : '<span class="muted">' + esc(t("specs.noEpic")) + "</span>";
    const next = ops.map((b) => '<span class="stat" title="' + esc(b.message) + '">' + PAUSE + esc(b.code) + "</span>").join("") +
      (f.error ? '<span class="snext err">' + esc(f.error) + "</span>" : '<code class="snext">' + esc(f.next_step || "") + "</code>");
    return '<tr class="srow' + (f.path === state.spec ? " on" : "") + '" tabindex="0" data-path="' + esc(f.path) + '">' +
      '<td class="sname"><span class="pt">' + (ops.length ? '<span class="mk" title="' + esc(t("specs.waitsOp")) + '">' + PAUSE + "</span>" : "") +
      "<span>" + esc(f.title || f.path) + "</span></span><code>" + esc(f.path) + "</code></td>" +
      '<td class="sepic">' + epic + "</td><td>" + tasks + '</td><td class="sloop">' + loopCell(t("phase.review"), f.review) + loopCell(t("phase.audit"), f.audit) +
      '</td><td class="snextc">' + next + "</td></tr>";
  }

  function drawSpecs() {
    const list = data("export-list");
    const files = (list && list.files) || [];
    const groups = PHASES.concat([["", tk("specs.unreadable")]]);
    const chip = (v, label, n) => '<button type="button" class="chip" data-phase="' + v + '" aria-pressed="' + (specsView.phase === v) + '">' +
      esc(label) + ' <span class="num muted">' + esc(t.num(n)) + "</span></button>";
    $("phasechips").innerHTML = chip("all", t("specs.all"), files.length) + chip("op", t("now.operator"), files.filter(waitsOp).length) +
      PHASES.map(([p, label]) => chip(p, t(label), files.filter((f) => f.phase === p).length)).join("");
    const q = specsView.q.trim().toLowerCase();
    const ph = specsView.phase;
    const focused = document.activeElement && document.activeElement.closest && document.activeElement.closest("#specs-body .srow");
    let n = 0;
    const body = groups.map(([p, label]) => {
      if (ph !== "all" && ph !== "op" && ph !== p) return "";
      const rows = files.filter((f) => (f.phase || "") === p && (ph !== "op" || waitsOp(f)) && specMatches(f, q));
      n += rows.length;
      return rows.length ? '<tbody><tr class="sgroup"><th colspan="5" scope="rowgroup">' + esc(t(label)) + ' <span class="num muted">' + esc(t.num(rows.length)) + "</span></th></tr>" +
        rows.map(specRow).join("") + "</tbody>" : "";
    }).join("");
    const col = (key) => '<th scope="col">' + esc(t(key)) + "</th>";
    $("specs-body").innerHTML = n
      ? '<table class="stable"><thead><tr>' + col(tk("specs.col.spec")) + col(tk("specs.col.epic")) + col(tk("specs.col.tasks")) + col(tk("specs.col.loops")) + col(tk("specs.col.next")) + "</tr></thead>" + body + "</table>"
      : none(list ? (q ? t("specs.noMatchQ", { q: specsView.q.trim() }) : t("specs.noMatch")) : "");
    $("specs-n").textContent = list ? t.num(n) : "";
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
  const VIEWS = [["read", tk("spec.view.read")], ["tasks", tk("spec.view.tasks")], ["history", tk("spec.view.history")]];
  const STEPS = ["ground", "review", "decompose", "implement", "audit", "release"];
  const STEP_LABEL = { ground: tk("step.ground"), review: tk("phase.review"), decompose: tk("phase.decompose"), implement: tk("phase.implement"), audit: tk("phase.audit"), release: tk("phase.release") };
  const CHECK = '<svg class="ic" viewBox="0 0 16 16" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M3.5 8.5l3 3 6-7"/></svg>';
  const listRow = (path) => ((data("export-list") || {}).files || []).find((x) => x.path === path) || null;
  const escalations = (d) => ((d.resume && d.resume.blockers) || []).filter((b) => b.class === "escalate");
  INFO.loop = tk("info.loop");

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
      body = { error: t("err.fetch", { what: "tp export --file " + path, message: err.message }), hint: t("err.stillRunning") };
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
      // A carried blocking finding holds the loop open whatever the streak says.
      if (l.carried_blocking > 0) return t("step.carried", { n: l.carried_blocking });
      if (l.streak >= l.clean_rounds_needed) return t("step.converged", { n: r.round });
      if (past(step)) return t("step.ended", { n: r.round });
      const p = { n: r.round, cap: l.cap, streak: l.streak, need: l.clean_rounds_needed };
      return l.cap ? t("step.progressCap", p) : t("step.progress", p);
    };
    const g = last(L.ground);
    const r = last(L.review);
    const a = last(L.audit);
    const counts = (f && f.tasks) || { open: 0, wip: 0, done: 0 };
    const n = counts.open + counts.wip + counts.done;
    return {
      ground: g ? (g.fails ? t("step.groundFails", { round: g.round, n: g.fails }) : t("step.groundOk", { n: g.round })) : t("step.notRun"),
      review: r ? loopLine(L.review, r, "review") : phase === "review" ? t("step.firstNext") : t("step.noRound"),
      decompose: n ? t("step.written", { n }) : phase === "decompose" ? t("step.writing") : t("step.afterReview"),
      implement: n ? t("spec.tasksDone", { done: counts.done, n }) : t("step.afterDecompose"),
      audit: a ? loopLine(L.audit, a, "audit") : phase === "audit" ? t("step.firstNext") : t("step.afterImplement"),
      release: phase === "release" ? t("step.released") : t("step.afterAudit"),
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
        '</span><span class="sl">' + esc(word(STEP_LABEL, p)) + '</span><span class="sd">' + esc(lines[p]) + "</span></li>";
    }).join("") + "</ol>";
  }

  // impactText says what releasing the spec does to the plan, from the
  // summary's release_impact, as markup: its ids in code, every value
  // escaped by t.html, and the parts set into the sentence's {parts} slot.
  function impactText(d) {
    if (d.resume.phase === "release") return esc(t("impact.released"));
    if (!d.epic) return esc(t("impact.noEpic"));
    const sum = data("summary");
    const imp = sum && sum.release_impact && sum.release_impact[d.path];
    if (!imp) return "";
    const parts = [];
    if (imp.frees.length) parts.push(t.html("impact.frees_html", { ids: imp.frees.join(", ") }));
    imp.gates.forEach((gt) => parts.push(t.html("impact.gate_html", { ms: gt.milestone, from: gt.members_done, to: gt.members_done_after, n: gt.members })));
    if (!parts.length) return esc(t("impact.none"));
    return t.html("impact.completes_html", { epic: d.epic }).replace("{parts}", parts.join(" " + esc(t("impact.and")) + " "));
  }

  function specHead(d, f) {
    const na = d.resume.next_action || {};
    const cmd = na.command || na.brief_command;
    const epic = d.epic
      ? '<span class="sh-epic">' + t.html("spec.epic_html", { epic: d.epic }) + (f && f.epic_state ? " " + esc(word(STAGE_LABEL, f.epic_state)) : "") + "</span>"
      : '<span class="sh-epic">' + esc(t("spec.noEpic")) + "</span>";
    return '<div class="sh-row"><h1>' + esc((f && f.title) || d.spec) + "</h1>" + epic + "</div>" +
      '<div class="loophead"><h2>' + esc(t("spec.loop")) + "</h2>" + info("loop", t("spec.loop")) + "</div>" + stepper(d, f) +
      '<p class="impact">' + impactText(d) + "</p>" +
      escalations(d).map((b) => '<div class="opnote"><span class="mk">' + PAUSE + "</span><div><b>" + esc(b.code) + "</b> " + esc(t("spec.waitsOp", { message: b.message })) + "</div></div>").join("") +
      '<div class="cmdline"><span class="lab">' + t.html("spec.next_html") + "</span>" +
      (cmd ? "<code>" + esc(cmd) + '</code><button type="button" class="key" data-copy="' + esc(cmd) + '">' + esc(t("copy.copy")) + "</button>" : "") +
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
    const banner = e ? '<div class="banner"><b>' + esc(t("spec.readFailed", { path: work.path })) + "</b><span>" + esc(e.error) + "</span>" +
      (e.hint ? '<span class="hint">' + esc(e.hint) + "</span>" : "") + (d ? '<span class="kept">' + esc(t("spec.kept")) + "</span>" : "") + "</div>" : "";
    const crumb = '<div class="crumb"><button type="button" class="link" id="crumb-specs">' + esc(t("tabs.specs")) + '</button><span aria-hidden="true">/</span><code>' + esc(work.path) + "</code></div>";
    $("spechead").innerHTML = crumb + banner + (d ? specHead(d, f) : e ? "" : none(t("spec.reading", { path: work.path })));
    const total = f && f.tasks ? f.tasks.open + f.tasks.wip + f.tasks.done : 0;
    const counts = {
      read: d ? t("count.sections", { n: (d.sections || []).filter((s) => s.anchor !== "§0").length }) : "",
      tasks: total ? t("spec.tasksDone", { done: f.tasks.done, n: total }) : t("specs.noTasks"),
      history: d ? t("count.rounds", { n: ["ground", "review", "audit"].reduce((s, k) => s + ((d.loops[k] && d.loops[k].rounds) || []).length, 0) }) : "",
    };
    $("subnav").innerHTML = d ? VIEWS.map(([v, label]) => '<button type="button" role="tab" data-view="' + v + '" aria-controls="sv-' + v + '" aria-selected="' + (work.view === v) + '">' +
      esc(t(label)) + '<span class="muted num">' + esc(counts[v]) + "</span></button>").join("") : "";
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
  INFO.reader = tk("info.reader");

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
        (n ? '<button type="button" class="tc" data-sec="' + esc(k) + '" aria-label="' + esc(t("read.showTasks", { n, anchor: s.anchor })) + '">' + esc(t("count.tasks", { n })) + "</button>" : "") + "</h2>" +
        (s.paragraphs || []).map((p) => blk(k, p)).join("") + "</section>";
    };
    $("sv-read").innerHTML = '<div class="doc"><div class="rmeta"><span>' + esc(reviews ? t("read.reviewed", { n: reviews }) : t("read.notReviewed")) + "</span><span>" +
      esc((d.tasks || []).length ? t("read.tasksNamed") : t("read.tasksLater")) + "</span>" + info("reader", t("read.about")) + "</div>" +
      "<h1>" + esc((pre && pre.heading) || d.spec) + "</h1>" + (pre && pre.paragraphs ? pre.paragraphs.map((p) => blk("0", p)).join("") : "") +
      (numbered.length ? '<nav class="toc" aria-label="' + esc(t("read.sections")) + '">' + numbered.map((s) => '<button type="button" data-goto="' + esc(secKey(s)) + '">' + esc(s.anchor + " " + secTitle(s)) + "</button>").join("") + "</nav>" : "") +
      (numbered.map(section).join("") || none(t("read.noSection"))) + "</div>";
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
  const LOOPS = [["ground", tk("step.ground")], ["review", tk("phase.review")], ["audit", tk("phase.audit")]];
  INFO.spechistory = tk("info.spechistory");
  // roundCount words a round's findings, or a ground round's FAILs.
  const roundCount = (r) => (r.fails !== undefined ? t("count.fails", { n: r.fails }) : t("count.findings", { n: r.findings || 0 }));
  const recorded = (iso) => iso.replace("T", " ").replace("Z", " UTC");

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
      return '<text x="0" y="' + (y + 4) + '">' + esc(t(label)) + '</text><line class="ax" x1="90" x2="580" y1="' + y + '" y2="' + y + '"/>' +
        ((L[k] && L[k].rounds) || []).filter((r) => !isNaN(Date.parse(r.recorded_at))).map((r) => {
          const title = t("spechist.dot", { loop: t(label), n: r.round, at: recorded(r.recorded_at), count: roundCount(r) }) + (r.clean ? ", " + t("hist.clean") : "");
          return '<circle class="' + (r.clean ? "clean" : "dirty") + '" cx="' + x(r.recorded_at) + '" cy="' + y + '" r="5"><title>' + esc(title) + "</title></circle>";
        }).join("");
    }).join("");
    return '<svg class="raxis" viewBox="0 0 600 ' + (LOOPS.length * 26 + 22) + '" role="img" aria-label="' + esc(t("spechist.axis")) + '">' + lanes +
      '<text x="90" y="' + (LOOPS.length * 26 + 18) + '">' + day(lo) + '</text><text x="580" y="' + (LOOPS.length * 26 + 18) + '" text-anchor="end">' + day(lo + span) + "</text></svg>";
  }

  // roundTable lists each loop's rounds under a line naming its cap and its
  // current run of clean rounds against the run it needs.
  function roundTable(L) {
    const body = LOOPS.map(([k, label]) => {
      const l = L[k];
      if (!l || !(l.rounds || []).length) return "";
      const cap = l.cap ? t("slice.cap", { n: l.cap }) : t("spechist.uncapped");
      const head = '<tr class="lgroup"><th colspan="4">' + esc(t(label)) + ' <span class="muted">' + esc(t("spechist.streak", { cap, streak: l.streak, need: l.clean_rounds_needed })) + "</span></th></tr>";
      return head + l.rounds.map((r) => "<tr><td>" + esc(t("hist.round", { n: r.round })) + "</td><td>" + esc(recorded(r.recorded_at)) + '</td><td class="n">' +
        esc(roundCount(r)) + "</td><td>" + (r.clean ? CHECK + " " + esc(t("hist.clean")) : '<span class="muted">' + esc(t("spechist.notClean")) + "</span>") + "</td></tr>").join("");
    }).join("");
    const col = (key) => "<th>" + esc(t(key)) + "</th>";
    return '<div class="scroll"><table class="loops"><thead><tr>' + col(tk("hist.col.round")) + col(tk("spechist.col.recorded")) + col(tk("spechist.col.findings")) + col(tk("spechist.col.clean")) + "</tr></thead><tbody>" + body + "</tbody></table></div>";
  }

  // SECTION_STATUS words a section's status in a snapshot difference.
  const SECTION_STATUS = { added: tk("secstat.added"), changed: tk("secstat.changed"), removed: tk("secstat.removed"), unchanged: tk("secstat.unchanged") };

  // snapshotDiff draws the sections a round's snapshot changed, added or
  // removed against the round before's.
  function snapshotDiff(d) {
    if (!d) return none(t("spechist.firstRound"));
    const changed = (d.changed || []).map((s) => '<div class="dl ins"><code>' + esc(s.heading) + '</code> <span class="muted sm">' + esc(word(SECTION_STATUS, s.status)) + "</span>" +
      (s.content && s.content.trim() ? '<div class="now sectext">' + esc(s.content.trim()) + "</div>" : "") + "</div>");
    const removed = (d.removed || []).map((s) => '<div class="dl del"><code>' + esc(s.heading) + "</code></div>");
    return (changed.length ? '<div class="dpart"><h4>' + esc(t("spechist.changed")) + "</h4>" + changed.join("") + "</div>" : "") +
      (removed.length ? '<div class="dpart"><h4>' + esc(t("spechist.removed")) + "</h4>" + removed.join("") + "</div>" : "") || none(t("spechist.noChange"));
  }

  function drawSpecHistory(d) {
    const L = d.loops || {};
    const review = ((L.review && L.review.rounds) || []).slice().reverse();
    const unattributed = t("spechist.unattributed");
    const pairs = review.map((r) => {
      const byRole = {};
      (r.finding_rows || []).forEach((f) => { (byRole[f.role || unattributed] = byRole[f.role || unattributed] || []).push({ severity: f.severity, location: f.location, finding: f.finding, resolved: f.disposition ? { status: f.disposition } : null }); });
      return '<div class="rpair"><h3 class="hsub">' + esc(t("spechist.reviewRound", { n: r.round })) + ' <span class="muted sm">' + esc(t("count.findings", { n: r.findings || 0 }) + (r.clean ? ", " + t("hist.clean") : "")) + "</span></h3>" +
        '<div class="hcmp"><div><h4 class="hsub">' + esc(r.round > 1 ? t("spechist.changedSince", { n: r.round - 1 }) : t("spechist.whatChanged")) + "</h4>" + snapshotDiff(r.snapshot_diff) + "</div>" +
        '<div><h4 class="hsub">' + esc(t("hist.roundFindings", { n: r.round })) + "</h4>" + findingsList(byRole) + "</div></div></div>";
    }).join("");
    const any = LOOPS.some(([k]) => L[k] && (L[k].rounds || []).length);
    $("sv-history").innerHTML = '<div class="hhead"><h2>' + esc(t("spec.view.history")) + "</h2>" + info("spechistory", t("spechist.about")) + "</div>" +
      (any ? roundAxis(L) + roundTable(L) + pairs : none(t("spechist.none")));
  }

  // ---------- the Spec tab's Tasks view (§13) ----------
  // BOARD is the Board's columns: key, label and the task states tp export
  // reports that each one holds. Ready and waiting hold the open tasks, so the
  // columns count what tp status counts as open, wip and done.
  const BOARD = [["ready", tk("board.ready"), ["ready"]], ["wip", tk("board.wip"), ["wip"]], ["waiting", tk("board.waiting"), ["waits_on_task", "waits_on_epic"]], ["done", tk("board.done"), ["done"]]];
  const TASK_LABEL = { ready: tk("task.state.ready"), wip: tk("task.state.wip"), waits_on_task: tk("task.state.waits_on_task"), waits_on_epic: tk("task.state.waits_on_epic"), done: tk("task.state.done") };
  const TASK_COLOR = { ready: "ready", wip: "active", waits_on_task: "moot", waits_on_epic: "held", done: "done" };
  const WIP_MARK = '<svg class="ic" viewBox="0 0 16 16" aria-hidden="true"><circle cx="8" cy="8" r="5.5" fill="none" stroke="currentColor" stroke-width="1.6"/><path d="M8 2.5a5.5 5.5 0 0 1 0 11z" fill="currentColor"/></svg>';
  const TASK_MARK = { done: CHECK, wip: WIP_MARK, waits_on_epic: PAUSE, waits_on_task: "", ready: "" };
  INFO.tasks = tk("info.tasks");
  INFO.progress = tk("info.progress");

  // ago says how long ago an RFC 3339 time was, as tpPure.relTime words it.
  const ago = (iso) => tpPure.relTime(Date.now() - Date.parse(iso), t);

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

  function taskMeta(task, ix) {
    if (task.state === "done") return task.closed_at ? t("task.closedAgo", { ago: ago(task.closed_at) }) : t("task.state.done");
    if (task.state === "wip") return task.started_at ? t("task.startedAgo", { ago: ago(task.started_at) }) : t("task.state.wip");
    if (task.state === "waits_on_task" || task.state === "waits_on_epic") return t("task.after", { ids: (task.waits_on || []).join(", ") });
    const n = (ix.dependents[task.id] || []).length;
    return n ? t("task.unblocks", { n }) : t("task.unblocksNothing");
  }

  function taskCard(task, ix) {
    const dim = work.secIds && !work.secIds.has(task.id);
    return '<button type="button" class="tcard' + (dim ? " dim" : "") + '" data-s="' + esc(task.state) + '" data-t="' + esc(task.id) + '" aria-pressed="' + (task.id === work.task) + '">' +
      '<span class="tt">' + (TASK_MARK[task.state] || "") + "<span>" + esc(task.title) + '</span></span><code class="tid">' + esc(task.id) +
      '</code><span class="tm">' + esc(taskMeta(task, ix)) + "</span></button>";
  }

  function stepsHTML(d, ix) {
    return '<svg class="wires" id="wires" aria-hidden="true"></svg>' + stepsOf(d, ix).map((g, i) => '<div class="step"><div class="step-h"><b>' + esc(t("tasks.step", { n: i + 1 })) +
      "</b><span>" + esc(i ? t("tasks.afterStep", { n: i }) : t("tasks.noneBefore")) + '</span></div><div class="cards">' + g.map((task) => taskCard(task, ix)).join("") + "</div></div>").join("");
  }

  function boardHTML(d, ix) {
    return BOARD.map(([key, label, states]) => {
      const ts = (d.tasks || []).filter((task) => states.includes(task.state));
      return '<section class="bcol" data-col="' + key + '" aria-label="' + esc(t(label)) + '"><div class="step-h"><b>' + esc(t(label)) + '</b><span class="num">' + esc(t.num(ts.length)) + "</span></div>" +
        '<div class="bcards">' + (ts.map((task) => taskCard(task, ix)).join("") || none(t("tasks.noneInColumn"))) + "</div></section>";
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
    (d.tasks || []).forEach((task) => (task.depends_on || []).forEach((x) => {
      const a = R[x];
      const b = R[task.id];
      if (!a || !b) return;
      const on = work.task && (work.task === x || work.task === task.id);
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
    const closed = tasks.filter((task) => task.state === "done" && task.closed_at).map((task) => Date.parse(task.closed_at)).sort((a, b) => a - b);
    if (!closed.length) return none(t("progress.noneClosed"));
    const day = 86400000;
    const first = Math.floor(closed[0] / day);
    const days = Math.max(1, Math.floor(Date.now() / day) - first);
    const W = 320;
    const H = 72;
    const pts = [];
    for (let i = 0; i <= days; i++) pts.push(closed.filter((c) => Math.floor(c / day) <= first + i).length);
    const xy = pts.map((v, i) => (i / days * W).toFixed(1) + "," + (H - 4 - v / tasks.length * (H - 8)).toFixed(1)).join(" ");
    const date = (ms) => new Date(ms).toISOString().slice(0, 10);
    return '<svg class="burn" viewBox="0 0 ' + W + " " + H + '" preserveAspectRatio="none" role="img" aria-label="' + esc(t("progress.burnAria", { done: closed.length, n: tasks.length, since: date(closed[0]) })) + '">' +
      '<line x1="0" x2="' + W + '" y1="4" y2="4" class="goal"/><polyline points="' + xy + '" class="line"/></svg>' +
      '<div class="burnx"><span>' + esc(date(closed[0])) + "</span><span>" + esc(t("progress.goal", { n: tasks.length })) + "</span><span>" + esc(t("progress.today")) + "</span></div>";
  }

  // running says how long a task in progress has run against its estimate.
  function running(task) {
    if (!task.started_at) return esc(t("task.state.wip"));
    const ran = Math.max(0, Math.round((Date.now() - Date.parse(task.started_at)) / 60000));
    const over = task.estimate_minutes && ran > task.estimate_minutes;
    const line = task.estimate_minutes ? t("task.running", { ran: t.dur(ran), est: t.dur(task.estimate_minutes) }) : t("task.runningNoEst", { ran: t.dur(ran) });
    return '<span class="' + (over ? "over" : "") + '">' + esc(line) + "</span>";
  }

  function progressHTML(d) {
    const tasks = d.tasks || [];
    if (!tasks.length) {
      return '<div class="pv"><h3>' + esc(t("progress.noTasks")) + "</h3>" + none(d.resume.phase === "decompose" ? t("progress.decomposing") : t("progress.inReview")) + "</div>";
    }
    const count = (states) => tasks.filter((task) => states.includes(task.state)).length;
    const seg = BOARD.map(([key, label, states]) => [key, t(label).toLocaleLowerCase(t.locale), count(states)]);
    const color = { ready: "ready", wip: "active", waiting: "moot", done: "done" };
    const order = ["done", "wip", "ready", "waiting"].map((k) => seg.find((s) => s[0] === k));
    const left = tasks.filter((task) => task.state !== "done");
    const est = left.reduce((s, task) => s + (task.estimate_minutes || 0), 0);
    const noEst = left.filter((task) => !task.estimate_minutes).length;
    const wip = tasks.filter((task) => task.state === "wip").map((task) => '<li><button class="link" type="button" data-t="' + esc(task.id) + '">' + esc(task.title) + "</button><small>" + running(task) + "</small></li>").join("");
    const leftLine = left.length ? t("progress.left", { est: t.dur(est) }) + (noEst ? ", " + t("progress.noEstimate", { n: noEst }) : "") + "." : t("progress.allDone");
    return '<div class="pv"><div class="pv-h"><h3>' + t.html("progress.head_html", { done: count(["done"]), n: tasks.length }) + "</h3>" + info("progress", t("progress.about")) + "</div>" +
      '<div class="pbar" role="img" aria-label="' + esc(order.map((s) => t.num(s[2]) + " " + s[1]).join(", ")) + '">' + order.map((s) => (s[2] ? '<i style="flex:' + s[2] + ";background:var(--" + color[s[0]] + ')"></i>' : "")).join("") + "</div>" +
      '<div class="pkey">' + order.map((s) => '<span><i style="background:var(--' + color[s[0]] + ')"></i><b class="num">' + esc(t.num(s[2])) + "</b>&nbsp;" + esc(s[1]) + "</span>").join("") + "</div>" +
      '<p class="note">' + esc(leftLine) + "</p>" +
      (wip ? "<div><h4>" + esc(t("progress.inProgress")) + '</h4><ul class="wiplist">' + wip + "</ul></div>" : "") +
      "<div><h4>" + esc(t("progress.closedOverTime")) + "</h4>" + closedOverTime(tasks) + "</div>" + '<p class="note">' + esc(t("progress.clickTask")) + "</p></div>";
  }

  // sectionOf finds the export's section for one of a task's source_sections.
  const sectionOf = (d, src) => (d.sections || []).find((s) => s.heading === String(src).replace(/^#+\s*/, "")) || null;

  function taskDetail(d, task, ix) {
    const link = (id) => '<button class="link" type="button" data-t="' + esc(id) + '">' + esc(id) + "</button>";
    const deps = (task.depends_on || []).map((x) => {
      const u = ix.by[x];
      const ok = !u || u.state === "done";
      return '<li><span class="mk" style="color:var(--' + (ok ? "done" : "blocked") + ')">' + (ok ? "✓" : "○") + "</span><span>" + (u ? link(x) + "<small>" + esc(u.title) + "</small>" : "<code>" + esc(x) + "</code>") + "</span></li>";
    }).join("") || '<li class="empty">' + esc(t("task.noDeps")) + "</li>";
    const needs = (ix.dependents[task.id] || []).map((x) => '<li><span class="mk">→</span><span>' + link(x) + "<small>" + esc(word(TASK_LABEL, ix.by[x].state)) + "</small></span></li>").join("") ||
      '<li class="empty">' + esc(t("task.noDependents")) + "</li>";
    const shas = task.commit_shas || (task.commit_sha ? [task.commit_sha] : []);
    const dt = (key) => "<dt>" + esc(t(key)) + "</dt>";
    let time;
    if (task.state === "done") {
      time = dt(tk("task.f.closed")) + "<dd>" + esc(task.closed_at ? ago(task.closed_at) : t("task.state.done")) + (shas.length ? ", " + esc(t("task.commits", { n: shas.length })) + " " + shas.map((s) => "<code>" + esc(s) + "</code>").join(" ") : "") + "</dd>" +
        dt(tk("task.f.evidence")) + '<dd class="ev">' + esc(task.closed_reason || t("task.noEvidence")) + "</dd>";
    } else if (task.state === "wip") {
      time = dt(tk("task.f.progress")) + "<dd>" + running(task) + "</dd>";
    } else {
      time = dt(tk("task.f.estimate")) + "<dd>" + esc(task.estimate_minutes ? t.dur(task.estimate_minutes) : t("task.noEstimate")) + "</dd>" + (task.waits_on && task.waits_on.length ? dt(tk("task.f.waitsOn")) + "<dd>" + esc(task.waits_on.join(", ")) + "</dd>" : "");
    }
    const cmd = task.state === "ready" ? "tp claim " + task.id : task.state === "wip" ? "tp done " + task.id + ' "<' + t("task.evidenceSlot") + '>"' : "";
    const acc = String(task.acceptance || "").split("\n").map((l) => l.replace(/^- /, "").trim()).filter(Boolean);
    const src = (task.source_sections || []).map((s) => {
      const sec = sectionOf(d, s);
      const p = sec && sec.paragraphs && sec.paragraphs[0];
      return '<figure class="excerpt"><figcaption>' + (sec ? '<button type="button" class="link" data-goto="' + esc(secKey(sec)) + '">' + esc(sec.anchor + " " + secTitle(sec)) + "</button>" : esc(String(s).replace(/^#+\s*/, ""))) + "</figcaption>" +
        (p ? "<blockquote>" + esc(p.text) + "</blockquote>" : "") + "</figure>";
    }).join("");
    return '<div class="td-head"><div><span class="stat" style="color:var(--' + (TASK_COLOR[task.state] || "muted") + ')">' + (TASK_MARK[task.state] || "") + esc(word(TASK_LABEL, task.state)) + "</span>" +
      "<h3>" + esc(task.title) + '</h3><code class="tid">' + esc(task.id) + '</code></div><button type="button" class="key" data-tclose aria-label="' + esc(t("task.close")) + '">×</button></div>' +
      "<dl>" + time + "</dl>" +
      (cmd ? '<div class="cmdline"><code>' + esc(cmd) + '</code><button type="button" class="key" data-copy="' + esc(cmd) + '">' + esc(t("copy.copy")) + "</button></div>" : "") +
      (task.state === "waits_on_epic" ? '<p class="opnote">' + t.html("task.epicWaits_html", { epic: d.epic || "" }) + "</p>" : "") +
      "<div><h4>" + esc(t("task.acceptance")) + '</h4><ul class="deps">' + acc.map((a) => '<li><span class="mk">–</span><span>' + esc(a) + "</span></li>").join("") + "</ul></div>" +
      '<div class="two-col"><div><h4>' + esc(t("task.dependsOn")) + '</h4><ul class="deps">' + deps + "</ul></div><div><h4>" + esc(t("task.neededBy")) + '</h4><ul class="deps">' + needs + "</ul></div></div>" +
      (src ? "<div><h4>" + esc(t("task.fromSpec")) + "</h4>" + src + "</div>" : "");
  }

  function drawTasks(d) {
    const ix = taskIndex(d);
    if (work.task && !ix.by[work.task]) work.task = null;
    const focused = document.activeElement && document.activeElement.closest && document.activeElement.closest("#sv-tasks [data-t]");
    const tasks = d.tasks || [];
    const tview = (v, key) => '<button type="button" data-tview="' + v + '" aria-pressed="' + (work.tview === v) + '">' + esc(t(key)) + "</button>";
    const sec = work.sec !== null ? (d.sections || []).find((s) => secKey(s) === work.sec) : null;
    if (!sec) work.sec = null;
    work.secIds = sec ? new Set(tasksBySection(d)[work.sec] || []) : null;
    const chip = sec ? '<span class="secchip">' + esc(t("tasks.fromSection", { n: work.secIds.size })) + ' <button type="button" class="link" data-goto="' + esc(work.sec) + '">' + esc(sec.anchor + " " + secTitle(sec)) + "</button>" +
      '<button type="button" class="key" data-secclear aria-label="' + esc(t("tasks.showAll")) + '">×</button></span>' : "";
    $("sv-tasks").innerHTML = '<section class="panel tpanel"><div class="toolbar"><h2>' + esc(t("spec.view.tasks")) + " " + info("tasks", t("spec.view.tasks")) + "</h2>" + chip + '<span class="spacer"></span>' +
      '<div class="seg" role="group" aria-label="' + esc(t("tasks.view")) + '">' + tview("steps", tk("tasks.steps")) + tview("board", tk("tasks.board")) + "</div></div>" +
      '<div class="stages' + (work.tview === "board" ? " board" : "") + '" id="stages">' + (tasks.length ? (work.tview === "board" ? boardHTML(d, ix) : stepsHTML(d, ix)) : none(t("progress.noTasks"))) + "</div>" +
      '<div class="legend">' + ["ready", "wip", "waits_on_task", "waits_on_epic", "done"].map((s) => '<span><i class="lg" data-s="' + s + '"></i>' + esc(word(TASK_LABEL, s)) + "</span>").join("") + "</div></section>" +
      '<aside class="panel tside" id="tside" aria-label="' + esc(t("tasks.side")) + '">' + (work.task ? taskDetail(d, ix.by[work.task], ix) : progressHTML(d)) + "</aside>";
    if (work.tview === "steps") drawWires(d);
    if (focused) {
      const again = $("sv-tasks").querySelector('.tcard[data-t="' + CSS.escape(focused.dataset.t) + '"]');
      if (again) again.focus();
    }
  }

  // copyKey copies a command and says so on the key for a moment.
  function copyKey(b) {
    const done = (text) => { b.textContent = text; setTimeout(() => { b.textContent = t("copy.copy"); }, 1200); };
    try {
      navigator.clipboard.writeText(b.dataset.copy).then(() => done(t("copy.copied")), () => done(t("copy.failed")));
    } catch (err) {
      done(t("copy.failed"));
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
  // tab, the spec, its view, the picked task, the selected epic, the
  // milestone the drawer shows (ms), the slice whose graph history is shown
  // and its two compared rounds, the lifecycle's stage filter, the graph
  // card's mode (gv), gate milestone (gm), depth (gd), cross-edge toggle
  // (cross) and search (q), which graph.js writes through
  // tpApp.graphParams (tpPure.graphParams), the Specs tab's phase and
  // filter, and the Tasks view's Steps or Board and the section whose tasks
  // it picks out, sec, a section anchor without its sign. A change of place
  // (tab, spec, view, task, epic, slice, rounds, the graph's mode and gate
  // milestone) adds a history entry; a filter replaces the current one.
  const URL_KEYS = ["tab", "spec", "view", "task", "epic", "q", "phase", "sq", "tview", "slice", "hr", "stage", "sec", "gv", "gm", "gd", "ms", "cross"];
  const PLACE_KEYS = ["tab", "spec", "view", "task", "epic", "ms", "slice", "hr", "gv", "gm"];
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
    put("ms", msView.detail);
    put("stage", mapView.stage);
    if (specsView.phase !== "all") put("phase", specsView.phase);
    put("sq", specsView.q);
    put("slice", hist.dir);
    if (hist.dir && hist.pair) put("hr", hist.pair.join("-"));
    if (window.tpApp && window.tpApp.graphParams) window.tpApp.graphParams().forEach(([k, v]) => put(k, v));
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
    const url = tpPure.addressURL(next.toString());
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
    msView.detail = p.get("ms") || null;
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
    if (window.tpApp && window.tpApp.readGraphPlace) window.tpApp.readGraphPlace(p);
    $("specs-q").value = specsView.q;
  }

  // tpApp is what graph.js, the graph card, reads from the page: the last
  // payloads, a panel hook, the selection, the marks, the search fold, the
  // lifecycle's stage filter and the address. onSelect, when graph.js sets
  // it, hears every change of selection and whether to centre it;
  // graphParams and readGraphPlace, when it sets them, write and read the
  // card's own address keys.
  window.tpApp = {
    data, panel, markOf, fold, selectEpic, syncURL,
    tr: () => t,
    selected: () => mapView.selected,
    inStage: (e) => !mapView.stage || e.status === mapView.stage,
    stage: () => mapView.stage,
    markText: (m) => tpPure.markText(m, words(MARK_LABEL)),
    markWord: (k) => word(MARK_LABEL, k),
    onSelect: null,
    graphNote: null,
    graphParams: null,
    readGraphPlace: null,
  };

  readURL();
  window.addEventListener("popstate", () => {
    address.applying = true;
    readURL();
    if (state.tab !== null) setTab(state.wantTab || (state.graph ? "epics" : "specs"));
    render();
    address.applying = false;
  });

  // The stream: a page that reconnects gets the current state again (§2).
  // It opens once the page's language is loaded, so the first draw is
  // already in it; a language file that cannot load leaves English, and
  // English failing too is said in English, the one text with no file. A
  // hidden tab closes it and a visible one opens a fresh one, which the
  // server answers with the current state first, so the page resyncs.
  const conn = $("conn");
  const connState = (s) => { conn.dataset.state = s; conn.textContent = t(CONN_TEXT[s]); };
  let source = null;
  let langReady = false;
  function openStream() {
    connState("connecting");
    source = new EventSource(STREAM);
    source.addEventListener("open", () => connState("live"));
    source.addEventListener("error", () => connState("lost"));
    EVENTS.forEach((name) => {
      source.addEventListener(name, (ev) => {
        let payload;
        try {
          payload = JSON.parse(ev.data);
        } catch (err) {
          payload = { error: t("err.notJSON", { name, message: err.message }), hint: t("err.notJSONHint") };
        }
        receive(name, payload);
      });
    });
  }
  document.addEventListener("visibilitychange", () => {
    if (!langReady) return;
    const step = tpPure.streamStep(document.hidden, source !== null);
    if (step === "close") {
      source.close();
      source = null;
      connState("paused");
    } else if (step === "open") {
      openStream();
    }
  });
  const lang = tpPure.pickLang(storedLang(), navigator.language);
  loadLang(lang).catch(() => loadLang("en")).catch(() => {
    $("banners").innerHTML = '<div class="banner"><b>The page\'s language file did not load</b><span>Reload the page; if it stays, check that tp dashboard is still running.</span></div>';
  }).then(() => {
    langReady = true;
    if (!document.hidden) openStream();
  });
})();
