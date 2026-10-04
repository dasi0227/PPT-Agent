const { app, BrowserWindow, dialog, Menu, session, shell } = require('electron');
const fs = require('node:fs');
const fsp = require('node:fs/promises');
const path = require('node:path');
const { BackendProcess } = require('./runtime.cjs');

const origin = 'http://127.0.0.1:8787';
app.setName('PPT-Agent');
app.setPath('userData', path.join(app.getPath('appData'), 'PPT-Agent'));
const ownsInstance = app.requestSingleInstanceLock();
let mainWindow;
let backend;
let initializer;
let backendLog;
let quitting = false;
let stopped = false;
let ready = false;

function showWindow() {
  if (!ready || quitting || !mainWindow || mainWindow.isDestroyed()) return;
  if (mainWindow.isMinimized()) mainWindow.restore();
  mainWindow.show();
  mainWindow.focus();
}

function allowedNavigation(url) {
  try { return new URL(url).origin === origin; } catch { return false; }
}

function openExternal(url) {
  try {
    if (['https:', 'http:'].includes(new URL(url).protocol) && !allowedNavigation(url)) {
      void shell.openExternal(url).catch(() => {});
    }
  } catch { /* Never forward unknown schemes to the operating system. */ }
}

async function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1440, height: 960, minWidth: 960, minHeight: 640,
    title: 'PPT-Agent', backgroundColor: '#ffffff', show: false,
    webPreferences: {
      nodeIntegration: false, nodeIntegrationInWorker: false,
      nodeIntegrationInSubFrames: false, contextIsolation: true,
      sandbox: true, webSecurity: true, webviewTag: false,
      spellcheck: false,
    },
  });
  mainWindow.on('close', event => {
    if (!quitting) {
      event.preventDefault();
      mainWindow.hide();
    }
  });
  mainWindow.webContents.on('will-navigate', (event, url) => {
    if (!allowedNavigation(url)) {
      event.preventDefault();
      openExternal(url);
    }
  });
  mainWindow.webContents.on('will-redirect', (event, url) => {
    if (!allowedNavigation(url)) event.preventDefault();
  });
  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    openExternal(url);
    return { action: 'deny' };
  });
  mainWindow.webContents.on('will-attach-webview', event => event.preventDefault());
  await mainWindow.loadURL(origin);
  ready = true;
  showWindow();
}

async function prepareConfig(userData) {
  const destination = path.join(userData, 'config.yaml');
  try { await fsp.access(destination); return; } catch { /* First launch. */ }
  const result = await dialog.showOpenDialog({
    title: '选择模型配置', properties: ['openFile'],
    filters: [{ name: 'YAML', extensions: ['yaml', 'yml'] }],
  });
  if (result.canceled || !result.filePaths[0]) throw new Error('未选择模型配置');
  await fsp.copyFile(result.filePaths[0], destination, fs.constants.COPYFILE_EXCL);
  await fsp.chmod(destination, 0o600);
}

async function initializeResources(resources, workRoot, env) {
  const exists = async file => {
    try { await fsp.access(file); return true; }
    catch (error) { if (error.code === 'ENOENT') return false; throw error; }
  };
  const pending = path.join(workRoot, '.desktop-initializing');
  if (await exists(path.join(workRoot, 'db', 'ppt.db')) && !await exists(pending)) return;
  await fsp.mkdir(workRoot, { recursive: true, mode: 0o700 });
  // Database creation precedes resource registration. Keep a marker so an
  // interrupted first launch retries the idempotent initializer on the next launch.
  await fsp.writeFile(pending, '', { mode: 0o600 });
  if (quitting) return;
  initializer = new BackendProcess({
    executable: path.join(resources, 'bin', 'init-resources'),
    args: ['--work-root', workRoot, '--seed-root', path.join(resources, 'seed')],
    cwd: app.getPath('userData'), env, log: backendLog,
  });
  try {
    const result = await initializer.completion;
    if (result.error) throw result.error;
    if (result.code !== 0) throw new Error(initializer.lastError.trim() || `资源初始化失败（退出码 ${result.code ?? result.signal}）`);
    await fsp.unlink(pending);
  } finally {
    initializer = undefined;
  }
}

