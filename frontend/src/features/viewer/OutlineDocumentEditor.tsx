import type { ReactNode } from 'react';
import type { ProjectContentSnapshot } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';
import { OutlineEditor } from './OutlineEditor';
import { outlineMutation } from './outlineEditing';

export function OutlineDocumentEditor({ snapshot, blocked, notice }: { snapshot: ProjectContentSnapshot; blocked?: string; notice?: ReactNode }) {
  const version = `${snapshot.hashes.outline ?? ''}:${snapshot.scene_revision ?? ''}`;
  return <OutlineEditor value={snapshot.outline} version={version} blocked={blocked} notice={notice}
    openSlide={id => {
      useDeckStore.getState().setCurrentSlideId(id);
      useDeckStore.getState().setGlobalView('outline');
    }}
    commit={async (command, capturedVersion) => {
      const store = useProjectStore.getState();
      const latest = store.contentByProjectId[snapshot.project_id];
      if (blocked) throw new Error(blocked);
      if (!latest || `${latest.hashes.outline ?? ''}:${latest.scene_revision ?? ''}` !== capturedVersion) throw new Error('目录已更新，请取消后重新编辑。');
      try {
        await store.mutateProject(snapshot.project_id, { ...outlineMutation(command), expected_hash: latest.hashes.outline, expected_scene_revision: latest.scene_revision });
      } catch (cause) {
        void store.loadProjectContent(snapshot.project_id);
        throw cause;
      }
    }} />;
}
