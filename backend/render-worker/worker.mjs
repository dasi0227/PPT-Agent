import { createServer } from 'node:http';
import { promises as fs } from 'node:fs';
import { extname, normalize, relative, resolve, sep } from 'node:path';
import process from 'node:process';
import readline from 'node:readline';
import { chromium } from 'playwright-core';

const MAX_INPUT_BYTES = 3 * 1024 * 1024;
const MAX_CLIPPING_ITEMS = 50;
const CHROME_CANDIDATES = process.platform === 'darwin'
  ? [
      '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
      '/Applications/Chromium.app/Contents/MacOS/Chromium',
      '/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge',
    ]
  : [
      '/usr/bin/google-chrome',
      '/usr/bin/chromium',
      '/usr/bin/chromium-browser',
      '/usr/bin/microsoft-edge',
    ];

async function executablePath() {
  const configured = process.env.PPT_CHROMIUM_PATH;
  const candidates = configured ? [configured, ...CHROME_CANDIDATES] : CHROME_CANDIDATES;
  for (const candidate of candidates) {
    try {
      await fs.access(candidate);
      return candidate;
    } catch {
      // Continue through fixed product-approved browser candidates.
    }
  }
  throw new Error('Chromium executable not found; set PPT_CHROMIUM_PATH');
}

async function health() {
  const path = await executablePath();
  const browser = await chromium.launch({
    executablePath: path,
    headless: true,
    args: ['--disable-background-networking', '--disable-component-update', '--no-first-run'],
  });
  await browser.close();
  return { ok: true, browser: 'chromium' };
}

function mime(path) {
  return {
    '.html': 'text/html; charset=utf-8',
    '.css': 'text/css; charset=utf-8',
    '.js': 'text/javascript; charset=utf-8',
    '.json': 'application/json; charset=utf-8',
    '.svg': 'image/svg+xml',
    '.png': 'image/png',
    '.jpg': 'image/jpeg',
    '.jpeg': 'image/jpeg',
    '.webp': 'image/webp',
    '.woff': 'font/woff',
    '.woff2': 'font/woff2',
  }[extname(path).toLowerCase()] ?? 'application/octet-stream';
}

function safeProjectPath(projectDir, urlPath) {
  const decoded = decodeURIComponent(urlPath.split('?')[0]);
  const clean = normalize(decoded).replace(/^([/\\])+/, '');
  const absolute = resolve(projectDir, clean);
  const rel = relative(resolve(projectDir), absolute);
  if (rel === '..' || rel.startsWith(`..${sep}`) || rel.includes('\0')) {
    throw new Error('resource path escapes project');
  }
  return absolute;
}

async function launchBrowser() {
  return chromium.launch({
    executablePath: await executablePath(),
    headless: true,
    args: [
      '--disable-background-networking',
      '--disable-component-update',
      '--disable-default-apps',
      '--disable-domain-reliability',
      '--disable-sync',
      '--metrics-recording-only',
      '--no-first-run',
    ],
  });
}

async function injectRuntimeFrame(page, frame) {
  if (!frame || frame.slide_id === undefined || !Number.isInteger(frame.ordinal) || !Number.isInteger(frame.total)) {
    throw new Error('invalid runtime frame context');
  }
  await page.evaluate(context => {
    document.querySelectorAll('[data-runtime-chrome]').forEach(node => node.remove());
    const positions = {
      'top-left': ['top:3.2%', 'left:3.4%'], 'top-center': ['top:3.2%', 'left:50%', 'transform:translateX(-50%)'],
      'top-right': ['top:3.2%', 'right:3.4%'], 'bottom-left': ['bottom:3.2%', 'left:3.4%'],
      'bottom-center': ['bottom:3.2%', 'left:50%', 'transform:translateX(-50%)'], 'bottom-right': ['bottom:3.2%', 'right:3.4%'],
      'left-edge': ['left:1.5%', 'top:50%', 'transform:translateY(-50%)'], 'right-edge': ['right:1.5%', 'top:50%', 'transform:translateY(-50%)'],
    };
    const chrome = Array.isArray(context.chrome) ? context.chrome : [];
    for (const item of chrome) {
      if (item.type === 'page_number' && context.numbering?.visible !== true) continue;
      const text = item.type === 'page_number' ? String(context.ordinal)
        : item.type === 'section_marker' ? context.section?.title
        : item.type === 'deck_title' ? context.deck_title : '';
      if (!text) continue;
      const node = document.createElement('div');
      node.dataset.runtimeChrome = item.type;
      if (item.type === 'page_number') node.dataset.runtimePageNumber = 'true';
      node.dataset.chromeStyle = item.style || '';
      node.textContent = text;
      if (item.type === 'page_number') node.setAttribute('aria-label', `第 ${context.ordinal} 页，共 ${context.total} 页`);
      node.style.cssText = [
        'position:fixed!important', 'z-index:2147483647!important', 'padding:.2em .45em!important',
        'color:rgba(20,25,35,.58)!important', 'font:500 14px/1.2 ui-monospace,SFMono-Regular,Menlo,monospace!important',
        'letter-spacing:.04em!important', 'pointer-events:none!important', ...(positions[item.placement] || positions['bottom-right']),
      ].join(';');
      document.documentElement.appendChild(node);
    }
  }, frame);
}

