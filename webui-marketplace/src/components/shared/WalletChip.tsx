import { Wallet } from 'lucide-react';
import { useWalletStore } from '../../app/store/walletStore';
import { cn } from '../../lib/cn';

export interface WalletChipProps {
  className?: string;
}

const formatAmount = (amount: number): string =>
  amount.toLocaleString('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });

export function WalletChip({ className }: WalletChipProps) {
  const balance = useWalletStore((state) => state.balance);
  const symbol = balance.currency === 'CNY' ? '¥' : balance.currency;

  return (
    <div
      className={cn(
        'inline-flex items-center gap-2 rounded-full border border-slate-200 bg-slate-50 px-3 py-1.5 text-xs text-slate-600',
        className,
      )}
      title={`可用 ${symbol}${formatAmount(balance.available)} · 冻结 ${symbol}${formatAmount(balance.frozen)}`}
    >
      <Wallet className="h-4 w-4 text-indigo-600" />
      <span className="font-semibold text-slate-900">
        {symbol}
        {formatAmount(balance.available)}
      </span>
      {balance.frozen > 0 && (
        <span className="hidden text-slate-400 sm:inline">
          冻结 {symbol}
          {formatAmount(balance.frozen)}
        </span>
      )}
    </div>
  );
}
