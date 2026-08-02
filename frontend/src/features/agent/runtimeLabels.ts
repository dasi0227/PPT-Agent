import type { Artifact, TargetLevel } from '../../api/types';
import type { RunStatus } from '../../stores/runStore';

export const artifactTargetLabels: Record<Artifact, string> = {
  blueprint: '蓝图',
  presentation: 'HTML',
};

export const levelLabels: Record<TargetLevel, string> = {
  slide: '当前页',
  deck: '整份',
};

export const runStatusLabels: Record<RunStatus, string> = {
  idle: '空闲',
  creating: '正在创建',
  running: '运行中',
  waiting: '等待回答',
  done: '已完成',
  error: '运行失败',
  canceled: '已取消',
};

export function targetLabel(artifact: Artifact, level: TargetLevel): string {
  return `${levelLabels[level]}${artifactTargetLabels[artifact]}`;
}
