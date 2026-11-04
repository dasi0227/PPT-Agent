import { useCallback, useEffect, useRef, useState } from 'react';
import type { ProjectContentSnapshot, Slide } from '../../api/types';
import { APIError, RequestCanceledError } from '../../api/client';
import { slidesApi } from '../../api/slides';
import { useProjectStore } from '../../stores/projectStore';
import { isProjectDeleted } from '../../stores/deletedTargets';

export type ResourceState<T> =
  | { status: 'idle' }
  | { status: 'loading'; previous?: T }
  | { status: 'ready'; data: T }
  | { status: 'error'; error: Error; previous?: T };

const MAX_HTML_ENTRIES = 24;
const MAX_HTML_BYTES = 16 * 1024 * 1024;
const MAX_LOCAL_STATES = 5;
const htmlCache = new Map<string, { html: string; bytes: number }>();
let cachedBytes = 0;

function forgetHTML(key: string) {
  const entry = htmlCache.get(key);
  if (!entry) return;
  cachedBytes -= entry.bytes;
  htmlCache.delete(key);
}

function rememberHTML(key: string, html: string) {
  const prefix = key.slice(0, key.indexOf(':sha256:') + 1);
  for (const candidate of htmlCache.keys()) {
    if (candidate === key || (prefix && candidate.startsWith(prefix))) forgetHTML(candidate);
  }
  const bytes = html.length * 2;
  if (bytes > MAX_HTML_BYTES) return;
  htmlCache.set(key, { html, bytes });
  cachedBytes += bytes;
  while (htmlCache.size > MAX_HTML_ENTRIES || cachedBytes > MAX_HTML_BYTES) {
    const oldest = htmlCache.keys().next().value;
    if (oldest === undefined) break;
    forgetHTML(oldest);
  }
}

export function hasRenderedHTML(slide: Slide): boolean {
  return Boolean(slide.html_path) && Boolean(slide.html_hash);
}

export function slideRenderKey(projectId: string, slide: Slide): string {
  return `${projectId}:${slide.id}:${slide.html_hash}`;
}

