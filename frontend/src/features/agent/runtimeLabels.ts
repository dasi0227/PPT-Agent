import type { Artifact, ScopeLevel } from '../../api/types';
import type { RunStatus } from '../../stores/runStore';

export const artifactTargetLabels: Record<Artifact, string> = {
  spec: '设计稿',
  ppt: 'HTML',
};

export const levelLabels: Record<ScopeLevel, string> = {
  slide: '当前页',
  deck: '整份',
};

export const runStatusLabels: Record<RunStatus, string> = {
  idle: '空闲',
  creating: '正在创建',
  running: '运行中',
  waiting: '等待回答',
  paused: '已暂停',
  recovering: '正在恢复',
  canceling: '正在取消',
  done: '已完成',
  error: '运行失败',
  canceled: '已取消',
};

export function targetLabel(artifact: Artifact, level: ScopeLevel): string {
  return `${levelLabels[level]}${artifactTargetLabels[artifact]}`;
}

export function presentUserText(text: string): string {
  return text.replace(/蓝图/g, '设计稿');
}
