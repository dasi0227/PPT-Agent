import type { SourceKind } from '../api/slideSources';

export interface StoredSourceDraft {
  id: string;
  sessionId: string;
  projectId: string;
  slideId: string;
  kind: SourceKind;
  sceneRevision: number;
  baseSourceHash: string;
  baseText: string;
  draftText: string;
  updatedAt: number;
  selectionAnchor?: number;
  scrollTop?: number;
  backup?: boolean;
}

const SESSION_KEY = 'ppt-agent-source-session';
const DB_NAME = 'ppt-agent-source-drafts';
const STORE = 'drafts';
let memorySessionId: string | null = null;
let databasePromise: Promise<IDBDatabase> | null = null;
const pending = new Map<string, Promise<void>>();
const tabInstanceId = crypto.randomUUID();
let sessionReady: Promise<void> | null = null;
let sessionChannel: BroadcastChannel | null = null;

// Browsers can clone sessionStorage when a tab is duplicated. Claim the stored
// identity before reading drafts so the copied tab cannot write into its peer.
export function initializeSourceDraftSession(): Promise<void> {
  if (sessionReady) return sessionReady;
  sessionReady = new Promise<void>((resolve) => {
    const claimed = sourceSessionId();
    if (typeof BroadcastChannel === 'undefined') { resolve(); return; }
    const channel = new BroadcastChannel('ppt-agent-source-session-claims');
    sessionChannel = channel;
    let collision = false;
    channel.onmessage = (event: MessageEvent<{ type: 'probe' | 'alive'; sessionId: string; sender: string; recipient?: string }>) => {
      const claim = event.data;
      if (claim.sender === tabInstanceId || claim.sessionId !== sourceSessionId()) return;
      if (claim.type === 'probe') channel.postMessage({ type: 'alive', sessionId: claim.sessionId, sender: tabInstanceId, recipient: claim.sender });
      if (claim.type === 'alive' && claim.recipient === tabInstanceId) collision = true;
    };
    channel.postMessage({ type: 'probe', sessionId: claimed, sender: tabInstanceId });
    window.setTimeout(() => {
      if (collision) {
        memorySessionId = crypto.randomUUID();
        try { sessionStorage.setItem(SESSION_KEY, memorySessionId); } catch { /* Keep this tab's in-memory identity. */ }
      }
      resolve();
    }, 80);
    window.addEventListener('pagehide', () => { channel.close(); if (sessionChannel === channel) sessionChannel = null; }, { once: true });
  });
  return sessionReady;
}

export function sourceSessionId(): string {
  if (memorySessionId) return memorySessionId;
  try {
    memorySessionId = sessionStorage.getItem(SESSION_KEY) || crypto.randomUUID();
    sessionStorage.setItem(SESSION_KEY, memorySessionId);
  } catch {
    memorySessionId = crypto.randomUUID();
  }
  return memorySessionId;
}

export const sourceDraftId = (projectId: string, slideId: string, kind: SourceKind) =>
  `${sourceSessionId()}:${projectId}:${slideId}:${kind}`;

function database(): Promise<IDBDatabase> {
  if (databasePromise) return databasePromise;
  databasePromise = new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, 1);
    request.onupgradeneeded = () => request.result.createObjectStore(STORE, { keyPath: 'id' });
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
  return databasePromise;
}

async function operation<T>(mode: IDBTransactionMode, action: (store: IDBObjectStore) => IDBRequest<T>): Promise<T> {
  const db = await database();
  return new Promise<T>((resolve, reject) => {
    const transaction = db.transaction(STORE, mode);
    const request = action(transaction.objectStore(STORE));
    transaction.oncomplete = () => resolve(request.result);
    request.onerror = () => reject(request.error);
    transaction.onerror = () => reject(transaction.error);
  });
}

export async function listSourceDrafts(): Promise<StoredSourceDraft[]> {
  await initializeSourceDraftSession();
  const all = await operation<StoredSourceDraft[]>('readonly', (store) => store.getAll());
  return all.filter((item) => item.sessionId === sourceSessionId());
}

// Serialize each resource's writes and deletes. An older debounce cannot
// resurrect a draft after a successful save or external replacement.
export function enqueueSourceDraft(id: string, value: StoredSourceDraft | null): Promise<void> {
  const previous = pending.get(id) ?? Promise.resolve();
  const next = previous.catch(() => {}).then(async () => {
    if (value) await operation('readwrite', (store) => store.put(value));
    else await operation('readwrite', (store) => store.delete(id));
  });
  pending.set(id, next);
  void next.then(() => { if (pending.get(id) === next) pending.delete(id); }, () => { if (pending.get(id) === next) pending.delete(id); });
  return next;
}

export async function flushSourceDrafts() {
  await Promise.all([...pending.values()]);
}

export async function backupProjectSourceDrafts(projectId: string, sceneRevision?: number) {
  await flushSourceDrafts();
  const records = (await listSourceDrafts()).filter((item) => item.projectId === projectId && !item.backup && item.draftText !== item.baseText && (sceneRevision === undefined || item.sceneRevision === sceneRevision));
  await Promise.all(records.map((item) => {
    const id = `${item.id}:backup:${item.sceneRevision}`;
    return enqueueSourceDraft(id, { ...item, id, backup: true, updatedAt: Date.now() });
  }));
}
