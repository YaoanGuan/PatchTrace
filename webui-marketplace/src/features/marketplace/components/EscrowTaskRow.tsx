import { Link } from 'react-router-dom';
import { ChevronRight, Lock } from 'lucide-react';
import { Badge, type BadgeVariant } from '../../../components/ui/Badge';
import type { EscrowStatus } from '../../../types/domain';
import type { EscrowRowModel } from '../types';
import { formatDateTime, formatMoney } from '../utils';

const STATE_LABEL: Record<EscrowStatus, string> = {
  NONE: '未托管',
  FROZEN: '托管中',
  RELEASED: '已释放',
  REFUNDED: '已退款',
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
      className="flex items-center gap-3 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm transition-colors hover:border-indigo-200 hover:bg-indigo-50/40"
    >
      <span className="rounded-lg bg-amber-50 p-1.5 text-amber-600">
        <Lock className="h-4 w-4" />
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate font-medium text-slate-800">{row.taskId}</p>
        <p className="text-xs text-slate-400">
          {row.at ? `冻结于 ${formatDateTime(row.at)}` : '托管记录'}
        </p>
      </div>
      <div className="text-right">
        <p className="font-semibold text-slate-900">{formatMoney(row.amount)}</p>
        <Badge variant={STATE_VARIANT[row.state]} className="mt-0.5">
          {STATE_LABEL[row.state]}
        </Badge>
      </div>
      <ChevronRight className="h-4 w-4 shrink-0 text-slate-300" />
    </Link>
  );
}
