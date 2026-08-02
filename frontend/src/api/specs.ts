import { fetchClient } from './client';
import type { Design, Materialization, Outline, SlideSpec, SpecProjectView } from './types';

export const specsApi = {
  getProject: (projectId: string) => fetchClient<SpecProjectView>(`/projects/${projectId}/spec`),
  patchOutline: (projectId: string, expectedRevision: number, outline: Outline) =>
    fetchClient<Outline>(`/projects/${projectId}/spec`, {
      method: 'PATCH',
      body: JSON.stringify({ expected_revision: expectedRevision, outline }),
    }),
  patchDesign: (projectId: string, expectedRevision: number, design: Design) =>
    fetchClient<Design>(`/projects/${projectId}/spec/design`, {
      method: 'PATCH',
      body: JSON.stringify({ expected_revision: expectedRevision, design }),
    }),
  getSlide: (slideId: string) =>
    fetchClient<{ spec: SlideSpec; materialization: Materialization }>(`/slides/${slideId}/spec`),
  patchSlide: (slideId: string, expectedRevision: number, spec: SlideSpec) =>
    fetchClient<SlideSpec>(`/slides/${slideId}/spec`, {
      method: 'PATCH',
      body: JSON.stringify({ expected_revision: expectedRevision, spec }),
    }),
};