export function useSlideRenderCache(projectId: string | null) {
  const [states, setStates] = useState<Record<string, ResourceState<string>>>({});
  const statesRef = useRef(states);
  const controllers = useRef(new Map<string, AbortController>());
  const pending = useRef(new Map<string, Promise<void>>());
  const currentRequestKey = useRef<string | null>(null);
  const activeKey = useRef<string | null>(null);
  const currentProject = useRef(projectId);
  const mounted = useRef(false);
  currentProject.current = projectId;

  const publish = useCallback((key: string, state: ResourceState<string>) => {
    const next = { ...statesRef.current };
    // Keep one version per page and at most five local states, including the active page.
    const pagePrefix = key.slice(0, key.indexOf(':sha256:') + 1);
    for (const candidate of Object.keys(next)) {
      if (candidate === key || (pagePrefix && candidate.startsWith(pagePrefix))) delete next[candidate];
    }
    next[key] = state;
    for (const candidate of Object.keys(next)) {
      if (Object.keys(next).length <= MAX_LOCAL_STATES) break;
      if (candidate !== activeKey.current) delete next[candidate];
    }
    statesRef.current = next;
    setStates(next);
  }, []);

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      controllers.current.forEach(controller => controller.abort());
      controllers.current.clear();
      pending.current.clear();
    };
  }, []);

  useEffect(() => {
    controllers.current.forEach(controller => controller.abort());
    controllers.current.clear();
    pending.current.clear();
    currentRequestKey.current = null;
    activeKey.current = null;
    statesRef.current = {};
    setStates({});
  }, [projectId]);

  useEffect(() => {
    if (!projectId) return;
    const prune = (snapshot: ProjectContentSnapshot | undefined, sceneChanged = false) => {
      if (!snapshot && !isProjectDeleted(projectId)) return;
      const valid = new Set(Object.entries(snapshot?.slides_by_id ?? {}).flatMap(([id, slide]) =>
        slide.html_hash ? [`${projectId}:${id}:${slide.html_hash}`] : []));
      for (const key of htmlCache.keys()) {
        if (key.startsWith(`${projectId}:`) && !valid.has(key)) forgetHTML(key);
      }
      for (const [key, controller] of controllers.current) {
        if (sceneChanged || !valid.has(key)) {
          controller.abort();
          controllers.current.delete(key);
          pending.current.delete(key);
        }
      }
      const next = Object.fromEntries(Object.entries(statesRef.current).filter(([key, value]) => valid.has(key) && (!sceneChanged || value.status === 'ready')));
      if (Object.keys(next).length !== Object.keys(statesRef.current).length) {
        statesRef.current = next;
        setStates(next);
      }
    };
    prune(useProjectStore.getState().contentByProjectId[projectId]);
    return useProjectStore.subscribe((state, previous) => {
      if (state.contentByProjectId[projectId] !== previous.contentByProjectId[projectId] || state.projects !== previous.projects) {
        const snapshot = state.contentByProjectId[projectId];
        prune(isProjectDeleted(projectId) ? undefined : snapshot,
          snapshot?.scene_revision !== previous.contentByProjectId[projectId]?.scene_revision);
      }
    });
  }, [projectId]);

  const load = useCallback(async (slide: Slide, priority: 'current' | 'prefetch' = 'prefetch') => {
    if (!projectId || currentProject.current !== projectId || !mounted.current || !hasRenderedHTML(slide) || isProjectDeleted(projectId)) return;
    if (useProjectStore.getState().contentByProjectId[projectId]?.slides_by_id[slide.id]?.html_hash !== slide.html_hash) return;
    const key = slideRenderKey(projectId, slide);
    if (priority === 'current') {
      activeKey.current = key;
      if (currentRequestKey.current && currentRequestKey.current !== key) {
        controllers.current.get(currentRequestKey.current)?.abort();
        controllers.current.delete(currentRequestKey.current);
        pending.current.delete(currentRequestKey.current);
      }
      currentRequestKey.current = key;
    }
    const cached = htmlCache.get(key);
    if (cached) {
      htmlCache.delete(key);
      htmlCache.set(key, cached);
      if (statesRef.current[key]?.status !== 'ready') publish(key, { status: 'ready', data: cached.html });
      return;
    }
    const existing = pending.current.get(key);
    if (existing) return existing;
    if (statesRef.current[key]?.status === 'ready') return;

    const previousState = Object.entries(statesRef.current).find(([candidate]) =>
      candidate.startsWith(`${projectId}:${slide.id}:`) && candidate !== key)?.[1];
    const previous = previousState?.status === 'ready' ? previousState.data
      : previousState && 'previous' in previousState ? previousState.previous : undefined;
    const controller = new AbortController();
    controllers.current.set(key, controller);
    publish(key, { status: 'loading', previous });
    const isCurrent = () => mounted.current && currentProject.current === projectId && !controller.signal.aborted
      && !isProjectDeleted(projectId)
      && useProjectStore.getState().contentByProjectId[projectId]?.slides_by_id[slide.id]?.html_hash === slide.html_hash;
    const request = (async () => {
      try {
        const html = await slidesApi.render(slide.id, slide.html_hash, controller.signal);
        if (!isCurrent()) return;
        rememberHTML(key, html);
        publish(key, { status: 'ready', data: html });
      } catch (error) {
        if (!isCurrent()) return;
        if (error instanceof RequestCanceledError) {
          publish(key, { status: 'idle' });
          return;
        }
        publish(key, { status: 'error', error: error instanceof Error ? error : new Error('HTML 加载失败'), previous });
        if (error instanceof APIError && error.code === 'CONTENT_CONFLICT') {
          void useProjectStore.getState().loadProjectContent(projectId);
        }
      } finally {
        if (controllers.current.get(key) === controller) {
          controllers.current.delete(key);
          pending.current.delete(key);
          if (currentRequestKey.current === key) currentRequestKey.current = null;
        }
      }
    })();
    pending.current.set(key, request);
    return request;
  }, [projectId, publish]);

  const getState = useCallback((slide: Slide): ResourceState<string> => {
    if (!projectId) return { status: 'idle' };
    const key = slideRenderKey(projectId, slide);
    const cached = htmlCache.get(key);
    if (cached) return { status: 'ready', data: cached.html };
    return states[key] ?? { status: 'idle' };
  }, [projectId, states]);

  return { getState, load };
}
