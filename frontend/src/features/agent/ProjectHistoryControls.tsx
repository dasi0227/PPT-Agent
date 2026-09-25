import { useComposerStore } from '../../stores/composerStore';
import { useActiveThreadId } from './useActiveSession';
import { useEffect, useId, useRef } from 'react';
import { Undo2 } from 'lucide-react';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogTitle } from '../../components/ui/dialog';
import { Button } from '../../components/ui/primitives';
import { ConfirmModal } from '../../components/ui/modal-confirm';
import { projectHistoryApi } from '../../api/projectHistory';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { applyHistoryScene, consumeHistoryNotice, reloadHistory, selectHistoryThread, useProjectHistoryStore } from '../../stores/projectHistoryStore';
import { useHistoryConfirmationStore } from '../../stores/historyConfirmationStore';

export function RollbackButton({ runId, steering }: { runId?: string; steering?: boolean }) {
  const projectId = useProjectStore((s) => s.activeProjectId);
  const state = useProjectHistoryStore((s) => projectId ? s.states[projectId] : undefined);
  const busy = useProjectHistoryStore((s) => s.busy);
  if (!projectId || !runId || steering || !state?.checkpoints.some((cp) => cp.run_id === runId)) return null;
  return <button type="button" aria-label="回退到此消息发送前" title="回退到此消息发送前" disabled={busy}
    className="inline-flex h-6 w-6 items-center justify-center rounded-md hover:bg-panel-muted hover:text-text-900 disabled:opacity-40 focus-visible:outline-none"
    onClick={() => void useProjectHistoryStore.getState().preview(projectId, runId)}><Undo2 className="h-3.5 w-3.5" /></button>;
}
export function HistoryBanner() {
  const projectId = useProjectStore((s) => s.activeProjectId);
  const state = useProjectHistoryStore((s) => projectId ? s.states[projectId] : undefined);
  const busy = useProjectHistoryStore((s) => s.busy);
  if (!projectId || !state?.latest) return null;
  return <div className="mb-2 flex justify-center">
    <Button type="button" variant="primary" disabled={busy} onClick={() => void useProjectHistoryStore.getState().preview(projectId)} className="h-7 rounded-full px-3 text-xs">恢复到最新</Button>
  </div>;
}
export function ProjectHistoryDialogs() {
  const descriptionId = useId();
  const impactId = useId();
  const projectId = useProjectStore((s) => s.activeProjectId);
  const threads = useThreadStore((s) => projectId ? s.threadsByProjectId[projectId] : undefined);
  const { dialog, busy, error } = useProjectHistoryStore();
  const pending = useHistoryConfirmationStore((s) => s.pending);
  const seen = useRef<{ id: string; scene: number } | null>(null);
  useEffect(() => {
    if (!projectId) return;
    let stopped = false;
    let loading = false;
    const refresh = async () => {
      if (loading || useProjectHistoryStore.getState().busy) return;
      loading = true;
      try {
        const state = await projectHistoryApi.state(projectId);
        if (stopped) return;
        const previous = seen.current;
        applyHistoryScene(projectId, state);
        if (previous?.id === projectId && previous.scene !== state.scene_revision) {
          reloadHistory(projectId, state); return;
        }
        seen.current = { id: projectId, scene: state.scene_revision };
        useProjectHistoryStore.setState((s) => ({
          states: { ...s.states, [projectId]: state },
          stateErrorByProjectId: { ...s.stateErrorByProjectId, [projectId]: false },
        }));
        selectHistoryThread(projectId);
        consumeHistoryNotice(projectId, state);
      } catch {
        useProjectHistoryStore.setState((s) => ({
          stateErrorByProjectId: { ...s.stateErrorByProjectId, [projectId]: true },
        }));
      }
      finally { loading = false; }
    };
    void refresh();
    const interval = window.setInterval(() => void refresh(), 3000);
    window.addEventListener('focus', refresh);
    return () => { stopped = true; window.clearInterval(interval); window.removeEventListener('focus', refresh); };
  }, [projectId]);
  useEffect(() => { if (projectId && threads) selectHistoryThread(projectId); }, [projectId, threads]);
  const close = () => { if (!busy) useProjectHistoryStore.setState({ dialog: null, error: null }); };
  const preview = dialog?.preview;
  return <>
    <Dialog open={Boolean(dialog)} onOpenChange={(open) => { if (!open) close(); }}>
      <DialogContent
        className="w-[calc(100vw-2rem)] min-w-0 max-w-[460px] max-h-[calc(100dvh-2rem)] gap-0 overflow-y-auto rounded-[10px] border-[#c7ced7] shadow-[0_16px_40px_-12px_rgb(23_32_43_/_20%)] max-[380px]:p-5"
        overlayClassName="bg-ink/[0.14] backdrop-blur-none"
        aria-describedby={preview ? `${descriptionId} ${impactId}` : descriptionId}
        onEscapeKeyDown={(event) => { if (busy) event.preventDefault(); }}
        onInteractOutside={(event) => { if (busy) event.preventDefault(); }}
      >
        <DialogTitle className="leading-[26px] tracking-[-0.2px]">{dialog?.runId ? '回到此任务发送前？' : '恢复到最新现场？'}</DialogTitle>
        <DialogDescription id={descriptionId} className="mt-1.5 text-[13px] leading-[21px] text-text-600">{dialog?.runId ? '项目和所有对话将一并回退。' : '恢复首次回退前的项目、对话和草稿。'}</DialogDescription>
        {preview && <section aria-label={dialog?.runId ? '目标任务' : '恢复目标'} className="mt-[22px] border-y border-border/60 pt-[17px] pb-5">
          <time className="block text-xs leading-[18px] text-text-600 tabular-nums" dateTime={new Date(preview.time).toISOString()}>{new Date(preview.time).toLocaleString('sv-SE', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}</time>
          <blockquote className="mt-2 line-clamp-3 max-h-[69px] [overflow-wrap:anywhere] text-sm font-normal leading-[23px] text-text-800">{preview.input.trim() || '（暂无输入）'}</blockquote>
        </section>}
        {error && <p role="alert" className="mt-3 text-sm text-danger">{error}</p>}
        <DialogFooter className="mt-5 flex-row items-center justify-between gap-4 sm:justify-between sm:space-x-0 max-[380px]:flex-wrap max-[380px]:gap-3">
          {preview && <p id={impactId} className="whitespace-nowrap text-xs leading-5 text-text-600">{dialog?.runId ? '撤回' : '恢复'} <strong className="font-semibold text-text-800 tabular-nums">{preview.runs}</strong> 个任务</p>}
          <div className="ml-auto flex gap-2">
            <Button variant="secondary" className="h-auto min-h-[34px] border-border bg-surface px-[13px] py-1.5 text-[13px] leading-5 text-text-700 focus-visible:underline focus-visible:underline-offset-4" disabled={busy} onClick={close}>取消</Button>
            <Button variant="primary" className="h-auto min-h-[34px] border border-transparent px-[13px] py-1.5 text-[13px] leading-5 focus-visible:underline focus-visible:underline-offset-4" disabled={busy} onClick={() => void useProjectHistoryStore.getState().execute()}>{busy ? '正在恢复项目…' : dialog?.runId ? '确认回退' : '确认恢复'}</Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
    <ConfirmModal
      open={Boolean(pending)}
      onOpenChange={(open) => {
        if (!open && pending && useHistoryConfirmationStore.getState().pending === pending) pending.resolve(false);
      }}
      title="丢弃原来的后续历史？"
      description="此操作不可撤销。继续创作将丢弃原来的后续历史，无法再恢复到最新现场。"
      confirmLabel="丢弃并继续"
      onConfirm={() => pending?.resolve(true)}
    />
  </>;
}

export function RestoredInputResources() {
  const threadId = useActiveThreadId();
  const input = useComposerStore((s) => threadId ? s.restoredInputs[threadId] : undefined);
  if (!input || !threadId || (!input.component_names?.length && !input.mentioned_slide_ids?.length)) return null;
  const update = (change: Partial<typeof input>) => useComposerStore.setState((s) => ({ restoredInputs: { ...s.restoredInputs, [threadId]: { ...input, ...change } } }));
  return <div className="mb-2 flex flex-wrap items-center gap-2 text-xs text-text-600">
    {input.component_names?.map((name) => <button key={name} type="button" className="rounded border border-border px-2 py-1 hover:bg-panel-muted" title="移除组件引用" onClick={() => update({ component_names: input.component_names?.filter((value) => value !== name) })}>组件 · {name} · 移除</button>)}
    {input.mentioned_slide_ids?.map((id) => <button key={id} type="button" className="rounded border border-border px-2 py-1 hover:bg-panel-muted" title="移除页面引用" onClick={() => update({ mentioned_slide_ids: input.mentioned_slide_ids?.filter((value) => value !== id) })}>页面 · {id} · 移除</button>)}
  </div>;
}