async function render(input, browser, handles = new Map()) {
  const started = Date.now();
  if (!input || typeof input.html !== 'string' || typeof input.project_dir !== 'string' ||
      typeof input.slide_id !== 'string' || typeof input.screenshot_path !== 'string') {
    throw new Error('invalid render request');
  }
  const width = input.viewport_width === 1600 ? 1600 : 1600;
  const height = input.viewport_height === 900 ? 900 : 900;
  const timeout = Math.min(Math.max(Number(input.timeout_ms) || 15000, 1000), 20000);
  const slidePath = `/slides/${encodeURIComponent(input.slide_id)}/index.html`;
  const failedResources = [];
  const handle = {
    canceled: false,
    context: undefined,
    server: undefined,
    async cancel() {
      this.canceled = true;
      if (this.context) await this.context.close().catch(() => {});
      if (this.server) await new Promise(resolveClose => this.server.close(resolveClose)).catch(() => {});
    },
  };
  handles.set(input.request_id, handle);
  const server = createServer(async (request, response) => {
    try {
      const path = request.url?.split('?')[0] ?? '/';
      if (path === slidePath) {
        response.writeHead(200, {
          'content-type': 'text/html; charset=utf-8',
          'content-security-policy': "default-src 'self' data: blob:; img-src 'self' data: blob:; font-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'",
          'cache-control': 'no-store',
        });
        response.end(input.html);
        return;
      }
      let absolute;
      let data;
      absolute = safeProjectPath(input.project_dir, path);
      data = await fs.readFile(absolute);
      response.writeHead(200, { 'content-type': mime(absolute), 'cache-control': 'no-store' });
      response.end(data);
    } catch {
      response.writeHead(404);
      response.end('not found');
    }
  });
  handle.server = server;
  await new Promise((accept, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', accept);
  });
  const address = server.address();
  const origin = `http://127.0.0.1:${address.port}`;
  let context;
  try {
    context = await browser.newContext({
      viewport: { width, height },
      deviceScaleFactor: 1,
      javaScriptEnabled: true,
      serviceWorkers: 'block',
    });
    handle.context = context;
    if (handle.canceled) throw new Error('RUN_CANCELED');
    await context.route('**/*', async route => {
      const url = new URL(route.request().url());
      if (url.origin === origin || url.protocol === 'data:' || url.protocol === 'blob:') {
        await route.continue();
      } else {
        failedResources.push(`blocked external resource: ${url.origin}${url.pathname}`);
        await route.abort('blockedbyclient');
      }
    });
    const page = await context.newPage();
    if (handle.canceled) throw new Error('RUN_CANCELED');
    const consoleErrors = [];
    page.on('console', message => {
      if (message.type() === 'error' && consoleErrors.length < 50) {
        consoleErrors.push(message.text().slice(0, 1000));
      }
    });
    page.on('pageerror', error => {
      if (consoleErrors.length < 50) consoleErrors.push(String(error.message).slice(0, 1000));
    });
    page.on('requestfailed', request => {
      const url = request.url();
      if (url.startsWith(origin) && failedResources.length < 50) {
        failedResources.push(`${request.failure()?.errorText ?? 'failed'}: ${url.slice(origin.length)}`);
      }
    });
    page.on('response', response => {
      if (response.status() >= 400 && failedResources.length < 50) {
        failedResources.push(`${response.status()}: ${response.url().slice(origin.length)}`);
      }
    });
    page.setDefaultTimeout(timeout);
    await page.goto(`${origin}${slidePath}`, { waitUntil: 'networkidle', timeout });
    await injectRuntimeFrame(page, input.frame);
    const fontStatus = await page.evaluate(async () => {
      if (!document.fonts) return 'unsupported';
      await document.fonts.ready;
      return document.fonts.status;
    });
    await page.evaluate(() => new Promise(resolveFrame => {
      requestAnimationFrame(() => requestAnimationFrame(resolveFrame));
    }));
    await page.waitForTimeout(100);
    const metrics = await page.evaluate(maxItems => {
      const stage = document.querySelector('.slide-stage') ?? document.documentElement;
      const stageRect = stage.getBoundingClientRect();
      const root = document.documentElement;
      const body = document.body;
      const scrollWidth = Math.max(root.scrollWidth, body?.scrollWidth ?? 0, stage.scrollWidth);
      const scrollHeight = Math.max(root.scrollHeight, body?.scrollHeight ?? 0, stage.scrollHeight);
      const clipping = [];
      for (const element of stage.querySelectorAll('*')) {
        const rect = element.getBoundingClientRect();
        if (rect.width === 0 && rect.height === 0) continue;
        if (rect.left < stageRect.left - 1 || rect.top < stageRect.top - 1 ||
            rect.right > stageRect.right + 1 || rect.bottom > stageRect.bottom + 1) {
          clipping.push({
            tag: element.tagName.toLowerCase(),
            id: element.id || undefined,
            class: typeof element.className === 'string' ? element.className.slice(0, 160) : undefined,
            rect: {
              left: Math.round(rect.left), top: Math.round(rect.top),
              right: Math.round(rect.right), bottom: Math.round(rect.bottom),
            },
          });
          if (clipping.length >= maxItems) break;
        }
      }
      return {
        content_size: { width: scrollWidth, height: scrollHeight },
        overflow: { horizontal: scrollWidth > window.innerWidth + 1, vertical: scrollHeight > window.innerHeight + 1 },
        clipping,
        runtime_chrome: Array.from(document.querySelectorAll('[data-runtime-chrome]')).map(node => node.dataset.runtimeChrome),
      };
    }, MAX_CLIPPING_ITEMS);
    const screenshot = await page.screenshot({
      path: input.screenshot_path,
      type: 'png',
      fullPage: false,
      animations: 'disabled',
      caret: 'hide',
    });
    return {
      screenshot_bytes: screenshot.length,
      content_size: metrics.content_size,
      overflow: metrics.overflow,
      clipping: metrics.clipping,
      runtime_chrome: metrics.runtime_chrome,
      console_errors: consoleErrors,
      failed_resources: [...new Set(failedResources)].slice(0, 50),
      font_status: fontStatus,
      duration_ms: Date.now() - started,
    };
  } finally {
    handles.delete(input.request_id);
    if (context) await context.close().catch(() => {});
    await new Promise(resolveClose => server.close(resolveClose)).catch(() => {});
  }
}

