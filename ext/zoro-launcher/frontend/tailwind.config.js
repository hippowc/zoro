/** @type {import('tailwindcss').Config} */
//
// ⚠️ content 只能列「真正会被浏览器加载的模板文件」。
//    绝对不要加 ./dist/alpine.js —— 那是压缩过的第三方代码，
//    Tailwind 会从里面提取出成百上千个假类名，styles.css 会爆炸式膨胀。
//    app.js 必须在列表里：Alpine 的 :class 三元表达式里的类名是字面量，
//    Tailwind 只能靠扫描源文件发现它们。
module.exports = {
  content: ['./dist/index.html', './dist/app.js'],

  // ⚠️ 不要设置 important: true。
  //    它会破坏「components 层可以被 utilities 层覆盖」的层叠顺序，
  //    把这次重构要消灭的 CSS 特异性战争原样带回来。
  theme: {
    extend: {
      // 颜色全部指向 CSS 变量：主题切换仍然只靠 <html data-theme="...">，
      // Tailwind 只负责生成 bg-glass / text-fg / border-stroke 这类工具类。
      colors: {
        fg: 'var(--fg)',
        muted: 'var(--muted)',
        glass: 'var(--glass)',
        'glass-strong': 'var(--glass-strong)',
        stroke: 'var(--stroke)',
        'stroke-soft': 'var(--stroke-soft)',
        active: 'var(--active)',
        mark: 'var(--mark)',
        link: '#3157d1',
      },
      // 对应旧 styles.css 的 font-family（由 preflight 应用到 html）。
      fontFamily: {
        sans: ['system-ui', '-apple-system', 'PingFang SC', 'Microsoft YaHei', 'sans-serif'],
      },
    },
  },

  // ⚠️ 不要加 plugin、不要加 corePlugins 覆盖、不要开 darkMode。
  //    深色主题走 data-theme CSS 变量，不走 Tailwind 的 dark: 变体。
  plugins: [],
};
