import { spawn } from 'node:child_process';
import { mkdir, mkdtemp, realpath } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createInterface } from 'node:readline';
import { fileURLToPath } from 'node:url';
import { copyProject, documentChanges, removeWorkspace } from './workspace.mjs';

const windows = process.platform === 'win32';
const controller = new AbortController();
function cancel() {
  controller.abort(Object.assign(new Error('Command canceled'), { code: 'RUN_CANCELED' }));
}
process.stdin.on('end', cancel);
process.on('SIGTERM', cancel);
process.on('SIGINT', cancel);

async function runSandbox(request, signal) {
  signal.throwIfAborted();
  const child = spawn(process.execPath, [fileURLToPath(new URL('worker.mjs', import.meta.url))], {
    detached: !windows, windowsHide: true, stdio: ['pipe', 'pipe', 'inherit'],
  });
  const finished = new Promise((resolve, reject) => {
    child.once('error', reject);
    child.once('close', (code, signal) => resolve({ code, signal }));
  });
  const chunks = [];
  let length = 0;
  const replyLimit = request.settings.execution.max_output_bytes * 2 + 65536;
  let hardStop;
  function kill() {
    if (!child.pid) return;
    try {
      if (windows) child.kill('SIGKILL');
      else process.kill(-child.pid, 'SIGKILL');
    } catch (error) {
      if (error.code !== 'ESRCH') throw error;
    }
  }
  function stop() {
    child.stdin.end();
    hardStop = setTimeout(kill, windows ? 5000 : 2000);
  }
  child.stdout.on('data', chunk => {
    length += chunk.length;
    if (length <= replyLimit) chunks.push(chunk);
    else kill();
  });
  // A failed launch or a worker that exits before consuming the request closes the pipe.
  child.stdin.on('error', () => {});
  signal.addEventListener('abort', stop, { once: true });
  try {
    child.stdin.write(JSON.stringify(request) + '\n');
    const status = await finished;
    signal.throwIfAborted();
    if (length > replyLimit || (windows ? status.code !== 0 : status.signal !== 'SIGKILL')) {
      throw new Error('Command sandbox exited without a valid result');
    }
    const reply = JSON.parse(Buffer.concat(chunks).toString('utf8'));
    if (reply.protocol !== 1) throw new Error('Invalid sandbox response');
    return reply;
  } finally {
    clearTimeout(hardStop);
    signal.removeEventListener('abort', stop);
    if (!windows) kill();
    child.stdin.destroy();
  }
}

async function execute(request) {
  if (!windows && process.platform !== 'darwin') throw new Error('Command sandbox requires macOS or Windows');
  const { signal } = controller;
  const pattern = new RegExp('^(?:' + request.document_pattern + ')', 'u');
  const isDocument = name => pattern.exec(name)?.[0] === name;
  const root = await realpath(await mkdtemp(join(windows ? tmpdir() : '/tmp', 'ppt-bash-')));
  try {
    const workspace = join(root, 'work');
    const scratch = join(root, 'scratch');
    await mkdir(workspace);
    await mkdir(scratch);
    const before = await copyProject(request.original_root, workspace, isDocument, signal);
    const reply = await runSandbox({ ...request, protocol: 1, workspace, scratch }, signal);
    const { exit_code: exitCode, duration_ms: durationMs, code, message } = reply;
    const changes = request.writable && !code && exitCode === 0
      ? await documentChanges(workspace, before, isDocument, signal) : [];
    return { exit_code: exitCode, duration_ms: durationMs, code, message, stdout: Buffer.from(reply.stdout, 'base64').toString('utf8'),
      stderr: Buffer.from(reply.stderr, 'base64').toString('utf8'), changes };
  } finally {
    await removeWorkspace(root);
  }
}

try {
  const lines = createInterface({ input: process.stdin, crlfDelay: Infinity });
  const line = await new Promise((resolve, reject) => {
    lines.once('line', resolve);
    lines.once('close', () => reject(new Error('Missing worker request')));
  });
  lines.close();
  process.stdin.resume();
  const reply = await execute(JSON.parse(line));
  controller.signal.throwIfAborted();
  process.stdout.write(JSON.stringify(reply));
} catch (error) {
  const failure = controller.signal.aborted ? controller.signal.reason : error;
  const code = ['RUN_CANCELED', 'COMMAND_NOT_ALLOWED'].includes(failure.code) ? failure.code : 'COMMAND_NOT_AVAILABLE';
  process.stdout.write(JSON.stringify({ code, message: failure.message, exit_code: -1, duration_ms: 0,
    stdout: '', stderr: '', changes: [] }));
} finally {
  process.stdin.destroy();
}
