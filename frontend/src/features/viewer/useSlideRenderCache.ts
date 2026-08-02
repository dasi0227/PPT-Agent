import { useCallback, useEffect, useRef, useState } from 'react';
import type { Slide } from '../../api/types';
import { RequestCanceledError } from '../../api/client';
import { slidesApi } from '../../api/slides';

export type ResourceState<T> =
  | { status: 'idle' }
  | { status: 'loading'; previous?: T }
  | { status: 'ready'; data: T }
  | { status: 'error'; error: Error; previous?: T };

const htmlCache = new Map<string, string>();

function revisionOf(slide: Slide): number {
  return slide.html_revision ?? slide.current_version ?? 0;
}

export function hasRenderedHTML(slide: Slide): boolean {
  return Boolean(slide.html_path) && revisionOf(slide) > 0;
}

export function slideRenderKey(projectId: string, slide: Slide): string {
  return `${projectId}:${slide.id}:${revisionOf(slide)}`;
}

export function useSlideRenderCache(projectId: string | null) {
  const [states, setStates] = useState<Record<string, ResourceState<string>>>({});
  const statesRef = useRef(states);
  const controllers = useRef(new Map<string, AbortController>());
  const currentRequestKey = useRef<string | null>(null);

  useEffect(() => () => {
    controllers.current.forEach((controller) => controller.abort());
    controllers.current.clear();
  }, []);

  useEffect(() => {
    statesRef.current = states;
  }, [states]);

  useEffect(() => {
    controllers.current.forEach((controller) => controller.abort());
    controllers.current.clear();
    currentRequestKey.current = null;
    setStates({});
  }, [projectId]);

  const load = useCallback(async (slide: Slide, priority: 'current' | 'prefetch' = 'prefetch') => {
    if (!projectId || !hasRenderedHTML(slide)) return;
    const key = slideRenderKey(projectId, slide);
    if (htmlCache.has(key)) {
      setStates((current) => current[key]?.status === 'ready'
        ? current
        : { ...current, [key]: { status: 'ready', data: htmlCache.get(key)! } });
      return;
    }
    if (controllers.current.has(key)) return;

    if (priority === 'current' && currentRequestKey.current && currentRequestKey.current !== key) {
      controllers.current.get(currentRequestKey.current)?.abort();
      controllers.current.delete(currentRequestKey.current);
    }
    if (priority === 'current') currentRequestKey.current = key;

    const previous = Object.entries(statesRef.current).find(([candidate]) =>
      candidate.startsWith(`${projectId}:${slide.id}:`) && candidate !== key
    )?.[1];
    const previousData = previous?.status === 'ready'
      ? previous.data
      : previous && 'previous' in previous
        ? previous.previous
        : undefined;
    const controller = new AbortController();
    controllers.current.set(key, controller);
    setStates((current) => ({ ...current, [key]: { status: 'loading', previous: previousData } }));
    try {
      const html = await slidesApi.render(slide.id, controller.signal);
      htmlCache.set(key, html);
      setStates((current) => ({ ...current, [key]: { status: 'ready', data: html } }));
    } catch (error) {
      if (error instanceof RequestCanceledError || controller.signal.aborted) return;
      setStates((current) => ({
        ...current,
        [key]: {
          status: 'error',
          error: error instanceof Error ? error : new Error('HTML 加载失败'),
          previous: previousData,
        },
      }));
    } finally {
      controllers.current.delete(key);
      if (currentRequestKey.current === key) currentRequestKey.current = null;
    }
  }, [projectId]);

  const getState = useCallback((slide: Slide): ResourceState<string> => {
    if (!projectId) return { status: 'idle' };
    const key = slideRenderKey(projectId, slide);
    const cached = htmlCache.get(key);
    if (cached !== undefined) return { status: 'ready', data: cached };
    return states[key] ?? { status: 'idle' };
  }, [projectId, states]);

  return { getState, load };
}

export function clearSlideRenderCache(): void {
  htmlCache.clear();
}
