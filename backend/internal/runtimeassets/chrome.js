// Shared by the editor preview and the standalone HTML player. Keep chrome
// outside the slide iframe so slide CSS/scripts cannot change its presentation.
(() => {
  const style = document.createElement('style');
  style.textContent = `
    .runtime-chrome { position:absolute; z-index:2; padding:3px 6px;
      color:rgba(20,25,35,.58); font:500 14px/1.2 ui-monospace,SFMono-Regular,Menlo,monospace;
      letter-spacing:.04em; pointer-events:none; }
    .runtime-chrome[data-placement^="top-"] { top:3.2%; }
    .runtime-chrome[data-placement^="bottom-"] { bottom:3.2%; }
    .runtime-chrome[data-placement$="-left"] { left:3.4%; }
    .runtime-chrome[data-placement$="-center"] { left:50%; transform:translateX(-50%); }
    .runtime-chrome[data-placement$="-right"] { right:3.4%; }
    .runtime-chrome[data-placement="left-edge"] { left:1.5%; top:50%; transform:translateY(-50%); }
    .runtime-chrome[data-placement="right-edge"] { right:1.5%; top:50%; transform:translateY(-50%); }
    .runtime-chrome[data-chrome-style~="compact"] { font-size:16px; }
    .runtime-chrome[data-chrome-style~="tiny"] { font-size:14px; }
    .runtime-chrome[data-chrome-style~="muted"] { color:rgba(20,25,35,.58); }
    .runtime-chrome[data-chrome-style~="mono"] { font-family:ui-monospace,SFMono-Regular,Menlo,monospace; }
    .runtime-chrome[data-chrome-style~="label"] { font-family:ui-sans-serif,system-ui,sans-serif;
      font-size:16px; font-weight:700; letter-spacing:.08em; text-transform:uppercase; }
  `;
  document.head.appendChild(style);

  window.PPTChrome = {
    render(canvas, context) {
      canvas.querySelectorAll('[data-runtime-chrome]').forEach(node => node.remove());
      const nodes = [];
      const chrome = Array.isArray(context.chrome) ? context.chrome : [];
      for (const item of chrome) {
        if (item.type === 'page_number' && context.numbering?.visible !== true) continue;
        const text = item.type === 'page_number' ? String(context.ordinal)
          : item.type === 'section_marker' ? context.section?.title
          : item.type === 'deck_title' ? context.deck_title : '';
        if (!text) continue;
        const node = document.createElement('div');
        node.className = 'runtime-chrome';
        node.dataset.runtimeChrome = item.type;
        node.dataset.placement = item.placement || 'bottom-right';
        node.dataset.chromeStyle = item.style || '';
        node.textContent = text;
        if (item.type === 'page_number') {
          node.dataset.runtimePageNumber = 'true';
          node.setAttribute('aria-label', `第 ${context.ordinal} 页，共 ${context.total} 页`);
        }
        canvas.appendChild(node);
        nodes.push(node);
      }
      return nodes;
    },
  };
})();
