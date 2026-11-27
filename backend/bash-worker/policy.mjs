import { realpath } from 'node:fs/promises';
import { isIP } from 'node:net';
import { isAbsolute, join, relative, sep } from 'node:path';
import { getDefaultWritePaths, SandboxRuntimeConfigSchema, VENDORED_SRT_WIN_EXE } from '@anthropic-ai/sandbox-runtime';

// This module only supplies upstream configuration. SRT owns path matching,
// Seatbelt generation, network proxies and resolved-address enforcement.
const macSystemReads = [
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
export const protectedNames = ['.git', '.runtime', '.run', '.commit-tmp', 'versions'];
const literal = path => ({ path, literal: true });

export async function runtimePolicy(request) {
  const windows = process.platform === 'win32';
  const { workspace, scratch, host_home, original_root, protected_read_paths, settings, writable } = request;
  const systemReads = windows ? [] : macSystemReads;
  if (windows) {
    // A developer checkout can put the native runner under the denied host
    // profile. Machine-wide installs already have system read/execute rights.
    const helperPath = relative(host_home, VENDORED_SRT_WIN_EXE);
    if (!isAbsolute(helperPath) && !helperPath.startsWith('..' + sep)) systemReads.push(VENDORED_SRT_WIN_EXE);
  }
  const readRoots = await Promise.all((settings.read_roots ?? []).map(root => realpath(root)));
  const allowedDomains = settings.network.allowed_domains ?? [];
  for (const entry of allowedDomains) {
    // SRT intentionally lets explicitly allowed IPs override its address deny
    // list. This application exposes hostname grants only, so that escape hatch
    // cannot reopen loopback/private addresses. SRT validates the full syntax.
    const host = entry.replace(/:\d+$/, '').replace(/^\[|\]$/g, '').toLowerCase();
    if (isIP(host) || /^[\d.]+$/.test(host) || host.includes(':')) {
      throw new Error('sandbox.network.allowed_domains accepts hostnames, not IP addresses');
    }
    if (host === 'localhost' || host.endsWith('.localhost') || host === 'local' || host.endsWith('.local')) {
      throw new Error('sandbox.network.allowed_domains does not allow local hostnames');
    }
  }
  return SandboxRuntimeConfigSchema.parse({
    ...(windows && { windows: { srtWin: { path: VENDORED_SRT_WIN_EXE } } }),
    filesystem: {
      // Windows starts from the sandbox account's NTFS permissions. A drive-root
      // deny would change host volume ACLs, including those of its system tools.
      denyRead: [literal(windows ? host_home : '/')],
      allowRead: [...systemReads, workspace, scratch, ...readRoots].map(literal),
      allowWrite: [scratch, ...(writable ? [workspace] : [])].map(literal),
      denyWrite: [
        ...protectedNames.map(path => join(workspace, path)),
        ...(!writable ? [workspace] : []),
        // Remove SRT's shared convenience directories. The isolated HOME's
        // defaults are already inside scratch; device stdio paths stay usable.
        ...(windows ? [] : getDefaultWritePaths().filter(path => !path.startsWith('/dev/') && !path.startsWith(scratch + '/'))),
      ].map(literal),
      allowGitConfig: false,
    },
    credentials: {
      files: [...credentialPaths.filter(path => !windows || !path.startsWith('Library/')).map(path => join(host_home, path)),
        original_root, ...(protected_read_paths ?? []), ...(windows ? [] : [import.meta.dirname])]
        .map(path => ({ path, mode: 'deny' })),
    },
    network: {
      allowedDomains,
      // SRT rejects single-label wildcard patterns. Local hostname grants are
      // rejected above; strict allowlisting and resolved-address rules remain.
      deniedDomains: ['localhost'],
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
