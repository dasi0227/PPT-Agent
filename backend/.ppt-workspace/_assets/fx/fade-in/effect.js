// fx: fade-in —— 淡入（CSS 动画驱动）。遵循 init/cleanup 契约（ASSET-005 / DS-ANIM-001）。
// 挂载约定：目标元素 data-animate="fade-in"，进入时添加 is-animating 触发过渡。
export function init(root) {
  const nodes = root.querySelectorAll('[data-animate="fade-in"]');
  nodes.forEach((el) => {
    el.style.transition = "opacity 400ms ease";
    el.style.opacity = "0";
    // 双 rAF 确保初始态先绘制再过渡。
    requestAnimationFrame(() => requestAnimationFrame(() => {
      el.style.opacity = "1";
    }));
  });
  return () => cleanup(root);
}

export function cleanup(root) {
  const nodes = root.querySelectorAll('[data-animate="fade-in"]');
  nodes.forEach((el) => {
    el.style.transition = "";
    el.style.opacity = "";
  });
}
