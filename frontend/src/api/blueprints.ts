import { fetchClient } from './client';
import type { BlueprintProjectView, DeckBlueprint, DesignSpec, Materialization, SlideBlueprint } from './types';

export const blueprintsApi = {
  getProject: (projectId: string) => fetchClient<BlueprintProjectView>(`/projects/${projectId}/blueprint`),
  patchProject: (projectId: string, expectedRevision: number, deck: DeckBlueprint) =>
    fetchClient<DeckBlueprint>(`/projects/${projectId}/blueprint`, {
      method: 'PATCH',
      body: JSON.stringify({ expected_revision: expectedRevision, deck }),
    }),
  patchDesignSpec: (projectId: string, expectedRevision: number, designSpec: DesignSpec) =>
    fetchClient<DesignSpec>(`/projects/${projectId}/blueprint/design`, {
      method: 'PATCH',
      body: JSON.stringify({ expected_revision: expectedRevision, design_spec: designSpec }),
    }),
  getSlide: (slideId: string) =>
    fetchClient<{ blueprint: SlideBlueprint; materialization: Materialization }>(`/slides/${slideId}/blueprint`),
  patchSlide: (slideId: string, expectedRevision: number, blueprint: SlideBlueprint) =>
    fetchClient<SlideBlueprint>(`/slides/${slideId}/blueprint`, {
      method: 'PATCH',
      body: JSON.stringify({ expected_revision: expectedRevision, blueprint }),
    }),
};
