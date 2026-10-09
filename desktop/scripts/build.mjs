import { cp, mkdir, readFile, realpath, rm, writeFile } from 'node:fs/promises';
import { spawnSync } from 'node:child_process';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

export const desktop = resolve(dirname(fileURLToPath(import.meta.url)), '..');
export const root = resolve(desktop, '..');
export const buildRoot = join(desktop, '.build');

export function run(command, args, cwd = root) {
  const result = spawnSync(command, args, { cwd, stdio: 'inherit' });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`${command} failed (${result.status ?? result.signal})`);
}

export async function build() {
  if (process.platform !== 'darwin') throw new Error('This package targets the current Mac.');
  await rm(buildRoot, { recursive: true, force: true });
  const runtime = join(buildRoot, 'runtime');
  const app = join(buildRoot, 'app');
  await mkdir(join(runtime, 'bin'), { recursive: true });
  await mkdir(app, { recursive: true });
  run('pnpm', ['build'], join(root, 'frontend'));
  run('go', ['build', '-trimpath', '-o', join(runtime, 'bin', 'ppt-agent-server'), './cmd/server'], join(root, 'backend'));
  run('go', ['build', '-trimpath', '-o', join(runtime, 'bin', 'init-resources'), './cmd/init-resources'], join(root, 'backend'));
  await cp(join(root, 'frontend', 'dist'), join(runtime, 'frontend'), { recursive: true });
  await cp(join(root, 'seed'), join(runtime, 'seed'), { recursive: true, filter: source => !source.endsWith('.DS_Store') });
  const worker = join(runtime, 'render-worker');
  await mkdir(join(worker, 'node_modules'), { recursive: true });
  for (const name of ['worker.mjs', 'style-inspection.mjs', 'package.json']) {
    await cp(join(root, 'backend', 'render-worker', name), join(worker, name));
  }
  // Resolve pnpm's symlink before copying so the .app has no repository links.
  const playwright = await realpath(join(root, 'backend', 'render-worker', 'node_modules', 'playwright-core'));
  await cp(playwright, join(worker, 'node_modules', 'playwright-core'), { recursive: true, dereference: true });
  for (const name of ['main.cjs', 'runtime.cjs']) await cp(join(desktop, name), join(app, name));
  const pkg = JSON.parse(await readFile(join(desktop, 'package.json'), 'utf8'));
  delete pkg.devDependencies;
  delete pkg.scripts;
  await writeFile(join(app, 'package.json'), JSON.stringify(pkg, null, 2) + '\n');

  const iconset = join(buildRoot, 'PPT-Agent.iconset');
  await mkdir(iconset);
  for (const size of [16, 32, 128, 256, 512]) {
    for (const scale of [1, 2]) {
      run('/usr/bin/sips', ['-s', 'format', 'png', '-z', String(size * scale), String(size * scale),
        join(root, 'frontend', 'public', 'logo.png'), '--out', join(iconset, `icon_${size}x${size}${scale === 2 ? '@2x' : ''}.png`)]);
    }
  }
  run('/usr/bin/iconutil', ['-c', 'icns', iconset, '-o', join(buildRoot, 'PPT-Agent.icns')]);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) await build();