async function readInput() {
  const chunks = [];
  let size = 0;
  for await (const chunk of process.stdin) {
    size += chunk.length;
    if (size > MAX_INPUT_BYTES) throw new Error('render input exceeds limit');
    chunks.push(chunk);
  }
  return JSON.parse(Buffer.concat(chunks).toString('utf8'));
}

async function serve() {
  const browser = await launchBrowser();
  process.stdout.write(`${JSON.stringify({ type: 'ready', ok: true })}\n`);
  const active = new Set();
  const handles = new Map();
  const close = async () => {
    await Promise.allSettled([...active]);
    await browser.close().catch(() => {});
  };
  process.once('SIGTERM', () => close().finally(() => process.exit(0)));
  process.once('SIGINT', () => close().finally(() => process.exit(0)));
  const lines = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
  for await (const line of lines) {
    if (Buffer.byteLength(line) > MAX_INPUT_BYTES) {
      process.stdout.write(`${JSON.stringify({ ok: false, error: 'render input exceeds limit' })}\n`);
      continue;
    }
    let request;
    try {
      request = JSON.parse(line);
    } catch {
      process.stdout.write(`${JSON.stringify({ ok: false, error: 'invalid render request JSON' })}\n`);
      continue;
    }
    if (request.type === 'cancel') {
      const handle = handles.get(request.request_id);
      if (handle) await handle.cancel();
      process.stdout.write(`${JSON.stringify({
        type: 'canceled', request_id: request.request_id, ok: false, error: 'RUN_CANCELED',
      })}\n`);
      continue;
    }
    if (request.type !== 'render') {
      process.stdout.write(`${JSON.stringify({ request_id: request.request_id, ok: false, error: 'invalid worker command' })}\n`);
      continue;
    }
    const task = render(request, browser, handles)
      .then(diagnostics => ({ request_id: request.request_id, ok: true, diagnostics }))
      .catch(error => ({ request_id: request.request_id, ok: false, error: String(error?.message ?? error) }))
      .then(response => process.stdout.write(`${JSON.stringify(response)}\n`));
    active.add(task);
    task.finally(() => active.delete(task));
  }
  await close();
}

try {
  if (process.argv.includes('--health')) {
    process.stdout.write(JSON.stringify(await health()));
  } else if (process.argv.includes('--serve')) {
    await serve();
  } else {
    const browser = await launchBrowser();
    try {
      const diagnostics = await render(await readInput(), browser);
      process.stdout.write(JSON.stringify({ ok: true, diagnostics }));
    } finally {
      await browser.close();
    }
  }
} catch (error) {
  process.stdout.write(JSON.stringify({ ok: false, error: String(error?.message ?? error) }));
  process.exitCode = 1;
}
