import { createServer } from 'node:http';
import { promises as fs } from 'node:fs';
import { extname, normalize, relative, resolve, sep } from 'node:path';
import process from 'node:process';
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

async function render(input) {
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
      const absolute = safeProjectPath(input.project_dir, path);
      const data = await fs.readFile(absolute);
      response.writeHead(200, { 'content-type': mime(absolute), 'cache-control': 'no-store' });
      response.end(data);
    } catch {
      response.writeHead(404);
      response.end('not found');
    }
  });
  await new Promise((accept, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', accept);
  });
  const address = server.address();
  const origin = `http://127.0.0.1:${address.port}`;
  let browser;
  try {
    browser = await chromium.launch({
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
    const context = await browser.newContext({
      viewport: { width, height },
      deviceScaleFactor: 1,
      javaScriptEnabled: true,
      serviceWorkers: 'block',
    });
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
      };
    }, MAX_CLIPPING_ITEMS);
    const screenshot = await page.screenshot({
      path: input.screenshot_path,
      type: 'png',
      fullPage: false,
      animations: 'disabled',
      caret: 'hide',
    });
    await context.close();
    return {
      screenshot_bytes: screenshot.length,
      content_size: metrics.content_size,
      overflow: metrics.overflow,
      clipping: metrics.clipping,
      console_errors: consoleErrors,
      failed_resources: [...new Set(failedResources)].slice(0, 50),
      font_status: fontStatus,
      duration_ms: Date.now() - started,
    };
  } finally {
    if (browser) await browser.close().catch(() => {});
    await new Promise(resolveClose => server.close(resolveClose));
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

try {
  if (process.argv.includes('--health')) {
    process.stdout.write(JSON.stringify(await health()));
  } else {
    const diagnostics = await render(await readInput());
    process.stdout.write(JSON.stringify({ ok: true, diagnostics }));
  }
} catch (error) {
  process.stdout.write(JSON.stringify({ ok: false, error: String(error?.message ?? error) }));
  process.exitCode = 1;
}
