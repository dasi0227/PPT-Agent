import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { TargetSelector } from './TargetSelector';

describe('TargetSelector', () => {
  it('shows scope and object as compact direct choices while preserving the supplied target fields', () => {
    const onTargetChange = vi.fn();
    render(
      <TargetSelector
        artifact="ppt"
        level="slide"
        onTargetChange={onTargetChange}
      />,
    );

    const trigger = screen.getByRole('button', { name: '目标：单页幻灯片' });
    expect(trigger.querySelector('svg')).toBeInTheDocument();
    expect(trigger).toHaveClass('composer-target-button');
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);
    expect(screen.getByRole('group', { name: '范围' })).toBeInTheDocument();
    expect(screen.getByRole('group', { name: '对象' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('menuitem', { name: '范围：整份' }));
    expect(onTargetChange).toHaveBeenCalledWith({ artifact: 'ppt', level: 'deck' });
  });

  it('maps the object control through the existing artifact protocol', () => {
    const onTargetChange = vi.fn();
    render(
      <TargetSelector
        artifact="ppt"
        level="slide"
        onTargetChange={onTargetChange}
      />,
    );

    const trigger = screen.getByRole('button', { name: '目标：单页幻灯片' });
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole('menuitem', { name: '对象：设计稿' }));
    expect(onTargetChange).toHaveBeenCalledWith({ artifact: 'spec', level: 'slide' });
  });

  it('locks to 整份设计稿 with guidance and no menu when locked', () => {
    const onTargetChange = vi.fn();
    render(
      <TargetSelector
        artifact="spec"
        level="deck"
        onTargetChange={onTargetChange}
        locked
      />,
    );

    const trigger = screen.getByRole('button', { name: '目标：整份设计稿' });
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);

    expect(screen.queryByRole('group', { name: '范围' })).not.toBeInTheDocument();
    expect(onTargetChange).not.toHaveBeenCalled();

    const hint = screen.getByRole('tooltip');
    expect(hint).toHaveTextContent('当前为空项目，请先确定整体的设计稿');
    expect(trigger).toHaveAttribute('aria-describedby', hint.id);
  });
});
