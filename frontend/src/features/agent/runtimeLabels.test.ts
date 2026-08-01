import { describe, expect, it } from 'vitest';
import {
  stageLabels,
  strategyLabels,
  targetLabel,
  toolLabel,
} from './runtimeLabels';

describe('runtime business labels', () => {
  it('maps every execution strategy to concise Chinese copy', () => {
    expect(strategyLabels).toEqual({
      respond: '直接回答',
      direct_action: '直接修改',
      compact_workflow: '轻量工作流',
      full_pev: '完整工作流',
    });
  });

  it('maps stages, tools, and targets without exposing internal names', () => {
    expect(stageLabels.execute).toBe('执行修改');
    expect(stageLabels.verify).toBe('验证结果');
    expect(toolLabel('write_staged_presentation_slide')).toBe('生成页面 HTML');
    expect(toolLabel('unknown_internal_tool')).toBe('执行项目操作');
    expect(targetLabel('presentation', 'slide')).toBe('当前页HTML');
  });
});
