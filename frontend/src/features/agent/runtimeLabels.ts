import type { CreateRunScopeInput, RunActivity, RunScope, ScopeSelectionKind } from '../../api/types';
import type { RunStatus } from '../../stores/runStore';


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

export const runActivityLabels: Record<RunActivity, string> = {
  'run.preparing': 'Dasi 正在准备任务',
  'run.recovering': 'Dasi 正在恢复任务',
  'run.analyzing': 'Dasi 正在确定下一步操作',
  'model.fallback': 'Dasi 已切换至备用模型',
  'run.retrying': 'Dasi 连接暂时不稳定，正在重试',
  'run.canceling': 'Dasi 正在停止任务',
  'plan.preparing': 'Dasi 正在整理执行方案',
  'presentation.structure.reading': 'Dasi 正在了解演示文稿结构',
  'presentation.design.reading': 'Dasi 正在了解当前视觉要求',
  'slide.content.reading': 'Dasi 正在检查页面内容',
  'reference.inspecting': 'Dasi 正在查看参考素材',
  'presentation.structure.updating': 'Dasi 正在调整演示文稿结构',
  'presentation.design.updating': 'Dasi 正在统一演示文稿的视觉要求',
  'slide.creating': 'Dasi 正在制作页面',
  'slide.updating': 'Dasi 正在优化页面',
  'slide.layout.checking': 'Dasi 正在渲染幻灯片',
  'resource.preparing': 'Dasi 正在准备创作资源',
  'command.executing': 'Dasi 正在执行辅助操作',
  'completion.reviewing': 'Dasi 正在进行最终检查',
};

export function targetLabel(scope: RunScope | CreateRunScopeInput): string {
  const selection = 'source' in scope ? scope.source.kind : scope.selection.kind;
  return scopeSelectionLabels[selection];
}
