import { fetchClient } from './client';
import { Slide } from './types';

export interface SlidePatch {
  title?: string;
  subtitle?: string;
  bullets?: string[];
  content_intent?: string;
  chart_intent?: { type: string; data_hint?: string };
  layout?: string;
  steps?: number;
}

export const slidesApi = {
  get: (id: string) => fetchClient<Slide>(`/slides/${id}`),
  patch: (id: string, patch: SlidePatch) =>
    fetchClient<Slide>(`/slides/${id}`, { method: 'PATCH', body: JSON.stringify(patch) }),
};
