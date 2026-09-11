(() => {
  const slideID = document.currentScript?.dataset.slideId || '';
  const blocked = new Set(['HTML', 'BODY', 'HEAD', 'SCRIPT', 'STYLE', 'NOSCRIPT', 'TEMPLATE']);
  const styleKeys = [
    'display', 'position', 'top', 'right', 'bottom', 'left', 'width', 'height',
    'min-width', 'min-height', 'max-width', 'max-height', 'box-sizing',
    'overflow', 'overflow-x', 'overflow-y', 'flex', 'flex-direction', 'flex-wrap',
    'align-items', 'align-content', 'justify-content', 'justify-items', 'gap',
    'row-gap', 'column-gap', 'grid-template-columns', 'grid-template-rows',
    'grid-column', 'grid-row', 'font-family', 'font-size', 'font-weight',
    'font-style', 'line-height', 'letter-spacing', 'text-align', 'text-transform',
    'color', 'background', 'background-color', 'border', 'border-radius',
    'opacity', 'transform', 'transform-origin', 'clip-path', 'z-index',
  ];
  let mode = 'none';
  let sessionID = '';
  let revision = 0;
  let htmlHash = '';
  let start = null;

  const layer = document.createElement('div');
  layer.dataset.domSelectionBridge = 'true';
  Object.assign(layer.style, {
    position: 'fixed',
    inset: '0',
    pointerEvents: 'none',
    zIndex: '2147483646',
  });

  function mountLayer() {
    if (!layer.isConnected && document.body) document.body.appendChild(layer);
  }

  function makeBox(color, fill) {
    mountLayer();
    const box = document.createElement('div');
    box.dataset.domSelectionBridge = 'true';
    Object.assign(box.style, {
      position: 'absolute', border: `2px solid ${color}`, background: fill,
      boxSizing: 'border-box', pointerEvents: 'none',
    });
    layer.appendChild(box);
    return box;
  }

  function draw(box, rect) {
    box.style.left = `${rect.x}px`;
    box.style.top = `${rect.y}px`;
    box.style.width = `${rect.width}px`;
    box.style.height = `${rect.height}px`;
  }

  function canvasRect(rect) {
    const x = Math.max(0, Math.min(1920, rect.left));
    const y = Math.max(0, Math.min(1080, rect.top));
    const right = Math.max(x, Math.min(1920, rect.right));
    const bottom = Math.max(y, Math.min(1080, rect.bottom));
    return { x, y, width: right - x, height: bottom - y };
  }

  function visibleRect(element) {
    let rect = canvasRect(element.getBoundingClientRect());
    let current = element.parentElement;
    while (current && rect.width > 0 && rect.height > 0) {
      const style = getComputedStyle(current);
      const clipX = ['hidden', 'clip', 'scroll', 'auto'].includes(style.overflowX);
      const clipY = ['hidden', 'clip', 'scroll', 'auto'].includes(style.overflowY);
      if (clipX || clipY) {
        const parent = canvasRect(current.getBoundingClientRect());
        const left = clipX ? Math.max(rect.x, parent.x) : rect.x;
        const top = clipY ? Math.max(rect.y, parent.y) : rect.y;
        const right = clipX ? Math.min(rect.x + rect.width, parent.x + parent.width) : rect.x + rect.width;
        const bottom = clipY ? Math.min(rect.y + rect.height, parent.y + parent.height) : rect.y + rect.height;
        rect = { x: left, y: top, width: Math.max(0, right - left), height: Math.max(0, bottom - top) };
      }
      current = current.parentElement;
    }
    return rect;
  }

  function visible(element) {
    if (!(element instanceof Element) || blocked.has(element.tagName) || element.closest('[data-dom-selection-bridge]')) return false;
    const style = getComputedStyle(element);
    if (style.display === 'none' || style.visibility === 'hidden' || Number(style.opacity) === 0) return false;
    const rect = visibleRect(element);
    return rect.width > 0 && rect.height > 0;
  }

  function clip(value, max) {
    return String(value || '').slice(0, max);
  }

  function safeValue(value) {
    const text = String(value || '');
    const scheme = text.trim().match(/^(data|blob|javascript):/i);
    return scheme ? `[${scheme[1].toLowerCase()}-url-removed]` : clip(text, 512);
  }

  function safeClone(element) {
    const clone = element.cloneNode(true);
    clone.querySelectorAll('script,style,noscript,template').forEach((node) => node.remove());
    [clone, ...clone.querySelectorAll('*')].forEach((node) => {
      [...node.attributes].forEach((attribute) => {
        const name = attribute.name.toLowerCase();
        if (name.startsWith('on') || name === 'srcdoc' || name === 'value' || name === 'checked' || name === 'selected') node.removeAttribute(attribute.name);
        else node.setAttribute(attribute.name, safeValue(attribute.value));
      });
      if (node.tagName === 'INPUT' || node.tagName === 'TEXTAREA' || node.tagName === 'SELECT') node.textContent = '';
    });
    let html = clone.outerHTML;
    const encoder = new TextEncoder();
    const truncated = encoder.encode(html).byteLength > 16384;
    if (truncated) {
      let low = 0;
      let high = html.length;
      while (low < high) {
        const middle = Math.ceil((low + high) / 2);
        if (encoder.encode(html.slice(0, middle)).byteLength <= 16384) low = middle;
        else high = middle - 1;
      }
      html = html.slice(0, low);
    }
    return { html, truncated };
  }

  function siblingIndex(element) {
    if (!element.parentElement) return 0;
    return [...element.parentElement.children].filter((node) => node.tagName === element.tagName).indexOf(element);
  }

  function ancestorSummary(element) {
    const out = [];
    let current = element.parentElement;
    while (current && out.length < 8 && current !== document.body) {
      out.push({ tag: current.tagName.toLowerCase(), stable_id: clip(current.id, 128), class_summary: clip(current.className, 256), sibling_index: siblingIndex(current) });
      current = current.parentElement;
    }
    return out;
  }

  function hashText(value) {
    let hash = 2166136261;
    for (let index = 0; index < value.length; index += 1) hash = Math.imul(hash ^ value.charCodeAt(index), 16777619);
    return `fnv1a:${(hash >>> 0).toString(16)}`;
  }

  function selectors(element) {
    const out = [];
    if (element.id) out.push(`#${CSS.escape(element.id)}`);
    const classes = [...element.classList].slice(0, 3).map((name) => `.${CSS.escape(name)}`).join('');
    if (classes) out.push(`${element.tagName.toLowerCase()}${classes}`);
    out.push(`${element.tagName.toLowerCase()}:nth-of-type(${siblingIndex(element) + 1})`);
    return out.slice(0, 3);
  }

  function edges(style, prefix, suffix) {
    const number = (key) => Number.parseFloat(style.getPropertyValue(key)) || 0;
    return {
      top: number(`${prefix}-top${suffix}`), right: number(`${prefix}-right${suffix}`),
      bottom: number(`${prefix}-bottom${suffix}`), left: number(`${prefix}-left${suffix}`),
    };
  }

  function targetSnapshot(element) {
    const style = getComputedStyle(element);
    const rect = visibleRect(element);
    const padding = edges(style, 'padding', '');
    const border = edges(style, 'border', '-width');
    const margin = edges(style, 'margin', '');
    const isFormControl = ['INPUT', 'TEXTAREA', 'SELECT'].includes(element.tagName);
    const textSummary = isFormControl ? '' : clip(element.textContent?.replace(/\s+/g, ' ').trim(), 1000);
    const ancestors = ancestorSummary(element);
    const classes = [...element.classList].sort().slice(0, 16);
    const attributes = {};
    [...element.attributes].forEach((attribute) => {
      const name = attribute.name.toLowerCase();
      if (name.startsWith('on') || ['srcdoc', 'value', 'checked', 'selected'].includes(name)) return;
      if (['id', 'class', 'role', 'title', 'alt', 'href', 'src'].includes(name) || name.startsWith('aria-') || name.startsWith('data-')) attributes[name] = safeValue(attribute.value);
    });
    const keyAttributes = Object.keys(attributes).sort().map((key) => `${key}=${attributes[key]}`).slice(0, 16);
    const fingerprint = {
      tag: element.tagName.toLowerCase(), stable_id: clip(element.id, 128), classes,
      key_attributes: keyAttributes, sibling_index: siblingIndex(element),
      text_summary_hash: hashText(textSummary), ancestors,
    };
    const computedStyle = {};
    styleKeys.forEach((key) => { const value = style.getPropertyValue(key); if (value) computedStyle[key] = clip(value, 512); });
    const safe = safeClone(element);
    return {
      target_id: `target_${hashText(JSON.stringify(fingerprint)).slice(6)}`,
      fingerprint, candidate_selectors: selectors(element), tag: element.tagName.toLowerCase(),
      attributes, text_summary: textSummary, outer_html: safe.html, outer_html_truncated: safe.truncated,
      ancestors, rect, status: 'active',
      box_model: {
        content: {
          x: rect.x + border.left + padding.left, y: rect.y + border.top + padding.top,
          width: Math.max(0, rect.width - border.left - border.right - padding.left - padding.right),
          height: Math.max(0, rect.height - border.top - border.bottom - padding.top - padding.bottom),
        },
        padding, border, margin,
      },
      computed_style: computedStyle,
    };
  }

  function send(type, detail = {}) {
    window.parent.postMessage({ bridge: 'ppt-dom-selection-v1', type, slide_id: slideID, session_id: sessionID, ...detail }, '*');
  }

  function selection(kind, rect, targets) {
    const fingerprints = targets.map((target) => JSON.stringify(target.fingerprint)).sort().join('|');
    const regionIdentity = kind === 'region' ? `|${Math.round(rect.x)},${Math.round(rect.y)},${Math.round(rect.width)},${Math.round(rect.height)}` : '';
    return {
      selection_id: '', marker_no: 0, kind, comment: '', slide_id: slideID,
      html_revision: revision, html_hash: htmlHash, canvas: { width: 1920, height: 1080 },
      rect, status: 'active', dom_targets: targets, chrome_targets: [],
      dedupe_key: hashText(`${slideID}|${htmlHash}|${kind}${regionIdentity}|${fingerprints}`),
    };
  }

  function emitSelection(kind, rect, elements) {
    if (elements.length > 50) {
      send('innerSelectionRejected', { message: '选择内容过多，请缩小范围。' });
      return false;
    }
    const targets = elements.map(targetSnapshot);
    const ids = new Map(elements.map((element, index) => [element, targets[index].target_id]));
    elements.forEach((element, index) => {
      let parent = element.parentElement;
      while (parent && !ids.has(parent)) parent = parent.parentElement;
      if (parent) targets[index].parent_target_id = ids.get(parent);
    });
    const value = selection(kind, rect, targets);
    const bytes = new TextEncoder().encode(JSON.stringify({ dom_targets: targets, chrome_targets: [] })).byteLength;
    if (bytes > 128 * 1024) {
      send('innerSelectionRejected', { message: '选择内容过大，请缩小范围。' });
      return false;
    }
    send('innerSelectionCreated', { selection: value });
    setMode('none');
    layer.replaceChildren();
    return true;
  }

  function point(event) {
    return { x: Math.max(0, Math.min(1920, event.clientX)), y: Math.max(0, Math.min(1080, event.clientY)) };
  }

  function elementAt(event) {
    layer.style.display = 'none';
    const element = document.elementFromPoint(event.clientX, event.clientY);
    layer.style.display = '';
    return visible(element) ? element : null;
  }

  function setMode(next) {
    mode = next;
    mountLayer();
    layer.style.pointerEvents = next === 'none' ? 'none' : 'auto';
    layer.style.cursor = next === 'none' ? '' : 'crosshair';
  }

  function stop(event) {
    event.preventDefault();
    event.stopImmediatePropagation();
  }

  document.addEventListener('pointermove', (event) => {
    if (mode === 'element') {
      layer.replaceChildren();
      const element = elementAt(event);
      if (element) draw(makeBox('#2563eb', 'rgba(37,99,235,.06)'), visibleRect(element));
    } else if (mode === 'region' && start) {
      const end = point(event);
      const rect = { x: Math.min(start.x, end.x), y: Math.min(start.y, end.y), width: Math.abs(end.x - start.x), height: Math.abs(end.y - start.y) };
      layer.replaceChildren();
      draw(makeBox('#7c3aed', 'rgba(124,58,237,.12)'), rect);
      stop(event);
    }
  }, true);

  document.addEventListener('click', (event) => {
    if (mode !== 'element') return;
    stop(event);
    const element = elementAt(event);
    if (!element) {
      send('innerSelectionEmpty', { message: '未选中有效元素，请重试。' });
      return;
    }
    emitSelection('element', visibleRect(element), [element]);
  }, true);

  document.addEventListener('pointerdown', (event) => {
    if (mode !== 'region' || event.button !== 0) return;
    start = point(event);
    stop(event);
  }, true);

  document.addEventListener('pointerup', (event) => {
    if (mode !== 'region' || !start) return;
    stop(event);
    const end = point(event);
    const rect = { x: Math.min(start.x, end.x), y: Math.min(start.y, end.y), width: Math.abs(end.x - start.x), height: Math.abs(end.y - start.y) };
    start = null;
    layer.replaceChildren();
    if (rect.width < 4 || rect.height < 4) {
      send('innerSelectionEmpty', { message: '未选中任何元素，请重新框选。' });
      return;
    }
    const elements = [...document.querySelectorAll('*')].filter((element) => {
      if (!visible(element)) return false;
      const item = visibleRect(element);
      return item.x >= rect.x && item.y >= rect.y && item.x + item.width <= rect.x + rect.width && item.y + item.height <= rect.y + rect.height;
    });
    if (!elements.length) {
      send('innerSelectionEmpty', { message: '未选中任何元素，请重新框选。' });
      return;
    }
    emitSelection('region', rect, elements);
  }, true);

  function matchesFingerprint(element, fingerprint) {
    if (!element || !fingerprint || element.tagName.toLowerCase() !== fingerprint.tag) return false;
    if (fingerprint.stable_id && element.id !== fingerprint.stable_id) return false;
    if (Array.isArray(fingerprint.classes) && !fingerprint.classes.every((name) => element.classList.contains(name))) return false;
    const text = clip(element.textContent?.replace(/\s+/g, ' ').trim(), 1000);
    return !fingerprint.text_summary_hash || hashText(text) === fingerprint.text_summary_hash;
  }

  function targetPresence(target) {
    let resolvedSelector = false;
    for (const selector of target.candidate_selectors || []) {
      try {
        const matches = [...document.querySelectorAll(selector)].filter((element) => matchesFingerprint(element, target.fingerprint));
        resolvedSelector = true;
        if (matches.length === 1) return 'present';
        if (matches.length > 1) return 'unknown';
      } catch { /* invalid candidate remains unresolved */ }
    }
    return resolvedSelector ? 'missing' : 'unknown';
  }

  window.addEventListener('message', (event) => {
    if (event.source !== window.parent || event.data?.bridge !== 'ppt-dom-selection-v1' || event.data?.slide_id !== slideID) return;
    if (event.data.type === 'innerProbeSelections' && event.data.session_id === sessionID && Array.isArray(event.data.selections)) {
      const statuses = event.data.selections.map((selection) => {
        const targets = (selection.dom_targets || []).map((target) => ({ target_id: target.target_id, status: targetPresence(target) === 'missing' ? 'content_deleted' : 'active' }));
        return { selection_id: selection.selection_id, status: (selection.chrome_targets || []).length > 0 || targets.some((target) => target.status === 'active') ? 'active' : 'content_deleted', targets };
      });
      send('innerSelectionPresence', { statuses });
      return;
    }
    if (event.data.type !== 'innerSetSelectionMode') return;
    if (!['none', 'element', 'region'].includes(event.data.mode) || typeof event.data.session_id !== 'string') return;
    setMode(event.data.mode);
    sessionID = event.data.session_id;
    revision = Number.isInteger(event.data.html_revision) ? event.data.html_revision : 0;
    htmlHash = typeof event.data.html_hash === 'string' ? event.data.html_hash : '';
    start = null;
    layer.replaceChildren();
  });

  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && mode !== 'none') {
      setMode('none');
      start = null;
      layer.replaceChildren();
      send('innerSelectionCanceled');
    }
  }, true);
})();
