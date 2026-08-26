export interface RuntimeSlide {
  id: string;
  html: string;
  frame: import('./runtimeFrame').RuntimeFrameContext;
}

export type PreviewCommand =
  | { type: 'updateDeck'; slides: RuntimeSlide[]; index: number }
  | { type: 'gotoSlide'; index: number };

export type RuntimeEvent =
  | { type: 'runtimeReady' }
  | { type: 'renderError'; message: string; index?: number };

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isIndex(value: unknown): value is number {
  return Number.isInteger(value) && (value as number) >= 0;
}

export function isRuntimeSlide(value: unknown): value is RuntimeSlide {
  return isRecord(value)
    && typeof value.id === 'string'
    && value.id.length > 0
    && typeof value.html === 'string'
    && isRecord(value.frame)
    && value.frame.slide_id === value.id
    && typeof value.frame.ordinal === 'number'
    && typeof value.frame.total === 'number';
}

export function isPreviewCommand(value: unknown): value is PreviewCommand {
  if (!isRecord(value) || typeof value.type !== 'string') return false;
  if (value.type === 'gotoSlide') return isIndex(value.index);
  if (value.type !== 'updateDeck' || !isIndex(value.index) || !Array.isArray(value.slides)) {
    return false;
  }
  return value.slides.every(isRuntimeSlide);
}

export function parseRuntimeEvent(value: unknown): RuntimeEvent | null {
  if (!isRecord(value) || typeof value.type !== 'string') return null;
  if (value.type === 'runtimeReady') return { type: 'runtimeReady' };
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
