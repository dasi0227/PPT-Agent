export function newClientIdentity(prefix: 'req' | 'msg'): string {
  const random = typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
    ? crypto.randomUUID()
    : `${Date.now()}_${Math.random().toString(36).slice(2, 12)}`;
  return `${prefix}_${random.replace(/[^A-Za-z0-9_-]/g, '_')}`;
}
