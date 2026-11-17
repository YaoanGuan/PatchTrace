import type { ReactNode } from 'react';
import { cn } from '../../lib/cn';

export interface QuestHeadingProps {
  /** 章节眉标，例如 MAIN QUEST · M3 */
  chapter: string;
  /** 中文主标题 */
  title: string;
  description?: string;
  /** 右侧槽位：奖励、难度、关卡状态 */
  badge?: ReactNode;
  className?: string;
}

/** 五个模块统一的游戏章节标题。 */
export function QuestHeading({
  chapter,
  title,
  description,
  badge,
  className,
}: QuestHeadingProps) {
  return (
    <div
      className={cn(
        'flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between',
        className,
      )}
    >
      <div className="min-w-0 space-y-1.5">
        <p className="flex items-center gap-2 font-mono text-[11px] uppercase tracking-[0.22em] text-neon">
          <span className="h-px w-6 bg-gradient-to-r from-transparent to-neon/70" />
          {chapter}
        </p>
        <h1 className="text-2xl font-semibold tracking-tight text-ink sm:text-3xl">
          {title}
        </h1>
        {description && (
          <p className="max-w-3xl text-sm leading-6 text-ink-muted">
            {description}
          </p>
        )}
      </div>
      {badge && <div className="shrink-0">{badge}</div>}
    </div>
  );
}
