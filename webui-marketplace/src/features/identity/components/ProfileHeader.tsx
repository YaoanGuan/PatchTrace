import {
  BadgeCheck,
  CalendarCheck2,
  Handshake,
  ShieldAlert,
  Star,
} from 'lucide-react';
import type { User } from '../../../types/domain';
import { useSessionStore } from '../../../app/store/sessionStore';
import { Badge } from '../../../components/ui/Badge';
import { EmptyState } from '../../../components/ui/EmptyState';
import { LevelBadge } from '../../../components/game/LevelBadge';
import { XpBar } from '../../../components/game/XpBar';
import { cn } from '../../../lib/cn';
import {
  CHANNEL_LABELS,
  ROLE_LABELS,
  deriveLevel,
} from '../types';

export interface ReputationPillProps {
  score: number;
  className?: string;
}

/** 声望胶囊：按分数区间换色，缺失时按 0 分降级展示。 */
export function ReputationPill({ score, className }: ReputationPillProps) {
  const normalized =
    typeof score === 'number' && Number.isFinite(score)
      ? Math.max(0, Math.min(5, score))
      : 0;

  const tone =
    normalized >= 4.7
      ? 'bg-emerald-500/10 text-emerald-300 ring-emerald-500/30'
      : normalized >= 4.2
        ? 'bg-neon/10 text-neon ring-neon/30'
        : normalized >= 3.5
          ? 'bg-amber-500/10 text-amber-300 ring-amber-500/30'
          : 'bg-rose-500/10 text-rose-300 ring-rose-500/30';

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-sm font-semibold ring-1 ring-inset',
        tone,
        className,
      )}
      title="综合声望（满分 5.0）"
    >
      <Star className="h-4 w-4 fill-current" aria-hidden="true" />
      <span className="font-mono">{normalized.toFixed(1)}</span>
      <span className="text-[11px] font-normal opacity-70">/ 5.0 声望</span>
    </span>
  );
}

export interface ProfileHeaderProps {
  user?: User | null;
  className?: string;
}

/**
 * 角色档案头部：头像、昵称、等级、声望与关键战绩指标。
 * 数据优先使用传入的 user，否则订阅 sessionStore.currentUser。
 */
export function ProfileHeader({ user, className }: ProfileHeaderProps) {
  const storeUser = useSessionStore((state) => state.currentUser);
  const profile = user ?? storeUser;

  if (!profile) {
    return (
      <EmptyState
        className={className}
        title="角色档案加载中"
        description="正在读取冒险者档案；若长时间未加载，请稍后刷新页面。"
      />
    );
  }

  const completedTasks =
    typeof profile.reputation?.completedTasks === 'number'
      ? profile.reputation.completedTasks
      : 0;
  const score =
    typeof profile.reputation?.score === 'number' ? profile.reputation.score : 0;
  const onTimeRate =
    typeof profile.reputation?.onTimeRate === 'number'
      ? profile.reputation.onTimeRate
      : 0;
  const disputes =
    typeof profile.reputation?.disputes === 'number'
      ? profile.reputation.disputes
      : 0;
  const roles = profile.roles ?? [];
  const initial = profile.displayName?.trim().charAt(0) || '?';
  const level = deriveLevel(completedTasks);
  const channelLabel = profile.verificationChannel
    ? CHANNEL_LABELS[profile.verificationChannel]
    : '未认证通道';

  return (
    <div
      className={cn('game-panel overflow-hidden', className)}
    >
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-panel-line bg-panel-soft/60 px-5 py-2.5">
        <p className="font-mono text-[10px] uppercase tracking-[0.22em] text-neon">
          CHARACTER SHEET
        </p>
        <span className="text-[11px] text-ink-dim">角色档案 · 真实副本战绩</span>
      </div>
      <div className="flex flex-col gap-5 p-5 sm:flex-row sm:items-center">
        <div className="relative flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-2xl bg-gradient-to-br from-indigo-500 to-sky-500 text-2xl font-semibold text-white shadow-inner">
          <span aria-hidden="true">{initial}</span>
          {profile.avatarUrl && (
            <img
              src={profile.avatarUrl}
              alt={profile.displayName || '用户头像'}
              className="absolute inset-0 h-full w-full object-cover"
              onError={(event) => {
                event.currentTarget.style.display = 'none';
              }}
            />
          )}
        </div>

        <div className="min-w-0 flex-1 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="truncate text-xl font-semibold tracking-tight text-ink">
              {profile.displayName || '未命名冒险者'}
            </h2>
            <LevelBadge code={level.code} title={level.title} />
            <Badge variant="outline">
              <BadgeCheck className="h-3.5 w-3.5" aria-hidden="true" />
              {channelLabel}
            </Badge>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <ReputationPill score={score} />
            {roles.map((role) => (
              <Badge
                key={role}
                variant={role === 'COMMANDER' ? 'success' : 'default'}
              >
                {ROLE_LABELS[role]}
              </Badge>
            ))}
          </div>

          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-ink-muted">
            <span className="inline-flex items-center gap-1">
              <CalendarCheck2 className="h-3.5 w-3.5" aria-hidden="true" />
              通关副本{' '}
              <span className="font-mono font-semibold text-ink">
                {completedTasks}
              </span>
            </span>
            <span className="inline-flex items-center gap-1">
              <Handshake className="h-3.5 w-3.5" aria-hidden="true" />
              连击率{' '}
              <span className="font-mono font-semibold text-ink">
                {Math.round(onTimeRate * 100)}%
              </span>
            </span>
            <span className="inline-flex items-center gap-1">
              <ShieldAlert className="h-3.5 w-3.5" aria-hidden="true" />
              翻车次数{' '}
              <span className="font-mono font-semibold text-ink">
                {disputes}
              </span>
            </span>
          </div>
        </div>

        <div className="w-full space-y-1.5 sm:w-48">
          <XpBar value={level.progress} label="升级进度" />
          <p className="text-[11px] text-ink-dim">
            {level.nextCode
              ? `距离 ${level.nextCode} 还需通关 ${level.remaining} 个副本`
              : '已达到当前最高等级'}
          </p>
        </div>
      </div>
    </div>
  );
}
