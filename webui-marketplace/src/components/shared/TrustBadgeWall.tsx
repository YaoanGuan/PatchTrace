import { BadgeCheck, Building2, GraduationCap } from 'lucide-react';
import type {
  TrustBadge,
  TrustBadgeStatus,
  TrustBadgeType,
} from '../../types/domain';
import { useSessionStore } from '../../app/store/sessionStore';
import { Badge, type BadgeVariant } from '../ui/Badge';
import { EmptyState } from '../ui/EmptyState';
import { cn } from '../../lib/cn';

export interface TrustBadgeWallProps {
  badges?: TrustBadge[];
  className?: string;
}

const badgeIcons: Record<TrustBadgeType, typeof GraduationCap> = {
  UNIVERSITY: GraduationCap,
  ENTERPRISE: Building2,
  TASK_CLEARANCE: BadgeCheck,
};

const statusLabels: Record<TrustBadgeStatus, string> = {
  VERIFIED: '已认证',
  PENDING: '审核中',
  REVOKED: '已撤销',
};

const statusVariants: Record<TrustBadgeStatus, BadgeVariant> = {
  VERIFIED: 'success',
  PENDING: 'warning',
  REVOKED: 'danger',
};

export function TrustBadgeWall({ badges, className }: TrustBadgeWallProps) {
  const userBadges = useSessionStore((state) => state.currentUser?.badges);
  const items = badges ?? userBadges ?? [];

  if (items.length === 0) {
    return (
      <EmptyState
        className={className}
        title="暂无认证徽章"
        description="完成高校、企业或任务通关认证后，这里会展示你的信任凭证。"
      />
    );
  }

  return (
    <div className={cn('grid gap-3 sm:grid-cols-2 lg:grid-cols-3', className)}>
      {items.map((badge) => {
        const Icon = badgeIcons[badge.type];
        return (
          <div
            key={badge.id}
            className="flex items-start gap-3 rounded-xl border border-slate-200 bg-white p-4 shadow-sm"
          >
            <span className="rounded-lg bg-indigo-50 p-2 text-indigo-600">
              <Icon className="h-4 w-4" />
            </span>
            <div className="min-w-0 flex-1 space-y-1">
              <div className="flex items-start justify-between gap-2">
                <h3 className="truncate text-sm font-medium text-slate-900">
                  {badge.label}
                </h3>
                <Badge variant={statusVariants[badge.status]}>
                  {statusLabels[badge.status]}
                </Badge>
              </div>
              <p className="truncate text-xs text-slate-500">{badge.issuer}</p>
              <p className="text-[11px] text-slate-400">
                认证时间 {badge.verifiedAt.slice(0, 10)}
              </p>
            </div>
          </div>
        );
      })}
    </div>
  );
}
