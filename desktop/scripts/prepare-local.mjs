import { constants } from 'node:fs';
import { access, chmod, copyFile, mkdir } from 'node:fs/promises';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { root } from './build.mjs';

if (process.platform !== 'darwin') throw new Error('This setup targets macOS.');
const userData = join(homedir(), 'Library', 'Application Support', 'PPT-Agent');
const destination = join(userData, 'config.yaml');
await mkdir(userData, { recursive: true, mode: 0o700 });
try {
  await access(destination);
  console.log(`保留已有桌面配置：${destination}`);
} catch (error) {
  if (error.code !== 'ENOENT') throw error;
  await copyFile(join(root, 'backend', 'config.yaml'), destination, constants.COPYFILE_EXCL);
  await chmod(destination, 0o600);
  console.log(`已准备桌面配置：${destination}`);
}
