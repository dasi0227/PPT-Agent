// fx: stagger-list —— 列表逐项进入（CSS 动画驱动）。遵循 init/cleanup 契约（ASSET-005）。
// 挂载约定：容器 data-animate="stagger-list"，其直接子项依次上浮淡入。
export function init(root) {
  const containers = root.querySelectorAll('[data-animate="stagger-list"]');
  const timers = [];
  containers.forEach((container) => {
    const items = Array.from(container.children);
    items.forEach((el, i) => {
      el.style.transition = "opacity 400ms ease, transform 400ms ease";
      el.style.opacity = "0";
      el.style.transform = "translateY(16px)";
      const t = setTimeout(() => {
        el.style.opacity = "1";
        el.style.transform = "translateY(0)";
      }, 80 * i);
      timers.push(t);
    });
  });
  return () => {
    timers.forEach(clearTimeout);
    cleanup(root);
  };
}

export function cleanup(root) {
  const containers = root.querySelectorAll('[data-animate="stagger-list"]');
  containers.forEach((container) => {
    Array.from(container.children).forEach((el) => {
      el.style.transition = "";
      el.style.opacity = "";
      el.style.transform = "";
    });
  });
}
