import { GitCompareArrows, ScanLine, ShieldAlert } from 'lucide-react';
import { Card } from '../../../components/ui/Card';
import { cn } from '../../../lib/cn';
import type { ExpertOverview } from '../../../types/domain';

export interface TrustPricingProps {
  pricing?: ExpertOverview['pricing'] | null;
  className?: string;
}

const SERVICE_ICONS = [GitCompareArrows, ScanLine, ShieldAlert];
const SERVICE_FALLBACK_LABELS = [
  '双人连携验证',
  '副本规则扫描',
  '大Boss 威胁封锁',
];

function normalizeService(service: string | undefined, index: number): string {
  const raw = (service ?? '').trim();
  const lower = raw.toLowerCase();
  if (
    lower.includes('valid') ||
    lower.includes('cross') ||
    raw.includes('交叉') ||
    raw.includes('验证')
  ) {
    return '双人连携验证';
  }
  if (
    lower.includes('compliance') ||
    lower.includes('scan') ||
    raw.includes('合规') ||
    raw.includes('扫描')
  ) {
    return '副本规则扫描';
  }
  if (
    lower.includes('risk') ||
    lower.includes('block') ||
    raw.includes('风险') ||
    raw.includes('阻断')
  ) {
    return '大Boss 威胁封锁';
  }
  return raw || SERVICE_FALLBACK_LABELS[index] || '副本裁判服务';
}

function formatPrice(price: number | undefined): string {
  if (typeof price !== 'number' || !Number.isFinite(price)) {
    return '询价';
  }
  return `¥${price.toLocaleString('zh-CN')}`;
}

/**
 * 副本裁判费计价卡：双人连携验证 / 副本规则扫描 / 大Boss 威胁封锁三档单价。
 * 数据来自 overview.pricing，空数组时降级为说明文案；金额与计费口径保持原样。
 */
export function TrustPricing({ pricing, className }: TrustPricingProps) {
  const items = pricing ?? [];

  return (
    <Card className={cn('p-4', className)}>
      <div className="flex items-baseline justify-between gap-2">
        <h2 className="text-sm font-semibold text-ink">副本裁判费</h2>
        <span className="text-[10px] text-ink-dim">按副本收取裁判费</span>
      </div>
      <p className="mt-0.5 text-[11px] text-ink-muted">
        不卖模型调用，只卖可信交付 · 不含上游 API 成本
      </p>

      {items.length === 0 ? (
        <div className="mt-3 rounded-lg border border-dashed border-panel-line bg-panel-soft px-3 py-4 text-center text-xs text-ink-muted">
          裁判费计价信息暂不可用
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
                <span className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-arcane/10 text-arcane ring-1 ring-inset ring-arcane/30">
                  <Icon className="h-4 w-4" />
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-xs font-medium text-ink">
                    {normalizeService(item.service, index)}
                  </p>
                  <p className="truncate text-[10px] text-ink-muted">
                    计费单位 · {item.unit || '次'}
                  </p>
                </div>
                <span className="shrink-0 font-mono text-sm font-semibold tabular-nums text-gold">
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
