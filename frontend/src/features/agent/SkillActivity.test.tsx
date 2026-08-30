import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { SkillActivity } from './SkillActivity';

describe('SkillActivity', () => {
  it('renders a compact expandable list with file links', () => {
    render(<SkillActivity skills={[
      {
        id: 'story',
        name: '演示叙事',
        description: '梳理页面叙事。',
        local_path: '/tmp/skills/story/SKILL.md',
        open_url: 'vscode://file/tmp/skills/story/SKILL.md',
      },
      {
        id: 'visual',
        name: '视觉层级',
        description: '优化信息层级。',
        open_url: 'vscode://file/tmp/skills/visual/SKILL.md',
      },
    ]} />);

    const toggle = screen.getByRole('button', { name: '已启用 2 个技能' });
    expect(screen.getByRole('list')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '演示叙事' })).toHaveAttribute(
      'href',
      'vscode://file/tmp/skills/story/SKILL.md',
    );
    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
  });
});
