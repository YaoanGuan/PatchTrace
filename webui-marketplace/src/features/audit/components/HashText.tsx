import { cn } from '../../../lib/cn';
import { truncateHash } from '../utils';

export interface HashTextProps {
  hash: string;
  className?: string;
}

/**
 * 统一哈希展示：等宽字体 + 前 10 / 后 6 位截断，title 悬浮显示全量。
 */
export function HashText({ hash, className }: HashTextProps) {
  if (!hash) {
    return <span className="text-slate-400">—</span>;
  }
  return (
    <code
      title={hash}
      className={cn(
        'inline-block rounded bg-slate-100 px-1.5 py-0.5 font-mono text-[11px] leading-5 text-slate-600',
        className,
      )}
    >
      {truncateHash(hash)}
    </code>
  );
}
