import { useEffect } from 'react';
import { AlertTriangle, CheckCircle2, LoaderCircle } from 'lucide-react';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '../../components/ui/dialog';
import { Button } from '../../components/ui/primitives';
import { useExportStore } from '../../stores/exportStore';

const formatName = { png: 'PNG 图片', pdf: 'PDF', html: 'HTML 演示包' } as const;
const phaseName = { snapshotting: '正在冻结演示文稿', rendering: '正在渲染页面', packaging: '正在打包文件' } as const;

export function ExportProgressDialog() {
  const session = useExportStore((state) => state.session);
  const download = useExportStore((state) => state.download);
  const retry = useExportStore((state) => state.retry);
  const close = useExportStore((state) => state.close);
  const operation = session?.operation;
  const blocking = operation ? ['accepted', 'running', 'ready', 'delivering'].includes(operation.status) : false;

  useEffect(() => {
    if (!blocking || !operation?.id) return;
    const beforeUnload = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ''; };
    const unload = () => { void fetch(`/api/v1/exports/${encodeURIComponent(operation.id)}`, { method: 'DELETE', keepalive: true }); };
    window.addEventListener('beforeunload', beforeUnload);
    window.addEventListener('unload', unload);
    return () => { window.removeEventListener('beforeunload', beforeUnload); window.removeEventListener('unload', unload); };
  }, [blocking, operation?.id]);

  if (!session || !operation) return null;
  const failed = operation.status === 'failed';
  const ready = operation.status === 'ready' || operation.status === 'delivering';
  const pages = operation.total_pages > 0 ? Math.round(operation.completed_pages / operation.total_pages * 90) : 4;
  const percent = ready ? 100 : operation.phase === 'snapshotting' ? 4 : operation.phase === 'packaging' ? Math.max(94, pages) : Math.max(8, pages);
  const issues = operation.error?.details?.slides ?? [];

  return (
    <Dialog open onOpenChange={() => {}}>
      <DialogContent
        className="max-w-md"
        onEscapeKeyDown={(event) => event.preventDefault()}
        onPointerDownOutside={(event) => event.preventDefault()}
        onInteractOutside={(event) => event.preventDefault()}
      >
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {failed ? <AlertTriangle className="h-5 w-5 text-danger" /> : ready ? <CheckCircle2 className="h-5 w-5 text-success" /> : <LoaderCircle className="h-5 w-5 animate-spin text-accent" />}
            {failed ? '导出失败' : ready ? '导出完成' : `正在导出 ${formatName[session.format]}`}
          </DialogTitle>
          <DialogDescription>
            {failed ? operation.error?.message : ready ? '文件已准备好，点击下载后将交给浏览器保存。' : phaseName[operation.phase ?? 'snapshotting']}
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

        {(failed || ready) && <DialogFooter>
          {failed && <Button variant="secondary" onClick={() => void close()}>关闭</Button>}
          {failed && <Button variant="primary" onClick={() => void retry()}>重新导出</Button>}
          {ready && <Button variant="primary" disabled={operation.status === 'delivering'} onClick={download}>{operation.status === 'delivering' ? '正在交给浏览器…' : '下载'}</Button>}
        </DialogFooter>}
      </DialogContent>
    </Dialog>
  );
}
