import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

/** 合并 Tailwind 类名，后写的冲突工具类覆盖前面的。 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
