// Resource-only updates: never replace the document or restart its scripts/animations.
(() => {
  const slideID = document.currentScript.dataset.slideId;
  let latest = 0;
  let pending = [];

  function send(type, detail) {
    parent.postMessage({ bridge: 'ppt-theme-v1', slide_id: slideID, type, ...detail }, '*');
  }
  function cancelPending() {
    pending.forEach(link => link.remove());
    pending = [];
  }
  function waitForLink(link) {
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => done(new Error('主题 CSS 加载超时')), 30000);
      const done = error => {
        clearTimeout(timer);
        link.onload = null;
        link.onerror = null;
        error ? reject(error) : resolve();
      };
      link.onload = () => done();
      link.onerror = () => done(new Error('主题 CSS 加载失败'));
    });
  }
  function stagedLink(href) {
    const link = document.createElement('link');
    link.rel = 'stylesheet';
    link.media = 'not all';
    link.href = href;
    return link;
  }
  async function apply(message) {
    const request = message.request_id;
    latest = request;
    cancelPending();
    if (!message.appearance) {
      send('themeApplyFailed', { request_id: request, message: '主题不可用，请选择现有主题' });
      return;
    }
    const previous = document.getElementById('theme-link');
    const previousBase = document.getElementById('base-link');
    const link = stagedLink(message.appearance.theme_css_url);
    const base = stagedLink('/api/v1/runtime/base.css');
    pending = [base, link];
    const ready = Promise.all(pending.map(waitForLink));
    previousBase.after(base);
    previous.after(link);
    try {
      await ready;
      await window.PPTFonts.prepare();
      if (request !== latest) { base.remove(); link.remove(); return; }
      // One synchronous commit; page CSS remains last in the cascade.
      previousBase.remove();
      previous.remove();
      base.id = 'base-link';
      link.id = 'theme-link';
      base.media = 'all';
      link.media = 'all';
      pending = [];
      document.documentElement.dataset.theme = message.theme_id;
      document.documentElement.dataset.appearance = message.appearance.hash;
      window.dispatchEvent(new CustomEvent('ppt:themechange', {
        detail: { themeId: message.theme_id, appearanceHash: message.appearance.hash },
      }));
      send('themeApplied', { request_id: request, appearance_hash: message.appearance.hash });
    } catch (error) {
      base.remove();
      link.remove();
      if (request === latest) {
        pending = [];
        send('themeApplyFailed', { request_id: request, message: error.message || '主题资源加载失败' });
      }
    }
  }
  window.addEventListener('message', event => {
    const message = event.data;
    if (event.source !== parent || message?.bridge !== 'ppt-theme-v1' || message.slide_id !== slideID) return;
    if (message.type === 'cancelTheme') {
      latest = message.request_id;
      cancelPending();
    } else if (message.type === 'applyTheme' && Number.isInteger(message.request_id)) {
      void apply(message);
    }
  });
  window.addEventListener('load', () => send('themeBridgeReady', {}), { once: true });
})();
