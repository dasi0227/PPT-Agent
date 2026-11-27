import { constants, createWriteStream } from 'node:fs';
import { chmod, lstat, mkdir, open, readdir, readlink, rmdir, stat, symlink, unlink, writeFile } from 'node:fs/promises';
import { dirname, isAbsolute, join, relative, resolve, sep } from 'node:path';
import { pipeline } from 'node:stream/promises';
import { protectedNames } from './policy.mjs';

function denied(path) {
  return Object.assign(new Error(`Expected an independent regular file: ${path}`), { code: 'COMMAND_NOT_ALLOWED' });
}

async function openFile(path, requireSingleLink) {
  const entry = await lstat(path, { bigint: true });
  if (!entry.isFile() || (requireSingleLink && entry.nlink !== 1n)) throw denied(path);
  const handle = await open(path, constants.O_RDONLY | (constants.O_NOFOLLOW ?? 0));
  try {
    const opened = await handle.stat({ bigint: true });
    if (!opened.isFile() || (requireSingleLink && opened.nlink !== 1n)
        || opened.ino !== entry.ino || opened.dev !== entry.dev) throw denied(path);
    return handle;
  } catch (error) {
    await handle.close();
    throw error;
  }
}

async function documentBytes(path, signal) {
  const handle = await openFile(path, true);
  try {
    return await handle.readFile({ signal });
  } finally {
    await handle.close();
  }
}

export async function copyProject(source, workspace, isDocument, signal) {
  const before = new Map();
  const root = await lstat(source).catch(error => {
    if (error.code !== 'ENOENT') throw error;
  });
  if (!root) return before;
  if (!root.isDirectory()) throw new Error('Invalid project directory');
  async function copyDirectory(dir, targetDir) {
    for (const entry of await readdir(dir, { withFileTypes: true })) {
      signal.throwIfAborted();
      if (dir === source && protectedNames.includes(entry.name.toLowerCase())) continue;
      const file = join(dir, entry.name);
      const target = join(targetDir, entry.name);
      if (dir === source && isDocument(entry.name)) {
        const bytes = await documentBytes(file, signal);
        before.set(entry.name, bytes);
        await writeFile(target, bytes, { flag: 'wx' });
      } else if (entry.isDirectory()) {
        await mkdir(target);
        await copyDirectory(file, target);
      } else if (entry.isFile()) {
        const handle = await openFile(file, false);
        try {
          const { mode } = await handle.stat();
          await pipeline(handle.createReadStream(), createWriteStream(target, { flags: 'wx', mode: mode & 0o777 }), { signal });
        } finally {
          await handle.close();
        }
      } else if (entry.isSymbolicLink()) {
        const resolved = resolve(dir, await readlink(file));
        const within = relative(source, resolved);
        if (isAbsolute(within) || within === '..' || within.startsWith('..' + sep)) continue;
        const link = relative(dirname(file), resolved) || '.';
        let type = 'file';
        if (process.platform === 'win32') {
          const destination = await stat(file).catch(error => {
            if (error.code !== 'ENOENT') throw error;
          });
          if (destination?.isDirectory()) type = 'dir';
        }
        await symlink(link, target, type);
      }
    }
  }
  await copyDirectory(source, workspace);
  return before;
}

export async function documentChanges(workspace, before, isDocument, signal) {
  if (!(await lstat(workspace)).isDirectory()) throw denied(workspace);
  const current = new Set((await readdir(workspace)).filter(isDocument));
  const changes = [];
  for (const fileName of [...new Set([...before.keys(), ...current])].sort()) {
    signal.throwIfAborted();
    const oldBytes = before.get(fileName);
    const newBytes = current.has(fileName) ? await documentBytes(join(workspace, fileName), signal) : null;
    if (oldBytes && newBytes && oldBytes.equals(newBytes)) continue;
    changes.push({ file_name: fileName, before: oldBytes?.toString('base64') ?? null, after: newBytes?.toString('base64') ?? null });
  }
  return changes;
}

export async function removeWorkspace(path) {
  const entry = await lstat(path);
  if (entry.isDirectory()) {
    if (process.platform !== 'win32') await chmod(path, 0o700);
    for (const name of await readdir(path)) await removeWorkspace(join(path, name));
    await rmdir(path);
  } else {
    try {
      await unlink(path);
    } catch (error) {
      if (process.platform !== 'win32' || error.code !== 'EPERM' || !entry.isFile() || entry.nlink !== 1) throw error;
      await chmod(path, 0o600);
      await unlink(path);
    }
  }
}
