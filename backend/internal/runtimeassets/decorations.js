// Shared by preview, image/PDF rendering and offline playback. Decorations live
// outside the slide iframe; HTML authors cannot alter these global elements.
(() => {
  const sheet = document.createElement('style');
  sheet.textContent = `
    .runtime-decoration { position:absolute; z-index:2; padding:3px 6px;
      max-width:40%; box-sizing:border-box; overflow-wrap:anywhere;
      color:var(--color-caption); font:500 16px/1.35 var(--font-sans);
      pointer-events:none; }
    .runtime-decoration[data-runtime-decoration="page_number"] { font-family:var(--font-mono); }
    .runtime-decoration[data-placement^="top-"] { top:3.2%; }
    .runtime-decoration[data-placement^="bottom-"] { bottom:3.2%; }
    .runtime-decoration[data-placement$="-left"] { left:3.4%; }
    .runtime-decoration[data-placement$="-center"] { left:50%; transform:translateX(-50%); text-align:center; }
    .runtime-decoration[data-placement$="-right"] { right:3.4%; text-align:right; }
    .runtime-decoration[data-placement="left-edge"] { left:1.5%; top:50%; transform:translateY(-50%); }
    .runtime-decoration[data-placement="right-edge"] { right:1.5%; top:50%; transform:translateY(-50%); }
  `;
  document.head.appendChild(sheet);

  window.PPTDecorations = {
    isValid(value) {
      if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).length !== 4) return false;
      const placements = ['top-left', 'top-center', 'top-right', 'bottom-left', 'bottom-center', 'bottom-right', 'left-edge', 'right-edge'];
      return ['page_number', 'deck_title', 'section_title', 'key_message'].every(key => {
        const item = value[key];
        return typeof item === 'string'
          && (placements.includes(item) || (key !== 'page_number' && item === 'none'));
      });
    },
    render(canvas, context) {
      if (!this.isValid(context.decorations)) throw new Error('invalid decorations configuration');
      for (const key of ['--color-caption', '--color-fg', '--font-sans', '--font-mono']) {
        canvas.style.setProperty(key, context.appearance?.decoration_tokens?.[key] || '');
      }
      canvas.querySelectorAll('[data-runtime-decoration]').forEach(node => node.remove());
      const content = {
        page_number: String(context.ordinal),
        deck_title: context.deck_title,
        section_title: context.section?.title,
        key_message: context.key_message,
      };
      const nodes = [];
      for (const type of ['page_number', 'deck_title', 'section_title', 'key_message']) {
        const item = context.decorations[type];
        const text = content[type];
        if (item === 'none' || !text?.trim()) continue;
        const node = document.createElement('div');
        node.className = 'runtime-decoration';
        node.dataset.runtimeDecoration = type;
        node.dataset.placement = item;
        node.textContent = text;
        if (type === 'page_number') {
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
