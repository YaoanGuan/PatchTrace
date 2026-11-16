import { Lock, Wallet } from 'lucide-react';
import { Card } from '../../../components/ui/Card';
import type { WalletBalance } from '../../../types/domain';
import { formatMoney } from '../utils';

export interface BalanceSummaryProps {
  balance: WalletBalance;
  className?: string;
}

export function BalanceSummary({ balance, className }: BalanceSummaryProps) {
  const total = balance.available + balance.frozen;

  return (
    <div className={className}>
      <div className="grid gap-3 sm:grid-cols-3">
        <Card className="p-4">
          <div className="flex items-center gap-2 text-xs text-slate-500">
            <Wallet className="h-4 w-4 text-indigo-600" />
            可用余额
          </div>
          <p className="mt-1 text-xl font-semibold text-slate-900">
            {formatMoney(balance.available)}
          </p>
        </Card>
        <Card className="p-4">
          <div className="flex items-center gap-2 text-xs text-slate-500">
            <Lock className="h-4 w-4 text-amber-500" />
            冻结金额
          </div>
          <p className="mt-1 text-xl font-semibold text-amber-600">
            {formatMoney(balance.frozen)}
          </p>
        </Card>
        <Card className="p-4">
          <div className="flex items-center gap-2 text-xs text-slate-500">
            <Wallet className="h-4 w-4 text-slate-400" />
            账户总额
          </div>
          <p className="mt-1 text-xl font-semibold text-slate-900">
            {formatMoney(total)}
          </p>
        </Card>
      </div>
    </div>
  );
}
