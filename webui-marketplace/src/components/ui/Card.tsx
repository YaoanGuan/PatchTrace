import type { HTMLAttributes } from 'react';
import { cn } from '../../lib/cn';

export interface CardProps extends HTMLAttributes<HTMLDivElement> {}

/** 赛博地牢面板：所有业务卡片的基础皮肤。 */
export function Card({ className, ...props }: CardProps) {
  return <div className={cn('game-panel', className)} {...props} />;
}
