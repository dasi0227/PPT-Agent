import { useExportStore } from '../../stores/exportStore';
import { useGitCommitStore } from '../../stores/gitCommitStore';
import { useProjectHistoryStore } from '../../stores/projectHistoryStore';
import { useProjectStore } from '../../stores/projectStore';
import { useRunStore } from '../../stores/runStore';
import { useThreadStore } from '../../stores/threadStore';

export function useAuthoringBlock(projectId: string | null): string | undefined {
  const sessions = useRunStore(state => state.sessions);
  const threads = useThreadStore(state => projectId ? state.threadsByProjectId[projectId] : undefined);
  const commit = useGitCommitStore(state => projectId ? state.sessions[projectId]?.status : undefined);
  const exporting = useExportStore(state => state.session);
  const history = useProjectHistoryStore(state => state.busy);
  const pending = useProjectStore(state => projectId ? state.mutationPendingByProjectId[projectId] : false);
  const error = useProjectStore(state => projectId ? state.contentErrorByProjectId[projectId] : undefined);
  if (history) return '项目历史正在切换';
  if (threads?.some(thread => ['creating', 'running', 'waiting', 'paused', 'recovering', 'canceling'].includes(sessions[thread.id]?.status ?? 'idle'))) return 'Agent 任务运行中，暂时不能修改';
  if (commit === 'creating' || commit === 'running') return '项目正在提交，暂时不能修改';
  if (exporting?.projectId === projectId && ['accepted', 'running', 'ready', 'delivering'].includes(exporting.operation.status)) return '项目正在导出，暂时不能修改';
  if (pending) return '正在保存修改';
  if (error) return '内容加载失败，请重试后编辑';
  return undefined;
}
