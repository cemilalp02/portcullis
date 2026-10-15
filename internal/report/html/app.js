// Progressive enhancement only: the report is complete without JavaScript.
// No network access, no HTML injection: elements are shown or hidden, text is
// only ever set with textContent.
(function () {
  "use strict";
  var root = document.documentElement;
  var each = function (list, fn) { Array.prototype.forEach.call(list, fn); };

  // Theme: follows the system unless the reader picks one.
  try {
    var saved = localStorage.getItem("portcullis-theme");
    if (saved === "light" || saved === "dark") { root.setAttribute("data-theme", saved); }
  } catch (e) { /* storage unavailable */ }
  var theme = document.getElementById("theme");
  if (theme) {
    theme.hidden = false;
    theme.addEventListener("click", function () {
      var dark = root.getAttribute("data-theme") === "dark" ||
        (!root.getAttribute("data-theme") && window.matchMedia("(prefers-color-scheme: dark)").matches);
      var next = dark ? "light" : "dark";
      root.setAttribute("data-theme", next);
      try { localStorage.setItem("portcullis-theme", next); } catch (e) { /* storage unavailable */ }
    });
  }

  // Filters: severity chips and text search.
  var chips = document.querySelectorAll(".chip[data-sev]");
  var search = document.getElementById("search");
  var findings = document.querySelectorAll("details.finding");
  var files = document.querySelectorAll(".file");
  var noMatch = document.getElementById("no-match");
  var count = document.getElementById("findings-count");

  function apply() {
    var on = {};
    var filtered = false;
    each(chips, function (c) {
      var pressed = c.getAttribute("aria-pressed") === "true";
      on[c.getAttribute("data-sev")] = pressed;
      if (!pressed) { filtered = true; }
    });
    var q = search ? search.value.trim().toLowerCase() : "";
    if (q !== "") { filtered = true; }
    var shown = 0;
    each(findings, function (f) {
      var visible = on[f.getAttribute("data-sev")] !== false &&
        (q === "" || f.getAttribute("data-text").indexOf(q) !== -1);
      f.hidden = !visible;
      if (visible) { shown++; }
    });
    each(files, function (g) {
      g.hidden = g.querySelector("details.finding:not([hidden])") === null;
    });
    if (noMatch) { noMatch.hidden = shown !== 0 || findings.length === 0; }
    if (count) {
      var total = count.getAttribute("data-total");
      count.textContent = filtered ? shown + " of " + total : total;
    }
  }

  each(chips, function (c) {
    if (c.classList.contains("zero")) { return; }
    c.disabled = false;
    c.addEventListener("click", function () {
      c.setAttribute("aria-pressed", c.getAttribute("aria-pressed") === "true" ? "false" : "true");
      apply();
    });
  });
  if (search) {
    document.getElementById("search-box").hidden = false;
    search.addEventListener("input", apply);
    search.addEventListener("keydown", function (e) {
      if (e.key === "Escape") { search.value = ""; apply(); search.blur(); }
    });
    document.addEventListener("keydown", function (e) {
      var t = e.target;
      var typing = t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable);
      if (e.key === "/" && !typing && !e.ctrlKey && !e.metaKey && !e.altKey) {
        e.preventDefault();
        search.focus();
      }
    });
  }

  // Expand or collapse every finding.
  var toggle = document.getElementById("toggle-all");
  function syncToggle() {
    var closed = false;
    each(findings, function (f) { if (!f.open) { closed = true; } });
    toggle.textContent = closed ? "Expand all" : "Collapse all";
  }
  if (toggle && findings.length > 0) {
    toggle.hidden = false;
    syncToggle();
    toggle.addEventListener("click", function () {
      var open = toggle.textContent === "Expand all";
      each(findings, function (f) { f.open = open; });
      syncToggle();
    });
    each(findings, function (f) { f.addEventListener("toggle", syncToggle); });
  }

  // Copy a fingerprint, e.g. for a baseline entry.
  each(document.querySelectorAll("button.copy"), function (b) {
    if (!navigator.clipboard) { return; }
    b.hidden = false;
    b.addEventListener("click", function () {
      navigator.clipboard.writeText(b.getAttribute("data-copy")).then(function () {
        b.classList.add("done");
        b.setAttribute("title", "Copied");
        setTimeout(function () { b.classList.remove("done"); b.setAttribute("title", "Copy fingerprint"); }, 1400);
      });
    });
  });

  window.addEventListener("beforeprint", function () {
    each(document.querySelectorAll("details"), function (d) { d.open = true; });
  });
})();
