export function ChangeStats({ insertions = 0, deletions = 0 }: { insertions?: number; deletions?: number }) {
  return <span className="shrink-0 whitespace-nowrap font-mono text-xs tabular-nums"><span className="text-success">+{insertions}</span>{' '}<span className="text-danger">−{deletions}</span></span>;
}
