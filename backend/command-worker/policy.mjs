import { realpath } from 'node:fs/promises';
import { isIP } from 'node:net';
import { join } from 'node:path';
import { getDefaultWritePaths, SandboxRuntimeConfigSchema } from '@anthropic-ai/sandbox-runtime';

// This module only supplies upstream configuration. SRT owns path matching,
// Seatbelt generation, network proxies and resolved-address enforcement.
const systemReads = [
  // Never grant /System as a whole: /System/Volumes/Data aliases user data.
  '/System/Library', '/bin', '/sbin', '/usr/bin', '/usr/sbin', '/usr/lib', '/usr/libexec',
  '/usr/share', '/Library/Apple',
  '/Library/Developer/CommandLineTools', '/Library/Frameworks',
  '/private/etc/ssl', '/private/etc/hosts', '/private/etc/resolv.conf',
  '/private/etc/services', '/private/var/db/dyld', '/private/var/db/timezone',
  '/dev/null', '/dev/fd', '/dev/random', '/dev/urandom',
  ...['/usr/local', '/opt/homebrew'].flatMap(root =>
    ['bin', 'sbin', 'lib', 'libexec', 'share', 'opt', 'Cellar', 'Frameworks', 'etc/openssl@3'].map(path => join(root, path))),
];
const credentialPaths = [
  '.ssh', '.aws', '.azure', '.kube', '.gnupg', '.docker', '.config/gcloud',
  '.config/gh', '.netrc', '.npmrc', '.git-credentials', '.dasi', '.codex',
  '.claude', 'Library/Keychains', 'Library/Application Support/PPT-Agent',
];
const protectedProjectPaths = ['.git', '.runtime', '.run', '.commit-tmp', 'versions'];
const literal = path => ({ path, literal: true });

export async function runtimePolicy(request) {
  const { workspace, scratch, host_home, original_root, protected_read_paths, settings, writable } = request;
  const readRoots = await Promise.all((settings.read_roots ?? []).map(root => realpath(root)));
  const allowedDomains = settings.network.allowed_domains ?? [];
  for (const entry of allowedDomains) {
    // SRT intentionally lets explicitly allowed IPs override its address deny
    // list. This application exposes hostname grants only, so that escape hatch
    // cannot reopen loopback/private addresses. SRT validates the full syntax.
    const host = entry.replace(/:\d+$/, '').replace(/^\[|\]$/g, '');
    if (isIP(host) || /^[\d.]+$/.test(host) || host.includes(':')) {
      throw new Error('sandbox.network.allowed_domains accepts hostnames, not IP addresses');
    }
  }
  return SandboxRuntimeConfigSchema.parse({
    filesystem: {
      denyRead: [literal('/')],
      allowRead: [...systemReads, workspace, scratch, ...readRoots].map(literal),
      allowWrite: [scratch, ...(writable ? [workspace] : [])].map(literal),
      denyWrite: [
        ...protectedProjectPaths.map(path => join(workspace, path)),
        ...(!writable ? [workspace] : []),
        // Remove SRT's shared convenience directories. The isolated HOME's
        // defaults are already inside scratch; device stdio paths stay usable.
        ...getDefaultWritePaths().filter(path => !path.startsWith('/dev/') && !path.startsWith(scratch + '/')),
      ].map(literal),
      allowGitConfig: false,
    },
    credentials: {
      files: [...credentialPaths.map(path => join(host_home, path)), original_root, ...(protected_read_paths ?? [])]
        .map(path => ({ path, mode: 'deny' })),
    },
    network: {
      allowedDomains,
      deniedDomains: ['localhost', '*.localhost', '*.local'],
      strictAllowlist: true,
      deniedResolvedAddresses: [
        '0.0.0.0/8', '10.0.0.0/8', '100.64.0.0/10', '127.0.0.0/8',
        '169.254.0.0/16', '172.16.0.0/12', '192.168.0.0/16',
        '198.18.0.0/15', '224.0.0.0/4', '240.0.0.0/4',
        '::/128', '::1/128', 'fc00::/7', 'fe80::/10', 'ff00::/8',
      ],
      allowUnixSockets: [], allowAllUnixSockets: false, allowLocalBinding: false, allowMachLookup: [],
    },
    allowAppleEvents: false, allowPty: false,
    enableWeakerNetworkIsolation: false, enableWeakerNestedSandbox: false,
    allowNestedUserNamespaces: false,
  });
}
