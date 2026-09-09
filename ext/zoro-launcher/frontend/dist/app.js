/* ============================================================
   zoro launcher 前端逻辑 —— 一个 Alpine 组件，一个状态源。
   规则（违反任何一条都会重新引入我们刚修掉的那类 bug）：
     R1 状态只存在于 launcher() 返回的对象里；模板是唯一读者。
        getter 里**只读不写**（items / mode / summaryText …），
        否则 Alpine 的响应式追踪会在渲染中途改状态 → 无限循环。
     R2 不出现 document.getElementById / querySelector / classList /
        innerHTML / setAttribute。唯一的例外是 fitWindowToContent 里
        读 $refs.shell 的几何尺寸，以及 $refs.input.focus()。
     R3 窗口尺寸只有一个写入点：fitWindowToContent()，只被
        ResizeObserver 触发。任何 action 后面都不要手动调它。
     R4 所有 bridge 返回值先判空再用（kb/facts/pitfalls.md P-2：
        Wails 把 Go 的空 slice 序列化成 JS null）。
     R5 候选行只有**一种形状**（见 makeRow 的注释）。搜索命中、命令、
        知识库列表、待确认添加，四个来源都产出同一种行；模板因此
        永远不需要知道「现在是什么模式」。新增一种视图 = 多一个来源，
        模板一行都不用改。
     R6 行里的 indexHtml 只允许由 highlight() / escapeHtml() 产出。
        x-html 是唯一一处注入点，别把用户输入或错误信息直接塞进去。
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

// ⚠️ 必须与 app.go 的 eventProgress 逐字一致（两边各写一次，改一处要改两处）。
var EVENT_PROGRESS = "zoro:progress";

// ---- 模块级非响应式草稿变量 ----
// 故意放在组件外：不会被 Alpine 变成响应式，也不可能被模板改到。
var queryTimer       = null; // 搜索防抖定时器（clearQuery 必须能取消它）
var selectToken      = 0;    // 异步预览的过期判定令牌
var lastWindowHeight = 0;    // 上次下发给原生窗口的高度，用于去重
var resizeRafId      = 0;    // requestAnimationFrame 合并句柄
var shellObserver    = null; // ResizeObserver 实例
var cmdCacheQuery    = null; // 命令行候选缓存（items getter 会被读多次）
var cmdCacheRows     = null;

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

// 取路径最后一段（/lib add 不给库名时用它当默认名）。同时处理 \ 以便 Windows。
function basename(p) {
  var s = String(p == null ? "" : p).replace(/[\/\\]+$/, "");
  var i = Math.max(s.lastIndexOf("/"), s.lastIndexOf("\\"));
  return i >= 0 ? s.slice(i + 1) : s;
}

/* ============================================================
   命令面注册表
   ------------------------------------------------------------
   加一条命令 = 在这里加一行 + 在组件里写一个同名方法。
   没有别的接线点：模板不认识命令，items getter 自动把它列出来。

   form 字段：
     args     字面量（"add"）必须逐字匹配；尖括号（"<name>"）是占位符，
              接受任意值，并且**只把占位符位置上的实参**传给 action。
     desc     候选行第二行的说明文字。
     action   组件上的方法名，签名 function (argv)。
     options  可选：占位符的枚举值。当用户已经打完 verb 时，
              一个 form 会展开成一行一个选项（/theme → 三行）。
   ============================================================ */
var THEMES = ["light-glass", "dark-glass", "minimal"];

var COMMANDS = [
  { verb: "lib", forms: [
    { args: [],                desc: "列出已声明的知识库",                                    action: "listLibraries" },
    { args: ["add", "<name>"], desc: "添加知识库：先弹原生目录框，选完再确认（name 可省略）", action: "addLibrary" },
  ] },
  { verb: "reindex", forms: [
    { args: [], desc: "强制重建全部知识库的索引", action: "reindex" },
  ] },
  { verb: "theme", forms: [
    { args: ["<name>"], desc: "切换主题（本次运行有效）", action: "setTheme", options: THEMES },
  ] },
  { verb: "help", forms: [
    { args: [], desc: "列出全部命令", action: "showHelp" },
  ] },
];

function isVerbPrefix(word) {
  for (var i = 0; i < COMMANDS.length; i++) {
    if (COMMANDS[i].verb.indexOf(word) === 0) return true;
  }
  return false;
}

