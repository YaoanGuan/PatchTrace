import { Receipt } from 'lucide-react';
import { Badge, type BadgeVariant } from '../../../components/ui/Badge';
import type {
  WalletTransaction,
  WalletTransactionStatus,
} from '../../../types/domain';
import {
  formatDateTime,
  formatMoney,
  transactionSign,
  transactionTone,
  TRANSACTION_TYPE_LABEL,
} from '../utils';

const STATUS_LABEL: Record<WalletTransactionStatus, string> = {
  PENDING: '处理中',
  SETTLED: '已结算',
  FAILED: '失败',
};

const STATUS_VARIANT: Record<WalletTransactionStatus, BadgeVariant> = {
  PENDING: 'warning',
  SETTLED: 'success',
  FAILED: 'danger',
};

export interface TransactionListProps {
  transactions: WalletTransaction[];
  taskId?: string;
  limit?: number;
}

export function TransactionList({
  transactions,
  taskId,
  limit,
}: TransactionListProps) {
  const items = (taskId
    ? transactions.filter((transaction) => transaction.taskId === taskId)
    : transactions
  ).slice(0, limit ?? transactions.length);

  if (items.length === 0) {
    return (
      <p className="rounded-lg border border-dashed border-slate-200 bg-slate-50 px-3 py-6 text-center text-xs text-slate-500">
        暂无资金流水
      </p>
    );
  }

  return (
    <div className="divide-y divide-slate-100 rounded-lg border border-slate-200 bg-white">
      {items.map((transaction) => (
        <div
          key={transaction.id}
          className="flex items-center gap-3 px-3 py-2.5"
        >
          <span className="rounded-lg bg-slate-100 p-1.5 text-slate-500">
            <Receipt className="h-3.5 w-3.5" />
          </span>
          <div className="min-w-0 flex-1">
            <p className="text-sm font-medium text-slate-800">
              {TRANSACTION_TYPE_LABEL[transaction.type]}
            </p>
            <p className="truncate text-xs text-slate-400">
              {transaction.taskId ? `${transaction.taskId} · ` : ''}
              {formatDateTime(transaction.at)}
            </p>
          </div>
          <div className="text-right">
            <p
              className={`text-sm font-semibold ${transactionTone(transaction.type)}`}
            >
              {transactionSign(transaction.type)}
              {formatMoney(transaction.amount)}
            </p>
            <Badge
              variant={STATUS_VARIANT[transaction.status]}
              className="mt-0.5"
            >
              {STATUS_LABEL[transaction.status]}
            </Badge>
          </div>
        </div>
      ))}
    </div>
  );
}
