// fx: counter-up —— 数字滚动（CSS/JS 驱动）。遵循 init/cleanup 契约（ASSET-005）。
// 挂载约定：目标元素 data-animate="counter-up"，可选 data-target 指定终值（默认取文本数值）。
export function init(root) {
  const nodes = root.querySelectorAll('[data-animate="counter-up"]');
  const rafs = [];
  nodes.forEach((el) => {
    const target = Number(el.dataset.target || el.textContent.replace(/[^0-9.]/g, "")) || 0;
    const duration = 800;
    const start = performance.now();
    const suffix = el.textContent.replace(/[0-9.\s]/g, "");
    const step = (now) => {
      const p = Math.min((now - start) / duration, 1);
      el.textContent = Math.round(target * p) + suffix;
      if (p < 1) {
        rafs.push(requestAnimationFrame(step));
      }
    };
    rafs.push(requestAnimationFrame(step));
  });
  return () => {
    rafs.forEach(cancelAnimationFrame);
  };
}

export function cleanup() {
  // counter-up 无遗留 DOM 样式；RAF 由 init 返回的清理函数取消。
}
