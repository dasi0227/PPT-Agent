const { spawn } = require('node:child_process');
const { createInterface } = require('node:readline');
const { setTimeout: delay } = require('node:timers/promises');

class BackendProcess {
  constructor({ executable, args, cwd, env, log }) {
    this.log = log;
    this.stopping = false;
    this.exited = false;
    this.lastError = '';
    this.child = spawn(executable, args, { cwd, env, stdio: ['pipe', 'pipe', 'pipe'] });
    // Closing this pipe (including after an Electron crash) asks Go to shut down.
    this.child.stdin.on('error', () => {});
    this.completion = new Promise(resolve => {
      const finish = result => {
        if (this.exited) return;
        this.exited = true;
        this.result = result;
        resolve(result);
      };
      this.child.once('error', error => finish({ error }));
      this.child.once('exit', (code, signal) => finish({ code, signal }));
    });
    this.child.stderr.on('data', chunk => {
      log.write(chunk);
      this.lastError = (this.lastError + chunk.toString()).slice(-3000);
    });
    this.ready = new Promise(resolve => {
      const lines = createInterface({ input: this.child.stdout });
      lines.on('line', line => {
        log.write(`${line}\n`);
        if (line.startsWith('PPT_AGENT_READY ')) resolve(line.slice('PPT_AGENT_READY '.length));
      });
    });
  }

  async waitReady(origin) {
    let timeout;
    const expired = new Promise((_, reject) => {
      timeout = setTimeout(() => reject(new Error('后台服务启动超时')), 60_000);
    });
    try {
      await Promise.race([
        (async () => {
          const reported = await this.ready;
          if (reported !== origin) throw new Error('后台服务返回了非预期地址');
          while (!this.exited && !this.stopping) {
            try {
              const response = await fetch(`${origin}/api/v1/healthz`, {
                signal: AbortSignal.timeout(1500), redirect: 'error',
              });
              await response.arrayBuffer();
              if (response.ok) return;
            } catch { /* The listener can become ready just before its handler. */ }
            await delay(200);
          }
          throw new Error('后台服务已停止');
        })(),
        this.completion.then(result => {
          throw result.error ?? new Error(this.lastError.trim() || `后台服务启动失败（退出码 ${result.code ?? result.signal}），请查看 backend.log`);
        }),
        expired,
      ]);
    } finally {
      clearTimeout(timeout);
    }
  }

  async stop() {
    if (this.stopPromise) return this.stopPromise;
    this.stopping = true;
    this.stopPromise = (async () => {
      if (this.exited) return;
      this.child.stdin.end();
      this.child.kill('SIGTERM');
      let timeout;
      const expired = new Promise(resolve => { timeout = setTimeout(() => resolve('timeout'), 25_000); });
      const result = await Promise.race([this.completion, expired]);
      clearTimeout(timeout);
      if (result === 'timeout') {
        this.log.write('Graceful shutdown timed out; terminating the backend.\n');
        this.child.kill('SIGKILL');
        await this.completion;
      }
    })();
    return this.stopPromise;
  }
}

module.exports = { BackendProcess };
