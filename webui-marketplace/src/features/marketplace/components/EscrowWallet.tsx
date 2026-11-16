import { useEffect } from 'react';
import { Lock } from 'lucide-react';
import { Card } from '../../../components/ui/Card';
import { useUiStore } from '../../../app/store/uiStore';
import { useWalletStore } from '../../../app/store/walletStore';
import { cn } from '../../../lib/cn';
import type { EscrowState } from '../../../types/domain';
import { deriveEscrowRows, errorMessage } from '../utils';
import { BalanceSummary } from './BalanceSummary';
import { EscrowTaskRow } from './EscrowTaskRow';
import { TransactionList } from './TransactionList';

export interface EscrowWalletProps {
  /** 详情页传入时，聚焦该任务的托管记录与流水。 */
  taskId?: string;
  /** 钱包流水尚未覆盖该任务时的兜底托管状态（来自任务详情）。 */
  escrow?: EscrowState;
  className?: string;
}

export function EscrowWallet({ taskId, escrow, className }: EscrowWalletProps) {
  const balance = useWalletStore((state) => state.balance);
  const transactions = useWalletStore((state) => state.transactions);
  const refresh = useWalletStore((state) => state.refresh);
  const pushToast = useUiStore((state) => state.pushToast);

  useEffect(() => {
    refresh().catch((error) => {
      pushToast({
        id: `wallet-error-${Date.now()}`,
        title: '钱包数据加载失败',
        description: errorMessage(error),
        variant: 'error',
      });
    });
  }, [pushToast, refresh]);

  const escrowRows = deriveEscrowRows(transactions);
  const walletRows = taskId
    ? escrowRows.filter((row) => row.taskId === taskId)
    : escrowRows;
  const visibleRows =
    taskId && escrow && escrow.state !== 'NONE' && walletRows.length === 0
      ? [
          {
            taskId,
            amount: escrow.amount,
            state: escrow.state,
            at: escrow.frozenAt,
          },
        ]
      : walletRows;

  return (
    <div className={cn('space-y-3', className)}>
      <div>
        <h2 className="flex items-center gap-1.5 text-sm font-semibold text-slate-900">
          <Lock className="h-4 w-4 text-indigo-600" />
          托管钱包
        </h2>
        <p className="mt-0.5 text-xs text-slate-500">
          赏金发布即冻结，AI 交叉复核与合规扫描通过后释放；争议期间资金保持冻结。
        </p>
      </div>

      <BalanceSummary balance={balance} />

      <Card>
        <div className="space-y-2 p-4">
          <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            托管任务
          </h3>
          {visibleRows.length === 0 ? (
            <p className="rounded-lg border border-dashed border-slate-200 bg-slate-50 px-3 py-6 text-center text-xs text-slate-500">
              {taskId ? '该任务暂无托管记录' : '暂无托管任务'}
            </p>
          ) : (
            <div className="space-y-2">
              {visibleRows.map((row) => (
                <EscrowTaskRow key={row.taskId} row={row} />
              ))}
            </div>
          )}
        </div>
      </Card>

      <Card>
        <div className="space-y-2 p-4">
          <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            资金流水
          </h3>
          <TransactionList transactions={transactions} taskId={taskId} />
        </div>
      </Card>
    </div>
  );
}
