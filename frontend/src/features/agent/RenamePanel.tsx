import React, { useEffect, useMemo, useState } from 'react';
import { Check, PencilLine, Power, PowerOff, Sparkles, X } from 'lucide-react';
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

	useEffect(() => {
		if (target && thread) {
			setTitle(thread.title);
			setError('');
		}
	}, [target?.projectId, target?.threadId, thread?.title]);

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
			close();
		} catch (cause) {
			setError(cause instanceof Error ? cause.message : '命名操作失败，请稍后重试');
		} finally {
			setBusy(null);
		}
	};

	return (
		<div className="absolute bottom-[132px] left-3 right-3 z-30 rounded-2xl border border-border-strong bg-surface p-4 shadow-[0_18px_48px_rgba(15,23,42,0.18)]" role="dialog" aria-label="会话命名设置">
			<div className="flex items-start justify-between gap-4">
				<div>
					<div className="text-sm font-semibold text-text-900">会话命名</div>
					<div className="mt-1 text-xs text-text-600">自动命名当前{thread.auto_rename_enabled ? '已开启' : '已关闭'}，名称会直接更新。</div>
				</div>
				<button type="button" onClick={close} className="inline-flex h-7 w-7 items-center justify-center rounded-md text-text-600 hover:bg-panel-muted" aria-label="关闭命名面板">
					<X className="h-4 w-4" strokeWidth={1.75} />
				</button>
			</div>

			<button type="button" onClick={() => void run('generate')} disabled={Boolean(busy) || generating}
				className="mt-4 flex w-full items-center gap-3 rounded-xl border border-accent/20 bg-accent-soft px-3 py-2.5 text-left text-accent hover:bg-accent/15 disabled:opacity-50">
				<Sparkles className="h-4 w-4 shrink-0" strokeWidth={1.75} />
				<span className="min-w-0 flex-1"><span className="block text-sm font-semibold">立即自动命名</span><span className="block text-xs opacity-75">根据当前需求和进度重新评估名称</span></span>
				{generating && <span className="text-xs">处理中</span>}
			</button>

			<div className="mt-3 rounded-xl border border-border bg-panel p-3">
				<label className="flex items-center gap-2 text-xs font-semibold text-text-900"><PencilLine className="h-3.5 w-3.5" />手动命名</label>
				<div className="mt-2 flex gap-2">
					<input value={title} onChange={(event) => setTitle(event.target.value)} maxLength={60} placeholder="输入会话名称"
						className="min-w-0 flex-1 rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text-900 outline-none focus:border-accent" />
					<button type="button" onClick={() => void run('manual')} disabled={Boolean(busy)}
						className="inline-flex h-9 items-center gap-1.5 rounded-lg bg-text-900 px-3 text-xs font-semibold text-white disabled:opacity-50">
						<Check className="h-3.5 w-3.5" />保存
					</button>
				</div>
				<div className="mt-1.5 text-[11px] text-text-400">保存后将关闭自动命名。</div>
			</div>

			<div className="mt-3 grid grid-cols-2 gap-2">
				<button type="button" onClick={() => void run('enable')} disabled={Boolean(busy)} className="flex items-center justify-center gap-2 rounded-lg border border-border px-3 py-2 text-xs font-semibold text-text-900 hover:bg-panel-muted disabled:opacity-50">
					<Power className="h-3.5 w-3.5 text-success" />开启自动命名
				</button>
				<button type="button" onClick={() => void run('disable')} disabled={Boolean(busy)} className="flex items-center justify-center gap-2 rounded-lg border border-border px-3 py-2 text-xs font-semibold text-text-900 hover:bg-panel-muted disabled:opacity-50">
					<PowerOff className="h-3.5 w-3.5 text-text-600" />关闭自动命名
				</button>
			</div>
			{error && <div role="alert" className="mt-3 text-xs text-danger">{error}</div>}
		</div>
	);
};
