// fx: particle-burst —— 粒子迸发（Canvas FX）。遵循 init/cleanup 契约（ASSET-005 / DS-ANIM-003）。
// 挂载约定：容器 data-fx="particle-burst"；配色读 token（DS-ANIM-004），页离开清理 RAF。
export function init(root) {
  const host = root.querySelector('[data-fx="particle-burst"]') || root;
  const canvas = document.createElement("canvas");
  canvas.width = host.clientWidth || 800;
  canvas.height = host.clientHeight || 450;
  canvas.style.position = "absolute";
  canvas.style.inset = "0";
  canvas.style.pointerEvents = "none";
  host.appendChild(canvas);

  const ctx = canvas.getContext("2d");
  const color = getComputedStyle(root).getPropertyValue("--color-primary").trim() || "#7aa2f7";
  const particles = Array.from({ length: 60 }, () => ({
    x: canvas.width / 2,
    y: canvas.height / 2,
    vx: (Math.random() - 0.5) * 8,
    vy: (Math.random() - 0.5) * 8,
    life: 1,
  }));

  let raf = 0;
  const tick = () => {
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.fillStyle = color;
    let alive = false;
    particles.forEach((p) => {
      p.x += p.vx;
      p.y += p.vy;
      p.life -= 0.015;
      if (p.life > 0) {
        alive = true;
        ctx.globalAlpha = Math.max(p.life, 0);
        ctx.beginPath();
        ctx.arc(p.x, p.y, 4, 0, Math.PI * 2);
        ctx.fill();
      }
    });
    ctx.globalAlpha = 1;
    if (alive) {
      raf = requestAnimationFrame(tick);
    }
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
