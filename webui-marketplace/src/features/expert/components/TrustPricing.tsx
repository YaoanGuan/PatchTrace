import { GitCompareArrows, ScanLine, ShieldAlert } from 'lucide-react';
import { Card } from '../../../components/ui/Card';
import { cn } from '../../../lib/cn';
import type { ExpertOverview } from '../../../types/domain';

export interface TrustPricingProps {
  pricing?: ExpertOverview['pricing'] | null;
  className?: string;
}

const SERVICE_ICONS = [GitCompareArrows, ScanLine, ShieldAlert];
const SERVICE_FALLBACK_LABELS = ['交叉验证', '合规扫描', '风险阻断'];

function normalizeService(service: string | undefined, index: number): string {
  const raw = (service ?? '').trim();
  const lower = raw.toLowerCase();
  if (lower.includes('valid') || lower.includes('cross')) {
    return '交叉验证';
  }
  if (lower.includes('compliance') || lower.includes('scan')) {
    return '合规扫描';
  }
  if (lower.includes('risk') || lower.includes('block')) {
    return '风险阻断';
  }
  return raw || SERVICE_FALLBACK_LABELS[index] || '信任服务';
}

function formatPrice(price: number | undefined): string {
  if (typeof price !== 'number' || !Number.isFinite(price)) {
    return '询价';
  }
  return `¥${price.toLocaleString('zh-CN')}`;
}

/**
 * 信任服务计价卡：交叉验证 / 合规扫描 / 风险阻断三档单价。
 * 数据来自 overview.pricing，空数组时降级为说明文案。
 */
export function TrustPricing({ pricing, className }: TrustPricingProps) {
  const items = pricing ?? [];

  return (
    <Card className={cn('p-4', className)}>
      <div className="flex items-baseline justify-between gap-2">
        <h2 className="text-sm font-semibold text-slate-900">信任服务计价</h2>
        <span className="text-[10px] text-slate-400">按任务计费</span>
      </div>
      <p className="mt-0.5 text-[11px] text-slate-500">
        不卖模型调用，只卖可信交付 · 不含上游 API 成本
      </p>

      {items.length === 0 ? (
        <div className="mt-3 rounded-lg border border-dashed border-slate-300 bg-slate-50 px-3 py-4 text-center text-xs text-slate-500">
          计价信息暂不可用
        </div>
      ) : (
        <ul className="mt-3 space-y-2">
          {items.map((item, index) => {
            const Icon = SERVICE_ICONS[index % SERVICE_ICONS.length];
            return (
              <li
                key={`${item.service ?? 'service'}-${index}`}
                className="flex items-center gap-2.5"
              >
                <span className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-indigo-50 text-indigo-600">
                  <Icon className="h-4 w-4" />
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-xs font-medium text-slate-800">
                    {normalizeService(item.service, index)}
                  </p>
                  <p className="truncate text-[10px] text-slate-500">
                    计费单位 · {item.unit || '次'}
                  </p>
                </div>
                <span className="shrink-0 text-sm font-semibold tabular-nums text-slate-900">
                  {formatPrice(item.price)}
                </span>
              </li>
            );
          })}
        </ul>
      )}
    </Card>
  );
}
