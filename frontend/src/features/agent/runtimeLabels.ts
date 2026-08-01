import type { Artifact, ExecutionStrategy, TargetLevel } from '../../api/types';
import type { RunStatus } from '../../stores/runStore';

export const strategyLabels: Record<ExecutionStrategy, string> = {
  respond: '直接回答',
  direct_action: '直接修改',
  compact_workflow: '轻量工作流',
  full_pev: '完整工作流',
};

export const stageLabels: Record<string, string> = {
  context: '准备上下文',
  plan: '制定执行方案',
  execute: '执行修改',
  verify: '验证结果',
  repair: '修复问题',
  commit: '提交版本',
  deliver: '完成交付',
};

export const toolLabels: Record<string, string> = {
  write_staged_slide_blueprint: '更新页面蓝图',
  write_staged_deck_blueprint: '更新整份蓝图',
  write_staged_presentation_slide: '生成页面 HTML',
  read_slide_blueprint: '读取页面蓝图',
  read_deck_blueprint: '读取整份蓝图',
  verify_blueprint: '验证蓝图',
  verify_presentation: '验证 HTML',
};

export const artifactLabels: Record<string, string> = {
  presentation_slide: '页面 HTML',
  blueprint_slide: '页面蓝图',
  blueprint_deck: '整份蓝图',
  design_spec: '设计语言',
};

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
  needs_input: '等待输入',
  done: '已完成',
  error: '运行失败',
  canceled: '已取消',
};

export function targetLabel(artifact: Artifact, level: TargetLevel): string {
  return `${levelLabels[level]}${artifactTargetLabels[artifact]}`;
}

export function toolLabel(tool: string): string {
  return toolLabels[tool] ?? '执行项目操作';
}
