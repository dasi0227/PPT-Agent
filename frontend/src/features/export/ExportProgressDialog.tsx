import { AlertTriangle } from 'lucide-react';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '../../components/ui/dialog';
import { Button } from '../../components/ui/primitives';
import { useExportStore } from '../../stores/exportStore';

export function ExportProgressDialog() {
  const session = useExportStore(state => state.session);
  const retry = useExportStore(state => state.retry);
  const close = useExportStore(state => state.close);

  if (session?.status !== 'error') return null;

  return (
    <Dialog open onOpenChange={open => { if (!open) close(); }}>
      <DialogContent className="max-w-md">
        <DialogHeader className="space-y-3">
          <DialogTitle className="flex items-center gap-2">
            <AlertTriangle className="h-5 w-5 text-danger" />
            导出失败
          </DialogTitle>
          <DialogDescription>{session.message}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="secondary" onClick={close}>关闭</Button>
          <Button variant="primary" onClick={() => void retry()}>重新导出</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
