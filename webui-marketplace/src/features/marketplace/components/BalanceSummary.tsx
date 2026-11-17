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
          <div className="flex items-center gap-2 text-xs text-ink-muted">
            <Wallet className="h-4 w-4 text-neon" />
            可用金币
          </div>
          <p className="mt-1 font-mono text-xl font-semibold text-gold">
            {formatMoney(balance.available)}
          </p>
        </Card>
        <Card className="p-4">
          <div className="flex items-center gap-2 text-xs text-ink-muted">
            <Lock className="h-4 w-4 text-gold" />
            赏金押金冻结
          </div>
          <p className="mt-1 font-mono text-xl font-semibold text-gold/80">
            {formatMoney(balance.frozen)}
          </p>
        </Card>
        <Card className="p-4">
          <div className="flex items-center gap-2 text-xs text-ink-muted">
            <Wallet className="h-4 w-4 text-ink-dim" />
            金币总额
          </div>
          <p className="mt-1 font-mono text-xl font-semibold text-ink">
            {formatMoney(total)}
          </p>
        </Card>
      </div>
    </div>
  );
}
