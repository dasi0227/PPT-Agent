import { useState } from 'react';
import { Download, FileImage, FileText, Code, X } from 'lucide-react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '../../components/ui/dialog';
import { IconButton } from '../../components/ui/primitives';
import { useAppShortcuts } from '../../lib/useAppShortcuts';
import type { ExportFormat } from '../../api/exports';

export function ExportButton({ disabled, reason, onExport }: { disabled: boolean; reason?: string; onExport: (format: ExportFormat) => void }) {
  const [open, setOpen] = useState(false);
  useAppShortcuts({ 'deck.export': () => { if (!disabled) setOpen(true); } });
  return (
    <>
      <button type="button" disabled={disabled} onClick={() => setOpen(true)} aria-label="导出" title={disabled ? reason : '导出完整演示文稿'} className="inline-flex h-8 w-8 items-center justify-center rounded-md text-text-600 hover:bg-panel-muted hover:text-text-900 disabled:cursor-not-allowed disabled:opacity-45">
        <Download className="h-4 w-4" strokeWidth={1.75} />
      </button>
      <Dialog open={open && !disabled} onOpenChange={setOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <div className="flex items-center justify-between"><DialogTitle>导出演示文稿</DialogTitle><IconButton label="关闭" onClick={() => setOpen(false)}><X className="h-4 w-4" /></IconButton></div>
            <DialogDescription>选择导出格式</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-2">
            {([
              ['png', 'PNG 图片', FileImage],
              ['pdf', 'PDF 文档', FileText],
              ['html', 'HTML 演示文稿', Code],
            ] as const).map(([format, label, Icon]) => (
              <button key={format} type="button" onClick={() => { setOpen(false); onExport(format); }} className="flex items-center gap-3 rounded-lg px-4 py-3 text-left text-sm text-text-900 hover:bg-accent-soft hover:text-accent focus-visible:bg-accent-soft">
                <Icon className="h-4 w-4 shrink-0" strokeWidth={1.75} />{label}
              </button>
            ))}
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
