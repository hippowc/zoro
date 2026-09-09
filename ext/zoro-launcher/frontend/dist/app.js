/* ============================================================
   zoro launcher 前端逻辑 —— 一个 Alpine 组件，一个状态源。
   规则（违反任何一条都会重新引入我们刚修掉的那类 bug）：
     R1 状态只存在于 launcher() 返回的对象里；模板是唯一读者。
     R2 不出现 document.getElementById / querySelector / classList /
        innerHTML / setAttribute。唯一的例外是 fitWindowToContent 里
        读 $refs.shell 的几何尺寸，以及 $refs.input.focus()。
     R3 窗口尺寸只有一个写入点：fitWindowToContent()，只被
        ResizeObserver 触发。任何 action 后面都不要手动调它。
     R4 所有 bridge 返回值先判空再用（kb/facts/pitfalls.md P-2：
        Wails 把 Go 的空 slice 序列化成 JS null）。
   ============================================================ */

"use strict";

// Wails 把 /wails/ipc.js + /wails/runtime.js 无 defer 地插到 <head> 最前面，
// 所以在 defer 脚本里 window.go / window.runtime 一定已就绪。
var bridge = (window.go && window.go.main && window.go.main.App) || null;

// ---- 窗口尺寸常量 ----
// ⚠️ MIN_WINDOW_HEIGHT 必须 <= main.go 的 MinHeight（当前两边都是 60）：
//    NSWindow 会用 userMinSize 约束 setFrame，Go 侧不降这里窗口就收不下去。
// ⚠️ WINDOW_WIDTH 必须等于 main.go 的 Width（780）：宽度永不自适应。
// MAX_WINDOW_HEIGHT 只是前端单侧的钳制上限（详情模式 568px 之上留余量），
// main.go 的 Height=76 仅是「只有搜索框」时的初值，首帧就被 ResizeObserver 纠正。
var WINDOW_WIDTH      = 780;
var MIN_WINDOW_HEIGHT = 60;
var MAX_WINDOW_HEIGHT = 580;

// ---- 模块级非响应式草稿变量 ----
// 故意放在组件外：不会被 Alpine 变成响应式，也不可能被模板改到。
var queryTimer       = null; // 搜索防抖定时器（clearQuery 必须能取消它）
var selectToken      = 0;    // 异步预览的过期判定令牌
var lastWindowHeight = 0;    // 上次下发给原生窗口的高度，用于去重
var resizeRafId      = 0;    // requestAnimationFrame 合并句柄
var shellObserver    = null; // ResizeObserver 实例

/* ============================================================
   纯函数：HTML 转义 + 命中高亮
   ⚠️ 与重构前的 app.js **逐字等价**，不要重写。
      Go 的 core.MatchRange.Start/End 是 Candidate.Index 上的 UTF-8
      **字节**偏移（core/query.go），不是字符下标。
      中文一个字 = 3 字节，直接 slice 字符串会切碎字符。
      必须走 TextEncoder → 按字节切 → TextDecoder。
   ============================================================ */
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

/* ============================================================
   根组件
   ============================================================ */