async function start() {
  const userData = app.getPath('userData');
  await fsp.mkdir(userData, { recursive: true, mode: 0o700 });
  app.setAppLogsPath();
  const logPath = path.join(app.getPath('logs'), 'backend.log');
  await fsp.mkdir(path.dirname(logPath), { recursive: true });
  try {
    if ((await fsp.stat(logPath)).size > 5 * 1024 * 1024) await fsp.rename(logPath, `${logPath}.previous`);
  } catch (error) { if (error.code !== 'ENOENT') throw error; }
  backendLog = fs.createWriteStream(logPath, { flags: 'a', mode: 0o600 });
  backendLog.on('error', error => console.error('Backend log:', error.message));
  await prepareConfig(userData);
  if (quitting) return;

  const resources = app.isPackaged ? path.join(process.resourcesPath, 'runtime') : path.join(__dirname, '.build', 'runtime');
  const workRoot = path.join(app.getPath('home'), '.dasi', 'ppt');
  const env = {
    ...process.env, PORT: '8787',
    PATH: ['/opt/homebrew/bin', '/usr/local/bin', '/usr/bin', '/bin', '/usr/sbin', '/sbin', process.env.PATH ?? ''].join(':'),
    PPT_RENDER_NODE: process.execPath,
    PPT_RENDER_WORKER: path.join(resources, 'render-worker', 'worker.mjs'),
    PPT_RENDER_ELECTRON: '1',
  };
  // Development-only Node injection flags must not leak into the worker.
  delete env.NODE_OPTIONS;
  delete env.NODE_PATH;
  await initializeResources(resources, workRoot, env);
  if (quitting) return;
  backend = new BackendProcess({
    executable: path.join(resources, 'bin', 'ppt-agent-server'),
    args: ['--desktop', '--work-root', workRoot, '--frontend-dir', path.join(resources, 'frontend')],
    cwd: userData, env, log: backendLog,
  });
  await backend.waitReady(origin);
  if (quitting) return;
  backend.completion.then(() => {
    if (quitting) return;
    dialog.showErrorBox('PPT-Agent 后台服务已停止', `日志：${logPath}`);
    app.quit();
  });

  session.defaultSession.setPermissionRequestHandler((contents, permission, callback, details) => {
    callback(contents === mainWindow?.webContents && details.isMainFrame &&
      allowedNavigation(details.requestingUrl) && ['clipboard-read', 'clipboard-sanitized-write', 'fullscreen'].includes(permission));
  });
  session.defaultSession.setPermissionCheckHandler((contents, permission, requestingOrigin, details) =>
    contents === mainWindow?.webContents && details.isMainFrame && requestingOrigin === origin &&
    ['clipboard-read', 'clipboard-sanitized-write', 'fullscreen'].includes(permission));
  session.defaultSession.on('will-download', (event, item, contents) => {
    if (contents !== mainWindow?.webContents || !allowedNavigation(item.getURL())) {
      event.preventDefault();
      return;
    }
    item.setSaveDialogOptions({ defaultPath: path.join(app.getPath('downloads'), path.basename(item.getFilename())) });
  });
  Menu.setApplicationMenu(Menu.buildFromTemplate([
    { role: 'appMenu' },
    { role: 'fileMenu', submenu: [
      { label: '关闭窗口', accelerator: 'CmdOrCtrl+W', click: () => mainWindow?.close() },
    ] },
    { role: 'editMenu' },
    { role: 'viewMenu' }, { role: 'windowMenu' },
  ]));
  await createWindow();
}

if (!ownsInstance) {
  app.quit();
} else {
  app.on('second-instance', showWindow);
  app.on('activate', showWindow);
  app.on('window-all-closed', () => {});
  app.on('before-quit', event => {
    if (stopped) return;
    event.preventDefault();
    if (quitting) return;
    quitting = true;
    void (async () => {
      await initializer?.stop();
      await backend?.stop();
      await session.defaultSession.cookies.flushStore();
      session.defaultSession.flushStorageData();
      if (backendLog) await new Promise(resolve => backendLog.end(resolve));
    })().catch(error => console.error(error)).finally(() => {
      stopped = true;
      app.quit();
    });
  });
  process.once('SIGTERM', () => app.quit());
  process.once('SIGINT', () => app.quit());
  app.whenReady().then(start).catch(error => {
    if (!quitting) dialog.showErrorBox('PPT-Agent 启动失败', `${error.message}\n\n日志：${path.join(app.getPath('logs'), 'backend.log')}`);
    app.quit();
  });
}
