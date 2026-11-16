import { CircleDollarSign } from 'lucide-react';
import { cn } from '../../../lib/cn';

export interface RewardInputProps {
  value: number | undefined;
  onChange: (value: number | undefined) => void;
  error?: string;
  disabled?: boolean;
  className?: string;
}

export function RewardInput({
  value,
  onChange,
  error,
  disabled,
  className,
}: RewardInputProps) {
  return (
    <div className={cn('space-y-1', className)}>
      <label
        htmlFor="marketplace-reward"
        className="text-xs font-medium text-slate-600"
      >
        赏金（CNY）
      </label>
      <div className="relative">
        <CircleDollarSign className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
        <span className="pointer-events-none absolute left-9 top-1/2 -translate-y-1/2 text-sm text-slate-500">
          ¥
        </span>
        <input
          id="marketplace-reward"
          type="number"
          min={1}
          step={10}
          disabled={disabled}
          value={value ?? ''}
          onChange={(event) => {
            const next = event.target.value;
            onChange(next === '' ? undefined : Number(next));
          }}
          placeholder="输入赏金金额"
          className={cn(
            'h-10 w-full rounded-lg border bg-white pl-16 pr-3 text-sm text-slate-900 outline-none transition-colors placeholder:text-slate-400 focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100 disabled:bg-slate-50',
            error ? 'border-rose-300' : 'border-slate-300',
          )}
        />
      </div>
      <p
        className={cn(
          'text-xs',
          error ? 'text-rose-600' : 'text-slate-400',
        )}
      >
        {error ?? '发布成功后赏金将进入平台托管，验收通过后释放给指挥官。'}
      </p>
    </div>
  );
}
