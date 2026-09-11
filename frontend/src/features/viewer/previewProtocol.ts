export interface RuntimeSlide {
  id: string;
  html: string;
  frame: import('./runtimeFrame').RuntimeFrameContext;
}

export type PreviewCommand =
  | { type: 'updateDeck'; slides: RuntimeSlide[]; index: number }
  | { type: 'gotoSlide'; index: number }
  | { type: 'setSelectionMode'; session_id: string; slide_id: string; mode: 'element' | 'region' | 'none'; html_revision: number; html_hash: string }
  | { type: 'renderDraftSelections'; session_id: string; slide_id: string; selections: Array<{ selection_id: string; marker_no: number; rect: import('../../api/types').CanvasRect; status: import('../../api/types').DOMSelectionStatus }> }
  | { type: 'probeDraftSelections'; session_id: string; slide_id: string; selections: import('../../api/types').DOMSelection[] };

export type RuntimeEvent =
  | { type: 'runtimeReady' }
  | { type: 'renderError'; message: string; index?: number }
  | { type: 'selectionCreated'; session_id: string; slide_id: string; selection: import('../../api/types').DOMSelection }
  | { type: 'selectionCanceled'; session_id: string; slide_id: string }
  | { type: 'selectionPresence'; session_id: string; slide_id: string; statuses: Array<{ selection_id: string; status: 'active' | 'content_deleted'; targets?: Array<{ target_id: string; status: 'active' | 'content_deleted' }> }> }
  | { type: 'selectionEmpty' | 'selectionRejected'; session_id: string; slide_id: string; message: string };

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isIndex(value: unknown): value is number {
  return Number.isInteger(value) && (value as number) >= 0;
}

function isRect(value: unknown): boolean {
  if (!isRecord(value)) return false;
  const numbers = [value.x, value.y, value.width, value.height];
  return numbers.every((item) => typeof item === 'number' && Number.isFinite(item))
    && Number(value.x) >= 0 && Number(value.y) >= 0 && Number(value.width) >= 0 && Number(value.height) >= 0
    && Number(value.x) + Number(value.width) <= 1920.001 && Number(value.y) + Number(value.height) <= 1080.001;
}

function isPositiveRect(value: unknown): boolean {
  return isRect(value) && Number((value as Record<string, unknown>).width) > 0 && Number((value as Record<string, unknown>).height) > 0;
}

function isStringRecord(value: unknown): boolean {
  return isRecord(value) && Object.entries(value).every(([key, item]) => key.length <= 128 && typeof item === 'string' && item.length <= 512);
}

function isEdges(value: unknown): boolean {
  return isRecord(value) && ['top', 'right', 'bottom', 'left'].every((key) => typeof value[key] === 'number' && Number.isFinite(value[key]));
}

function isFingerprint(value: unknown, tag: string): boolean {
  return isRecord(value) && value.tag === tag && Number.isInteger(value.sibling_index) && Number(value.sibling_index) >= 0
    && (value.classes === undefined || (Array.isArray(value.classes) && value.classes.length <= 16 && value.classes.every((item) => typeof item === 'string')))
    && (value.key_attributes === undefined || (Array.isArray(value.key_attributes) && value.key_attributes.length <= 16 && value.key_attributes.every((item) => typeof item === 'string')))
    && (value.ancestors === undefined || (Array.isArray(value.ancestors) && value.ancestors.length <= 8));
}

function isDOMSelectionSnapshot(value: unknown): boolean {
  if (!isRecord(value) || !['element', 'region'].includes(String(value.kind)) || typeof value.slide_id !== 'string'
    || !Number.isInteger(value.html_revision) || typeof value.html_hash !== 'string' || value.status !== 'active'
    || !isRecord(value.canvas) || value.canvas.width !== 1920 || value.canvas.height !== 1080 || !isPositiveRect(value.rect)
    || !Array.isArray(value.dom_targets) || value.dom_targets.length > 50 || !Array.isArray(value.chrome_targets)
    || value.dom_targets.length + value.chrome_targets.length === 0) return false;
  const domValid = value.dom_targets.every((target) => isRecord(target) && typeof target.target_id === 'string' && target.target_id.length > 0
    && typeof target.tag === 'string' && target.tag.length > 0 && isPositiveRect(target.rect)
    && isFingerprint(target.fingerprint, target.tag)
    && Array.isArray(target.candidate_selectors) && target.candidate_selectors.length <= 3 && target.candidate_selectors.every((item) => typeof item === 'string')
    && isRecord(target.box_model) && isRect(target.box_model.content) && isEdges(target.box_model.padding) && isEdges(target.box_model.border) && isEdges(target.box_model.margin)
    && isStringRecord(target.attributes ?? {}) && isStringRecord(target.computed_style ?? {}));
  const chromeValid = value.chrome_targets.every((target) => isRecord(target) && ['page_number', 'section_marker', 'key_message', 'deck_title'].includes(String(target.type))
    && typeof target.placement === 'string' && typeof target.style === 'string' && typeof target.text === 'string' && isPositiveRect(target.rect));
  return domValid && chromeValid;
}

