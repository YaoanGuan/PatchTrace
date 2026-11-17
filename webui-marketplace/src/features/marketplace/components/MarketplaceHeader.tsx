import { Bot, Briefcase } from 'lucide-react';
import { QuestHeading } from '../../../components/game/QuestHeading';
import { useSessionStore } from '../../../app/store/sessionStore';

const ROLE_LABEL = {
  EMPLOYER: '雇主视角',
  COMMANDER: 'AI 指挥官视角',
} as const;

const ROLE_HINT = {
  EMPLOYER: '可发布悬赏、冻结金币赏金、查看关卡进度与赏金押金。',
  COMMANDER: '可接受挑战、推进副本、提交战利品并查看裁判复核结果。',
} as const;

export function MarketplaceHeader() {
  const activeRole = useSessionStore((state) => state.activeRole);
  const Icon = activeRole === 'EMPLOYER' ? Briefcase : Bot;

  return (
    <QuestHeading
      chapter="QUEST BOARD · M3"
      title="悬赏大厅 · 主线任务板"
      description="在这里接副本、打怪、升级：AI 指挥官接受挑战并提交战利品，平台裁判独立复核，金币赏金经赏金押金完成掉落结算，通关后写入战报回放。"
      badge={
        <div className="flex max-w-xs shrink-0 items-start gap-2 rounded-xl border border-panel-line bg-panel px-3 py-2">
          <span className="rounded-lg bg-neon/10 p-1.5 text-neon">
            <Icon className="h-4 w-4" />
          </span>
          <div className="min-w-0">
            <p className="text-xs text-ink-dim">当前视角</p>
            <p className="text-sm font-semibold text-ink">
              {ROLE_LABEL[activeRole]}
            </p>
            <p className="mt-0.5 text-xs leading-4 text-ink-muted">
              {ROLE_HINT[activeRole]}
            </p>
          </div>
        </div>
      }
    />
  );
}
