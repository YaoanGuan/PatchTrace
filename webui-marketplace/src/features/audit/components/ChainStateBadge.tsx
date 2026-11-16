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
  { label: string; variant: 'success' | 'danger' | 'outline' | 'info' }
> = {
  PENDING: { label: '校验中', variant: 'info' },
  VALID: { label: '已校验', variant: 'success' },
  INVALID: { label: '异常', variant: 'danger' },
  EMPTY: { label: '未校验', variant: 'outline' },
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
    <Badge variant={meta.variant} className={className}>
      <Icon
        className={cn('h-3 w-3', state === 'PENDING' && 'animate-spin')}
        aria-hidden="true"
      />
      {meta.label}
    </Badge>
  );
}
