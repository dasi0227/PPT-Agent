import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { projectHistoryApi } from '../api/projectHistory';
import { slideSourcesApi, type SourceKind, type SlideSourceDocument } from '../api/slideSources';
import { assertProjectSourcesSaved, showSourceFile, sourceKey, useSourceEditorStore } from './sourceEditorStore';
import { useDeckStore } from './deckStore';
import { useProjectStore } from './projectStore';
import { sourceDraftId, sourceSessionId, type StoredSourceDraft } from '../lib/sourceDraftStorage';
import { APIError, NetworkError } from '../api/client';

vi.mock('../lib/sourceDraftStorage', async (importOriginal) => ({
  ...await importOriginal<typeof import('../lib/sourceDraftStorage')>(),
  enqueueSourceDraft: async () => {},
  flushSourceDrafts: async () => {},
  listSourceDrafts: async () => [],
  backupProjectSourceDrafts: async () => {},
}));

describe('project source launch gate', () => {
  beforeEach(() => {
    vi.spyOn(projectHistoryApi, 'state').mockResolvedValue({ revision: 1, scene_revision: 0, checkpoints: [] });
    useSourceEditorStore.setState({ files: {}, drafts: {}, indexReady: true, indexError: null });
  });
  afterEach(() => vi.restoreAllMocks());

  it.each(['manifest', 'design'] as const)('keeps %s drafts in the project launch gate and opens the right document', async (kind) => {
    const id = sourceDraftId('project', '', kind);
    useSourceEditorStore.setState({ drafts: { [id]: {
      id, sessionId: sourceSessionId(), projectId: 'project', slideId: '', kind,
      sceneRevision: 0, baseSourceHash: 'sha256:old', baseText: '{}', draftText: '{"changed":true}', updatedAt: 1,
    } } });
    await expect(assertProjectSourcesSaved('project')).rejects.toThrow('有 1 个源文件尚未保存');
    useDeckStore.setState({ currentSlideId: null, activeDocument: null, contentMode: 'preview' });
    showSourceFile({ slideId: '', kind });
    expect(useDeckStore.getState()).toMatchObject({ currentSlideId: null, activeDocument: kind, contentMode: 'source' });
  });

  it('formats project JSON during save-all and stops on the failed project document', async () => {
    const documents = new Map<SourceKind, SlideSourceDocument>();
    const drafts: Record<string, StoredSourceDraft> = {};
    for (const kind of ['design', 'manifest'] as const) {
      const id = sourceDraftId('project', '', kind);
      drafts[id] = { id, sessionId: sourceSessionId(), projectId: 'project', slideId: '', kind,
        sceneRevision: 0, baseSourceHash: kind, baseText: '{"value":1}', draftText: '{"value":2}', updatedAt: 1 };
      documents.set(kind, { project_id: 'project', slide_id: '', kind, path: `${kind}.json`, language: 'json',
        content: drafts[id].baseText, source_hash: kind, content_hash: kind, scene_revision: 0, writable: true, readonly_reason: null });
    }
    useSourceEditorStore.setState({ drafts });
    vi.spyOn(slideSourcesApi, 'get').mockImplementation(async (_project, _slide, kind) => documents.get(kind)!);
    vi.spyOn(useProjectStore.getState(), 'loadProjectContent').mockResolvedValue();
    const save = vi.spyOn(slideSourcesApi, 'save').mockImplementation(async (_project, _slide, kind, text) => {
      if (kind === 'design') throw new APIError(422, 'SOURCE_VALIDATION_FAILED', '装饰位置无效');
      return { changed: true, document: { ...documents.get(kind)!, content: text, source_hash: 'saved' } };
    });
    expect(await useSourceEditorStore.getState().saveAll('project')).toBe(false);
    expect(save.mock.calls.map((call) => call[2])).toEqual(['manifest', 'design']);
    expect(save.mock.calls[0][3]).toBe('{\n  "value": 2\n}\n');
    expect(useSourceEditorStore.getState().files[sourceKey('project', '', 'manifest')].baseSourceHash).toBe('saved');
    expect(useSourceEditorStore.getState().files[sourceKey('project', '', 'design')].draftText).toBe('{"value":2}');
    expect(useDeckStore.getState()).toMatchObject({ activeDocument: 'design', contentMode: 'source' });
  });

  it('blocks a draft on another page and permits launch once it is cleared', async () => {
    const id = sourceDraftId('project', 'another-slide', 'html');
    const draft: StoredSourceDraft = {
      id, sessionId: sourceSessionId(), projectId: 'project', slideId: 'another-slide', kind: 'html',
      sceneRevision: 0, baseSourceHash: 'sha256:old', baseText: 'old', draftText: 'new', updatedAt: 1,
    };
    useSourceEditorStore.setState({ drafts: { [id]: draft } });
    await expect(assertProjectSourcesSaved('project')).rejects.toThrow('有 1 个源文件尚未保存');
    useSourceEditorStore.setState({ drafts: {} });
    await expect(assertProjectSourcesSaved('project')).resolves.toBeUndefined();
  });

  it('loads the newest external source and removes the old draft and undo identity', async () => {
    const id = sourceDraftId('project', 'slide', 'html');
    const draft: StoredSourceDraft = {
      id, sessionId: sourceSessionId(), projectId: 'project', slideId: 'slide', kind: 'html',
      sceneRevision: 0, baseSourceHash: 'sha256:old', baseText: '<old>', draftText: '<local>', updatedAt: 1,
    };
    useSourceEditorStore.setState({ drafts: { [id]: draft }, files: { 'project:slide:html': {
      projectId: 'project', slideId: 'slide', kind: 'html', phase: 'ready', sceneRevision: 0,
      baseSourceHash: 'sha256:old', baseContentHash: 'sha256:old', baseText: '<old>', draftText: '<local>',
      editGeneration: 1, resetVersion: 2, writable: true, readonlyReason: null, draftPersistence: 'saved',
    } } });
    vi.spyOn(slideSourcesApi, 'get').mockResolvedValue({
      project_id: 'project', slide_id: 'slide', kind: 'html', path: 'slides/slide/index.html', language: 'html',
      content: '<new>', source_hash: 'sha256:new', content_hash: 'sha256:new', scene_revision: 0,
      writable: true, readonly_reason: null,
    });
    await useSourceEditorStore.getState().load('project', 'slide', 'html', true);
    const result = useSourceEditorStore.getState();
    expect(result.files['project:slide:html']).toMatchObject({ baseText: '<new>', draftText: '<new>', resetVersion: 3 });
    expect(result.drafts[id]).toBeUndefined();
  });

  it('re-reads an uncertain save result before allowing another edit', async () => {
    const key = 'project:slide:spec';
    useSourceEditorStore.setState({ files: { [key]: {
      projectId: 'project', slideId: 'slide', kind: 'spec', phase: 'ready', sceneRevision: 0,
      baseSourceHash: 'sha256:old', baseContentHash: 'sha256:old', baseText: '{"a":1}', draftText: '{"a":2}',
      editGeneration: 1, resetVersion: 1, writable: true, readonlyReason: null, draftPersistence: 'saved',
    } } });
    vi.spyOn(slideSourcesApi, 'save').mockRejectedValue(new NetworkError(new Error('connection lost')));
    vi.spyOn(slideSourcesApi, 'get').mockResolvedValue({
      project_id: 'project', slide_id: 'slide', kind: 'spec', path: 'slides/slide/spec.json', language: 'json',
      content: '{"a":1}', source_hash: 'sha256:old', content_hash: 'sha256:old', scene_revision: 0,
      writable: true, readonly_reason: null,
    });
    expect(await useSourceEditorStore.getState().save(key)).toBe(false);
    expect(useSourceEditorStore.getState().files[key]).toMatchObject({
      phase: 'ready', draftText: '{"a":2}', baseSourceHash: 'sha256:old',
      error: '保存结果未确认，正式文件仍为原版本；请检查后重试',
    });
  });
});
