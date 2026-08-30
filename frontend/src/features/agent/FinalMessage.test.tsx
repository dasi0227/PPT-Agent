import { act, fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import type { ProjectContentSnapshot, PublicTarget } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { FinalChangeSummary } from './FinalMessage';

const snapshot: ProjectContentSnapshot = {
  manifest: {
    version: '4.0',
    revision: 1,
    project_id: 'p1',
    title: 'Demo',
    goal: 'Explain',
    audience: 'Team',
    language: 'zh-CN',
    requirements: [],
    prohibitions: [],
    canvas: { aspect_ratio: '16:9' },
    numbering: { enabled: true, hidden_roles: [], format: 'number' },
    created_at: 1,
    updated_at: 1,
  },
  outline: {
    version: '4.0',
    revision: 1,
    project_id: 'p1',
    sections: [{
      id: 'sec-1',
      title: 'Section',
      purpose: 'Purpose',
      slides: [
        { slide_id: 'slide-first', label: 'First', role: 'content' },
        { slide_id: 'slide-second', label: 'Second', role: 'content' },
      ],
      subsections: [],
    }],
    created_at: 1,
    updated_at: 1,
  },
  design: {
    version: '4.0',
    revision: 1,
    project_id: 'p1',
    theme: 'clean',
    direction: 'minimal',
    density: 'medium',
    chrome: [],
    created_at: 1,
    updated_at: 1,
  },
  slides_by_id: {},
};

const targets: PublicTarget[] = [
  { type: 'slide', slide_id: 'slide-second', part: 'html', display_name: '第 2 页' },
  { type: 'deck', part: 'design' },
  { type: 'slide', slide_id: 'slide-second', part: 'spec', display_name: '第 2 页' },
  { type: 'deck', part: 'manifest' },
  { type: 'slide', slide_id: 'slide-first', part: 'html', display_name: '第 1 页' },
  { type: 'deck', part: 'outline' },
  { type: 'slide', slide_id: 'slide-first', part: 'spec', display_name: '第 1 页' },
  { type: 'slide', slide_id: 'slide-first', part: 'spec', display_name: '第 1 页' },
];

describe('FinalChangeSummary', () => {
  beforeEach(() => {
    act(() => {
      useProjectStore.setState({
        activeProjectId: 'p1',
        contentByProjectId: { p1: snapshot },
      });
    });
  });

  it('counts unique files and orders global resources before page artifacts', () => {
    const { container } = render(<FinalChangeSummary targets={targets} />);

    fireEvent.click(screen.getByRole('button', { name: /7 个文件已更改/ }));

    const text = container.textContent ?? '';
    const labels = [
      'manifest.json',
      'outline.json',
      'design.json',
      'slide-first/spec.json',
      'slide-second/spec.json',
      'slide-first/index.html',
      'slide-second/index.html',
    ];
    const positions = labels.map((label) => text.indexOf(label));

    expect(positions.every((position) => position >= 0)).toBe(true);
    expect(positions).toEqual([...positions].sort((left, right) => left - right));
  });
});