// null = 不匹配；"exact" = 实参已经够了一条完整命令；"completion" = 还差占位符。
// ⚠️ 多打的实参只有「末尾是占位符」的形态能吸收：
//    否则 "/lib add" 会被零参形态判成 exact，回车就去列库而不是加库了。
function matchArgs(formArgs, typedArgs) {
  var n = Math.min(formArgs.length, typedArgs.length);
  for (var i = 0; i < n; i++) {
    if (formArgs[i].charAt(0) === "<") continue;
    if (formArgs[i] !== typedArgs[i]) return null;
  }
  if (typedArgs.length < formArgs.length) return "completion";
  if (typedArgs.length === formArgs.length) return "exact";
  var last = formArgs[formArgs.length - 1] || "";
  return last.charAt(0) === "<" ? "exact" : null;
}

// 只把「占位符位置上的实参」交给 action：
// addLibrary 收到 ["notes"]，setTheme 收到 ["dark-glass"]，
// 于是 action 里永远不用去猜 argv[0] 是不是子命令字面量。
function argValues(formArgs, typedArgs) {
  var out = [];
  for (var i = 0; i < formArgs.length; i++) {
    if (formArgs[i].charAt(0) !== "<") continue;
    if (i < typedArgs.length) out.push(typedArgs[i]);
  }
  return out;
}

