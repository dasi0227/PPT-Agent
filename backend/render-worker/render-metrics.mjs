// Passed directly to Frame.evaluate: keep all DOM helpers inside this function.
export function measureSlideContent(maxItems) {
  const stage = document.querySelector('.slide-stage') ?? document.documentElement;
  const stageRect = stage.getBoundingClientRect();
  const root = document.documentElement;
  const body = document.body;
  const scrollWidth = Math.max(root.scrollWidth, body?.scrollWidth ?? 0, stage.scrollWidth);
  const scrollHeight = Math.max(root.scrollHeight, body?.scrollHeight ?? 0, stage.scrollHeight);
  const describe = element => ({
    tag: element.tagName.toLowerCase(),
    id: element.id || undefined,
    class: typeof element.className === 'string' ? element.className.slice(0, 160) : undefined,
  });
  const edges = rect => Object.fromEntries(['left', 'top', 'right', 'bottom'].map(side => [side, Math.round(rect[side])]));
  const out_of_bounds = [];
  for (const element of stage.querySelectorAll('*')) {
    const rect = element.getBoundingClientRect();
    if (rect.width === 0 && rect.height === 0) continue;
    if (rect.left < stageRect.left - 1 || rect.top < stageRect.top - 1 ||
        rect.right > stageRect.right + 1 || rect.bottom > stageRect.bottom + 1) {
      out_of_bounds.push({ ...describe(element), rect: edges(rect) });
      if (out_of_bounds.length >= maxItems) break;
    }
  }

  const text_clipping = [];
  const walker = document.createTreeWalker(stage, NodeFilter.SHOW_TEXT);
  const styleCache = new Map();
  const styleOf = element => {
    if (!styleCache.has(element)) styleCache.set(element, getComputedStyle(element));
    return styleCache.get(element);
  };
  const clips = value => /^(hidden|clip|auto|scroll)$/.test(value);
  const establishesBlock = style => style.transform !== 'none' || style.perspective !== 'none' ||
    style.filter !== 'none' || /\b(paint|layout|strict|content)\b/.test(style.contain) ||
    /\b(transform|perspective|filter)\b/.test(style.willChange);
  const excess = (rect, clip, x, y) => ({
    left: x ? Math.max(0, clip.left - rect.left) : 0,
    right: x ? Math.max(0, rect.right - clip.right) : 0,
    top: y ? Math.max(0, clip.top - rect.top) : 0,
    bottom: y ? Math.max(0, rect.bottom - clip.bottom) : 0,
  });
  const isClipped = (rect, clip, x, y) => rect.width > 0 && rect.height > 0 &&
    Object.values(excess(rect, clip, x, y)).some(value => value > 1);

  for (let node; (node = walker.nextNode()) && text_clipping.length < maxItems;) {
    const element = node.parentElement;
    if (!element || !node.data.trim() || element.closest('script,style,noscript,template')) continue;
    const textStyle = styleOf(element);
    if (textStyle.visibility !== 'visible' || textStyle.display === 'none') continue;
    const ancestors = [];
    let invisible = false;
    for (let ancestor = element; ancestor; ancestor = ancestor.parentElement) {
      const style = styleOf(ancestor);
      if (style.display === 'none' || Number(style.opacity) === 0 || style.contentVisibility === 'hidden') {
        invisible = true;
        break;
      }
      ancestors.push({ element: ancestor, style });
    }
    if (invisible) continue;
    const range = document.createRange();
    const start = node.data.search(/\S/);
    const end = node.data.trimEnd().length;
    range.setStart(node, start);
    range.setEnd(node, end);
    const rects = Array.from(range.getClientRects());
    if (rects.length === 0) continue;
    let positioned = '';
    for (const { element: ancestor, style } of ancestors) {
      // An absolutely/fixed positioned descendant can escape an overflow box
      // between itself and its containing block; that box does not clip it.
      if (positioned && (establishesBlock(style) || (positioned === 'absolute' && style.position !== 'static'))) positioned = '';
      const paintClip = /\b(paint|strict|content)\b/.test(style.contain);
      const clipX = paintClip || clips(style.overflowX);
      const clipY = paintClip || clips(style.overflowY);
      if (!positioned && ancestor instanceof HTMLElement && style.display !== 'inline' && (clipX || clipY)) {
        const rect = ancestor.getBoundingClientRect();
        const scaleX = ancestor.offsetWidth ? rect.width / ancestor.offsetWidth : 1;
        const scaleY = ancestor.offsetHeight ? rect.height / ancestor.offsetHeight : 1;
        const clip = {
          left: rect.left + ancestor.clientLeft * scaleX,
          top: rect.top + ancestor.clientTop * scaleY,
          right: rect.left + (ancestor.clientLeft + ancestor.clientWidth) * scaleX,
          bottom: rect.top + (ancestor.clientTop + ancestor.clientHeight) * scaleY,
        };
        // overflow:clip may deliberately extend its rectangular clipping edge.
        const margin = Number.parseFloat((style.overflowClipMargin || '').match(/[\d.]+px$/)?.[0] ?? '0');
        if (style.overflowX === 'clip') { clip.left -= margin * scaleX; clip.right += margin * scaleX; }
        if (style.overflowY === 'clip') { clip.top -= margin * scaleY; clip.bottom += margin * scaleY; }
        if (rects.some(rect => isClipped(rect, clip, clipX, clipY))) {
          const clipped = { left: 0, right: 0, top: 0, bottom: 0 };
          let firstClipped = -1;
          let lastClipped = -1;
          // Measure glyph ranges only for suspect text nodes. Whitespace alone
          // beyond the edge must not make an otherwise readable slide fail.
          for (let offset = start; offset < end;) {
            const char = String.fromCodePoint(node.data.codePointAt(offset));
            range.setStart(node, offset);
            range.setEnd(node, offset + char.length);
            offset += char.length;
            if (!char.trim()) continue;
            let hidden = false;
            for (const glyph of range.getClientRects()) {
              if (!isClipped(glyph, clip, clipX, clipY)) continue;
              hidden = true;
              const amount = excess(glyph, clip, clipX, clipY);
              for (const side of Object.keys(clipped)) clipped[side] = Math.max(clipped[side], amount[side]);
            }
            if (hidden) {
              if (firstClipped < 0) firstClipped = offset - char.length;
              lastClipped = offset;
            }
          }
          const directions = Object.keys(clipped).filter(side => clipped[side] > 1);
          if (directions.length) {
            text_clipping.push({
              element: describe(element), container: describe(ancestor),
              text: node.data.slice(firstClipped, lastClipped).replace(/\s+/g, ' ').trim().slice(0, 120),
              directions,
              clipped_px: Object.fromEntries(directions.map(side => [side, Math.round(clipped[side] * 10) / 10])),
              clip_rect: edges(clip),
            });
            break; // Report the nearest offending container once per text node.
          }
        }
      }
      if (style.position === 'absolute' || style.position === 'fixed') positioned = style.position;
    }
  }
  return {
    content_size: { width: scrollWidth, height: scrollHeight },
    overflow: { horizontal: scrollWidth > window.innerWidth + 1, vertical: scrollHeight > window.innerHeight + 1 },
    out_of_bounds, text_clipping,
  };
}
