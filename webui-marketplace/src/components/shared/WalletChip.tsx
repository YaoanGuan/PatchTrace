import { Coins } from 'lucide-react';
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

/** 金币袋：HUD 上的可用金币与押金。 */
export function WalletChip({ className }: WalletChipProps) {
  const balance = useWalletStore((state) => state.balance);
  const symbol = balance.currency === 'CNY' ? '¥' : balance.currency;

  return (
    <div
      className={cn(
        'inline-flex items-center gap-2 rounded border border-gold/30 bg-gold/10 px-3 py-1.5 text-xs text-gold',
        className,
      )}
      title={`可用金币 ${symbol}${formatAmount(balance.available)} · 押金冻结 ${symbol}${formatAmount(balance.frozen)}`}
    >
      <Coins className="h-4 w-4" aria-hidden="true" />
      <span className="font-mono font-semibold">
        {symbol}
        {formatAmount(balance.available)}
      </span>
      {balance.frozen > 0 && (
        <span className="hidden font-mono text-[10px] text-ink-dim sm:inline">
          押金 {symbol}
          {formatAmount(balance.frozen)}
        </span>
      )}
    </div>
  );
}
