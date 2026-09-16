import { ClipboardList, Hammer, MessageCircleQuestion, MessagesSquare, type LucideIcon } from 'lucide-react';
import type { RunMode } from '../../api/types';

export interface ModeMeta {
  label: string;
  description: string;
  icon: LucideIcon;
}

export const MODE_ORDER: RunMode[] = ['execute', 'chat', 'grill', 'plan'];

export const MODE_META: Record<RunMode, ModeMeta> = {
  execute: { label: '开发', description: '开发代码，制作 PPT（默认）', icon: Hammer },
  chat: { label: '讨论', description: '陪你进行头脑风暴（只读）', icon: MessagesSquare },
  grill: { label: '盘问', description: '跟你确认决策细节（只读）', icon: MessageCircleQuestion },
  plan: { label: '计划', description: '开发前生成计划供你拍板', icon: ClipboardList },
};
