import { AlertTriangle, Clock, Loader2, ShieldCheck } from 'lucide-react';
import { Badge, type BadgeVariant } from '../../../components/ui/Badge';
import type { PipelineStageStatus } from '../../../types/domain';

const CONFIG: Record<
  PipelineStageStatus,
  { label: string; variant: BadgeVariant; icon: typeof ShieldCheck; spin?: boolean }
> = {
  PENDING: {
    label: '待定',
    variant: 'outline',
    icon: Clock,
  },
  RUNNING: {
    label: '裁判复核中',
    variant: 'info',
    icon: Loader2,
    spin: true,
  },
  DONE: {
    label: '通关',
    variant: 'success',
    icon: ShieldCheck,
  },
  FAILED: {
    label: '挑战失败',
    variant: 'danger',
    icon: AlertTriangle,
  },
};

export interface CrossCheckBadgeProps {
  status: PipelineStageStatus;
  className?: string;
}

export function CrossCheckBadge({ status, className }: CrossCheckBadgeProps) {
  const config = CONFIG[status];
  const Icon = config.icon;

  return (
    <Badge variant={config.variant} className={className}>
      <Icon className={config.spin ? 'h-3 w-3 animate-spin' : 'h-3 w-3'} />
      {config.label}
    </Badge>
  );
}
