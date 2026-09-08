(function () {
  "use strict";

  var queryEl = document.getElementById("query");
  var resultsEl = document.getElementById("results");
  var previewEl = document.getElementById("preview");
  var statusEl = document.getElementById("status");

  var bridge = window.go && window.go.main && window.go.main.App;
  if (!bridge) {
    statusEl.textContent = "未检测到 Wails bridge（请在 zoro-launcher 中运行）";
    return;
  }

  var current = [];

  function setStatus(text) {
    statusEl.textContent = text || "";
  }

  function renderResults(candidates) {
    current = candidates || [];
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

      var sub = document.createElement("div");
      sub.className = "item-sub";
      sub.textContent = c.library + " · " + c.path + ":" + c.start;

      li.appendChild(title);
      li.appendChild(sub);
      li.addEventListener("click", function () { select(i); });
      resultsEl.appendChild(li);
    });
    if (current.length) select(0);
  }

  function select(i) {
    var items = resultsEl.querySelectorAll(".item");
    items.forEach(function (el, idx) {
      el.classList.toggle("active", idx === i);
    });
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
  queryEl.addEventListener("keydown", function (e) {
    if (e.key !== "Enter") return;
    e.preventDefault();
    var c = current[0];
    if (c) {
      bridge.LoadRaw(c.library, c.path, c.start)
        .then(function (raw) { return bridge.Copy(raw || ""); })
        .then(function () { setStatus("已复制到剪贴板"); })
        .catch(function (e) { setStatus("复制失败：" + e); });
    }
  });

  queryEl.focus();
  doQuery();
})();
