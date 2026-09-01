import type { Theme } from '../../api/types';

export type ThemeShowcaseMode = 'cover' | 'content' | 'chart';

export const themeShowcaseModes: Array<{ value: ThemeShowcaseMode; label: string }> = [
  { value: 'cover', label: '封面' },
  { value: 'content', label: '内容页' },
  { value: 'chart', label: '图表页' },
];

export function readThemeToken(css: string, name: string): string {
  return css.match(new RegExp(`${name}\\s*:\\s*([^;}]+)`))?.[1]?.trim() ?? '';
}

export function themePalette(theme: Theme): string[] {
  const css = theme.css ?? '';
  return ['--color-bg', '--color-fg', '--color-primary', '--color-accent']
    .map((name) => readThemeToken(css, name))
    .filter(Boolean);
}

function firstFont(fontStack: string): string {
  return fontStack.split(',')[0]?.trim().replace(/^['"]|['"]$/g, '') || 'System Sans';
}

export function themeTypography(theme: Theme) {
  const css = theme.css ?? '';
  const bodyStack = readThemeToken(css, '--font-sans') || 'system-ui, sans-serif';
  const serifStack = readThemeToken(css, '--font-serif') || 'Georgia, serif';
  const declaredDisplay = readThemeToken(css, '--font-display');
  const prefersSerif = /editorial|serif|xiaohongshu/i.test(`${theme.id} ${theme.name}`);
  const displayStack = declaredDisplay || (prefersSerif ? serifStack : bodyStack);
  const displayName = firstFont(displayStack);
  const bodyName = firstFont(bodyStack);

  return {
    displayStack,
    label: displayName === bodyName ? displayName : `${displayName} / ${bodyName}`,
  };
}

function showcaseContent(mode: ThemeShowcaseMode): string {
  if (mode === 'cover') {
    return `<main class="showcase cover-page">
      <header class="slide-chrome"><span>DASI PPT AGENT</span><span>PRODUCT REVIEW</span></header>
      <section class="cover-copy">
        <p class="kicker">AI-NATIVE PRESENTATION</p>
        <h1><span class="cover-line">让表达先于</span><br><mark>工具</mark>发生</h1>
        <p class="lede">从一句需求开始，完成内容组织、视觉设计与页面交付。</p>
      </section>
      <aside class="type-specimen" aria-hidden="true">
        <span class="specimen-word">Dasi</span>
        <span class="specimen-rule"></span>
        <strong>IDEA<br>TO<br>SLIDES</strong>
      </aside>
      <footer class="slide-footer"><span>Dasi PPT Agent</span><span>Theme Showcase</span></footer>
    </main>`;
  }

  if (mode === 'content') {
    return `<main class="showcase content-page">
      <header class="slide-chrome"><span>DASI PPT AGENT</span><span>CONTENT SYSTEM</span></header>
      <section class="content-title">
        <p class="kicker">DESIGN PRINCIPLES</p>
        <h1>好页面先回答一个问题</h1>
      </section>
      <section class="content-columns">
        <blockquote>
          “这一页真正希望观众记住什么？”
          <span>先确定结论，再选择版式。</span>
        </blockquote>
        <div class="principles">
          <article><strong>观点先行</strong><p>标题直接表达结论，不让观众自己寻找重点。</p></article>
          <article><strong>结构服务内容</strong><p>并列、对比和流程使用不同版式，不依赖同一种卡片。</p></article>
          <article><strong>视觉保持克制</strong><p>颜色只负责强调，字体与留白建立阅读节奏。</p></article>
        </div>
      </section>
      <footer class="slide-footer"><span>Content before decoration</span><span>Theme Showcase</span></footer>
    </main>`;
  }

  return `<main class="showcase chart-page">
    <header class="slide-chrome"><span>DASI PPT AGENT</span><span>SAMPLE DATA</span></header>
    <section class="chart-title">
      <div><p class="kicker">QUARTERLY OUTPUT</p><h1>交付节奏持续提升</h1></div>
      <div class="legend"><span><i></i>已交付页面</span><span><i></i>本期重点</span></div>
    </section>
    <section class="chart-layout">
      <article class="chart-surface">
        <div class="chart-axis"><span>80</span><span>60</span><span>40</span><span>20</span><span>0</span></div>
        <div class="bar-chart">
          <div><i style="height:32%"></i><span>1月</span></div>
          <div><i style="height:45%"></i><span>2月</span></div>
          <div><i style="height:51%"></i><span>3月</span></div>
          <div><i style="height:63%"></i><span>4月</span></div>
          <div><i style="height:72%"></i><span>5月</span></div>
          <div><i class="accent-bar" style="height:88%"></i><span>6月</span></div>
        </div>
      </article>
      <aside class="chart-note">
        <p class="kicker">KEY TAKEAWAY</p>
        <strong>模板复用后，团队把更多时间留给内容判断。</strong>
        <div><span>页面产出</span><b>72</b></div>
      </aside>
    </section>
    <footer class="slide-footer"><span>示例数据</span><span>Theme Showcase</span></footer>
  </main>`;
}

export function buildThemeShowcase(theme: Theme, mode: ThemeShowcaseMode): string {
  const css = theme.css ?? '';
  const displayStack = themeTypography(theme).displayStack;

  return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><style>${css}</style><style>
    *{box-sizing:border-box}html,body{margin:0;width:100%;height:100%;overflow:hidden}body{background:var(--color-bg);color:var(--color-fg);font-family:var(--font-sans);-webkit-font-smoothing:antialiased}.showcase{width:100%;height:100%;overflow:hidden;padding:5.2% 6.2% 4.2%;background:var(--theme-background,var(--color-bg));color:var(--color-fg)}h1,p,blockquote{margin:0}h1{font-family:${displayStack};font-size:clamp(38px,5.6vw,var(--text-title));font-weight:750;line-height:1.02;letter-spacing:-.04em}.kicker{color:var(--color-primary);font-family:var(--font-mono,monospace);font-size:clamp(8px,.8vw,11px);font-weight:800;letter-spacing:.1em}.lede{color:color-mix(in srgb,var(--color-fg) 68%,transparent);font-size:clamp(11px,1.25vw,17px);line-height:1.55}.slide-chrome,.slide-footer{display:flex;align-items:center;justify-content:space-between;color:color-mix(in srgb,var(--color-fg) 54%,transparent);font-family:var(--font-mono,monospace);font-size:clamp(7px,.72vw,10px);letter-spacing:.08em}.slide-footer{border-top:1px solid var(--color-border,color-mix(in srgb,var(--color-fg) 18%,transparent));padding-top:2.2%}
    .cover-page{display:grid;grid-template-columns:1.24fr .76fr;grid-template-rows:auto 1fr auto;column-gap:7%}.cover-page .slide-chrome,.cover-page .slide-footer{grid-column:1/3}.cover-copy{align-self:center}.cover-copy h1{max-width:11ch;margin:4% 0 3%;font-size:clamp(48px,7vw,var(--text-title))}.cover-line{white-space:nowrap}.cover-copy mark{background:none;color:var(--color-primary)}.cover-copy .lede{max-width:31ch}.type-specimen{position:relative;align-self:center;aspect-ratio:4/5;overflow:hidden;border:1px solid var(--color-border,color-mix(in srgb,var(--color-fg) 18%,transparent));border-radius:var(--radius-lg,var(--radius-md));background:var(--color-surface,var(--color-muted));box-shadow:var(--shadow-pop,var(--shadow-card));padding:10%;transform:translateY(-4%)}.specimen-word{display:block;color:var(--color-primary);font-family:${displayStack};font-size:clamp(44px,6.5vw,92px);font-weight:800;line-height:.88;letter-spacing:-.08em}.specimen-rule{display:block;width:68%;height:7px;margin-top:15%;background:linear-gradient(90deg,var(--color-primary) 0 76%,var(--color-accent) 76%)}.type-specimen strong{position:absolute;right:10%;bottom:9%;font-family:var(--font-mono,monospace);font-size:clamp(10px,1.15vw,16px);line-height:1.08;text-align:right}
    .content-page{display:grid;grid-template-rows:auto auto 1fr auto;gap:4.5%}.content-title h1{max-width:16ch;margin-top:2.5%;font-size:clamp(34px,4.6vw,66px)}.content-columns{display:grid;min-height:0;grid-template-columns:.9fr 1.1fr;gap:7%;align-items:stretch}blockquote{display:flex;flex-direction:column;justify-content:space-between;border-left:5px solid var(--color-accent);padding:7% 8%;border-radius:0 var(--radius-md) var(--radius-md) 0;background:var(--color-muted);font-family:${displayStack};font-size:clamp(19px,2.5vw,35px);font-weight:700;line-height:1.28}blockquote span{max-width:28ch;color:color-mix(in srgb,var(--color-fg) 62%,transparent);font-family:var(--font-sans);font-size:clamp(9px,.95vw,13px);font-weight:500}.principles{display:grid;align-content:center}.principles article{padding:4.5% 0;border-bottom:1px solid var(--color-border,color-mix(in srgb,var(--color-fg) 17%,transparent))}.principles article:first-child{padding-top:0}.principles article:last-child{border-bottom:0;padding-bottom:0}.principles strong{display:block;color:var(--color-primary);font-family:${displayStack};font-size:clamp(14px,1.6vw,22px)}.principles p{max-width:38ch;margin-top:1.5%;color:color-mix(in srgb,var(--color-fg) 66%,transparent);font-size:clamp(9px,1vw,13px);line-height:1.5}
    .chart-page{display:grid;grid-template-rows:auto auto 1fr auto;gap:4%}.chart-title{display:flex;align-items:end;justify-content:space-between;gap:6%}.chart-title h1{margin-top:2.5%;font-size:clamp(32px,4.2vw,60px)}.legend{display:flex;gap:18px;padding-bottom:.6%;color:color-mix(in srgb,var(--color-fg) 62%,transparent);font-size:clamp(8px,.85vw,11px)}.legend span{display:flex;align-items:center;gap:6px}.legend i{width:9px;height:9px;background:var(--color-primary)}.legend span:last-child i{background:var(--color-accent)}.chart-layout{display:grid;min-height:0;grid-template-columns:1.42fr .58fr;gap:3%}.chart-surface,.chart-note{border:1px solid var(--color-border,color-mix(in srgb,var(--color-fg) 17%,transparent));border-radius:var(--radius-lg,var(--radius-md));background:var(--color-surface,var(--color-muted));box-shadow:var(--shadow-card)}.chart-surface{display:grid;min-height:0;grid-template-columns:32px 1fr;padding:4.5% 5% 3.5%}.chart-axis{display:flex;flex-direction:column;justify-content:space-between;padding-bottom:20px;color:color-mix(in srgb,var(--color-fg) 48%,transparent);font-family:var(--font-mono,monospace);font-size:clamp(7px,.68vw,9px)}.bar-chart{display:flex;min-height:0;align-items:end;justify-content:space-around;gap:6%;border-left:1px solid var(--color-border,color-mix(in srgb,var(--color-fg) 16%,transparent));border-bottom:1px solid var(--color-border,color-mix(in srgb,var(--color-fg) 16%,transparent));padding:0 5%}.bar-chart div{display:flex;height:100%;flex:1;flex-direction:column;justify-content:end;align-items:center}.bar-chart i{display:block;width:68%;border-radius:var(--radius-sm) var(--radius-sm) 0 0;background:var(--color-primary);opacity:.74}.bar-chart i.accent-bar{background:var(--color-accent);opacity:1}.bar-chart span{height:20px;padding-top:6px;color:color-mix(in srgb,var(--color-fg) 54%,transparent);font-size:clamp(7px,.72vw,10px)}.chart-note{display:flex;flex-direction:column;justify-content:space-between;padding:9%}.chart-note>strong{font-family:${displayStack};font-size:clamp(17px,2.15vw,30px);line-height:1.2}.chart-note>div{border-top:1px solid var(--color-border,color-mix(in srgb,var(--color-fg) 17%,transparent));padding-top:8%}.chart-note>div span{display:block;color:color-mix(in srgb,var(--color-fg) 58%,transparent);font-size:clamp(8px,.8vw,11px)}.chart-note b{display:block;margin-top:2%;color:var(--color-accent);font-size:clamp(30px,4vw,58px);line-height:1}
  </style></head><body>${showcaseContent(mode)}</body></html>`;
}
