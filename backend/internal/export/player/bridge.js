// The sandboxed slide cannot access the player DOM. Forward only playback input.
(() => {
  addEventListener('keydown', event => {
    if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey) return;
    if (event.target instanceof Element && event.target.closest('input,textarea,select,button,a,[contenteditable]:not([contenteditable="false"]),[role="textbox"]')) return;
    if (!['ArrowLeft', 'ArrowRight', ' ', 'Home', 'End'].includes(event.key)) return;
    event.preventDefault();
    parent.postMessage({ type: 'ppt-player-input', key: event.key }, '*');
  });
})();
