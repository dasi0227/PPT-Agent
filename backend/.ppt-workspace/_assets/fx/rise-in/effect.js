// fx: rise-in —— 上浮淡入（CSS 动画驱动）。遵循 init/cleanup 契约（ASSET-005）。
// 挂载约定：目标元素 data-animate="rise-in"。
export function init(root) {
  const nodes = root.querySelectorAll('[data-animate="rise-in"]');
  nodes.forEach((el) => {
    el.style.transition = "opacity 500ms ease, transform 500ms ease";
    el.style.opacity = "0";
    el.style.transform = "translateY(24px)";
    requestAnimationFrame(() => requestAnimationFrame(() => {
      el.style.opacity = "1";
      el.style.transform = "translateY(0)";
    }));
  });
  return () => cleanup(root);
}

export function cleanup(root) {
  const nodes = root.querySelectorAll('[data-animate="rise-in"]');
  nodes.forEach((el) => {
    el.style.transition = "";
    el.style.opacity = "";
    el.style.transform = "";
  });
}
