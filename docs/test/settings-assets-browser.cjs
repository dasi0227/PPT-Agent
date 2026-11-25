// Run against the disposable services from start-settings-assets.sh.
const assert = require('node:assert/strict');
const { randomUUID } = require('node:crypto');
const path = require('node:path');
const { chromium } = require(require.resolve('playwright', { paths: [
  __dirname, ...process.env.PATH.split(path.delimiter).map(dir => path.dirname(dir)),
  ...(process.env.NODE_PATH || '').split(path.delimiter),
] }));
const origin = 'http://127.0.0.1:' + (process.env.FRONTEND_PORT || '15173');
const prefix = '验证-' + randomUUID().slice(0, 8);
const key = randomUUID(); // disposable, never logged
const button = (page, name) => page.getByRole('button', { name, exact: true });
const dialog = page => page.getByRole('dialog');
const endpoint = response => new URL(response.url()).pathname;
const go = (page, route) => page.goto(origin + route, { waitUntil: 'domcontentloaded' });
async function action(page, route, click) {
  const wait = page.waitForResponse(r => endpoint(r) === '/api' + route && r.request().method() === 'POST');
  await click();
  const response = await wait;
  const raw = await response.text();
  const body = raw && JSON.parse(raw);
  assert.equal(response.status(), 200);
  assert.ok(!body?.error, body?.error?.message);
  return { body, sent: response.request().postDataJSON() };
}
async function get(page, route) {
  const result = await (await page.request.get(origin + '/api' + route)).json();
  assert.ok(!result.error, result.error?.message);
  return result;
}
async function choice(page, label, name) {
  await page.getByRole('combobox', { name: label, exact: true }).click();
  await page.getByRole('option', { name, exact: true }).click();
}
async function settings(page) {
  await go(page, '/');
  await page.getByRole('link', { name: '设置' }).click();
  const original = await get(page, '/settings/models');
  const name = prefix + '-模型';
  await button(page, '添加模型').click();
  let form = page.getByRole('form', { name: '编辑 新模型' });
  await form.getByPlaceholder('配置名称').fill(name);
  await form.getByLabel('API 基础地址').fill('https://example.invalid/v1///');
  await form.getByPlaceholder('模型标识').fill('integration');
  await form.getByPlaceholder('输入 API key').fill(key);
  await button(form, '保存').click();
  await form.getByText('请选择厂商。', { exact: true }).waitFor();
  await choice(page, '厂商', 'gpt');
  const created = await action(page, '/settings/models/create', () => button(form, '保存').click());
  assert.deepEqual(Object.keys(created.body), ['id']);
  const model = await get(page, '/settings/models/' + created.body.id);
  assert.equal(model.provider, 'gpt');
  assert.equal(model.base_url, 'https://example.invalid/v1');
  assert.ok(!('key' in model));
  await button(page, '模型分配').click();
  const routed = await action(page, '/settings/routes/save', () => choice(page, '主路备用模型', name));
  assert.deepEqual(routed.sent, { route: 'main_fallback', model_id: created.body.id });
  await button(page, '模型配置').click();
  await button(page, '编辑 ' + name).click();
  form = page.getByRole('form', { name: '编辑 ' + name, exact: true });
  assert.equal(await form.getByPlaceholder('已配置，留空保留').inputValue(), '');
  await form.getByPlaceholder('配置名称').fill(name + '-改名');
  const updated = await action(page, '/settings/models/update', () => button(form, '保存').click());
  assert.equal(updated.body, '');
  assert.deepEqual(updated.sent, { id: created.body.id, name: name + '-改名' });
  await page.reload({ waitUntil: 'domcontentloaded' });
  await button(page, '模型分配').click();
  await page.getByRole('combobox', { name: '主路备用模型' }).getByText(name + '-改名', { exact: true }).waitFor();
  const stored = await page.evaluate(() => JSON.stringify([localStorage, sessionStorage]));
  assert.ok(!stored.includes(key));
  const restore = original.models.find(row => row.id === original.routes.main_fallback)?.name || '不启用备用模型';
  await action(page, '/settings/routes/save', () => choice(page, '主路备用模型', restore));
  await button(page, '模型配置').click();
  await button(page, '编辑 ' + name + '-改名').click();
  await button(page, '删除 ' + name + '-改名').click();
  await action(page, '/settings/models/delete', () => button(dialog(page), '删除').click());
  console.log('PASS 模型显式厂商、创建、密钥只写、改名、按 ID 路由、刷新、删除');

  await button(page, '智能体').click();
  const toggle = page.getByRole('switch', { name: '自动命名' });
  const before = await get(page, '/settings/behavior');
  if (!before.auto_rename_enabled) await action(page, '/settings/behavior/save', () => toggle.click());
  const off = await action(page, '/settings/behavior/save', () => toggle.click());
  assert.deepEqual(off.sent, { auto_rename_enabled: false });
  await page.reload({ waitUntil: 'domcontentloaded' });
  await button(page, '智能体').click();
  await page.locator('[role=switch][aria-checked=false][aria-labelledby="agent-auto_rename_enabled"]').waitFor();
  assert.deepEqual(await get(page, '/settings/behavior'), { ...before, auto_rename_enabled: false });
  console.log('PASS 行为关闭、局部请求、刷新');

  await button(page, '快捷键').click();
  const trigger = page.getByRole('textbox', { name: '命令菜单' });
  const shortcuts = await get(page, '/settings/shortcuts');
  await trigger.fill(shortcuts.reference_trigger);
  await page.getByRole('alert').filter({ hasText: '绑定冲突' }).waitFor();
  assert.deepEqual(await get(page, '/settings/shortcuts'), shortcuts);
  const nextTrigger = [...'~!^&*'].find(char => !Object.values(shortcuts).includes(char));
  const saved = await action(page, '/settings/shortcuts/save', () => trigger.fill(nextTrigger));
  assert.deepEqual(saved.sent, { ...shortcuts, command_trigger: nextTrigger });
  await page.getByRole('textbox', { name: '上一页' }).press('Alt+Shift+K');
  await page.getByRole('alert').filter({ hasText: '普通键' }).waitFor();
  const shortcutKey = shortcuts.previous_slide === 'Option+KeyK' ? 'J' : 'K';
  await action(page, '/settings/shortcuts/save', () => page.getByRole('textbox', { name: '上一页' }).press('Alt+' + shortcutKey));
  await page.reload({ waitUntil: 'domcontentloaded' });
  await button(page, '快捷键').click();
  await trigger.waitFor();
  assert.equal(await trigger.inputValue(), nextTrigger);
  assert.equal((await get(page, '/settings/shortcuts')).previous_slide, 'Option+Key' + shortcutKey);
  console.log('PASS 快捷键冲突、双修饰键拒绝、单修饰键整组保存、刷新');

  const app = process.platform === 'win32' ? 'C:\\Fixture\\Example.exe' : '/Fixture/Example.app';
  // Set up an application without driving the operating system's native picker.
  const seed = await page.evaluate(async app => {
    const res = await fetch('/api/settings/fileopen/save', { method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ default_open: 'system', json_open: 'inherit', html_open: 'inherit', custom_apps: [app, app] }) });
    return res.json();
  }, app);
  assert.deepEqual(seed.custom_apps, [app]);
  await page.reload({ waitUntil: 'domcontentloaded' });
  await button(page, '文件打开').click();
  await action(page, '/settings/fileopen/save', () => choice(page, 'HTML 文件', 'Example'));
  await page.reload({ waitUntil: 'domcontentloaded' });
  await button(page, '文件打开').click();
  await page.getByRole('combobox', { name: 'HTML 文件' }).getByText('Example', { exact: true }).waitFor();
  const removed = await action(page, '/settings/fileopen/save', () => button(page, '删除 Example').click());
  assert.equal(removed.sent.html_open, app);
  assert.equal(removed.body.html_open, 'inherit');
  await page.getByRole('combobox', { name: 'HTML 文件' }).getByText('跟随默认').waitFor();
  console.log('PASS 文件设置同源保存、路径去重、刷新和删除应用后的服务端回退');
}
async function assets(page) {
  await page.getByRole('link', { name: '仓库', exact: true }).click();
  for (const [type, title, source] of [
    ['theme', '主题', ':root { --color-bg: white; }'], ['component', '组件', '<p>Example</p>'],
    ['skill', '技能', '# Example'], ['snippet', '短语', 'Example'],
  ]) {
    if (type !== 'theme') {
      await page.getByRole('link', { name: title, exact: true }).click();
      await page.waitForURL('**/assets/' + type);
      await page.getByRole('heading', { name: title, exact: true }).waitFor();
    }
    const name = prefix + title;
    const tag = prefix + '-标签';
    await button(page, '管理标签').click();
    await dialog(page).getByLabel('标签名称').fill(tag);
    const createdTag = await action(page, '/assets/tags/create', () => button(dialog(page), '创建标签').click());
    await dialog(page).getByText(tag, { exact: true }).waitFor();
    await page.keyboard.press('Escape');
    await button(page, '创建' + title).click();
    await dialog(page).getByLabel('名称', { exact: true }).fill(name);
    await dialog(page).getByLabel('描述', { exact: true }).fill('浏览器联调');
    await dialog(page).getByLabel('正文', { exact: true }).fill(source);
    await button(dialog(page), tag).click();
    const created = await action(page, '/assets/create', () => button(dialog(page), '保存').click());
    assert.deepEqual(Object.keys(createdTag.body), ['id']);
    assert.deepEqual(Object.keys(created.body), ['id']);
    const id = created.body.id;
    assert.deepEqual((await get(page, '/assets/' + id)).tag_ids, [createdTag.body.id]);
    let content = source;
    for (const suffix of ['1', '2']) {
      const read = page.waitForResponse(r => endpoint(r) === '/api/assets/' + id);
      await button(page, '编辑' + name).click();
      assert.equal((await (await read).json()).content, content);
      await dialog(page).getByLabel('正文').fill(source + suffix);
      if (suffix === '2') await button(dialog(page), tag).click();
      const updated = await action(page, '/assets/update', () => button(dialog(page), '保存').click());
      assert.equal(updated.body, '');
      content = source + suffix;
      assert.equal(updated.sent.content, content);
      assert.equal(updated.sent.asset_id, id);
      assert.ok(!('type' in updated.sent));
      if (suffix === '2') assert.deepEqual(updated.sent.tag_ids, []);
    }
    await page.reload({ waitUntil: 'domcontentloaded' });
    const query = page.getByRole('textbox', { name: '搜索' + title });
    const search = page.waitForResponse(r => endpoint(r) === '/api/assets' && new URL(r.url()).searchParams.get('query') === name);
    await query.fill(name);
    assert.deepEqual((await (await search).json()).items.map(row => row.id), [id]);
    await page.getByRole('heading', { name, exact: true }).waitFor();
    for (const enabled of [false, true]) {
      const toggle = await action(page, '/assets/update', () => page.getByRole('switch', { name: '切换' + title + '状态' }).click());
      assert.deepEqual(toggle.sent, { asset_id: id, enabled });
      await page.getByText(enabled ? '已启用' : '已关闭', { exact: true }).waitFor();
    }
    await button(page, '管理标签').click();
    page.once('dialog', notice => notice.accept());
    await action(page, '/assets/tags/delete', () => button(dialog(page), '删除标签' + tag).click());
    await dialog(page).getByText(tag, { exact: true }).waitFor({ state: 'hidden' });
    await page.keyboard.press('Escape');
    await button(page, '删除' + name).click();
    await action(page, '/assets/delete', () => button(dialog(page), '删除').click());
    await page.getByRole('heading', { name, exact: true }).waitFor({ state: 'hidden' });
    assert.equal((await get(page, '/assets?type=' + type + '&query=' + encodeURIComponent(name))).items.length, 0);
    console.log('PASS ' + title + ' 创建、标签、连续保存、搜索、启停、刷新、删除');
  }
}
(async () => {
  const browser = await chromium.launch({ channel: process.env.CHROME_CHANNEL || 'chrome' });
  try {
    const context = await browser.newContext({ viewport: { width: 1440, height: 960 }, reducedMotion: 'reduce' });
    context.setDefaultTimeout(10000);
    const page = await context.newPage();
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await settings(page);
    try { await assets(page); }
    catch (cause) {
      console.error(page.url(), (await page.locator('body').innerText()).slice(0, 1800), errors);
      throw cause;
    }
    assert.deepEqual(errors, []);
  } finally { await browser.close(); }
})().catch(error => { console.error(error.message); process.exitCode = 1; });
