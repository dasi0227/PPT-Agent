import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { APIError } from '../api/client';
import { exportsApi, type ExportOperation } from '../api/exports';
import { isProjectExportBlocking, useExportStore } from './exportStore';

const stream = vi.hoisted(() => ({ messages: [] as Array<(operation: ExportOperation) => void>, close: vi.fn() }));
vi.mock('../api/exports', async (importOriginal) => ({
  ...await importOriginal<typeof import('../api/exports')>(),
  subscribeExport: (_id: string, message: (operation: ExportOperation) => void) => {
    stream.messages.push(message);
    return stream.close;
  },
}));

const operation = (id: string): ExportOperation => ({
  id, project_id: 'pro_one', format: 'png', status: 'ready', completed_pages: 1,
  total_pages: 1, warnings: [],
});

beforeEach(() => { stream.messages.length = 0; stream.close.mockClear(); useExportStore.setState({ session: null }); });
afterEach(() => { useExportStore.getState().session?.streamClose?.(); useExportStore.setState({ session: null }); vi.restoreAllMocks(); });

test('an expired task unlocks the page and retry creates a fresh export', async () => {
  const create = vi.spyOn(exportsApi, 'create').mockResolvedValueOnce(operation('exp_old')).mockResolvedValueOnce(operation('exp_new'));
  vi.spyOn(exportsApi, 'get').mockRejectedValue(new APIError(410, 'EXPORT_CONSUMED', '导出文件已失效'));
  await useExportStore.getState().start('pro_one', 'png');
  await useExportStore.getState().refresh('exp_old');
  expect(isProjectExportBlocking('pro_one')).toBe(false);
  expect(stream.close).toHaveBeenCalled();
  await useExportStore.getState().retry();
  expect(useExportStore.getState().session?.operation.id).toBe('exp_new');
  expect(create.mock.calls[0][2]).not.toBe(create.mock.calls[1][2]);
  stream.messages[0]({ ...operation('exp_old'), status: 'consumed' });
  expect(useExportStore.getState().session?.operation.id).toBe('exp_new');
});

test('retry retains the original task when cancellation fails instead of creating another export', async () => {
  const create = vi.spyOn(exportsApi, 'create').mockResolvedValue(operation('exp_one'));
  vi.spyOn(exportsApi, 'cancel').mockRejectedValue(new APIError(500, 'INTERNAL', '清理失败'));
  await useExportStore.getState().start('pro_one', 'png');
  stream.messages[0]({ ...operation('exp_one'), status: 'failed' });
  await useExportStore.getState().retry();
  expect(create).toHaveBeenCalledTimes(1);
  expect(useExportStore.getState().session?.operation).toMatchObject({ id: 'exp_one', status: 'failed' });
});

test('termination requested during creation cancels the returned task before allowing a fresh export', async () => {
  let resolveCreate!: (value: ExportOperation) => void;
  const create = vi.spyOn(exportsApi, 'create')
    .mockReturnValueOnce(new Promise<ExportOperation>((resolve) => { resolveCreate = resolve; }))
    .mockResolvedValueOnce(operation('exp_new'));
  const cancel = vi.spyOn(exportsApi, 'cancel').mockResolvedValue(undefined);
  const starting = useExportStore.getState().start('pro_one', 'png');
  await useExportStore.getState().cancel();
  expect(useExportStore.getState().session?.canceling).toBe(true);
  expect(isProjectExportBlocking('pro_one')).toBe(true);
  expect(cancel).not.toHaveBeenCalled();
  resolveCreate(operation('exp_old'));
  await starting;
  expect(cancel).toHaveBeenCalledWith('exp_old');
  expect(useExportStore.getState().session).toBeNull();
  expect(isProjectExportBlocking('pro_one')).toBe(false);
  await useExportStore.getState().start('pro_one', 'png');
  expect(create.mock.calls[0][2]).not.toBe(create.mock.calls[1][2]);
  stream.messages[0]({ ...operation('exp_old'), status: 'ready' });
  expect(useExportStore.getState().session?.operation.id).toBe('exp_new');
});
