import React, { useEffect, useMemo, useState } from 'react';
import { Signature, Sparkles, X } from 'lucide-react';
import { useThreadStore } from '../../stores/threadStore';

export const RenamePanel: React.FC = () => {
	const target = useThreadStore((state) => state.renamePanelTarget);
	const threads = useThreadStore((state) => target ? state.threadsByProjectId[target.projectId] : undefined);
	const pendingByThread = useThreadStore((state) => state.pendingNamingOperationByThreadId);
	const close = useThreadStore((state) => state.closeRenamePanel);
	const perform = useThreadStore((state) => state.performNamingAction);
	const thread = useMemo(() => threads?.find((item) => item.id === target?.threadId), [target?.threadId, threads]);
	const [title, setTitle] = useState('');
	const [busy, setBusy] = useState<string | null>(null);
	const [error, setError] = useState('');
	const [manualExpanded, setManualExpanded] = useState(false);

	useEffect(() => {
		setManualExpanded(false);
		setError('');
	}, [target]);

	useEffect(() => {
		if (target && thread && !manualExpanded) {
			setTitle(thread.title);
			setError('');
		}
	}, [target?.projectId, target?.threadId, thread?.title, manualExpanded]);

	useEffect(() => {
		if (target && !thread && threads) close();
	}, [close, target, thread, threads]);

	if (!target || !thread) return null;
	const generating = Boolean(pendingByThread[thread.id]);
	const run = async (action: 'generate' | 'manual' | 'enable' | 'disable') => {
		const cleanTitle = title.trim();
		if (action === 'manual' && (cleanTitle.length < 1 || Array.from(cleanTitle).length > 60)) {
			setError('名称需为 1–60 个字符');
			return;
		}
		setBusy(action);
		setError('');
		try {
			await perform(target.projectId, target.threadId, action, action === 'manual' ? cleanTitle : undefined);
			if (action === 'generate' || action === 'manual') close();
		} catch (cause) {
			setError(cause instanceof Error ? cause.message : '命名操作失败，请稍后重试');
		} finally {
			setBusy(null);
		}
	};

	return (
		<div className="scrollbar-none mx-3 mb-2 max-h-[50dvh] shrink-0 overflow-y-auto rounded-2xl border border-border-strong bg-surface p-4" role="dialog" aria-label="会话命名设置">
			<div className="flex items-center gap-3">
				<div className="shrink-0 text-sm font-semibold text-text-900">会话命名</div>
				<div className="ml-auto flex items-center gap-2">
					<span className="text-xs text-text-600">自动命名</span>
					<button type="button" role="switch" aria-label="自动命名" aria-checked={thread.auto_rename_enabled}
						disabled={Boolean(busy)} onClick={() => void run(thread.auto_rename_enabled ? 'disable' : 'enable')}
						title={thread.auto_rename_enabled ? '自动命名已开启' : '自动命名已关闭'}
						className={`h-5 w-9 shrink-0 rounded-full p-0.5 transition-colors disabled:opacity-50 ${thread.auto_rename_enabled ? 'bg-success' : 'bg-border-strong'}`}>
						<span className={`block h-4 w-4 rounded-full bg-white shadow-sm transition-transform ${thread.auto_rename_enabled ? 'translate-x-4' : 'translate-x-0'}`} />
					</button>
				</div>
				<button type="button" onClick={close} className="inline-flex h-7 w-7 items-center justify-center rounded-md text-text-600 hover:bg-panel-muted" aria-label="关闭命名面板">
					<X className="h-4 w-4" strokeWidth={1.75} />
				</button>
			</div>

			<button type="button" onClick={() => void run('generate')} disabled={Boolean(busy) || generating}
				className="group mt-4 flex w-full items-center gap-3 rounded-xl border border-border bg-panel px-3 py-2.5 text-left text-text-900 enabled:hover:bg-accent-soft enabled:hover:text-accent disabled:opacity-50">
				<Sparkles className="h-4 w-4 shrink-0" strokeWidth={1.75} />
				<span className="min-w-0 flex-1"><span className="block text-sm font-semibold">立即自动命名</span><span className="block text-xs text-text-600 group-[:enabled:hover]:text-accent/75">根据当前需求和进度重新评估名称</span></span>
				{generating && <span className="text-xs">处理中</span>}
			</button>

			<div className={`mt-3 overflow-hidden rounded-xl border border-border transition-colors ${manualExpanded ? 'bg-accent-soft' : 'bg-panel'}`}>
				<button type="button" disabled={Boolean(busy)} aria-expanded={manualExpanded} aria-controls="manual-rename-form"
					onClick={() => setManualExpanded((expanded) => !expanded)}
					className={`group flex w-full items-center gap-3 px-3 py-2.5 text-left enabled:hover:bg-accent-soft enabled:hover:text-accent disabled:opacity-50 ${manualExpanded ? 'text-accent' : 'text-text-900'}`}>
					<Signature className="h-4 w-4 shrink-0" strokeWidth={1.75} />
					<span className="min-w-0 flex-1"><span className="block text-sm font-semibold">手动命名</span><span className={`block text-xs group-[:enabled:hover]:text-accent/75 ${manualExpanded ? 'text-accent/75' : 'text-text-600'}`}>自定义会话名称</span></span>
				</button>
				{manualExpanded && <form id="manual-rename-form" className="px-3 pb-3" onSubmit={(event) => { event.preventDefault(); if (!busy) void run('manual'); }}>
					<input autoFocus aria-label="会话名称，按 Enter 保存" disabled={Boolean(busy)} value={title} onChange={(event) => setTitle(event.target.value)} maxLength={60} placeholder="输入会话名称，按 Enter 保存"
						onKeyDown={(event) => { if (event.key === 'Enter' && (event.nativeEvent.isComposing || event.keyCode === 229)) event.preventDefault(); }}
						className="w-full min-w-0 rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text-900 outline-none transition-colors focus:border-border-strong focus-visible:ring-0 focus-visible:ring-offset-0 disabled:opacity-50" />
				</form>}
			</div>
			{error && <div role="alert" className="mt-3 text-xs text-danger">{error}</div>}
		</div>
	);
};
