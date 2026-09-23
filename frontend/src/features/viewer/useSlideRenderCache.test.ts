import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Slide } from '../../api/types';
import { APIError } from '../../api/client';
import { slidesApi } from '../../api/slides';
import { clearSlideRenderCache, useSlideRenderCache } from './useSlideRenderCache';

const { refresh } = vi.hoisted(() => ({ refresh: vi.fn().mockResolvedValue(undefined) }));
vi.mock('../../stores/projectStore', () => ({ useProjectStore: { getState: () => ({ loadProjectContent: refresh }) } }));
vi.mock('../../api/slides', () => ({ slidesApi: { render: vi.fn() } }));

describe('preview content identity', () => {
  beforeEach(() => { clearSlideRenderCache(); vi.clearAllMocks(); });

  it('refreshes a stale snapshot without caching the conflicting response', async () => {
    const oldSlide = { id: 's1', html_path: 'slides/s1/index.html', html_hash: 'sha256:old' } as Slide;
    const newSlide = { ...oldSlide, html_hash: 'sha256:new' };
    vi.mocked(slidesApi.render).mockRejectedValueOnce(new APIError(409, 'CONTENT_CONFLICT', '页面内容已更新'));
    const { result } = renderHook(() => useSlideRenderCache('p1'));
    await act(async () => { await result.current.load(oldSlide); });
    expect(slidesApi.render).toHaveBeenCalledWith('s1', oldSlide.html_hash, expect.any(AbortSignal));
    expect(result.current.getState(oldSlide).status).toBe('error');
    expect(refresh).toHaveBeenCalledWith('p1');

    vi.mocked(slidesApi.render).mockResolvedValueOnce('<h1>new</h1>');
    await act(async () => { await result.current.load(newSlide); });
    expect(result.current.getState(newSlide)).toEqual({ status: 'ready', data: '<h1>new</h1>' });
    expect(result.current.getState(oldSlide).status).toBe('error');
  });
});
