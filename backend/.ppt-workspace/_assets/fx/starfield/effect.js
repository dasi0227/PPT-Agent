// fx: starfield —— 星空（Canvas FX）。遵循 init/cleanup 契约（ASSET-005 / DS-ANIM-003）。
// 挂载约定：容器 data-fx="starfield"；配色读 token（DS-ANIM-004），页离开清理 RAF。
export function init(root) {
  const host = root.querySelector('[data-fx="starfield"]') || root;
  const canvas = document.createElement("canvas");
  canvas.width = host.clientWidth || 800;
  canvas.height = host.clientHeight || 450;
  canvas.style.position = "absolute";
  canvas.style.inset = "0";
  canvas.style.pointerEvents = "none";
  host.appendChild(canvas);

  const ctx = canvas.getContext("2d");
  const color = getComputedStyle(root).getPropertyValue("--color-fg").trim() || "#c0caf5";
  const stars = Array.from({ length: 120 }, () => ({
    x: Math.random() * canvas.width,
    y: Math.random() * canvas.height,
    z: Math.random() * 1.5 + 0.5,
  }));

  let raf = 0;
  const tick = () => {
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.fillStyle = color;
    stars.forEach((s) => {
      s.y += s.z * 0.4;
      if (s.y > canvas.height) {
        s.y = 0;
        s.x = Math.random() * canvas.width;
      }
      ctx.globalAlpha = s.z / 2;
      ctx.fillRect(s.x, s.y, s.z, s.z);
    });
    ctx.globalAlpha = 1;
    raf = requestAnimationFrame(tick);
  };
  raf = requestAnimationFrame(tick);

  return () => {
    cancelAnimationFrame(raf);
    canvas.remove();
  };
}

export function cleanup() {
  // 清理由 init 返回的函数完成（取消 RAF、移除 canvas）。
}
