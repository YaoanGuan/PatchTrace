import { Search, ShieldCheck } from 'lucide-react';
import { Card } from '../../../components/ui/Card';
import { Tabs } from '../../../components/ui/Tabs';
import { cn } from '../../../lib/cn';
import type { ChainFilter } from '../types';

export interface AuditFilterBarProps {
  keyword: string;
  chainFilter: ChainFilter;
  counts: {
    total: number;
    valid: number;
    invalid: number;
    unverified: number;
  };
  onKeywordChange(value: string): void;
  onChainFilterChange(value: ChainFilter): void;
}

const FILTERS: { key: ChainFilter; label: string }[] = [
  { key: 'ALL', label: '全部' },
  { key: 'VALID', label: '已校验' },
  { key: 'INVALID', label: '异常' },
];

export function AuditFilterBar({
  keyword,
  chainFilter,
  counts,
  onKeywordChange,
  onChainFilterChange,
}: AuditFilterBarProps) {
  const countFor = (key: ChainFilter) =>
    key === 'ALL' ? counts.total : key === 'VALID' ? counts.valid : counts.invalid;

  return (
    <Card className="flex flex-col gap-4 p-4 sm:flex-row sm:items-center sm:justify-between">
      <div className="relative w-full sm:max-w-sm">
        <Search
          className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400"
          aria-hidden="true"
        />
        <input
          type="search"
          value={keyword}
          onChange={(event) => onKeywordChange(event.target.value)}
          placeholder="搜索任务标题、ID 或发布者"
          aria-label="搜索审计任务"
          className={cn(
            'h-10 w-full rounded-lg border border-slate-300 bg-white pl-9 pr-3 text-sm text-slate-700 placeholder:text-slate-400',
            'focus:border-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-500/20',
          )}
        />
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <span className="inline-flex items-center gap-1.5 text-xs font-medium text-slate-500">
          <ShieldCheck className="h-4 w-4 text-indigo-500" aria-hidden="true" />
          哈希链状态
        </span>
        <Tabs
          items={FILTERS.map((filter) => ({
            key: filter.key,
            label: `${filter.label} ${countFor(filter.key)}`,
          }))}
          value={chainFilter}
          onChange={(key) => onChainFilterChange(key as ChainFilter)}
        />
        {counts.unverified > 0 && (
          <span className="text-xs text-slate-400">
            另有 {counts.unverified} 条暂无证据链
          </span>
        )}
      </div>
    </Card>
  );
}
