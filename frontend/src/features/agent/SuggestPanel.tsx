import React, { useEffect, useRef, useState } from 'react';
import { Lightbulb, X } from 'lucide-react';
import { Switch } from '../../components/ui/switch';
import { threadsApi } from '../../api/threads';
import { useThreadStore } from '../../stores/threadStore';
import { sessionCanWrite, useSessionStore } from '../../stores/sessionStore';
import { useProjectStore } from '../../stores/projectStore';
import { useAgentSettingsStore } from '../../stores/agentSettingsStore';

export const SuggestPanel: React.FC = () => {
  const automaticEnabled = useAgentSettingsStore(state => state.value?.auto_suggest_enabled === true);
  const target = useThreadStore(state => state.suggestPanelTarget);
  const thread = useThreadStore(state => target
    ? state.threadsByProjectId[target.projectId]?.find(item => item.id === target.threadId) : undefined);
  const activeProjectId = useProjectStore(state => state.activeProjectId);
  const activeThreadId = useThreadStore(state => target ? state.activeThreadIdByProjectId[target.projectId] : null);
  const close = useThreadStore(state => state.closeSuggestPanel);
  const setAutomatic = useThreadStore(state => state.setAutoSuggest);
  const session = useSessionStore(state => target ? state.sessions[target.threadId] : undefined);
  const [busy, setBusy] = useState<'generate' | 'settings' | null>(null);
  const [error, setError] = useState('');
  const generateCommandId = useRef<string | null>(null);
  useEffect(() => { setError(''); setBusy(null); generateCommandId.current = null; }, [target]);
  useEffect(() => {
    if (target && (!thread || target.projectId !== activeProjectId || target.threadId !== activeThreadId)) close();
  }, [target, thread, activeProjectId, activeThreadId, close]);
  if (!target || !thread) return null;
  const hasHistory = Boolean(session && !session.isEmpty);
  const canGenerate = sessionCanWrite(session) && hasHistory && !session.thread?.active_run_id;
  const generating = busy === 'generate' || Object.values(session?.commandsById ?? {}).some(command =>
    command.kind === 'suggest' && ['accepted', 'running', 'cancel_requested'].includes(command.status));
  const generateDescription = !hasHistory
    ? '当前会话暂无记录'
    : !canGenerate
      ? '任务结束后可生成建议'
      : '根据当前会话和项目生成后续建议';
  const generate = async () => {
    if (busy || generating || !canGenerate) return;
    setBusy('generate'); setError('');
    const commandId = generateCommandId.current ?? `suggest:${crypto.randomUUID()}`;
    generateCommandId.current = commandId;
    try {
      await threadsApi.suggest(thread.id, commandId);
      if (generateCommandId.current === commandId) generateCommandId.current = null;
      if (useThreadStore.getState().suggestPanelTarget === target) close();
    } catch (cause) {
      if (useThreadStore.getState().suggestPanelTarget === target)
        setError(cause instanceof Error ? cause.message : '生成失败，请重试');
    } finally {
      if (useThreadStore.getState().suggestPanelTarget === target) setBusy(null);
    }
  };
  const toggle = async () => {
    setBusy('settings'); setError('');
    try { await setAutomatic(target.projectId, thread.id, !thread.auto_suggest_enabled); }
    catch (cause) {
      if (useThreadStore.getState().suggestPanelTarget === target)
        setError(cause instanceof Error ? cause.message : '设置失败，请重试');
    } finally {
      if (useThreadStore.getState().suggestPanelTarget === target) setBusy(null);
    }
  };
  return (
    <div className="scrollbar-none mx-3 mb-2 max-h-[50dvh] min-w-0 shrink-0 overflow-y-auto rounded-2xl border border-border-strong bg-surface p-4 [overflow-wrap:anywhere]" role="dialog" aria-label="后续建议设置">
      <div className="agent-inline-panel-header">
        <div className="flex min-w-0 items-center gap-2">
          <Lightbulb className="h-4 w-4 shrink-0" strokeWidth={1.75} />
          <div className="min-w-0 text-sm font-semibold text-text-900">后续建议</div>
        </div>
        <div className="agent-inline-panel-settings">
          <span className="text-xs text-text-600">自动建议</span>
          <Switch aria-label="自动建议" checked={automaticEnabled && thread.auto_suggest_enabled}
            disabled={Boolean(busy) || !automaticEnabled} onCheckedChange={() => void toggle()} title={automaticEnabled && thread.auto_suggest_enabled ? '自动建议已开启' : '自动建议已关闭'} />
        </div>
        <button type="button" onClick={close} className="agent-inline-panel-close inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-text-600 ui-interactive" aria-label="关闭建议面板" title="关闭建议面板">
          <X className="h-4 w-4" strokeWidth={1.75} />
        </button>
      </div>
      <button type="button" onClick={() => void generate()} disabled={Boolean(busy) || generating || !canGenerate}
        aria-describedby="suggest-generation-description"
        className="group mt-4 flex w-full items-center gap-3 rounded-xl border border-border bg-panel px-3 py-2.5 text-left text-text-900 ui-interactive disabled:opacity-50">
        <span className="min-w-0 flex-1">
          <span className="block text-sm font-semibold">立即生成</span>
          <span id="suggest-generation-description" className="block text-xs text-text-600">{generateDescription}</span>
        </span>
        {generating && <span className="shrink-0 text-xs">生成中</span>}
      </button>
      {error && <div role="alert" className="mt-3 text-xs text-danger">{error}</div>}
    </div>
  );
};
