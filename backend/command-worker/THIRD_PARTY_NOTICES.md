# Command worker dependencies

Pinned by `package-lock.json`; upstream source is not vendored or modified.
The desktop package includes the complete dependency directories and their
original license/copyright notices.

| Package | Version | License | Source |
| --- | --- | --- | --- |
| @anthropic-ai/sandbox-runtime | 0.0.81 | Apache-2.0 | https://github.com/anthropics/sandbox-runtime |
| @pondwader/socks5-server | 1.0.10 | MIT | https://www.npmjs.com/package/@pondwader/socks5-server |
| commander | 12.1.0 | MIT | https://github.com/tj/commander.js |
| node-forge | 1.4.2 | BSD-3-Clause (selected from the upstream dual license) | https://github.com/digitalbazaar/forge |
| zod | 3.25.76 | MIT | https://github.com/colinhacks/zod |

`sandbox-runtime` is an upstream research preview. Update the exact version and
lockfile deliberately, review upstream permission changes, and repeat the manual
security acceptance checklist before shipping an updated runtime.