function launcher() {
  return {
    // ---------- 状态（模板唯一可见的东西）----------
    query: "",
    results: [],          // core.Candidate[]：{library,title,index,path,start,matches}
    activeIndex: -1,      // -1 = 尚未选中（此时不显示预览面板）
    detailMode: false,
    previewHtml: "",
    previewError: "",
    previewLoading: false,
    statusText: "",       // 瞬态消息：没有匹配 / 查询失败 / 已复制 …
    bootStatus: "",       // 启动时 Status() 返回的库概要，只在有结果时兜底显示
    theme: "light-glass", // 经 :data-theme 绑定到 <html>

    // ---------- 派生值：getter ----------
    // Alpine 的 x-data 对象被 reactive()（Proxy）包裹，getter 内部的
    // this.results 访问会被自动追踪 → 这就是响应式 computed。
    // 用 getter 而不是数据字段：零个写入点，不可能「忘记更新汇总」。
    get summaryText() {
      return this.results.length ? this.results.length + " 条命中" : "";
    },
    get footerText() {
      return this.statusText || this.bootStatus;
    },
    // 启动时（无结果、无瞬态消息）状态栏隐藏 → 窗口就是搜索框那么高。
    // 有结果时显示库概要 + 快捷键提示；有瞬态消息时即使 0 结果也显示
    //（旧版把「没有匹配」写进了一个隐藏的元素里，用户永远看不到）。
    get showFooter() {
      return !this.detailMode &&
             (this.statusText.length > 0 || this.results.length > 0);
    },

    // ---------- 生命周期 ----------
    init: function () {
      if (!bridge) {
        // 对齐旧行为：在浏览器里直接打开 index.html 时给出可读提示，不抛异常。
        this.statusText = "未检测到 Wails bridge（请在 zoro-launcher 中运行）";
        return;
      }
      if (bridge.GetTheme) {
        bridge.GetTheme()
          .then(function (t) { if (t) this.theme = t; }.bind(this))
          .catch(function () {});
      }
      bridge.Status()
        .then(function (s) { this.bootStatus = s || ""; }.bind(this))
        .catch(function (e) { this.statusText = "读取状态失败：" + e; }.bind(this));

      // ⚠️ init() 执行时子树指令还没 flush，$refs.shell / $refs.input 都不存在。
      //    必须放进 $nextTick。
      this.$nextTick(function () {
        this.startAutoResize();
        if (this.$refs.input) this.$refs.input.focus();
      }.bind(this));
    },

    destroy: function () {
      if (shellObserver) { shellObserver.disconnect(); shellObserver = null; }
      clearTimeout(queryTimer);
      if (resizeRafId) { cancelAnimationFrame(resizeRafId); resizeRafId = 0; }
    },

    // ---------- 搜索 ----------
    // 不用 x-model + .debounce：Alpine 的 debounce 无法从外部取消，
    // clearQuery() 之后那个挂起的写入会把刚清空的文本「复活」。
    // 自己持有定时器 = clearQuery 能取消 = 只有一条清空路径。
    onQueryInput: function (value) {
      this.query = value || "";
      clearTimeout(queryTimer);
      queryTimer = setTimeout(function () { this.doQuery(); }.bind(this), 80);
    },

    clearQuery: function () {
      clearTimeout(queryTimer);
      this.query = "";
      this.resetResults();
      if (this.$refs.input) this.$refs.input.focus();
    },

    doQuery: function () {
      if (!bridge) return;
      var q = (this.query || "").trim();
      if (!q) { this.resetResults(); this.statusText = ""; return; }
      var self = this;
      bridge.Query(q)
        .then(function (list) {
          self.results = list || [];          // R4 / P-2 兜底
          self.activeIndex = -1;
          self.previewHtml = "";
          self.previewError = "";
          self.previewLoading = false;
          self.statusText = self.results.length ? "" : "没有匹配";
        })
        .catch(function (e) { self.statusText = "查询失败：" + e; });
    },

    // 唯一的「全部收起」路径，取代旧 hideBody() 里 8 行 classList 操作。
    resetResults: function () {
      this.results = [];
      this.activeIndex = -1;
      this.detailMode = false;
      this.previewHtml = "";
      this.previewError = "";
      this.previewLoading = false;
    },

    // ---------- 选择 / 预览 ----------
    select: function (i) {
      if (!bridge) return;
      if (i < 0 || i >= this.results.length) return;
      this.activeIndex = i;
      var c = this.results[i];
      var token = ++selectToken;              // 取代旧的 active !== idx 判定
      var self = this;
      this.previewLoading = true;
      this.previewError = "";

      bridge.LoadRaw(c.library, c.path, c.start)
        .then(function (raw) { return bridge.RenderHTML(raw || ""); })
        .then(function (html) {
          if (token !== selectToken) return;  // 过期响应直接丢弃
          self.previewHtml = html || "";
          self.previewLoading = false;
          self.statusText = "";
        })
        .catch(function (e) {
          if (token !== selectToken) return;
          self.previewHtml = "";
          // 错误进独立状态字段，不再往预览元素里写文本 ——
          // 返回按钮永远不会被错误信息覆盖掉。
          self.previewError = "预览失败：" + e;
          self.previewLoading = false;
          self.statusText = "预览失败";
        });

      this.scrollActiveIntoView();
    },

    scrollActiveIntoView: function () {
      var self = this;
      this.$nextTick(function () {
        var list = self.$refs.list;
        if (!list) return;
        // ⚠️ 用 querySelectorAll('li')，不要用 list.children：
        //    <template x-for> 自身也是 <ul> 的子节点，children 下标会整体错位 1。
        var el = list.querySelectorAll("li")[self.activeIndex];
        if (el && el.scrollIntoView) el.scrollIntoView({ block: "nearest" });
      });
    },

    move: function (delta) {
      if (!this.results.length) return;
      var n = this.results.length;
      var next = this.activeIndex < 0
        ? (delta > 0 ? 0 : n - 1)
        : (this.activeIndex + delta + n) % n;
      this.select(next);
    },

    ensureActive: function () {
      if (this.activeIndex < 0 && this.results.length) this.activeIndex = 0;
      return this.activeIndex >= 0 ? this.results[this.activeIndex] : null;
    },

    // ---------- 详情模式 ----------
    enterDetail: function () {
      var c = this.ensureActive();
      // ⚠️ 没有命中就不进详情模式。旧代码无条件 showDetailMode()，
      //    在 0 结果时会撑出一整屏空白窗口 —— 与「窗口贴合内容」的目标直接冲突。
      if (!c) return;
      this.detailMode = true;
      this.select(this.activeIndex);   // 只发一次 LoadRaw（旧代码会发两次）
    },

    exitDetail: function () {
      this.detailMode = false;
      var self = this;
      this.$nextTick(function () {
        if (self.$refs.input) self.$refs.input.focus();
      });
    },

    // ---------- 动作 ----------
    copyActive: function () {
      var c = this.ensureActive();
      if (!c || !bridge) return;
      var self = this;
      bridge.LoadRaw(c.library, c.path, c.start)
        .then(function (raw) { return bridge.Copy(raw || ""); })
        .then(function () { self.statusText = "已复制到剪贴板"; })
        .catch(function (e) { self.statusText = "复制失败：" + e; });
    },

    openActive: function () {
      var c = this.ensureActive();
      if (!c || !bridge) return;
      var self = this;
      bridge.OpenSource(c.library, c.path)
        .then(function (p) { self.statusText = "已打开源文件：" + (p || ""); })
        .catch(function (e) { self.statusText = "打开失败：" + e; });
    },

    // 保留以对齐 app.go 的 PopoutResult；当前未绑定到任何按键或按钮
    //（v3.2 时它绑在 Enter 上，后来 Enter 改成了详情模式）。
    popoutActive: function () {
      var c = this.ensureActive();
      if (!c || !bridge) return;
      var self = this;
      bridge.PopoutResult(c.library, c.path, c.start)
        .then(function () { self.statusText = "已在浏览器中打开"; })
        .catch(function (e) { self.statusText = "打开失败：" + e; });
    },

    hide: function () {
      if (bridge) bridge.Hide().catch(function () {});
      this.statusText = "";
    },

    // ---------- 键盘 ----------
    onKeydown: function (e) {
      // ⚠️⚠️ IME 保护必须是本函数的第一件事，在所有分支之前，不可移动、不可删。
      //   中文输入法组词期间的 Enter/Esc/方向键必须原样交给输入法，
      //   否则用户按 Enter 选词会误触发详情模式。WebKit 下 keyCode 是 229。
      if (e.isComposing || e.keyCode === 229) return;

      var meta = e.metaKey || e.ctrlKey;

      if (e.key === "Escape") {
        e.preventDefault();
        if (this.detailMode) this.exitDetail(); else this.hide();
        return;
      }
      if (e.key === "ArrowDown") { e.preventDefault(); this.move(1);  return; }
      if (e.key === "ArrowUp")   { e.preventDefault(); this.move(-1); return; }
      if (e.key === "Enter") {
        e.preventDefault();
        if (meta) this.openActive(); else this.enterDetail();
        return;
      }
      if ((e.key === "c" || e.key === "C") && meta) {
        e.preventDefault();
        this.copyActive();
        return;
      }
    },

    // ---------- 高亮（包一层，模板里 this 明确）----------
    highlightHtml: function (text, matches) {
      return highlight(text || "", matches || []);
    },

    // ---------- 窗口自适应高度（唯一写入点，见 R3）----------
    startAutoResize: function () {
      var shell = this.$refs.shell;
      if (!shell) return;
      if (typeof ResizeObserver === "undefined") { this.fitWindowToContent(); return; }
      var self = this;
      // 只观察 shell。任何状态变化（结果增删、预览异步到达、详情模式切换、
      // 主题切换导致的字号变化、字体加载完成）都会改变 shell 高度并自动触发。
      // 因此**不需要**在任何 action 后面手动调 fit —— 少一个能忘记的步骤。
      shellObserver = new ResizeObserver(function () { self.requestWindowFit(); });
      shellObserver.observe(shell);   // observe() 会立刻回调一次 → 首帧即校正
      this.requestWindowFit();
    },

    requestWindowFit: function () {
      if (resizeRafId) return;         // 一帧内的多次变化合并成一次下发
      var self = this;
      resizeRafId = requestAnimationFrame(function () {
        resizeRafId = 0;
        self.fitWindowToContent();
      });
    },

    fitWindowToContent: function () {
      var shell = this.$refs.shell;
      if (!shell) return;
      if (!window.runtime || typeof window.runtime.WindowSetSize !== "function") return;

      var h = Math.ceil(shell.getBoundingClientRect().height);
      if (!h || h < 0) return;

      var target = Math.max(MIN_WINDOW_HEIGHT, Math.min(MAX_WINDOW_HEIGHT, h));
      if (target === lastWindowHeight) return;   // 去重：绝不重复下发同一尺寸
      lastWindowHeight = target;

      // darwin 的 SetSize 保持窗口**顶边不动**，所以窗口向下生长 / 从底部收缩，
      // 正是 Spotlight 的行为。宽度固定 780，永不改变。
      // macOS 上窗口点 == CSS px，不要乘 devicePixelRatio。
      // ⚠️ 不要在这里调用 WindowSetMinSize：darwin 的 SetMinSize 会走
      //    adjustWindowSize()，那个 setFrame 不重新锚定顶边，窗口会跳一下。
      window.runtime.WindowSetSize(WINDOW_WIDTH, target);
    },
  };
}
