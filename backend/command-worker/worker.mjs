import { spawn } from 'node:child_process';
import { access, mkdir, realpath } from 'node:fs/promises';
import { constants } from 'node:fs';
import { join } from 'node:path';
import { createInterface } from 'node:readline';
import { randomUUID } from 'node:crypto';

// One worker, one SRT singleton, one command. stdin stays open as a parent
// lifetime pipe. Go creates a fresh process group before starting this worker.
const controller = new AbortController();
let stopping = false;
let failure;
let hardStop;
let manager;
let deadline;
let stdout = [];
let stderr = [];
let captured = 0;
let limit = 8 << 20;
const started = Date.now();
const result = { protocol: 1, exit_code: -1, code: '', message: '' };

function signalGroup(signal) {
  try { process.kill(-process.pid, signal); } catch { /* Go also reaps this group. */ }
}

function stop(code, message) {
  if (stopping) return;
  stopping = true;
  failure ??= { code, message };
  controller.abort();
  signalGroup('SIGTERM');
  hardStop = setTimeout(() => signalGroup('SIGKILL'), 2000);
}

process.on('SIGTERM', () => stop('RUN_CANCELED', 'Command canceled.'));
process.on('SIGINT', () => stop('RUN_CANCELED', 'Command canceled.'));
process.stdin.on('end', () => stop('RUN_CANCELED', 'Command parent disconnected.'));
process.stdout.on('error', () => { signalGroup('SIGKILL'); process.exit(1); });

function capture(chunks, chunk) {
  const remaining = Math.max(0, limit - captured);
  if (remaining) chunks.push(chunk.subarray(0, remaining));
  captured += Math.min(remaining, chunk.length);
  if (chunk.length > remaining) {
    failure ??= { code: 'COMMAND_OUTPUT_LIMIT', message: 'Command output exceeded its combined stdout/stderr limit.' };
    stop(failure.code, failure.message);
  }
}

