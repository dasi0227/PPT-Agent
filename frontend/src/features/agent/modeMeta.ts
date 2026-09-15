import { ClipboardList, Hammer, MessageCircleQuestion, MessagesSquare, type LucideIcon } from 'lucide-react';
import type { RunMode } from '../../api/types';

export interface ModeMeta {
  label: string;
  description: string;
  icon: LucideIcon;
}

export const MODE_ORDER: RunMode[] = ['execute', 'chat', 'grill', 'plan'];

export const MODE_META: Record<RunMode, ModeMeta> = {
  execute: { label: '开发', description: '直接开发并修改项目内容', icon: Hammer },
  chat: { label: '讨论', description: '只分析和交流，不修改项目内容', icon: MessagesSquare },
  grill: { label: '盘问', description: '只读探索，需要时可向你提问', icon: MessageCircleQuestion },
  plan: { label: '计划', description: '只写计划并回显，不修改项目内容', icon: ClipboardList },
};