function isSessionMessage(value: Record<string, unknown>): boolean {
  return typeof value.session_id === 'string' && value.session_id.length > 0 && typeof value.slide_id === 'string' && value.slide_id.length > 0;
}

export function isRuntimeSlide(value: unknown): value is RuntimeSlide {
  return isRecord(value)
    && typeof value.id === 'string'
    && value.id.length > 0
    && typeof value.html === 'string'
    && isRecord(value.frame)
    && value.frame.slide_id === value.id
    && isRecord(value.frame.canvas)
    && value.frame.canvas.width === 1920
    && value.frame.canvas.height === 1080
    && value.frame.canvas.aspect_ratio === '16:9'
		&& (value.frame.project_id === undefined || typeof value.frame.project_id === 'string')
    && typeof value.frame.theme_id === 'string'
    && typeof value.frame.ordinal === 'number'
    && typeof value.frame.total === 'number';
}

export function isPreviewCommand(value: unknown): value is PreviewCommand {
  if (!isRecord(value) || typeof value.type !== 'string') return false;
  if (value.type === 'gotoSlide') return isIndex(value.index);
  if (value.type === 'setSelectionMode') return isSessionMessage(value) && ['element', 'region', 'none'].includes(String(value.mode)) && Number.isInteger(value.html_revision) && typeof value.html_hash === 'string';
  if (value.type === 'renderDraftSelections') return isSessionMessage(value) && Array.isArray(value.selections) && value.selections.every((item) => isRecord(item) && typeof item.selection_id === 'string' && Number.isInteger(item.marker_no) && isRect(item.rect) && ['active','content_deleted','page_deleted'].includes(String(item.status)));
  if (value.type === 'probeDraftSelections') return isSessionMessage(value) && Array.isArray(value.selections) && value.selections.length <= 8;
  if (value.type !== 'updateDeck' || !isIndex(value.index) || !Array.isArray(value.slides)) {
    return false;
  }
  return value.slides.every(isRuntimeSlide);
}

export function parseRuntimeEvent(value: unknown): RuntimeEvent | null {
  if (!isRecord(value) || typeof value.type !== 'string') return null;
  if (value.type === 'runtimeReady') return { type: 'runtimeReady' };
  if (value.type === 'selectionEmpty' || value.type === 'selectionRejected') {
    return isSessionMessage(value) && typeof value.message === 'string' ? value as RuntimeEvent : null;
  }
  if (value.type === 'selectionCreated') {
    if (!isSessionMessage(value) || !isDOMSelectionSnapshot(value.selection)) return null;
    const selection = value.selection as Record<string, unknown>;
    if (new TextEncoder().encode(JSON.stringify({ dom_targets: selection.dom_targets, chrome_targets: selection.chrome_targets })).byteLength > 128 * 1024) return null;
    return value as RuntimeEvent;
  }
  if (value.type === 'selectionCanceled' && isSessionMessage(value)) return value as RuntimeEvent;
  if (value.type === 'selectionPresence' && isSessionMessage(value) && Array.isArray(value.statuses) && value.statuses.every((item) => isRecord(item) && typeof item.selection_id === 'string' && (item.status === 'active' || item.status === 'content_deleted') && (item.targets === undefined || (Array.isArray(item.targets) && item.targets.every((target) => isRecord(target) && typeof target.target_id === 'string' && (target.status === 'active' || target.status === 'content_deleted')))))) return value as RuntimeEvent;
  if (value.type !== 'renderError' || typeof value.message !== 'string') return null;
  if (value.index !== undefined && !isIndex(value.index)) return null;
  return {
    type: 'renderError',
    message: value.message,
    ...(value.index === undefined ? {} : { index: value.index }),
  };
}

export function runtimeEventFromFrame(
  event: MessageEvent,
  frameWindow: Window | null,
): RuntimeEvent | null {
  if (!frameWindow || event.source !== frameWindow) return null;
  return parseRuntimeEvent(event.data);
}
