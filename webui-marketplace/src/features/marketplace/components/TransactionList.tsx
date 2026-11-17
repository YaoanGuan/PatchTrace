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
      <p className="rounded-lg border border-dashed border-panel-line bg-panel-soft px-3 py-6 text-center text-xs text-ink-muted">
        暂无金币流水
      </p>
    );
  }

  return (
    <div className="divide-y divide-panel-line rounded-lg border border-panel-line bg-panel">
      {items.map((transaction) => (
        <div
          key={transaction.id}
          className="flex items-center gap-3 px-3 py-2.5"
        >
          <span className="rounded-lg bg-panel-soft p-1.5 text-ink-muted">
            <Receipt className="h-3.5 w-3.5" />
          </span>
          <div className="min-w-0 flex-1">
            <p className="text-sm font-medium text-ink">
              {TRANSACTION_TYPE_LABEL[transaction.type]}
            </p>
            <p className="truncate text-xs text-ink-dim">
              {transaction.taskId ? `副本 ${transaction.taskId} · ` : ''}
              {formatDateTime(transaction.at)}
            </p>
          </div>
          <div className="text-right">
            <p
              className={`font-mono text-sm font-semibold ${transactionTone(transaction.type)}`}
            >
              {transactionSign(transaction.type)}
              {formatMoney(transaction.amount)}
            </p>
            <Badge
              variant={STATUS_VARIANT[transaction.status]}
              className="mt-0.5 whitespace-normal text-right"
            >
              {STATUS_LABEL[transaction.status]}
            </Badge>
          </div>
        </div>
      ))}
    </div>
  );
}
