(function () {
  "use strict";

  var queryEl = document.getElementById("query");
  var resultsEl = document.getElementById("results");
  var previewEl = document.getElementById("preview");
  var statusEl = document.getElementById("status");
  var hideBtn = document.getElementById("hide");

  var bridge = window.go && window.go.main && window.go.main.App;
  if (!bridge) {
    statusEl.textContent = "未检测到 Wails bridge（请在 zoro-launcher 中运行）";
    return;
  }

  var current = [];
  var active = 0;

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

    // core 的 MatchRange 使用 UTF-8 byte offsets，且都落在合法码点边界。
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

  function renderResults(candidates) {
    current = candidates || [];
    active = 0;
    resultsEl.innerHTML = "";
    if (!current.length) {
      resultsEl.innerHTML = '<li class="empty">没有匹配条目</li>';
      return;
    }
    current.forEach(function (c, i) {
      var li = document.createElement("li");
      li.className = "item" + (i === 0 ? " active" : "");
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
    if (current.length) select(0, true);
  }

  function scrollActiveIntoView() {
    var items = resultsEl.querySelectorAll(".item");
    var el = items[active];
    if (el && el.scrollIntoView) el.scrollIntoView({ block: "nearest" });
  }

  function select(i, skipScroll) {
    active = i;
    var items = resultsEl.querySelectorAll(".item");
    items.forEach(function (el, idx) {
      el.classList.toggle("active", idx === i);
    });
    if (!skipScroll) scrollActiveIntoView();
    var c = current[i];
    if (!c) return;
    bridge.LoadRaw(c.library, c.path, c.start)
      .then(function (raw) { return bridge.RenderHTML(raw || ""); })
      .then(function (html) { previewEl.innerHTML = html; setStatus(""); })
      .catch(function (e) {
        previewEl.textContent = "预览失败：" + e;
        setStatus("预览失败");
      });
  }

  function move(delta) {
    if (!current.length) return;
    var next = (active + delta + current.length) % current.length;
    select(next);
  }

  function copyActive() {
    var c = current[active];
    if (!c) return;
    bridge.LoadRaw(c.library, c.path, c.start)
      .then(function (raw) { return bridge.Copy(raw || ""); })
      .then(function () { setStatus("已复制到剪贴板"); })
      .catch(function (e) { setStatus("复制失败：" + e); });
  }

  function openActive() {
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
    var q = queryEl.value || "";
    bridge.Query(q)
      .then(function (candidates) {
        renderResults(candidates);
        setStatus(candidates.length ? candidates.length + " 条命中" : "没有匹配");
      })
      .catch(function (e) { setStatus("查询失败：" + e); });
  }

  var debounceTimer = null;
  queryEl.addEventListener("input", function () {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(doQuery, 80);
  });

  document.addEventListener("keydown", function (e) {
    var meta = e.metaKey || e.ctrlKey;
    if (e.key === "Escape") {
      e.preventDefault();
      hide();
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
        copyActive();
      }
    }
  });

  hideBtn.addEventListener("click", hide);

  bridge.Status()
    .then(function (s) { setStatus(s); })
    .catch(function (e) { setStatus("读取状态失败：" + e); });

  queryEl.focus();
  doQuery();
})();
