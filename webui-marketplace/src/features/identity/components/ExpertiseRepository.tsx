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
  AGENT: '战友技能',
  DATA_ASSET: '战利品档案',
  PROMPT_TEMPLATE: '咒语模板',
};

export interface ExpertiseCardProps {
  item: ExpertiseItem;
  className?: string;
}

/** 战友技能 / 咒语模板卡片。 */
export function ExpertiseCard({ item, className }: ExpertiseCardProps) {
  const Icon = KIND_ICONS[item.kind] ?? Layers;
  const usageCount =
    typeof item.usageCount === 'number' ? item.usageCount : 0;

  return (
    <Card className={cn('flex h-full flex-col gap-3 p-4', className)}>
      <div className="flex items-start gap-3">
        <span className="rounded-lg bg-arcane/10 p-2 text-arcane">
          <Icon className="h-4 w-4" aria-hidden="true" />
        </span>
        <div className="min-w-0 flex-1">
          <h4 className="truncate text-sm font-medium text-ink">
            {item.title || '未命名技能'}
          </h4>
          <p className="mt-0.5 text-xs text-ink-muted">
            {KIND_LABELS[item.kind] ?? '技能资产'}
          </p>
        </div>
        {item.level && <Badge variant="info">{item.level}</Badge>}
      </div>

      <div className="mt-auto flex items-center justify-between border-t border-panel-line pt-3 text-[11px] text-ink-muted">
        <span className="inline-flex items-center gap-1">
          <Gauge className="h-3.5 w-3.5" aria-hidden="true" />
          召唤 <span className="font-mono">{usageCount}</span> 次
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

/** 战利品档案卡片：与技能卡片区分视觉，强调语料/数据集的被引用次数。 */
export function DataAssetCard({ item, className }: DataAssetCardProps) {
  const usageCount =
    typeof item.usageCount === 'number' ? item.usageCount : 0;

  return (
    <Card
      className={cn(
        'flex h-full flex-col gap-3 border-emerald-500/25 bg-emerald-500/5 p-4',
        className,
      )}
    >
      <div className="flex items-start gap-3">
        <span className="rounded-lg bg-emerald-500/10 p-2 text-emerald-300">
          <Database className="h-4 w-4" aria-hidden="true" />
        </span>
        <div className="min-w-0 flex-1">
          <h4 className="truncate text-sm font-medium text-ink">
            {item.title || '未命名战利品'}
          </h4>
          <p className="mt-0.5 text-xs text-ink-muted">
            {KIND_LABELS.DATA_ASSET}
          </p>
        </div>
        {item.level && <Badge variant="success">{item.level}</Badge>}
      </div>

      <div className="mt-auto flex items-center justify-between border-t border-emerald-500/20 pt-3 text-[11px] text-emerald-300">
        <span className="inline-flex items-center gap-1">
          <Layers className="h-3.5 w-3.5" aria-hidden="true" />
          被引用 <span className="font-mono">{usageCount}</span> 次
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
 * 技能图鉴：战友技能 / 战利品档案 / 咒语模板三个 Tab。
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
    EXPERTISE_TABS.find((tab) => tab.key === activeTab)?.label ?? '技能';
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
        <span className="text-xs text-ink-muted">
          共 <span className="font-mono">{source.length}</span> 项技能沉淀
        </span>
      </div>

      {filtered.length === 0 ? (
        <EmptyState
          icon={<Layers className="h-8 w-8 text-ink-dim" />}
          title={`暂无${activeLabel}沉淀`}
          description="通关带复核的副本后，战友技能、战利品档案与咒语模板会自动归档到这里。"
          action={
            onUploadCredential ? (
              <Button
                variant="outline"
                size="sm"
                onClick={onUploadCredential}
              >
                <Upload className="h-4 w-4" aria-hidden="true" />
                上传成就凭证
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
