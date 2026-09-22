import { useComposerStore } from '../../stores/composerStore';
import { useActiveThreadId } from './useActiveSession';
import { useEffect, useRef } from 'react';
import { Undo2 } from 'lucide-react';
import { LongContent } from './LongContent';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogTitle } from '../../components/ui/dialog';
import { Button } from '../../components/ui/primitives';
import { projectHistoryApi } from '../../api/projectHistory';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { applyHistoryScene, reloadHistory, selectHistoryThread, useProjectHistoryStore } from '../../stores/projectHistoryStore';
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
  return <div className="mb-2 flex items-center justify-between gap-3 rounded-md border border-border bg-panel-muted px-3 py-2 text-xs text-text-600">
    <span>已回退 · 继续创作前会确认是否丢弃后续历史</span>
    <button type="button" disabled={busy} onClick={() => void useProjectHistoryStore.getState().preview(projectId)} className="shrink-0 rounded px-1 py-1 font-semibold text-accent hover:bg-surface disabled:opacity-40">恢复到最新</button>
  </div>;
}
export function ProjectHistoryDialogs() {
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
        if (previous?.id === projectId && previous.scene !== state.scene_revision) { reloadHistory(projectId, state); return; }
        seen.current = { id: projectId, scene: state.scene_revision };
        useProjectHistoryStore.setState((s) => ({
          states: { ...s.states, [projectId]: state },
          stateErrorByProjectId: { ...s.stateErrorByProjectId, [projectId]: false },
        }));
        selectHistoryThread(projectId);
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
      <DialogContent className="w-[calc(100vw-2rem)] min-w-0 max-w-[480px] max-h-[calc(100dvh-2rem)] overflow-y-auto gap-0" onEscapeKeyDown={(event) => { if (busy) event.preventDefault(); }} onInteractOutside={(event) => { if (busy) event.preventDefault(); }}>
        <DialogTitle className="leading-7">{dialog?.runId ? '回到此任务发送前？' : '恢复到最新现场？'}</DialogTitle>
        <DialogDescription className="mt-2 text-text-600 leading-[22px]">{dialog?.runId ? '项目内容和所有对话将一并回退。' : '恢复首次回退前的项目内容和所有对话，并用当时的草稿替换当前草稿。'}</DialogDescription>
        {preview && <section aria-label="回退目标与影响范围" className="mt-5 rounded-md bg-panel-muted px-4 py-3.5">
          <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1.5 text-xs leading-5 text-text-600 tabular-nums">
            <time dateTime={new Date(preview.time).toISOString()}>{new Date(preview.time).toLocaleString('sv-SE', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}</time>
            <span className="whitespace-nowrap">{dialog?.runId ? '撤回' : '恢复'} <strong className="text-sm font-semibold text-text-900">{preview.runs}</strong> 个任务{dialog?.runId ? '（含本次）' : ''}</span>
          </div>
          {dialog?.runId && <LongContent key={dialog.runId} className="mt-2" maxHeight={160} fadeClassName="from-panel-muted/0 via-panel-muted/90 to-panel-muted">
            <blockquote className="whitespace-pre-wrap break-words text-sm leading-[23px] text-text-900">{preview.input || '（无文字输入）'}</blockquote>
          </LongContent>}
        </section>}
        {dialog?.runId && <p className="mt-3.5 text-xs leading-5 text-text-600">这条指令将替换输入框草稿。回退后仍可恢复到最新状态。</p>}
        {error && <p role="alert" className="mt-3 text-sm text-danger">{error}</p>}
        <DialogFooter className="mt-6 flex-row justify-end gap-2 sm:space-x-0"><Button variant="secondary" disabled={busy} onClick={close}>取消</Button><Button variant="primary" disabled={busy} onClick={() => void useProjectHistoryStore.getState().execute()}>{busy ? '正在恢复项目…' : dialog?.runId ? '确认回退' : '确认恢复'}</Button></DialogFooter>
      </DialogContent>
    </Dialog>
    <Dialog open={Boolean(pending)} onOpenChange={(open) => { if (!open) pending?.resolve(false); }}>
      <DialogContent>
        <DialogTitle>丢弃原来的后续历史？</DialogTitle>
        <DialogDescription className="text-text-600">{pending?.message} 取消会保留当前草稿和恢复入口。</DialogDescription>
        <DialogFooter><Button variant="secondary" onClick={() => pending?.resolve(false)}>取消</Button><Button variant="primary" onClick={() => pending?.resolve(true)}>丢弃并继续</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  </>;
}

export function RestoredInputResources() {
  const threadId = useActiveThreadId();
  const input = useComposerStore((s) => threadId ? s.restoredInputs[threadId] : undefined);
  if (!input || !threadId) return null;
  const update = (change: Partial<typeof input>) => useComposerStore.setState((s) => ({ restoredInputs: { ...s.restoredInputs, [threadId]: { ...input, ...change } } }));
  return <div className="mb-2 flex flex-wrap items-center gap-2 text-xs text-text-600">
    {input.scope && <span>已保留原始目标范围 <button type="button" className="rounded px-1 py-1 text-accent hover:bg-panel-muted" onClick={() => update({ scope: undefined })}>按当前选择更新</button></span>}
    {input.component_names?.map((name) => <button key={name} type="button" className="rounded border border-border px-2 py-1 hover:bg-panel-muted" title="移除组件引用" onClick={() => update({ component_names: input.component_names?.filter((value) => value !== name) })}>组件 · {name} · 移除</button>)}
    {input.mentioned_slide_ids?.map((id) => <button key={id} type="button" className="rounded border border-border px-2 py-1 hover:bg-panel-muted" title="移除页面引用" onClick={() => update({ mentioned_slide_ids: input.mentioned_slide_ids?.filter((value) => value !== id) })}>页面 · {id} · 移除</button>)}
  </div>;
}
