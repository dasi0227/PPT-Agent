import { useState } from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { Skill } from '../../api/types';
import { SkillSelector } from './SkillSelector';

const skills: Skill[] = [
  { id: 'one', name: '技能一', description: '第一个技能' },
  { id: 'two', name: '技能二', description: '第二个技能' },
  { id: 'three', name: '技能三', description: '第三个技能' },
  { id: 'four', name: '技能四', description: '第四个技能' },
];

function Harness() {
  const [selected, setSelected] = useState<string[]>([]);
  return (
    <SkillSelector
      skills={skills}
      selectedIds={selected}
      loading={false}
      onToggle={(id) => setSelected((current) => (
        current.includes(id) ? current.filter((value) => value !== id) : [...current, id].slice(0, 3)
      ))}
    />
  );
}

describe('SkillSelector', () => {
  it('shows only names and descriptions and limits selection to three skills', () => {
    render(<Harness />);
    const trigger = screen.getByRole('button', { name: '技能' });
    expect(trigger.querySelector('svg')).toBeInTheDocument();
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);

    for (const name of ['技能一', '技能二', '技能三']) {
      fireEvent.click(screen.getByRole('menuitemcheckbox', { name: `技能：${name}` }));
    }

    expect(trigger).toHaveTextContent('技能 3');
    expect(screen.getByRole('menuitemcheckbox', { name: '技能：技能四' })).toHaveAttribute('data-disabled');
    expect(screen.getByText('第一个技能')).toBeInTheDocument();
    expect(screen.queryByPlaceholderText(/搜索/)).toBeNull();
  });
});
