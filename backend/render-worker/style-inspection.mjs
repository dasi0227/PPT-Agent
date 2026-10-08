// Serialized by Playwright into the existing slide frame. No extra page or
// browser session is created; all measurements describe the rendered source.
export function inspectStyles(selectors) {
  const identity = element => ({
    tag: element.tagName.toLowerCase(),
    id: element.id || '',
    class: (element.getAttribute('class') || '').slice(0, 160),
  });
  return selectors.map(selector => {
    let matches;
    try {
      matches = document.querySelectorAll(selector);
    } catch (error) {
      return { selector, match_count: 0, elements: [], error: String(error.message).slice(0, 300) };
    }
    const elements = Array.from(matches).slice(0, 3).map(element => {
      const style = getComputedStyle(element);
      const background_layers = [];
      for (let node = element; node && background_layers.length < 8; node = node.parentElement) {
        const layer = getComputedStyle(node);
        if ((layer.backgroundColor && layer.backgroundColor !== 'transparent' && layer.backgroundColor !== 'rgba(0, 0, 0, 0)') ||
            (layer.backgroundImage && layer.backgroundImage !== 'none')) {
          background_layers.push({ ...identity(node), color: layer.backgroundColor, image: layer.backgroundImage, opacity: layer.opacity });
        }
      }
      return {
        ...identity(element),
        computed: {
          color: style.color, background_color: style.backgroundColor, background_image: style.backgroundImage,
          font_size: style.fontSize, line_height: style.lineHeight, display: style.display,
          visibility: style.visibility, opacity: style.opacity, fill: style.fill, stroke: style.stroke,
        },
        background_layers,
      };
    });
    return { selector, match_count: matches.length, elements };
  });
}