try {
  const lines = createInterface({ input: process.stdin, crlfDelay: Infinity });
  const request = JSON.parse(await new Promise((resolve, reject) => {
    lines.once('line', resolve);
    lines.once('close', () => reject(new Error('Missing command request.')));
    process.stdin.once('error', reject);
  }));
  lines.close();
  // Keep the lifetime pipe flowing after the single request line.
  process.stdin.resume();
  const [major, minor] = process.versions.node.split('.').map(Number);
  if (process.platform !== 'darwin' || major < 22 || (major === 22 && minor < 12)) {
    throw new Error('Command sandbox requires macOS and Node >=22.12.0.');
  }
  delete process.env.ELECTRON_RUN_AS_NODE;
  if (request.protocol !== 1 || typeof request.command !== 'string' || typeof request.writable !== 'boolean') {
    throw new Error('Invalid command worker request.');
  }
  limit = request.settings.execution.max_output_bytes;
  if (!Number.isSafeInteger(limit) || limit < 1024 || limit > (64 << 20)) throw new Error('Invalid command output limit.');
  const timeout = request.settings.execution.timeout_seconds;
  if (!Number.isInteger(timeout) || timeout < 1 || timeout > 900) throw new Error('Invalid command timeout.');
  deadline = setTimeout(() => stop('COMMAND_TIMEOUT', 'Command exceeded its execution deadline.'), timeout * 1000);
  request.workspace = await realpath(request.workspace);
  request.scratch = await realpath(request.scratch);
  process.chdir(request.workspace);
  const home = join(request.scratch, 'home');
  const tmp = join(request.scratch, 'tmp');
  await mkdir(home, { recursive: true, mode: 0o700 });
  await mkdir(tmp, { recursive: true, mode: 0o700 });
  // Go supplies a small environment from scratch. No inherited proxy, model
  // key, SSH agent, Node injection option or shell startup file reaches SRT.
  Object.assign(process.env, {
    HOME: home, TMPDIR: tmp, CLAUDE_CODE_TMPDIR: tmp,
    XDG_CONFIG_HOME: join(home, '.config'), XDG_CACHE_HOME: join(home, '.cache'),
    XDG_DATA_HOME: join(home, '.local/share'),
    npm_config_cache: join(home, '.npm'), PIP_CACHE_DIR: join(home, '.cache/pip'),
    GIT_CONFIG_NOSYSTEM: '1', GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_SYSTEM: '/dev/null',
    GIT_OPTIONAL_LOCKS: '0', GIT_TERMINAL_PROMPT: '0', GIT_PAGER: 'cat', PAGER: 'cat',
    GIT_CONFIG_COUNT: '3', GIT_CONFIG_KEY_0: 'core.fsmonitor', GIT_CONFIG_VALUE_0: 'false',
    GIT_CONFIG_KEY_1: 'diff.external', GIT_CONFIG_VALUE_1: '',
    GIT_CONFIG_KEY_2: 'core.hooksPath', GIT_CONFIG_VALUE_2: '/dev/null',
  });
  await Promise.all(['/usr/bin/sandbox-exec', '/bin/bash'].map(path => access(path, constants.X_OK)));
  const { SandboxManager } = await import('@anthropic-ai/sandbox-runtime');
  const { runtimePolicy } = await import('./policy.mjs');
  manager = SandboxManager;
  await manager.initialize(await runtimePolicy(request));
  const commandID = randomUUID();
  const wrapped = await manager.wrapWithSandbox(request.command, '/bin/bash', undefined, controller.signal, { commandId: commandID });
  controller.signal.throwIfAborted();
  // Never spawn request.command directly. Only the upstream wrapper is run.
  const child = spawn('/bin/bash', ['--noprofile', '--norc', '-c', wrapped], {
    cwd: request.workspace, env: process.env, stdio: ['ignore', 'pipe', 'pipe'],
  });
  child.stdout.on('data', chunk => capture(stdout, chunk));
  child.stderr.on('data', chunk => capture(stderr, chunk));
  const closed = new Promise(resolve => child.once('close', () => resolve(true)));
  const exit = await Promise.race([
    new Promise((resolve, reject) => {
      child.once('error', reject);
      child.once('exit', code => resolve(code));
    }),
    // A command may ignore SIGTERM. Still return the bounded failure result
    // before the final SIGKILL instead of waiting forever for its exit event.
    new Promise(resolve => {
      const aborted = () => setTimeout(() => resolve(null), 100);
      if (controller.signal.aborted) aborted();
      else controller.signal.addEventListener('abort', aborted, { once: true });
    }),
  ]);
  result.exit_code = exit ?? -1;
  // Briefly drain output already in pipes without waiting for background jobs
  // to close inherited descriptors. The final group SIGKILL stops those jobs.
  stopping = true;
  const drained = await Promise.race([
    closed,
    new Promise(resolve => setTimeout(() => resolve(false), 250)),
  ]);
  child.stdout.destroy();
  child.stderr.destroy();
  if (!drained) {
    failure ??= { code: 'COMMAND_NOT_ALLOWED', message: 'Command left output streams open after exiting; background jobs are not supported.' };
  }
  if (!failure && manager.getSandboxViolationStore().getTotalCount() > 0) {
    failure = { code: 'COMMAND_NOT_ALLOWED', message: 'Command attempted access denied by the sandbox.' };
  }
  if (!failure && result.exit_code !== 0) {
    const diagnostic = Buffer.concat(stderr).toString('utf8');
    const code = /Operation not permitted|Permission denied/.test(diagnostic) ? 'COMMAND_NOT_ALLOWED'
      : [126, 127].includes(result.exit_code) || diagnostic.trimStart().startsWith('sandbox-exec:') ? 'COMMAND_NOT_AVAILABLE'
        : 'COMMAND_EXIT_NONZERO';
    failure = { code, message: `Command exited with code ${result.exit_code}.` };
  }
} catch (error) {
  failure ??= { code: 'COMMAND_NOT_AVAILABLE', message: String(error?.message ?? error).slice(0, 4096) };
} finally {
  stopping = true;
  clearTimeout(deadline);
  // reset closes upstream proxies; never keep a proxy between commands.
  if (manager) {
    try {
      await Promise.race([
        manager.reset(),
        new Promise((_, reject) => setTimeout(() => reject(new Error('Sandbox cleanup timed out.')), 500)),
      ]);
    } catch (error) {
      failure ??= { code: 'COMMAND_NOT_AVAILABLE', message: String(error.message).slice(0, 4096) };
    }
  }
  clearTimeout(hardStop);
  if (failure) Object.assign(result, failure);
  result.stdout = Buffer.concat(stdout).toString('base64');
  result.stderr = Buffer.concat(stderr).toString('base64');
  result.duration_ms = Date.now() - started;
  // The terminal record is written only after cleanup. Terminating our entire
  // group after the write flushes lets Go freeze the candidate without survivors
  // in this group, including successful commands that backgrounded a job.
  process.stdout.write(JSON.stringify(result) + '\n', () => {
    signalGroup('SIGKILL');
    process.exit(1);
  });
}
