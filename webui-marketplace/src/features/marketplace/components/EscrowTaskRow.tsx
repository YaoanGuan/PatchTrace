import { Link } from 'react-router-dom';
import { ChevronRight, Lock } from 'lucide-react';
import { Badge, type BadgeVariant } from '../../../components/ui/Badge';
import type { EscrowStatus } from '../../../types/domain';
import type { EscrowRowModel } from '../types';
import { formatDateTime, formatMoney } from '../utils';

const STATE_LABEL: Record<EscrowStatus, string> = {
  NONE: '未缴押金',
  FROZEN: '押金冻结中',
  RELEASED: '押金已释放',
  REFUNDED: '押金已退回',
};

const STATE_VARIANT: Record<EscrowStatus, BadgeVariant> = {
  NONE: 'outline',
  FROZEN: 'warning',
  RELEASED: 'success',
  REFUNDED: 'default',
};

export interface EscrowTaskRowProps {
  row: EscrowRowModel;
}

export function EscrowTaskRow({ row }: EscrowTaskRowProps) {
  return (
    <Link
      to={`/marketplace/tasks/${row.taskId}`}
      className="flex items-center gap-3 rounded-lg border border-panel-line bg-panel px-3 py-2 text-sm transition-colors hover:border-neon/40 hover:bg-neon/5"
    >
      <span className="rounded-lg bg-gold/10 p-1.5 text-gold">
        <Lock className="h-4 w-4" />
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate font-medium text-ink">副本 {row.taskId}</p>
        <p className="text-xs text-ink-dim">
          {row.at ? `冻结于 ${formatDateTime(row.at)}` : '赏金押金记录'}
        </p>
      </div>
      <div className="text-right">
        <p className="font-mono font-semibold text-gold">{formatMoney(row.amount)}</p>
        <Badge variant={STATE_VARIANT[row.state]} className="mt-0.5 whitespace-normal text-right">
          {STATE_LABEL[row.state]}
        </Badge>
      </div>
      <ChevronRight className="h-4 w-4 shrink-0 text-ink-dim" />
    </Link>
  );
}
