import { packager } from '@electron/packager';
import { readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { build, buildRoot, desktop } from './build.mjs';

await build();
const pkg = JSON.parse(await readFile(join(desktop, 'package.json'), 'utf8'));
const paths = await packager({
  dir: join(buildRoot, 'app'), out: join(desktop, 'out'),
  name: 'PPT-Agent', platform: 'darwin', arch: process.arch,
  electronVersion: pkg.devDependencies.electron,
  appBundleId: 'com.dasi.ppt-agent', appCategoryType: 'public.app-category.productivity',
  appVersion: pkg.version, buildVersion: pkg.version,
  icon: join(buildRoot, 'PPT-Agent.icns'),
  extraResource: [join(buildRoot, 'runtime')],
  asar: true, overwrite: true, prune: false,
  // Local build; public distribution signing/notarization is a separate release task.
  osxSign: { identity: '-', identityValidation: false, preAutoEntitlements: false, continueOnError: false,
    optionsForFile: () => ({ hardenedRuntime: false }) },
});
for (const output of paths) console.log(`App: ${join(output, 'PPT-Agent.app')}`);
