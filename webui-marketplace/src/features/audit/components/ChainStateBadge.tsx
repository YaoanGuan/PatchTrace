import { Loader2, Shield, ShieldAlert, ShieldCheck } from 'lucide-react';
import { Badge } from '../../../components/ui/Badge';
import { cn } from '../../../lib/cn';
import type { ChainState } from '../types';

export interface ChainStateBadgeProps {
  state: ChainState;
  className?: string;
}

const STATE_META: Record<
  ChainState,
  {
    label: string;
    variant: 'success' | 'danger' | 'outline' | 'info';
    tone?: string;
  }
> = {
  PENDING: { label: '封印校验中', variant: 'info' },
  VALID: {
    label: '封印完好',
    variant: 'success',
    tone: 'border-clear/30 bg-clear/10 text-clear',
  },
  INVALID: {
    label: '封印破损',
    variant: 'danger',
    tone: 'border-boss/30 bg-boss/10 text-boss',
  },
  EMPTY: { label: '暂无封印', variant: 'outline' },
};

export function ChainStateBadge({ state, className }: ChainStateBadgeProps) {
  const meta = STATE_META[state];
  const Icon =
    state === 'VALID'
      ? ShieldCheck
      : state === 'INVALID'
        ? ShieldAlert
        : state === 'PENDING'
          ? Loader2
          : Shield;

  return (
    <Badge variant={meta.variant} className={cn(meta.tone, className)}>
      <Icon
        className={cn('h-3 w-3', state === 'PENDING' && 'animate-spin')}
        aria-hidden="true"
      />
      {meta.label}
    </Badge>
  );
}
