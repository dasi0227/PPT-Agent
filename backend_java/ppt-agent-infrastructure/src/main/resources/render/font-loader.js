// Prepare a fresh FontFace set before committing appearance. Failed faces are
// never installed, and retry creates new faces instead of reusing rejected ones.
(() => {
  const root = new URL('.', document.currentScript.src);
  let prepared = null;
  let attempt = 0;
  async function prepare() {
    if (prepared) return prepared;
    const currentAttempt = attempt++;
    const controller = new AbortController();
    let timer;
    const loading = (async () => {
      const response = await fetch(new URL('fonts.css', root), { cache: 'no-cache', signal: controller.signal });
      if (!response.ok) throw new Error('本地字体样式加载失败');
      const css = await response.text();
      // This parser consumes only our embedded fonts.css, not arbitrary theme CSS.
      const faces = [...css.matchAll(/@font-face\s*\{([^}]+)\}/g)].map(([, rule]) => {
        const family = rule.match(/font-family\s*:\s*"([^"]+)"/)?.[1];
        const source = rule.match(/src\s*:\s*url\("([^"]+)"\)/)?.[1];
        const weight = rule.match(/font-weight\s*:\s*([^;]+)/)?.[1]?.trim();
        if (!family || !source || !weight) throw new Error('本地字体描述无效');
        const url = new URL(source, root);
        if (currentAttempt) url.searchParams.set('retry', String(currentAttempt));
        return new FontFace(family, `url("${url.href}")`, { weight, style: 'normal', display: 'block' });
      });
      if (!faces.length) throw new Error('未找到本地字体');
      await Promise.all(faces.map(face => face.load()));
      return faces;
    })();
    const pending = Promise.race([loading, new Promise((_, reject) => {
      timer = setTimeout(() => { controller.abort(); reject(new Error('本地字体加载超时')); }, 30000);
    })]).then(faces => {
      faces.forEach(face => document.fonts.add(face));
    }).catch(error => {
      if (prepared === pending) prepared = null;
      throw error;
    }).finally(() => clearTimeout(timer));
    prepared = pending;
    return pending;
  }
  window.PPTFonts = { prepare };
})();
