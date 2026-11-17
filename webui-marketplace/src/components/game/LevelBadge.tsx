import { Sparkles } from 'lucide-react';
import { cn } from '../../lib/cn';

export interface LevelBadgeProps {
  code: string;
  title: string;
  size?: 'sm' | 'md';
  className?: string;
}

/** 等级徽章：Lv.12 · 漏洞猎人。 */
export function LevelBadge({
  code,
  title,
  size = 'sm',
  className,
}: LevelBadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded border border-gold/40 bg-gold/10 font-mono font-semibold tracking-wide text-gold',
        size === 'sm' ? 'px-2 py-0.5 text-[11px]' : 'px-3 py-1 text-sm',
        className,
      )}
      title={`当前等级：${code} · ${title}`}
    >
      <Sparkles className="h-3 w-3" aria-hidden="true" />
      {code} · {title}
    </span>
  );
}
