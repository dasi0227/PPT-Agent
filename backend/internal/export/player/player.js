(() => {
  const slides = window.__PPT_SLIDES__ || [];
  const stage = document.getElementById('stage');
  const canvas = document.getElementById('canvas');
  let frame = document.getElementById('slide');
  let index = -1;
  function resize() {
    const scale = Math.min(stage.clientWidth / 1920, stage.clientHeight / 1080);
    canvas.style.transform = 'translate(' + ((stage.clientWidth - 1920 * scale) / 2) + 'px,'
      + ((stage.clientHeight - 1080 * scale) / 2) + 'px) scale(' + scale + ')';
  }
  function show(value) {
    if (!slides.length) return;
    const target = Math.max(0, Math.min(slides.length - 1, value));
    if (target === index) return;
    index = target;
    const slide = slides[index];
    // New browsing context per page: late messages from the old page are ignored.
    const replacement = document.createElement('iframe');
    replacement.id = 'slide';
    replacement.title = '第 ' + (index + 1) + ' 页';
    replacement.setAttribute('sandbox', 'allow-scripts');
    replacement.src = slide.src;
    frame.replaceWith(replacement);
    frame = replacement;
    window.PPTDecorations.render(canvas, slide.frame);
  }
  function navigate(key) {
    if (key === 'ArrowLeft') show(index - 1);
    else if (key === 'ArrowRight' || key === ' ') show(index + 1);
    else if (key === 'Home') show(0);
    else if (key === 'End') show(slides.length - 1);
    else return false;
    return true;
  }
  function interactive(target) {
    return target instanceof Element && target.closest('input,textarea,select,button,a,[contenteditable]:not([contenteditable="false"]),[role="textbox"]');
  }
  addEventListener('fullscreenchange', resize);
  addEventListener('resize', resize);
  addEventListener('keydown', event => {
    if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey || interactive(event.target)) return;
    if (navigate(event.key)) event.preventDefault();
  });
  addEventListener('message', event => {
    if (event.source !== frame.contentWindow || !event.data || event.data.type !== 'ppt-player-input') return;
    if (typeof event.data.key === 'string') navigate(event.data.key);
  });
  resize();
  show(0);
})();
