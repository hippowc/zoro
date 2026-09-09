(function () {
  "use strict";

  var bridge = window.go && window.go.main && window.go.main.App;
  if (!bridge) {
    var statusFallback = document.getElementById("status");
    if (statusFallback) statusFallback.textContent = "未检测到 Wails bridge（请在 zoro-launcher 中运行）";
    return;
  }

  if (bridge.GetTheme) {
    bridge.GetTheme().then(function (theme) {
      if (theme) document.documentElement.setAttribute("data-theme", theme);
    }).catch(function () {});
  }

  var queryEl = document.getElementById("query");
  var bodyEl = document.getElementById("body");
  var resultsEl = document.getElementById("results");
  var previewEl = document.getElementById("preview");
  var statusEl = document.getElementById("status");
  var clearBtn = document.getElementById("clear");
  var summaryEl = document.getElementById("summary");
  var backBtn = document.getElementById("backBtn");
  var shellEl = document.querySelector(".shell");
  var statusFooterEl = document.querySelector(".status");

  var current = [];
  var active = -1;
  var detailMode = false;

  function escapeHtml(s) {
    return String(s == null ? "" : s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }

  function highlight(text, matches) {
    var esc = escapeHtml(text);
    if (!matches || !matches.length) return esc;

    var enc = new TextEncoder();
    var dec = new TextDecoder();
    var bytes = enc.encode(text);

    var ranges = matches
      .filter(function (r) { return r && r.end > r.start; })
      .sort(function (a, b) { return a.start - b.start; });

    var merged = [];
    ranges.forEach(function (r) {
      var s = Math.max(0, Math.min(r.start, bytes.length));
      var e = Math.max(s, Math.min(r.end, bytes.length));
      if (e <= s) return;
      var last = merged.length ? merged[merged.length - 1] : null;
      if (last && s <= last.end) {
        if (e > last.end) last.end = e;
        return;
      }
      merged.push({ start: s, end: e });
    });

    var out = "";
    var pos = 0;
    merged.forEach(function (r) {
      out += escapeHtml(dec.decode(bytes.slice(pos, r.start)));
      out += "<mark>" + escapeHtml(dec.decode(bytes.slice(r.start, r.end))) + "</mark>";
      pos = r.end;
    });
    out += escapeHtml(dec.decode(bytes.slice(pos)));
    return out;
  }

  function setStatus(text) {
    statusEl.textContent = text || "";
  }

  function setSummary(text) {
    if (!summaryEl) return;
    summaryEl.textContent = text || "";
    if (text) {
      summaryEl.classList.remove("is-hidden");
    } else {
      summaryEl.classList.add("is-hidden");
    }
  }

  function updateClear() {
    clearBtn.classList.toggle("is-visible", queryEl.value.length > 0);
  }

  function showBody() {
    bodyEl.classList.add("is-visible");
    if (statusFooterEl) statusFooterEl.classList.remove("is-hidden");
  }

  function hideBody() {
    bodyEl.classList.remove("is-visible", "show-results", "show-preview", "show-detail");
    resultsEl.innerHTML = "";
    previewEl.innerHTML = backBtn ? backBtn.outerHTML : "";
    current = [];
    active = -1;
    detailMode = false;
    setSummary("");
    if (statusFooterEl) statusFooterEl.classList.add("is-hidden");
    if (shellEl) shellEl.classList.remove("show-detail");
  }

  function showResults() {
    bodyEl.classList.remove("show-preview", "show-detail");
    bodyEl.classList.add("show-results");
    previewEl.innerHTML = backBtn ? backBtn.outerHTML : "";
    showBody();
  }

  function showPreview() {
    bodyEl.classList.remove("show-results", "show-detail");
    bodyEl.classList.add("show-preview");
    showBody();
  }

  function showDetailMode() {
    detailMode = true;
    bodyEl.classList.remove("show-results", "show-preview");
    bodyEl.classList.add("show-detail");
    showBody();
    if (shellEl) shellEl.classList.add("show-detail");
  }

  function exitDetailMode() {
    detailMode = false;
    bodyEl.classList.remove("show-detail");
    bodyEl.classList.add("show-results");
    if (shellEl) shellEl.classList.remove("show-detail");
  }

  function renderResults(candidates) {
    current = candidates || [];
    active = -1;
    resultsEl.innerHTML = "";

    if (!current.length) {
      hideBody();
      return;
    }

    showResults();
    setSummary(current.length + " 条命中");
    current.forEach(function (c, i) {
      var li = document.createElement("li");
      li.className = "item";
      li.dataset.index = String(i);

      var title = document.createElement("div");
      title.className = "item-title";
      title.textContent = c.title;

      var index = document.createElement("div");
      index.className = "item-index";
      index.innerHTML = highlight(c.index, c.matches);

      var sub = document.createElement("div");
      sub.className = "item-sub";
      sub.textContent = c.library + " · " + c.path + ":" + c.start;

      li.appendChild(title);
      li.appendChild(index);
      li.appendChild(sub);
      li.addEventListener("click", function () { select(i); });
      resultsEl.appendChild(li);
    });
  }

  function scrollActiveIntoView() {
    var items = resultsEl.querySelectorAll(".item");
    var el = items[active];
    if (el && el.scrollIntoView) el.scrollIntoView({ block: "nearest" });
  }

  function select(i) {
    if (i < 0 || i >= current.length) return;
    active = i;
    var items = resultsEl.querySelectorAll(".item");
    items.forEach(function (el, idx) {
      el.classList.toggle("active", idx === i);
    });
    scrollActiveIntoView();

    var c = current[i];
    var idx = i;

    if (!detailMode) {
      showPreview();
    }

    bridge.LoadRaw(c.library, c.path, c.start)
      .then(function (raw) { return bridge.RenderHTML(raw || ""); })
      .then(function (html) {
        if (active !== idx) return;
        var backBtnHtml = backBtn ? backBtn.outerHTML : "";
        previewEl.innerHTML = backBtnHtml + html;
        setStatus("");
      })
      .catch(function (e) {
        if (active !== idx) return;
        previewEl.textContent = "预览失败：" + e;
        setStatus("预览失败");
      });
  }

  function move(delta) {
    if (!current.length) return;
    var next;
    if (active < 0) {
      next = delta > 0 ? 0 : current.length - 1;
    } else {
      next = (active + delta + current.length) % current.length;
    }
    select(next);
  }

  function ensureActive() {
    if (active < 0 && current.length) select(0);
  }

  function copyActive() {
    if (!current.length) return;
    ensureActive();
    var c = current[active];
    if (!c) return;
    bridge.LoadRaw(c.library, c.path, c.start)
      .then(function (raw) { return bridge.Copy(raw || ""); })
      .then(function () { setStatus("已复制到剪贴板"); })
      .catch(function (e) { setStatus("复制失败：" + e); });
  }

  function popoutActive() {
    if (!current.length) return;
    ensureActive();
    var c = current[active];
    if (!c) return;
    bridge.PopoutResult(c.library, c.path, c.start)
      .then(function () { setStatus("已在浏览器中打开"); })
      .catch(function (e) { setStatus("打开失败：" + e); });
  }

  function openActive() {
    if (!current.length) return;
    ensureActive();
    var c = current[active];
    if (!c) return;
    bridge.OpenSource(c.library, c.path)
      .then(function (p) { setStatus("已打开源文件：" + p); })
      .catch(function (e) { setStatus("打开失败：" + e); });
  }

  function hide() {
    bridge.Hide().catch(function () {});
    setStatus("");
  }

  function doQuery() {
    var q = (queryEl.value || "").trim();
    if (!q) {
      hideBody();
      setStatus("");
      return;
    }
    bridge.Query(q)
      .then(function (candidates) {
        candidates = candidates || [];
        renderResults(candidates);
        if (!candidates.length) {
          setStatus("没有匹配");
        }
      })
      .catch(function (e) { setStatus("查询失败：" + e); });
  }

  var debounceTimer = null;
  queryEl.addEventListener("input", function () {
    updateClear();
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(doQuery, 80);
  });

  clearBtn.addEventListener("click", function () {
    queryEl.value = "";
    updateClear();
    hideBody();
    setStatus("");
    queryEl.focus();
  });

  document.addEventListener("keydown", function (e) {
    var meta = e.metaKey || e.ctrlKey;

    // Ignore Enter during IME composition (e.g., Chinese input method)
    if (e.isComposing || e.keyCode === 229) {
      return;
    }

    if (e.key === "Escape") {
      e.preventDefault();
      if (detailMode) {
        exitDetailMode();
      } else {
        hide();
      }
      return;
    }

    if (e.key === "ArrowDown") {
      e.preventDefault();
      move(1);
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      move(-1);
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      if (meta) {
        openActive();
      } else {
        ensureActive();
        showDetailMode();
        if (active >= 0 && active < current.length) {
          select(active);
        }
      }
      return;
    }
    if ((e.key === "c" || e.key === "C") && meta) {
      e.preventDefault();
      copyActive();
      return;
    }
  });

  // Event delegation for back button (survives innerHTML updates)
  previewEl.addEventListener("click", function (e) {
    var btn = e.target.closest(".back-btn");
    if (btn && detailMode) {
      exitDetailMode();
    }
  });

  bridge.Status()
    .then(function (s) { setStatus(s); })
    .catch(function (e) { setStatus("读取状态失败：" + e); });

  updateClear();
  queryEl.focus();
})();
