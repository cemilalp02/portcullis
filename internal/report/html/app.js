// Progressive enhancement only: the report is complete without JavaScript.
// No network access, no HTML injection: elements are shown or hidden, text is
// only ever read with textContent.
(function () {
  "use strict";
  var root = document.documentElement;

  var theme = document.getElementById("theme");
  function setTheme(t) {
    if (t) { root.setAttribute("data-theme", t); } else { root.removeAttribute("data-theme"); }
    try { if (t) { localStorage.setItem("portcullis-theme", t); } else { localStorage.removeItem("portcullis-theme"); } } catch (e) { /* storage unavailable */ }
  }
  try { var saved = localStorage.getItem("portcullis-theme"); if (saved === "light" || saved === "dark") { root.setAttribute("data-theme", saved); } } catch (e) { /* storage unavailable */ }
  if (theme) {
    theme.hidden = false;
    theme.addEventListener("click", function () {
      var dark = root.getAttribute("data-theme") === "dark" ||
        (!root.getAttribute("data-theme") && window.matchMedia("(prefers-color-scheme: dark)").matches);
      setTheme(dark ? "light" : "dark");
    });
  }

  var counters = Array.prototype.slice.call(document.querySelectorAll(".counter[data-sev]"));
  var search = document.getElementById("search");
  var findings = Array.prototype.slice.call(document.querySelectorAll("details.finding"));
  var files = Array.prototype.slice.call(document.querySelectorAll(".file"));
  var noMatch = document.getElementById("no-match");
  var count = document.getElementById("findings-count");

  function apply() {
    var on = {};
    var filtered = false;
    counters.forEach(function (c) {
      var pressed = c.getAttribute("aria-pressed") === "true";
      on[c.getAttribute("data-sev")] = pressed;
      if (!pressed) { filtered = true; }
    });
    var q = search ? search.value.trim().toLowerCase() : "";
    if (q !== "") { filtered = true; }
    var shown = 0;
    findings.forEach(function (f) {
      var visible = on[f.getAttribute("data-sev")] !== false &&
        (q === "" || f.getAttribute("data-text").indexOf(q) !== -1);
      f.hidden = !visible;
      if (visible) { shown++; }
    });
    files.forEach(function (g) {
      g.hidden = g.querySelector("details.finding:not([hidden])") === null;
    });
    if (noMatch) { noMatch.hidden = shown !== 0 || findings.length === 0; }
    if (count) {
      var total = count.getAttribute("data-total");
      count.textContent = filtered ? shown + " of " + total : total;
    }
  }

  counters.forEach(function (c) {
    c.disabled = false;
    c.addEventListener("click", function () {
      c.setAttribute("aria-pressed", c.getAttribute("aria-pressed") === "true" ? "false" : "true");
      apply();
    });
  });
  if (search) {
    search.hidden = false;
    search.addEventListener("input", apply);
  }

  Array.prototype.forEach.call(document.querySelectorAll("button.copy"), function (b) {
    b.hidden = false;
    b.addEventListener("click", function () {
      var value = b.getAttribute("data-copy");
      if (navigator.clipboard) {
        navigator.clipboard.writeText(value).then(function () {
          var old = b.textContent;
          b.textContent = "copied";
          setTimeout(function () { b.textContent = old; }, 1200);
        });
      }
    });
  });

  window.addEventListener("beforeprint", function () {
    Array.prototype.forEach.call(document.querySelectorAll("details"), function (d) { d.open = true; });
  });
})();
