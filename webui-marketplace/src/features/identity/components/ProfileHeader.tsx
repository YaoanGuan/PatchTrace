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

/** 信誉分胶囊：按分数区间换色，缺失时按 0 分降级展示。 */
export function ReputationPill({ score, className }: ReputationPillProps) {
  const normalized =
    typeof score === 'number' && Number.isFinite(score)
      ? Math.max(0, Math.min(5, score))
      : 0;

  const tone =
    normalized >= 4.7
      ? 'bg-emerald-50 text-emerald-700 ring-emerald-600/20'
      : normalized >= 4.2
        ? 'bg-indigo-50 text-indigo-700 ring-indigo-600/20'
        : normalized >= 3.5
          ? 'bg-amber-50 text-amber-700 ring-amber-600/20'
          : 'bg-rose-50 text-rose-700 ring-rose-600/20';

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-sm font-semibold ring-1 ring-inset',
        tone,
        className,
      )}
      title="综合信誉分（满分 5.0）"
    >
      <Star className="h-4 w-4 fill-current" aria-hidden="true" />
      {normalized.toFixed(1)}
      <span className="text-[11px] font-normal opacity-70">/ 5.0 信誉分</span>
    </span>
  );
}

export interface ProfileHeaderProps {
  user?: User | null;
  className?: string;
}

/**
 * 个人档案头部：头像、昵称、等级、信誉分与关键履约指标。
 * 数据优先使用传入的 user，否则订阅 sessionStore.currentUser。
 */
export function ProfileHeader({ user, className }: ProfileHeaderProps) {
  const storeUser = useSessionStore((state) => state.currentUser);
  const profile = user ?? storeUser;

  if (!profile) {
    return (
      <EmptyState
        className={className}
        title="个人信息加载中"
        description="正在读取账号档案；若长时间未加载，请稍后刷新页面。"
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
      className={cn(
        'overflow-hidden rounded-xl border border-slate-200 bg-white shadow-sm',
        className,
      )}
    >
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
            <h2 className="truncate text-xl font-semibold tracking-tight text-slate-900">
              {profile.displayName || '未命名用户'}
            </h2>
            <Badge variant="info">
              {level.code} · {level.title}
            </Badge>
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

          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-slate-500">
            <span className="inline-flex items-center gap-1">
              <CalendarCheck2 className="h-3.5 w-3.5" aria-hidden="true" />
              完成任务 {completedTasks}
            </span>
            <span className="inline-flex items-center gap-1">
              <Handshake className="h-3.5 w-3.5" aria-hidden="true" />
              准时率 {Math.round(onTimeRate * 100)}%
            </span>
            <span className="inline-flex items-center gap-1">
              <ShieldAlert className="h-3.5 w-3.5" aria-hidden="true" />
              争议 {disputes}
            </span>
          </div>
        </div>

        <div className="w-full space-y-1.5 sm:w-48">
          <div className="flex items-center justify-between text-[11px] text-slate-500">
            <span>等级进度</span>
            <span>{level.progress}%</span>
          </div>
          <div
            role="progressbar"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={level.progress}
            className="h-2 w-full overflow-hidden rounded-full bg-slate-100"
          >
            <div
              className="h-full rounded-full bg-gradient-to-r from-indigo-500 to-sky-500 transition-all"
              style={{ width: `${level.progress}%` }}
            />
          </div>
          <p className="text-[11px] text-slate-400">
            {level.nextCode
              ? `距离 ${level.nextCode} 还需完成 ${level.remaining} 个任务`
              : '已达到当前最高等级'}
          </p>
        </div>
      </div>
    </div>
  );
}
