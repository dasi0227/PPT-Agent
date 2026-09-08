import type { CreateRunScopeInput, RunScope, ScopeObject, ScopeSelectionKind } from '../../api/types';
import type { RunStatus } from '../../stores/runStore';

export const scopeObjectLabels: Record<ScopeObject, string> = {
  spec: '设计稿',
  html: '幻灯片',
  presentation: '演示文稿',
  global: '全局资源',
};

export const scopeSelectionLabels: Record<ScopeSelectionKind, string> = {
  current_page: '当前页',
  all_pages: '全部页',
  custom_pages: '自选页',
  custom_sections: '自选章',
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

export function targetLabel(scope: RunScope | CreateRunScopeInput): string {
  const selection = 'source' in scope ? scope.source.kind : scope.selection.kind;
  return `${scopeSelectionLabels[selection]} · ${scopeObjectLabels[scope.object]}`;
}

export function presentUserText(text: string): string {
  return text.replace(/蓝图/g, '设计稿');
}
