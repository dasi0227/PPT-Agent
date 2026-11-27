// Resource-only updates: never replace the document or restart its scripts/animations.
(() => {
  const slideID = document.currentScript.dataset.slideId;
  let latest = 0;
  let pending = [];
  let visible = document.documentElement.dataset.previewActive !== 'false';
  const pausedMedia = new Set();
  const pausedAnimations = new Set();
  function setVisibility(active) {
    visible = active;
    document.documentElement.dataset.previewActive = String(active);
    if (!active) {
      for (const animation of document.getAnimations?.() || []) {
        if (animation.playState === 'running') { pausedAnimations.add(animation); animation.pause(); }
      }
      document.querySelectorAll('video,audio').forEach(media => {
        if (!media.paused) { pausedMedia.add(media); media.pause(); }
      });
    } else {
      pausedAnimations.forEach(animation => animation.play());
      pausedAnimations.clear();
      pausedMedia.forEach(media => { media.play()?.catch(() => {}); });
      pausedMedia.clear();
    }
  }
  document.addEventListener('play', event => {
    if (!visible && event.target instanceof HTMLMediaElement) {
      pausedMedia.add(event.target);
      event.target.pause();
    }
  }, true);

  function send(type, detail) {
    parent.postMessage({ bridge: 'ppt-theme-v1', slide_id: slideID, type, ...detail }, '*');
  }
  function cancelPending() {
    pending.forEach(link => link.remove());
    pending = [];
  }
  function waitForLink(link) {
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => done(new Error('幻灯片样式加载超时')), 30000);
      const done = error => {
        clearTimeout(timer);
        link.onload = null;
        link.onerror = null;
        error ? reject(error) : resolve();
      };
      link.onload = () => done();
      link.onerror = () => done(new Error('幻灯片样式加载失败'));
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
    const { request_id: request, theme_id: themeId, appearance } = message;
    const themeUrl = themeId === null ? null : appearance?.theme_css_url;
    latest = request;
    cancelPending();
    if (themeId !== null && !themeUrl) {
      send('themeApplyFailed', { request_id: request, message: '主题不可用，请选择现有主题' });
      return;
    }
    const appearanceHash = themeId === null ? '' : appearance.hash;
    const previous = document.getElementById('theme-link');
    const previousBase = document.getElementById('base-link');
    const currentHash = document.documentElement.dataset.appearance;
    const reuse = previousBase?.sheet && (currentHash === undefined || currentHash === appearanceHash)
      && (themeUrl ? previous?.sheet && previous.href === new URL(themeUrl, document.baseURI).href : !previous);
    const staged = [];
    if (!reuse) {
      staged.push(stagedLink('/api/runtime/base.css'));
      if (themeUrl) staged.push(stagedLink(themeUrl));
    }
    pending = staged;
    try {
      const ready = Promise.all(staged.map(waitForLink));
      document.head.prepend(...staged);
      await ready;
      if (request !== latest) {
        staged.forEach(link => link.remove());
        return;
      }
      // One synchronous commit; page CSS remains last in the cascade.
      if (!reuse) {
        previousBase?.remove();
        previous?.remove();
        const [base, theme] = staged;
        base.id = 'base-link';
        if (theme) theme.id = 'theme-link';
        staged.forEach(link => { link.media = 'all'; });
      }
      pending = [];
      if (themeId === null) delete document.documentElement.dataset.theme;
      else document.documentElement.dataset.theme = themeId;
      document.documentElement.dataset.appearance = appearanceHash;
      window.dispatchEvent(new CustomEvent('ppt:themechange', {
        detail: { themeId, appearanceHash },
      }));
      send('themeApplied', { request_id: request, appearance_hash: appearanceHash });
    } catch (error) {
      staged.forEach(link => link.remove());
      if (request === latest) {
        pending = [];
        send('themeApplyFailed', { request_id: request, message: error.message || '主题资源加载失败' });
      }
    }
  }
  window.addEventListener('message', event => {
    const message = event.data;
    if (event.source !== parent || message?.bridge !== 'ppt-theme-v1' || message.slide_id !== slideID) return;
    if (message.type === 'setPreviewVisibility' && typeof message.active === 'boolean') {
      setVisibility(message.active);
    } else if (message.type === 'cancelTheme') {
      latest = message.request_id;
      cancelPending();
    } else if (message.type === 'applyTheme' && Number.isInteger(message.request_id)) {
      void apply(message);
    }
  });
  window.addEventListener('load', () => send('themeBridgeReady', {}), { once: true });
})();
