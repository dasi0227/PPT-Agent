import { useEffect } from 'react';
import { AlertTriangle, CheckCircle2, LoaderCircle, X } from 'lucide-react';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '../../components/ui/dialog';
import { Button, IconButton } from '../../components/ui/primitives';
import { useExportStore } from '../../stores/exportStore';
import { exportsApi } from '../../api/exports';

const formatName = { png: 'PNG 图片', pdf: 'PDF', html: 'HTML 演示包' } as const;
const phaseName = { snapshotting: '正在冻结演示文稿', rendering: '正在渲染页面', packaging: '正在打包文件' } as const;

export function ExportProgressDialog() {
  const session = useExportStore((state) => state.session);
  const download = useExportStore((state) => state.download);
  const retry = useExportStore((state) => state.retry);
  const close = useExportStore((state) => state.close);
  const refresh = useExportStore((state) => state.refresh);
  const cancel = useExportStore((state) => state.cancel);
  const operation = session?.operation;
  const blocking = !session?.dismissed && operation ? ['accepted', 'running', 'ready', 'delivering'].includes(operation.status) : false;

  useEffect(() => {
    if (!blocking) return;
    const id = operation?.id;
    const pagehide = () => {
      if (id) void fetch(`/api/v1/exports/${encodeURIComponent(id)}`, { method: 'DELETE', keepalive: true }).catch(() => {});
    };
    let pending = false;
    const heartbeat = async () => {
      if (!id || pending) return;
      pending = true;
      try { await exportsApi.heartbeat(id); } catch { /* Reconcile gone and terminal tasks below. */ }
      await refresh(id);
      pending = false;
    };
    void heartbeat();
    const timer = window.setInterval(() => void heartbeat(), 15_000);
    window.addEventListener('pagehide', pagehide);
    return () => { window.clearInterval(timer); window.removeEventListener('pagehide', pagehide); };
  }, [blocking, operation?.id, refresh]);

  if (!session || !operation || session.dismissed) return null;
  const failed = operation.status === 'failed';
  const conflict = failed && operation.error?.code === 'EXPORT_ALREADY_ACTIVE';
  const ready = operation.status === 'ready' || operation.status === 'delivering';
  const canceling = !!session.canceling;
  const canCancel = ['accepted', 'running'].includes(operation.status);
  const pages = operation.total_pages > 0 ? Math.round(operation.completed_pages / operation.total_pages * 90) : 4;
  const percent = ready ? 100 : operation.phase === 'snapshotting' ? 4 : operation.phase === 'packaging' ? Math.max(94, pages) : Math.max(8, pages);
  const issues = operation.error?.details?.slides ?? [];

  return (
    <Dialog open onOpenChange={(open) => { if (!open) void close(); }}>
      <DialogContent
        className="max-w-md"
        onEscapeKeyDown={(event) => { if (canceling) event.preventDefault(); }}
        onPointerDownOutside={(event) => event.preventDefault()}
        onInteractOutside={(event) => event.preventDefault()}
      >
        <IconButton label="关闭导出" className="absolute right-3 top-3" disabled={canceling} onClick={() => void close()}>
          <X className="h-4 w-4" />
        </IconButton>
        <DialogHeader className="space-y-3 pr-6">
          <DialogTitle className="flex items-center gap-2">
            {canceling ? <LoaderCircle className="h-5 w-5 animate-spin text-accent" /> : failed ? <AlertTriangle className="h-5 w-5 text-danger" /> : ready ? <CheckCircle2 className="h-5 w-5 text-success" /> : <LoaderCircle className="h-5 w-5 animate-spin text-accent" />}
            {canceling ? (canCancel ? '正在终止导出' : '正在清理文件') : conflict ? '暂时无法导出' : failed ? '导出失败' : ready ? '导出完成' : `正在导出 ${formatName[session.format]}`}
          </DialogTitle>
          <DialogDescription>
            {canceling ? (canCancel ? '正在停止处理并清理临时文件…' : '正在清理临时文件…') : failed ? operation.error?.message : ready ? '文件已准备好，点击下载到本地。' : phaseName[operation.phase ?? 'snapshotting']}
          </DialogDescription>
        </DialogHeader>

        {!failed && (
          <div className="space-y-2">
            <div className="h-2 overflow-hidden rounded-full bg-panel-muted" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={percent}>
              <div className="h-full rounded-full bg-accent transition-[width] duration-300" style={{ width: `${percent}%` }} />
            </div>
            <div className="flex justify-between text-xs text-text-400">
              <span>{operation.total_pages > 0 ? `已处理 ${operation.completed_pages} / ${operation.total_pages} 页` : '正在准备页面'}</span>
              <span>{percent}%</span>
            </div>
          </div>
        )}

        {issues.length > 0 && <ul className="max-h-40 overflow-y-auto rounded-md border border-danger/20 bg-danger-soft px-4 py-3 text-sm text-danger">
          {issues.map((issue) => <li key={issue.slide_id}>第 {issue.ordinal} 页 · {issue.title}</li>)}
        </ul>}

        {ready && operation.artifact && <div className="rounded-md border border-border bg-panel-muted px-3 py-2 text-sm">
          <div className="truncate font-medium text-text-900">{operation.artifact.filename}</div>
          <div className="mt-0.5 text-xs text-text-400">{(operation.artifact.size_bytes / 1024 / 1024).toFixed(2)} MB</div>
        </div>}

        {operation.warnings.length > 0 && <div className="rounded-md border border-warning/25 bg-warning-soft px-3 py-2 text-xs text-text-700">{operation.warnings.join(' ')}</div>}

        {session.cancelError && <p role="alert" className="text-sm text-danger">{session.cancelError}</p>}

        <DialogFooter>
          {canCancel && <Button variant="ghost" className="text-danger hover:bg-danger-soft hover:text-danger" disabled={canceling} onClick={() => void cancel()}>{canceling ? '正在终止…' : '终止'}</Button>}
          {failed && <Button variant="secondary" disabled={canceling} onClick={() => void close()}>关闭</Button>}
          {failed && <Button variant="primary" disabled={canceling} onClick={() => void retry()}>{conflict ? '重试' : '重新导出'}</Button>}
          {ready && <Button variant="primary" disabled={canceling || operation.status === 'delivering'} onClick={download}>{operation.status === 'delivering' ? '正在交给浏览器…' : '下载'}</Button>}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
