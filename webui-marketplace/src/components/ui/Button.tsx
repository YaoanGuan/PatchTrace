import type { ButtonHTMLAttributes } from 'react';
import { cn } from '../../lib/cn';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'outline';

export type ButtonSize = 'sm' | 'md' | 'lg';

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
}

const variantClasses: Record<ButtonVariant, string> = {
  primary:
    'game-cut bg-gradient-to-b from-gold to-[#d9a531] font-semibold text-slate-950 shadow-gold hover:from-[#ffdd8c] hover:to-gold focus-visible:ring-gold/50',
  secondary:
    'border border-panel-line bg-panel-soft text-ink hover:border-neon/50 hover:text-neon focus-visible:ring-neon/40',
  ghost:
    'bg-transparent text-ink-muted hover:bg-panel-soft hover:text-ink focus-visible:ring-neon/30',
  danger:
    'game-cut bg-gradient-to-b from-rose-500 to-rose-600 font-semibold text-white shadow-boss hover:from-rose-400 hover:to-rose-500 focus-visible:ring-rose-500/50',
  outline:
    'border border-panel-line bg-transparent text-ink-muted hover:border-neon/60 hover:text-neon focus-visible:ring-neon/40',
};

const sizeClasses: Record<ButtonSize, string> = {
  sm: 'h-8 px-3 text-xs',
  md: 'h-10 px-4 text-sm',
  lg: 'h-11 px-5 text-base',
};

export function Button({
  variant = 'primary',
  size = 'md',
  className,
  type = 'button',
  ...props
}: ButtonProps) {
  return (
    <button
      type={type}
      className={cn(
        'inline-flex items-center justify-center gap-2 rounded-md font-medium tracking-wide transition-all focus:outline-none focus-visible:ring-2 disabled:pointer-events-none disabled:opacity-50',
        variantClasses[variant],
        sizeClasses[size],
        className,
      )}
      {...props}
    />
  );
}
