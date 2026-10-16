import React, { useEffect, useState } from 'react';
import { Lightbulb, X } from 'lucide-react';
import { threadsApi } from '../../api/threads';
import { useThreadStore } from '../../stores/threadStore';
import { useRunStore } from '../../stores/runStore';
import { useProjectStore } from '../../stores/projectStore';

export const SuggestPanel: React.FC = () => {
  const target = useThreadStore(state => state.suggestPanelTarget);
  const thread = useThreadStore(state => target
    ? state.threadsByProjectId[target.projectId]?.find(item => item.id === target.threadId) : undefined);
  const activeProjectId = useProjectStore(state => state.activeProjectId);
  const activeThreadId = useThreadStore(state => target ? state.activeThreadIdByProjectId[target.projectId] : null);
  const close = useThreadStore(state => state.closeSuggestPanel);
  const setAutomatic = useThreadStore(state => state.setAutoSuggest);
  const session = useRunStore(state => target ? state.sessions[target.threadId] : undefined);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  useEffect(() => { setError(''); setBusy(false); }, [target]);
  useEffect(() => {
    if (target && (!thread || target.projectId !== activeProjectId || target.threadId !== activeThreadId)) close();
  }, [target, thread, activeProjectId, activeThreadId, close]);
  if (!target || !thread) return null;
  const canGenerate = Boolean(session?.activeRunId && ['done', 'error', 'canceled'].includes(session.status));
  const generating = busy || session?.nextInputSuggestions?.status === 'pending';
  const generate = async () => {
    if (generating || !canGenerate) return;
    setBusy(true); setError('');
    try {
      await threadsApi.suggest(thread.id);
      if (useThreadStore.getState().suggestPanelTarget === target) close();
    } catch (cause) {
      if (useThreadStore.getState().suggestPanelTarget === target)
        setError(cause instanceof Error ? cause.message : '生成失败，请重试');
    } finally {
      if (useThreadStore.getState().suggestPanelTarget === target) setBusy(false);
    }
  };
  const toggle = async () => {
    setBusy(true); setError('');
    try { await setAutomatic(target.projectId, thread.id, !thread.auto_suggest_enabled); }
    catch (cause) {
      if (useThreadStore.getState().suggestPanelTarget === target)
        setError(cause instanceof Error ? cause.message : '设置失败，请重试');
    } finally {
      if (useThreadStore.getState().suggestPanelTarget === target) setBusy(false);
    }
  };
  return (
    <div className="scrollbar-none mx-3 mb-2 max-h-[50dvh] shrink-0 overflow-y-auto rounded-2xl border border-border-strong bg-surface p-4" role="dialog" aria-label="后续建议设置">
      <div className="flex items-center gap-3">
        <Lightbulb className="h-4 w-4 shrink-0" strokeWidth={1.75} />
        <div className="text-sm font-semibold text-text-900">后续建议</div>
        <div className="ml-auto flex items-center gap-2">
          <span className="text-xs text-text-600">自动建议</span>
          <button type="button" role="switch" aria-label="自动建议" aria-checked={thread.auto_suggest_enabled}
            disabled={busy} onClick={() => void toggle()} title={thread.auto_suggest_enabled ? '自动建议已开启' : '自动建议已关闭'}
            className={`h-5 w-9 shrink-0 rounded-full p-0.5 transition-colors disabled:opacity-50 ${thread.auto_suggest_enabled ? 'bg-accent' : 'bg-border-strong'}`}>
            <span className={`block h-4 w-4 rounded-full bg-white shadow-sm transition-transform ${thread.auto_suggest_enabled ? 'translate-x-4' : 'translate-x-0'}`} />
          </button>
        </div>
        <button type="button" onClick={close} className="inline-flex h-7 w-7 items-center justify-center rounded-md text-text-600 ui-interactive" aria-label="关闭建议面板" title="关闭建议面板">
          <X className="h-4 w-4" strokeWidth={1.75} />
        </button>
      </div>
      <button type="button" onClick={() => void generate()} disabled={generating || !canGenerate}
        className="mt-4 flex w-full items-center justify-between rounded-xl border border-border bg-panel px-3 py-2.5 text-left text-sm font-semibold text-text-900 ui-interactive disabled:opacity-50">
        <span>立即生成</span>
        {generating && <span className="text-xs font-normal">处理中</span>}
      </button>
      {error && <div role="alert" className="mt-3 text-xs text-danger">{error}</div>}
    </div>
  );
};