function commandRow(cmd, form, shownArgs, argv, rank) {
  return {
    rank: rank,
    row: {
      id:        "/" + cmd.verb + (shownArgs.length ? " " + shownArgs.join(" ") : ""),
      title:     "/" + cmd.verb + (shownArgs.length ? " " + shownArgs.join(" ") : ""),
      indexHtml: highlight(form.desc, []),
      meta:      "",
      action:    "run-command",
      run:       form.action,
      argv:      argv,
    },
  };
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
    statusText: "",       // 瞬态消息：没有匹配 / 查询失败 / 已复制 / 进度事件 …
    bootStatus: "",       // 启动时 Status() 返回的库概要，只在没有瞬态消息时兜底显示
    theme: "light-glass", // 经 :data-theme 绑定到 <html>

    // 两个「子视图」状态。它们都不是搜索结果，而是命令执行后的一次快照：
    //   dataView   `/lib` 的知识库列表（{label, rows}）
    //   pendingAdd `/lib add` 选完目录后等待用户确认的那一行（{name, root}）
    // 打字会清空它们（onQueryInput），Esc 会退回上一层（onKeydown）。
    // 它们为 null 时，items 才回落到命令行候选或搜索命中。
    dataView: null,
    pendingAdd: null,

    // ---------- 派生值：getter ----------
    // Alpine 的 x-data 对象被 reactive()（Proxy）包裹，getter 内部的
    // this.xxx 访问会被自动追踪 → 这就是响应式 computed。
    // 用 getter 而不是数据字段：零个写入点，不可能「忘记更新汇总」。

    // "command" | "search"：完全由输入框里的文本决定，不是一个可以被写坏的开关。
    get mode() {
      return this.parseCommand() ? "command" : "search";
    },

    // 当前是哪一类行。items / summaryText / hint / previewVisible 都从这里分派，
    // 所以「模式」这件事只有 parseCommand + 两个子视图状态三个判定点。
    get rowKind() {
      if (this.pendingAdd) return "confirm";
      if (this.dataView) return "data";
      return this.mode === "command" ? "command" : "result";
    },

    // ⭐ 模板渲染的**唯一**列表。四种来源，一种行形状（R5）：
    //    { id, title, indexHtml, meta, action, payload? , run?, argv? }
    //    action ∈ "open-result" | "run-command" | "confirm-add" | "reveal-library"
    //    runActive() 是这个字段的唯一消费者。
    get items() {
      var kind = this.rowKind;

      if (kind === "confirm") {
        var p = this.pendingAdd;
        return [{
          id:        "confirm-add",
          title:     "/lib add " + p.name,
          indexHtml: highlight(p.root, []),
          meta:      "写入 zoro.toml 并建立索引",
          action:    "confirm-add",
        }];
      }

      if (kind === "data") return this.dataView.rows;

      if (kind === "command") return this.commandItems();

      return this.results.map(function (c, i) {
        return {
          id:        c.library + "|" + c.path + "|" + c.start + "|" + i,
          title:     c.title || "",
          indexHtml: highlight(c.index || "", c.matches || []),
          meta:      c.library + " · " + c.path + ":" + c.start,
          action:    "open-result",
          payload:   c,
        };
      });
    },

    // 预览面板只在「详情模式」或「选中的行是一条搜索命中」时出现。
    // 命令行 / 库列表没有可预览的东西 —— 不去碰 results，也不去重算 items。
    get previewVisible() {
      if (this.detailMode) return true;
      return this.rowKind === "result" &&
             this.activeIndex >= 0 &&
             this.activeIndex < this.results.length;
    },

    get summaryText() {
      switch (this.rowKind) {
        case "confirm": return "确认添加知识库";
        case "data":    return this.dataView.label;
        case "command": return this.commandItems().length + " 条命令";
        default:        return this.results.length ? this.results.length + " 条命中" : "";
      }
    },

    get footerText() {
      return this.statusText || this.bootStatus;
    },

    // 启动时（无结果、无瞬态消息）状态栏隐藏 → 窗口就是搜索框那么高。
    // 有内容时显示库概要 + 快捷键提示；有瞬态消息时即使 0 行也显示
    //（旧版把「没有匹配」写进了一个隐藏的元素里，用户永远看不到）。
    get showFooter() {
      if (this.detailMode) return false;
      if (this.statusText.length > 0) return true;
      if (this.rowKind === "result") return this.results.length > 0;
      return this.items.length > 0;   // confirm / data / command 行数很少，重算不贵
    },

    // 快捷键提示随视图变化。⚠️ 只描述**当前**视图真能用的键，
    //    写死一行「↵ 详情 · ⌘C 复制」会让命令行下的用户按了没反应。
    get hint() {
      switch (this.rowKind) {
        case "confirm": return "↵ 确认添加 · Esc 取消";
        case "data":    return "↵ 在 Finder 打开 · Esc 返回列表";
        case "command": return "↵ 执行 · ↑↓ 选择 · Esc 隐藏";
        default:        return "↵ 详情 · ⌘↵ 打开源文件 · ⌘C 复制 · Esc 隐藏/返回";
      }
    },

    // ---------- 命令行解析 ----------
    // 返回 {verb, args} 或 null（null = 这次输入是搜索，不是命令）。
    parseCommand: function () {
      var q = this.query || "";
      // 只有「第一个字符就是 /」才算命令。开头留一个空格 = 逃生舱，
      // 这样 " /usr/local" 这种路径仍然能当搜索词用。
      if (q.charAt(0) !== "/") return null;

      var tokens = q.split(/\s+/).filter(Boolean);
      var head = tokens.length ? tokens[0].slice(1) : "";
      if (!head) return { verb: "", args: [] };      // 只有一个 "/" → 列出全部命令
      // 不是任何已注册 verb 的前缀 → 当成普通搜索词（"/root/zoro" 仍然可搜）。
      if (!isVerbPrefix(head)) return null;
      return { verb: head, args: tokens.slice(1) };
    },

    // 命令行候选。按 query 缓存：items getter 在一次渲染里会被读好几次。
    commandItems: function () {
      var q = this.query;
      if (cmdCacheQuery === q && cmdCacheRows) return cmdCacheRows;
      cmdCacheQuery = q;
      cmdCacheRows = this.buildCommandItems(this.parseCommand());
      return cmdCacheRows;
    },

    buildCommandItems: function (p) {
      if (!p) return [];
      var scored = [];

      var each = function (fn) {
        COMMANDS.forEach(function (cmd) {
          if (p.verb && cmd.verb.indexOf(p.verb) !== 0) return;
          cmd.forms.forEach(function (form) { fn(cmd, form); });
        });
      };

      // 第一遍：正常匹配。exact 排在 completion 前面，同一档里**参数多的排前面**
      //（"/lib add notes" 必须压过 "/lib"，否则回车会去列库而不是加库）。
      each(function (cmd, form) {
        var m = matchArgs(form.args, p.args);
        if (!m) return;
        var exact = m === "exact";
        var rank = (exact ? 0 : 100) + (10 - form.args.length);

        // 枚举参数展开：只在用户已经打完 verb 时做，否则 "/" 会被撑成十几行。
        if (form.options && p.verb === cmd.verb) {
          var last = exact ? p.args[p.args.length - 1] || "" : "";
          var opts = form.options.filter(function (o) { return !exact || o.indexOf(last) === 0; });
          if (opts.length) {
            var head = form.args.slice(0, form.args.length - 1);
            opts.forEach(function (o) {
              scored.push(commandRow(cmd, form, head.concat([o]),
                                     argValues(head, p.args).concat([o]), rank));
            });
            return;
          }
          // 没有选项匹配前缀（/theme foo）：仍然给一行，让 action 报出可读的错误。
        }

        scored.push(commandRow(cmd, form,
                               form.args.length === 0 ? [] : (exact ? p.args : form.args),
                               exact ? argValues(form.args, p.args) : [],
                               rank));
      });

      // 第二遍兜底：verb 对得上但实参对不上（"/lib xyz"）。
      // 这时**必须**列出该 verb 的所有形态，而不是给一个空面板 ——
      // 空面板等于静默失败，用户会以为工具坏了。
      if (!scored.length) {
        each(function (cmd, form) {
          scored.push(commandRow(cmd, form, form.args, [], 200 + (10 - form.args.length)));
        });
      }

      scored.sort(function (a, b) { return a.rank - b.rank; });   // V8/JSC 的 sort 稳定
      return scored.map(function (s) { return s.row; });
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

      // 建索引 / 重建索引可能要好几百毫秒。Go 侧每做完一步就推一行进度，
      // 否则窗口看起来像卡死了（用户会去点第二下，然后并发写就来了）。
      if (window.runtime && typeof window.runtime.EventsOn === "function") {
        var self = this;
        window.runtime.EventsOn(EVENT_PROGRESS, function (msg) { self.statusText = msg || ""; });
      }

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
      if (window.runtime && typeof window.runtime.EventsOff === "function") {
        window.runtime.EventsOff(EVENT_PROGRESS);
      }
    },

    // ---------- 输入 ----------
    // 不用 x-model + .debounce：Alpine 的 debounce 无法从外部取消，
    // clearQuery() 之后那个挂起的写入会把刚清空的文本「复活」。
    // 自己持有定时器 = clearQuery 能取消 = 只有一条清空路径。
    onQueryInput: function (value) {
      this.query = value || "";
      // 打字 = 放弃上一次命令留下的快照视图（它们是快照，不是搜索状态）。
      this.dataView = null;
      this.pendingAdd = null;
      clearTimeout(queryTimer);

      if (this.mode === "command") {
        // 命令面是纯派生的：不打桥、不防抖。items getter 立刻给出候选。
        this.resetResults();
        this.statusText = "";
        return;
      }
      queryTimer = setTimeout(function () { this.doQuery(); }.bind(this), 80);
    },

    clearQuery: function () {
      clearTimeout(queryTimer);
      this.query = "";
      this.dataView = null;
      this.pendingAdd = null;
      this.resetResults();
      if (this.$refs.input) this.$refs.input.focus();
    },

    doQuery: function () {
      if (!bridge) return;
      var q = (this.query || "").trim();
      if (!q) { this.resetResults(); this.statusText = ""; return; }
      if (this.mode === "command") return;   // 防抖到期时用户可能已经改成了命令
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

    // 唯一的「搜索结果全部收起」路径，取代旧 hideBody() 里 8 行 classList 操作。
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
      var items = this.items;
      if (i < 0 || i >= items.length) return;
      this.activeIndex = i;

      var it = items[i];
      if (it.action !== "open-result") {
        // 命令行 / 库列表行没有原文可预览：清掉预览，previewVisible 自然为 false。
        this.previewHtml = "";
        this.previewError = "";
        this.previewLoading = false;
        return;
      }
      if (!bridge) return;

      var c = it.payload;
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

    // 点击的唯一入口（模板 @click）。
    // 搜索结果：点击只选中，回车才进详情模式 —— 详情会占满整窗，误点代价高。
    // 其它行（命令 / 库列表 / 待确认）本身就是一颗按钮，点了不执行等于死按钮。
    activate: function (i) {
      this.select(i);
      var it = this.items[i];
      if (it && it.action !== "open-result") this.runActive();
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
      var n = this.items.length;
      if (!n) return;
      var next = this.activeIndex < 0
        ? (delta > 0 ? 0 : n - 1)
        : (this.activeIndex + delta + n) % n;
      this.select(next);
    },

    // 没选中时默认第 0 行（回车直接执行最匹配的那条）。
    activeItem: function () {
      var items = this.items;
      if (!items.length) return null;
      var i = this.activeIndex < 0 ? 0 : this.activeIndex;
      return items[i] || null;
    },

    // 只有搜索命中行才有 Candidate。命令行下按 ⌘C / ⌘↵ 因此是安全的空操作。
    activeResult: function () {
      var it = this.activeItem();
      return it && it.action === "open-result" ? it.payload : null;
    },

    // ---------- 详情模式 ----------
    enterDetail: function () {
      var c = this.activeResult();
      // ⚠️ 没有命中就不进详情模式。旧代码无条件 showDetailMode()，
      //    在 0 结果时会撑出一整屏空白窗口 —— 与「窗口贴合内容」的目标直接冲突。
      if (!c) return;
      if (this.activeIndex < 0) this.activeIndex = 0;
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

    // ---------- 回车的唯一分派点 ----------
    // 模板里没有 @keydown.enter；所有回车都经过这里，按行的 action 字段分派。
    // 加一种行 = 在 switch 里加一个 case，不需要动键盘处理。
    runActive: function () {
      var it = this.activeItem();
      if (!it) return;
      switch (it.action) {
        case "open-result":    this.enterDetail();          break;
        case "run-command":    this.runCommand(it);         break;
        case "confirm-add":    this.confirmAddLibrary();    break;
        case "reveal-library": this.revealLibrary(it.payload); break;
      }
    },

    runCommand: function (row) {
      var fn = this[row.run];
      if (typeof fn !== "function") {
        this.statusText = "命令未实现：" + row.title;
        return;
      }
      fn.call(this, row.argv || []);
    },

    // ---------- 命令 action（名字必须与注册表里的 action 一致）----------

    showHelp: function () {
      this.onQueryInput("/");      // 回到「列出全部命令」，顺带清掉子视图
    },

    listLibraries: function () {
      if (!bridge || !bridge.ListLibraries) { this.unsupported("/lib"); return; }
      var self = this;
      bridge.ListLibraries()
        .then(function (list) {
          var libs = list || [];                     // R4 / P-2
          if (!libs.length) {
            self.dataView = null;
            self.statusText = "还没有声明任何知识库，用 /lib add 添加一个";
            return;
          }
          self.dataView = {
            label: libs.length + " 个知识库",
            rows: libs.map(function (l) {
              return {
                id:        "lib:" + l.name,
                title:     l.name + (l.isDefault ? " · 默认" : ""),
                indexHtml: highlight(l.root || "", []),
                meta:      (l.blocks || 0) + " 个块",
                action:    "reveal-library",
                payload:   l.name,
              };
            }),
          };
          self.activeIndex = -1;
          self.statusText = "";
        })
        .catch(function (e) { self.statusText = "读取知识库列表失败：" + e; });
    },

    // 两步：先弹原生目录框（路径含空格/中文/需要 Tab 补全，输入框里打不动），
    // 选完把结果放进 pendingAdd 让用户确认 —— 写 zoro.toml 是不可逆的，
    // 不能在一次回车里静默完成。
    addLibrary: function (argv) {
      if (!bridge || !bridge.PickDirectory) { this.unsupported("/lib add"); return; }
      var wanted = (argv && argv[0]) || "";
      var self = this;
      bridge.PickDirectory()
        .then(function (root) {
          if (!root) {                        // 空串 = 用户取消，**不是**错误
            self.statusText = "已取消（没有选择目录）";
            return;
          }
          self.pendingAdd = { name: wanted || basename(root), root: root };
          self.activeIndex = 0;
          self.statusText = "";
        })
        .catch(function (e) { self.statusText = "选择目录失败：" + e; });
    },

    confirmAddLibrary: function () {
      var p = this.pendingAdd;
      if (!p) return;
      if (!bridge || !bridge.AddLibrary) { this.unsupported("/lib add"); return; }
      var self = this;
      this.statusText = "正在添加知识库：" + p.name;
      bridge.AddLibrary(p.name, p.root)
        .then(function (rep) {
          self.pendingAdd = null;
          self.dataView = null;
          self.resetResults();
          self.query = "";                 // 直接改状态；不走 onQueryInput（那会触发搜索）
          self.statusText = "已添加知识库 " + (rep && rep.name ? rep.name : p.name) +
                            "（" + ((rep && rep.blocks) || 0) + " 个块）" +
                            (rep && rep.createdDir ? "，目录是新建的" : "");
          self.reloadBootStatus();         // 库数变了，状态栏的概要必须跟着变
        })
        .catch(function (e) {
          // 保留 pendingAdd：错误多半是库名重复/非法，用户改一下输入框就能重试；
          // Esc 才是取消。清掉的话他得重新走一遍目录选择框。
          self.statusText = "添加失败：" + e;
        });
    },

    revealLibrary: function (name) {
      if (!bridge || !bridge.RevealLibrary) { this.unsupported("打开库目录"); return; }
      var self = this;
      bridge.RevealLibrary(name)
        .then(function (p) { self.statusText = "已在 Finder 中打开：" + (p || ""); })
        .catch(function (e) { self.statusText = "打开失败：" + e; });
    },

    reindex: function () {
      if (!bridge || !bridge.Reindex) { this.unsupported("/reindex"); return; }
      var self = this;
      this.statusText = "正在重建索引…";
      bridge.Reindex()
        .then(function (msg) {
          self.dataView = null;            // 块数变了，列表快照作废
          self.statusText = msg || "索引已重建";
          self.reloadBootStatus();
        })
        .catch(function (e) { self.statusText = "重建索引失败：" + e; });
    },

    setTheme: function (argv) {
      var name = (argv && argv[0]) || "";
      if (THEMES.indexOf(name) < 0) {
        this.statusText = "未知主题：" + (name || "(空)") + "，可用：" + THEMES.join(" / ");
        return;
      }
      this.theme = name;                   // :data-theme 是绑定，改状态即换肤
      this.statusText = "已切换主题：" + name + "（本次运行有效，重启后回到配置文件里的值）";
    },

    // 前端比后端新（或反过来）时不要让整条命令静默失败。
    unsupported: function (what) {
      this.statusText = "当前 Launcher 不支持 " + what + "（前后端版本不一致，请重新构建）";
    },

    reloadBootStatus: function () {
      if (!bridge || !bridge.Status) return;
      var self = this;
      bridge.Status()
        .then(function (s) { self.bootStatus = s || ""; })
        .catch(function () {});
    },

    // ---------- 动作（搜索结果专用）----------
    copyActive: function () {
      var c = this.activeResult();
      if (!c || !bridge) return;
      var self = this;
      bridge.LoadRaw(c.library, c.path, c.start)
        .then(function (raw) { return bridge.Copy(raw || ""); })
        .then(function () { self.statusText = "已复制到剪贴板"; })
        .catch(function (e) { self.statusText = "复制失败：" + e; });
    },

    openActive: function () {
      var c = this.activeResult();
      if (!c || !bridge) return;
      var self = this;
      bridge.OpenSource(c.library, c.path)
        .then(function (p) { self.statusText = "已打开源文件：" + (p || ""); })
        .catch(function (e) { self.statusText = "打开失败：" + e; });
    },

    // 保留以对齐 app.go 的 PopoutResult；当前未绑定到任何按键或按钮
    //（v3.2 时它绑在 Enter 上，后来 Enter 改成了详情模式）。
    popoutActive: function () {
      var c = this.activeResult();
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
        // Esc 是一架梯子：一次退一层，退到最外面才是隐藏窗口。
        if (this.pendingAdd) { this.pendingAdd = null; this.statusText = "已取消添加"; return; }
        if (this.dataView)   { this.dataView = null;   this.statusText = "";           return; }
        if (this.detailMode) { this.exitDetail(); return; }
        this.hide();
        return;
      }
      if (e.key === "ArrowDown") { e.preventDefault(); this.move(1);  return; }
      if (e.key === "ArrowUp")   { e.preventDefault(); this.move(-1); return; }
      if (e.key === "Enter") {
        e.preventDefault();
        if (meta) this.openActive(); else this.runActive();
        return;
      }
      if ((e.key === "c" || e.key === "C") && meta) {
        e.preventDefault();
        this.copyActive();
        return;
      }
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
