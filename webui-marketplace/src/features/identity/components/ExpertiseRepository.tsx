import { useState } from 'react';
import {
  Bot,
  Clock3,
  Database,
  Gauge,
  Layers,
  ScrollText,
  Upload,
} from 'lucide-react';
import type { ExpertiseItem, ExpertiseKind } from '../../../types/domain';
import { useSessionStore } from '../../../app/store/sessionStore';
import { Badge } from '../../../components/ui/Badge';
import { Button } from '../../../components/ui/Button';
import { Card } from '../../../components/ui/Card';
import { EmptyState } from '../../../components/ui/EmptyState';
import { Tabs } from '../../../components/ui/Tabs';
import { cn } from '../../../lib/cn';
import {
  EXPERTISE_TABS,
  formatShortDate,
  type ExpertiseTabKey,
} from '../types';

const KIND_ICONS: Record<ExpertiseKind, typeof Bot> = {
  AGENT: Bot,
  DATA_ASSET: Database,
  PROMPT_TEMPLATE: ScrollText,
};

const KIND_LABELS: Record<ExpertiseKind, string> = {
  AGENT: 'Agent 能力',
  DATA_ASSET: '数据资产',
  PROMPT_TEMPLATE: '提示词模板',
};

export interface ExpertiseCardProps {
  item: ExpertiseItem;
  className?: string;
}

/** Agent 能力 / 提示词模板卡片。 */
export function ExpertiseCard({ item, className }: ExpertiseCardProps) {
  const Icon = KIND_ICONS[item.kind] ?? Layers;
  const usageCount =
    typeof item.usageCount === 'number' ? item.usageCount : 0;

  return (
    <Card className={cn('flex h-full flex-col gap-3 p-4', className)}>
      <div className="flex items-start gap-3">
        <span className="rounded-lg bg-indigo-50 p-2 text-indigo-600">
          <Icon className="h-4 w-4" aria-hidden="true" />
        </span>
        <div className="min-w-0 flex-1">
          <h4 className="truncate text-sm font-medium text-slate-900">
            {item.title || '未命名能力'}
          </h4>
          <p className="mt-0.5 text-xs text-slate-500">
            {KIND_LABELS[item.kind] ?? '能力资产'}
          </p>
        </div>
        {item.level && <Badge variant="info">{item.level}</Badge>}
      </div>

      <div className="mt-auto flex items-center justify-between border-t border-slate-100 pt-3 text-[11px] text-slate-500">
        <span className="inline-flex items-center gap-1">
          <Gauge className="h-3.5 w-3.5" aria-hidden="true" />
          使用 {usageCount} 次
        </span>
        <span className="inline-flex items-center gap-1">
          <Clock3 className="h-3.5 w-3.5" aria-hidden="true" />
          更新 {formatShortDate(item.updatedAt)}
        </span>
      </div>
    </Card>
  );
}

export interface DataAssetCardProps {
  item: ExpertiseItem;
  className?: string;
}

/** 数据资产卡片：与能力卡片区分视觉，强调语料/数据集的被引用次数。 */
export function DataAssetCard({ item, className }: DataAssetCardProps) {
  const usageCount =
    typeof item.usageCount === 'number' ? item.usageCount : 0;

  return (
    <Card
      className={cn(
        'flex h-full flex-col gap-3 border-emerald-100 bg-emerald-50/30 p-4',
        className,
      )}
    >
      <div className="flex items-start gap-3">
        <span className="rounded-lg bg-emerald-100 p-2 text-emerald-700">
          <Database className="h-4 w-4" aria-hidden="true" />
        </span>
        <div className="min-w-0 flex-1">
          <h4 className="truncate text-sm font-medium text-slate-900">
            {item.title || '未命名数据资产'}
          </h4>
          <p className="mt-0.5 text-xs text-slate-500">
            {KIND_LABELS.DATA_ASSET}
          </p>
        </div>
        {item.level && <Badge variant="success">{item.level}</Badge>}
      </div>

      <div className="mt-auto flex items-center justify-between border-t border-emerald-100 pt-3 text-[11px] text-emerald-800/80">
        <span className="inline-flex items-center gap-1">
          <Layers className="h-3.5 w-3.5" aria-hidden="true" />
          被引用 {usageCount} 次
        </span>
        <span className="inline-flex items-center gap-1">
          <Clock3 className="h-3.5 w-3.5" aria-hidden="true" />
          更新 {formatShortDate(item.updatedAt)}
        </span>
      </div>
    </Card>
  );
}

export interface ExpertiseRepositoryProps {
  items?: ExpertiseItem[];
  className?: string;
  onUploadCredential?: () => void;
}

/**
 * 专家库：Agent 能力 / 数据资产 / 提示词模板三个 Tab。
 * 数据来自 sessionStore.currentUser.expertise；子项为空时渲染 EmptyState。
 */
export function ExpertiseRepository({
  items,
  className,
  onUploadCredential,
}: ExpertiseRepositoryProps) {
  const storeExpertise = useSessionStore(
    (state) => state.currentUser?.expertise,
  );
  const source = items ?? storeExpertise ?? [];
  const [activeTab, setActiveTab] = useState<ExpertiseTabKey>('AGENT');

  const activeLabel =
    EXPERTISE_TABS.find((tab) => tab.key === activeTab)?.label ?? '能力';
  const filtered = source.filter((item) => item.kind === activeTab);

  return (
    <section className={cn('space-y-3', className)}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <Tabs
          items={EXPERTISE_TABS.map((tab) => ({
            key: tab.key,
            label: tab.label,
          }))}
          value={activeTab}
          onChange={(key) => setActiveTab(key as ExpertiseTabKey)}
        />
        <span className="text-xs text-slate-500">
          共 {source.length} 项能力沉淀
        </span>
      </div>

      {filtered.length === 0 ? (
        <EmptyState
          icon={<Layers className="h-8 w-8 text-slate-400" />}
          title={`暂无${activeLabel}沉淀`}
          description="完成带复核的任务后，Agent 能力、数据资产与提示词模板会自动归档到这里。"
          action={
            onUploadCredential ? (
              <Button
                variant="outline"
                size="sm"
                onClick={onUploadCredential}
              >
                <Upload className="h-4 w-4" aria-hidden="true" />
                上传能力凭证
              </Button>
            ) : undefined
          }
        />
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {filtered.map((item) =>
            activeTab === 'DATA_ASSET' ? (
              <DataAssetCard key={item.id} item={item} />
            ) : (
              <ExpertiseCard key={item.id} item={item} />
            ),
          )}
        </div>
      )}
    </section>
  );
}
