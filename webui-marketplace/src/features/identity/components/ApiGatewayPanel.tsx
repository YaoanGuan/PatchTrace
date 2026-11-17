import { useCallback, useEffect, useState } from 'react';
import {
  AlertCircle,
  Check,
  ChevronRight,
  Code2,
  Copy,
  KeyRound,
  Loader2,
  MessageSquare,
  MousePointer2,
  Plus,
  RefreshCw,
  ServerCog,
  ShieldCheck,
  Terminal,
  Trash2,
} from 'lucide-react';
import type {
  ApiGatewayOverview,
  ApiKeyRecord,
  CreateApiKeyResponse,
  ModelCatalogItem,
} from '../../../types/api';
import {
  createApiKey,
  getApiGateway,
  revokeApiKey,
} from '../../../lib/api/client';
import { useUiStore } from '../../../app/store/uiStore';
import { Badge } from '../../../components/ui/Badge';
import { Button } from '../../../components/ui/Button';
import { Card } from '../../../components/ui/Card';
import { EmptyState } from '../../../components/ui/EmptyState';
import { Modal } from '../../../components/ui/Modal';
import { cn } from '../../../lib/cn';

type LoadStatus = 'idle' | 'loading' | 'ready' | 'error';

const BASE_URL = 'https://api.patchtrace.example/v1';

const SCOPE_OPTIONS = [
  { value: 'tasks:read', label: '读取副本', hint: '查看副本列表与详情' },
  { value: 'tasks:write', label: '接取副本', hint: '发布、接取与提交战利品' },
  { value: 'validate', label: '裁判复核', hint: '发起 双人连携验证与规则扫描' },
  { value: 'compliance', label: '规则查询', hint: '读取规则与 Boss 风险报告' },
];

const LOOP_STEPS = [
  '召唤模型',
  '接取副本',
  '裁判复核',
  '领取赏金',
  '技能沉淀',
];

const INTEGRATION_GUIDES = [
  {
    name: 'Claude Code',
    icon: Terminal,
    keywords: ['claude', 'anthropic'],
    fallbackModel: 'claude-sonnet-4-5',
    tip: '设置 ANTHROPIC_BASE_URL 与 ANTHROPIC_AUTH_TOKEN，战友选补给站目录中的 Claude 系列。',
  },
  {
    name: 'Codex CLI',
    icon: Code2,
    keywords: ['gpt', 'openai'],
    fallbackModel: 'gpt-5.1-codex',
    tip: '配置 OPENAI_BASE_URL 与 OPENAI_API_KEY，战友代号填补给站目录中的 GPT 系列。',
  },
  {
    name: 'Cursor',
    icon: MousePointer2,
    keywords: ['deepseek', 'qwen', 'claude'],
    fallbackModel: 'deepseek-v3',
    tip: '在 Settings → Models 中添加 OpenAI Compatible 供应商，填入 Base URL 与召唤密钥。',
  },
  {
    name: 'Chatbox',
    icon: MessageSquare,
    keywords: ['qwen', 'glm', 'kimi', 'minimax', 'gpt'],
    fallbackModel: 'qwen-max',
    tip: '选择 OpenAI API 兼容模式，填入 Base URL、召唤密钥与战友代号即可对话。',
  },
];

const formatInt = (value: number): string =>
  Number.isFinite(value) ? value.toLocaleString('zh-CN') : '0';

const formatPrice = (value: number): string =>
  Number.isFinite(value)
    ? `¥${value.toLocaleString('zh-CN', {
        minimumFractionDigits: 2,
        maximumFractionDigits: 4,
      })}`
    : '¥--';

const formatContext = (value: number): string => {
  if (!Number.isFinite(value) || value <= 0) {
    return '—';
  }
  if (value >= 1_000_000) {
    return `${(value / 1_000_000).toLocaleString('zh-CN', {
      maximumFractionDigits: 1,
    })}M`;
  }
  if (value >= 1_000) {
    return `${Math.round(value / 1_000)}K`;
  }
  return String(value);
};

const formatDateTime = (iso?: string | null): string => {
  if (!iso) {
    return '从未使用';
  }
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return '从未使用';
  }
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
};

const modelStatusLabel = (status: ModelCatalogItem['status']): string =>
  status === 'ONLINE' ? 'ONLINE · 待命' : 'DEGRADED · 虚弱';

function ModelCard({ model }: { model: ModelCatalogItem }) {
  return (
    <div className="rounded-lg border border-arcane/25 bg-arcane/5 p-3 transition-colors hover:border-arcane/60">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold text-arcane">
            {model.name}
          </p>
          <p className="truncate text-[11px] text-ink-dim">{model.provider}</p>
        </div>
        <Badge variant={model.status === 'ONLINE' ? 'success' : 'warning'}>
          {modelStatusLabel(model.status)}
        </Badge>
      </div>

      <dl className="mt-3 grid grid-cols-2 gap-x-3 gap-y-2 text-[11px]">
        <div>
          <dt className="text-ink-dim">上下文窗口</dt>
          <dd className="font-mono font-medium tabular-nums text-ink">
            {formatContext(model.contextWindow)}
          </dd>
        </div>
        <div>
          <dt className="text-ink-dim">延迟</dt>
          <dd className="font-mono font-medium tabular-nums text-ink">
            {formatInt(model.latencyMs)} ms
          </dd>
        </div>
        <div>
          <dt className="text-ink-dim">输入价格 / 百万 token</dt>
          <dd className="font-mono font-medium tabular-nums text-gold">
            {formatPrice(model.inputPricePerMTok)}
          </dd>
        </div>
        <div>
          <dt className="text-ink-dim">输出价格 / 百万 token</dt>
          <dd className="font-mono font-medium tabular-nums text-gold">
            {formatPrice(model.outputPricePerMTok)}
          </dd>
        </div>
      </dl>

      {(model.capabilities ?? []).length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1">
          {(model.capabilities ?? []).map((capability) => (
            <Badge key={capability} variant="outline">
              {capability}
            </Badge>
          ))}
        </div>
      )}
    </div>
  );
}

/**
 * 技能补给站：可召唤的 AI 战友、召唤密钥管理、魔力消耗卡、召唤指南与五步闭环条。
 * 数据来自 getApiGateway；缔结契约的 plaintextKey 只在弹窗内展示一次。
 */
export function ApiGatewayPanel() {
  const pushToast = useUiStore((state) => state.pushToast);

  const [overview, setOverview] = useState<ApiGatewayOverview | null>(null);
  const [status, setStatus] = useState<LoadStatus>('idle');
  const [createOpen, setCreateOpen] = useState(false);
  const [label, setLabel] = useState('');
  const [scopes, setScopes] = useState<string[]>(['tasks:read']);
  const [creating, setCreating] = useState(false);
  const [created, setCreated] = useState<CreateApiKeyResponse | null>(null);
  const [revokingId, setRevokingId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setStatus('loading');
    try {
      const response = await getApiGateway();
      setOverview(response);
      setStatus('ready');
    } catch {
      setStatus('error');
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const models = overview?.models ?? [];
  const keys = overview?.keys ?? [];
  const usage = overview?.usage;

  const pickModel = (keywords: string[], fallback: string): string => {
    const found = models.find((model) => {
      const haystack = `${model.provider} ${model.name} ${model.id}`.toLowerCase();
      return keywords.some((keyword) => haystack.includes(keyword));
    });
    return found?.id ?? fallback;
  };

  const closeCreate = () => {
    setCreateOpen(false);
    setLabel('');
    setScopes(['tasks:read']);
    setCreated(null);
  };

  const toggleScope = (scope: string) => {
    setScopes((current) =>
      current.includes(scope)
        ? current.filter((item) => item !== scope)
        : [...current, scope],
    );
  };

  const labelValid = label.trim().length > 0;
  const scopesValid = scopes.length > 0;

  const handleCreate = async () => {
    if (!labelValid || !scopesValid) {
      pushToast({
        id: `gateway-key-invalid-${Date.now()}`,
        title: '请完善召唤契约信息',
        description: '需要填写契约名称并至少授予一项权限。',
        variant: 'warning',
      });
      return;
    }
    setCreating(true);
    try {
      const response = await createApiKey({
        label: label.trim(),
        scopes,
      });
      setCreated(response);
      pushToast({
        id: `gateway-key-success-${Date.now()}`,
        title: '召唤契约已缔结',
        description: '完整召唤密钥仅显示一次，请立即复制保存。',
        variant: 'success',
      });
      void load();
    } catch (error) {
      pushToast({
        id: `gateway-key-error-${Date.now()}`,
        title: '缔结契约失败',
        description:
          error instanceof Error && error.message
            ? error.message
            : '请稍后重试。',
        variant: 'error',
      });
    } finally {
      setCreating(false);
    }
  };

  const handleCopy = async () => {
    if (!created) {
      return;
    }
    try {
      if (!navigator.clipboard?.writeText) {
        throw new Error('Clipboard API unavailable');
      }
      await navigator.clipboard.writeText(created.plaintextKey);
      pushToast({
        id: `gateway-key-copy-${Date.now()}`,
        title: '已复制到剪贴板',
        description: '请妥善保存，关闭弹窗后将无法再次查看完整召唤密钥。',
        variant: 'success',
      });
    } catch {
      pushToast({
        id: `gateway-key-copy-error-${Date.now()}`,
        title: '复制失败',
        description: '请手动选中完整召唤密钥后复制。',
        variant: 'warning',
      });
    }
  };

  const handleRevoke = async (key: ApiKeyRecord) => {
    if (key.status === 'REVOKED' || revokingId) {
      return;
    }
    setRevokingId(key.id);
    try {
      await revokeApiKey(key.id);
      pushToast({
        id: `gateway-key-revoke-${Date.now()}`,
        title: '召唤契约已解除',
        description: `${key.label} 契约已失效，后续召唤将被拒绝。`,
        variant: 'success',
      });
      await load();
    } catch (error) {
      pushToast({
        id: `gateway-key-revoke-error-${Date.now()}`,
        title: '解除契约失败',
        description:
          error instanceof Error && error.message
            ? error.message
            : '请稍后重试。',
        variant: 'error',
      });
    } finally {
      setRevokingId(null);
    }
  };

  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="space-y-1">
          <h2 className="flex items-center gap-1.5 text-sm font-semibold text-ink">
            <ServerCog className="h-4 w-4 text-neon" aria-hidden="true" />
            技能补给站
          </h2>
          <p className="text-xs text-ink-muted">
            一个 Base URL 召唤主流 AI 战友，统一计量魔力、鉴权与结算
          </p>
        </div>
        {status === 'ready' && (
          <Button variant="ghost" size="sm" onClick={() => void load()}>
            <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
            刷新
          </Button>
        )}
      </div>

      {status === 'error' ? (
        <Card className="p-5">
          <EmptyState
            icon={<AlertCircle className="h-8 w-8 text-rose-400" />}
            title="技能补给站加载失败"
            description="无法读取战友名册、召唤密钥与魔力消耗，请检查网络或稍后重试。"
            action={
              <Button variant="outline" size="sm" onClick={() => void load()}>
                <RefreshCw className="h-4 w-4" aria-hidden="true" />
                重试
              </Button>
            }
          />
        </Card>
      ) : status === 'loading' && !overview ? (
        <Card className="flex items-center justify-center gap-2 p-10 text-sm text-ink-muted">
          <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
          正在读取技能补给站数据…
        </Card>
      ) : overview ? (
        <>
          {/* 可召唤的 AI 战友 */}
          <Card className="p-5">
            <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
              <div className="space-y-1">
                <h3 className="text-sm font-semibold text-arcane">
                  可召唤的 AI 战友
                </h3>
                <p className="text-xs text-ink-muted">
                  价格按每百万 token 计，延迟为最近一次探测结果
                </p>
              </div>
              <Badge variant="info">
                <span className="font-mono">{models.length}</span> 位战友
              </Badge>
            </div>
            {models.length === 0 ? (
              <EmptyState
                className="py-8"
                title="暂无可用战友"
                description="战友名册为空，请稍后刷新或联系公会管理员。"
              />
            ) : (
              <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                {models.map((model) => (
                  <ModelCard key={model.id} model={model} />
                ))}
              </div>
            )}
          </Card>

          {/* 魔力消耗卡 */}
          {usage && (
            <Card className="p-5">
              <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
                <div className="space-y-1">
                  <h3 className="text-sm font-semibold text-ink">
                    魔力消耗
                  </h3>
                  <p className="font-mono text-xs text-ink-muted">
                    {usage.period}
                  </p>
                </div>
                <Badge variant="outline">统一计量</Badge>
              </div>
              <dl className="grid grid-cols-2 gap-3 md:grid-cols-4">
                <div className="rounded-lg bg-panel-soft p-3">
                  <dt className="text-[11px] text-ink-dim">输入魔力</dt>
                  <dd className="mt-1 font-mono text-lg font-semibold tabular-nums text-arcane">
                    {formatInt(usage.tokensIn)}
                  </dd>
                </div>
                <div className="rounded-lg bg-panel-soft p-3">
                  <dt className="text-[11px] text-ink-dim">输出魔力</dt>
                  <dd className="mt-1 font-mono text-lg font-semibold tabular-nums text-arcane">
                    {formatInt(usage.tokensOut)}
                  </dd>
                </div>
                <div className="rounded-lg bg-panel-soft p-3">
                  <dt className="text-[11px] text-ink-dim">施法次数</dt>
                  <dd className="mt-1 font-mono text-lg font-semibold tabular-nums text-neon">
                    {formatInt(usage.requests)}
                  </dd>
                </div>
                <div className="rounded-lg border border-gold/25 bg-gold/5 p-3">
                  <dt className="text-[11px] text-gold/80">本期金币消耗</dt>
                  <dd className="mt-1 font-mono text-lg font-semibold tabular-nums text-gold">
                    {formatPrice(usage.spend)}
                  </dd>
                </div>
              </dl>
            </Card>
          )}

          {/* 召唤密钥管理 */}
          <Card className="p-5">
            <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
              <div className="space-y-1">
                <h3 className="flex items-center gap-1.5 text-sm font-semibold text-ink">
                  <KeyRound
                    className="h-4 w-4 text-neon"
                    aria-hidden="true"
                  />
                  召唤密钥管理
                </h3>
                <p className="text-xs text-ink-muted">
                  同一召唤密钥可用于终端、IDE 与聊天客户端
                </p>
              </div>
              <Button size="sm" onClick={() => setCreateOpen(true)}>
                <Plus className="h-4 w-4" aria-hidden="true" />
                缔结契约
              </Button>
            </div>

            {keys.length === 0 ? (
              <EmptyState
                className="py-8"
                title="还没有召唤密钥"
                description="缔结第一份召唤契约，即可在 Claude Code / Codex CLI / Cursor / Chatbox 中召唤战友。"
                action={
                  <Button size="sm" onClick={() => setCreateOpen(true)}>
                    <Plus className="h-4 w-4" aria-hidden="true" />
                    缔结契约
                  </Button>
                }
              />
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full text-left text-xs">
                  <thead>
                    <tr className="border-b border-panel-line text-ink-dim">
                      <th className="pb-2 pr-3 font-medium">契约名称</th>
                      <th className="pb-2 pr-3 font-medium">召唤密钥</th>
                      <th className="pb-2 pr-3 font-medium">契约权限</th>
                      <th className="pb-2 pr-3 font-medium">状态</th>
                      <th className="pb-2 pr-3 font-medium">最后召唤</th>
                      <th className="pb-2 text-right font-medium">操作</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-panel-line">
                    {keys.map((key) => (
                      <tr key={key.id} className="align-top">
                        <td className="py-3 pr-3">
                          <p className="font-medium text-ink">
                            {key.label}
                          </p>
                          <p className="font-mono text-[11px] text-ink-dim">
                            缔结于 {formatDateTime(key.createdAt)}
                          </p>
                        </td>
                        <td className="py-3 pr-3">
                          <code className="rounded bg-panel-soft px-2 py-1 font-mono text-[11px] text-neon">
                            {key.maskedKey}
                          </code>
                        </td>
                        <td className="py-3 pr-3">
                          <div className="flex flex-wrap gap-1">
                            {(key.scopes ?? []).map((scope) => (
                              <Badge key={scope} variant="outline">
                                {scope}
                              </Badge>
                            ))}
                          </div>
                        </td>
                        <td className="py-3 pr-3">
                          <Badge
                            variant={
                              key.status === 'ACTIVE' ? 'success' : 'default'
                            }
                          >
                            {key.status === 'ACTIVE'
                              ? 'ACTIVE · 契约生效'
                              : 'REVOKED · 已解除'}
                          </Badge>
                        </td>
                        <td className="py-3 pr-3 font-mono text-ink-muted">
                          {formatDateTime(key.lastUsedAt)}
                        </td>
                        <td className="py-3 text-right">
                          <Button
                            variant="ghost"
                            size="sm"
                            disabled={
                              key.status === 'REVOKED' ||
                              revokingId === key.id
                            }
                            onClick={() => void handleRevoke(key)}
                            className="text-rose-300 hover:bg-rose-500/10 hover:text-rose-200"
                          >
                            {revokingId === key.id ? (
                              <Loader2
                                className="h-3.5 w-3.5 animate-spin"
                                aria-hidden="true"
                              />
                            ) : (
                              <Trash2
                                className="h-3.5 w-3.5"
                                aria-hidden="true"
                              />
                            )}
                            解除契约
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </Card>

          {/* 召唤指南 */}
          <Card className="p-5">
            <div className="mb-4 space-y-1">
              <h3 className="text-sm font-semibold text-ink">召唤指南</h3>
              <p className="text-xs text-ink-muted">
                统一使用 OpenAI / Anthropic 兼容协议，无需改代码即可更换 AI 战友
              </p>
            </div>
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              {INTEGRATION_GUIDES.map((guide) => {
                const Icon = guide.icon;
                const model = pickModel(guide.keywords, guide.fallbackModel);
                return (
                  <div
                    key={guide.name}
                    className="rounded-lg border border-panel-line bg-panel-soft p-4"
                  >
                    <p className="flex items-center gap-1.5 text-sm font-semibold text-ink">
                      <Icon
                        className="h-4 w-4 text-neon"
                        aria-hidden="true"
                      />
                      {guide.name}
                    </p>
                    <dl className="mt-3 space-y-2 text-[11px]">
                      <div>
                        <dt className="text-ink-dim">Base URL</dt>
                        <dd className="break-all font-mono text-neon">
                          {BASE_URL}
                        </dd>
                      </div>
                      <div>
                        <dt className="text-ink-dim">战友代号示例</dt>
                        <dd className="break-all font-mono text-arcane">
                          {model}
                        </dd>
                      </div>
                      <div>
                        <dt className="text-ink-dim">配置要点</dt>
                        <dd className="leading-5 text-ink-muted">{guide.tip}</dd>
                      </div>
                    </dl>
                  </div>
                );
              })}
            </div>
          </Card>

          {/* 五步闭环条 */}
          <Card className="p-5">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
              <h3 className="text-sm font-semibold text-ink">API 五步闭环</h3>
              <Badge variant="info">召唤模型 → 接取副本 → 裁判复核 → 领取赏金 → 技能沉淀</Badge>
            </div>
            <ol className="flex flex-wrap items-center gap-2">
              {LOOP_STEPS.map((step, index) => (
                <li key={step} className="flex items-center gap-2">
                  <span className="inline-flex items-center gap-1.5 rounded-full border border-neon/30 bg-neon/10 px-3 py-1 text-xs font-medium text-neon">
                    <span className="flex h-4 w-4 items-center justify-center rounded-full bg-neon font-mono text-[10px] font-semibold text-slate-950">
                      {index + 1}
                    </span>
                    {step}
                  </span>
                  {index < LOOP_STEPS.length - 1 && (
                    <ChevronRight
                      className="h-3.5 w-3.5 text-ink-dim"
                      aria-hidden="true"
                    />
                  )}
                </li>
              ))}
            </ol>
            <p className="mt-3 flex items-center gap-1.5 text-[11px] text-ink-muted">
              <ShieldCheck
                className="h-3.5 w-3.5 text-clear"
                aria-hidden="true"
              />
              每次召唤、裁判复核与赏金结算都会写入可追溯封印链，沉淀为团队技能资产。
            </p>
          </Card>
        </>
      ) : null}

      {/* 缔结 / 展示召唤密钥弹窗 */}
      <Modal
        open={createOpen}
        onClose={creating ? () => undefined : closeCreate}
        title={created ? '召唤契约已缔结' : '缔结召唤契约'}
        description={
          created
            ? '完整召唤密钥只显示一次，关闭后无法再次查看。'
            : '为不同工具缔结独立契约，便于追踪与解除。'
        }
        footer={
          created ? (
            <div className="flex items-center justify-end gap-2">
              <Button variant="secondary" onClick={handleCopy}>
                <Copy className="h-4 w-4" aria-hidden="true" />
                复制召唤密钥
              </Button>
              <Button onClick={closeCreate}>我已保存，关闭</Button>
            </div>
          ) : (
            <div className="flex items-center justify-end gap-2">
              <Button
                variant="secondary"
                onClick={closeCreate}
                disabled={creating}
              >
                取消
              </Button>
              <Button
                onClick={() => void handleCreate()}
                disabled={!labelValid || !scopesValid || creating}
              >
                {creating && (
                  <Loader2
                    className="h-4 w-4 animate-spin"
                    aria-hidden="true"
                  />
                )}
                缔结契约
              </Button>
            </div>
          )
        }
      >
        {created ? (
          <div className="space-y-3">
            <div className="rounded-xl border border-amber-500/40 bg-amber-500/10 p-4">
              <p className="flex items-center gap-1.5 text-xs font-semibold text-amber-200">
                <AlertCircle className="h-4 w-4" aria-hidden="true" />
                仅显示一次，请立即复制保存
              </p>
              <code className="mt-2 block whitespace-pre-wrap break-all rounded-lg bg-panel-soft px-3 py-2 font-mono text-xs text-gold ring-1 ring-inset ring-amber-500/30">
                {created.plaintextKey}
              </code>
              <div className="mt-3 flex justify-end">
                <Button size="sm" variant="outline" onClick={handleCopy}>
                  <Copy className="h-4 w-4" aria-hidden="true" />
                  复制完整召唤密钥
                </Button>
              </div>
            </div>
            <dl className="grid grid-cols-2 gap-3 text-xs">
              <div>
                <dt className="text-ink-dim">契约名称</dt>
                <dd className="font-medium text-ink">
                  {created.key.label}
                </dd>
              </div>
              <div>
                <dt className="text-ink-dim">召唤密钥</dt>
                <dd className="font-mono text-neon">
                  {created.key.maskedKey}
                </dd>
              </div>
              <div className="col-span-2">
                <dt className="text-ink-dim">契约权限</dt>
                <dd className="mt-1 flex flex-wrap gap-1">
                  {(created.key.scopes ?? []).map((scope) => (
                    <Badge key={scope} variant="outline">
                      {scope}
                    </Badge>
                  ))}
                </dd>
              </div>
            </dl>
          </div>
        ) : (
          <div className="space-y-4">
            <div>
              <label
                htmlFor="gateway-key-label"
                className="mb-1.5 block text-xs font-medium text-ink-muted"
              >
                契约名称
              </label>
              <input
                id="gateway-key-label"
                type="text"
                maxLength={40}
                placeholder="例如：Claude Code · 本机"
                value={label}
                onChange={(event) => setLabel(event.target.value)}
                className="h-10 w-full rounded-lg border border-panel-line bg-panel-soft px-3 text-sm text-ink outline-none transition-colors placeholder:text-ink-dim focus:border-neon/60 focus:ring-2 focus:ring-neon/20"
              />
            </div>

            <div>
              <p className="mb-1.5 text-xs font-medium text-ink-muted">
                契约权限（至少授予一项）
              </p>
              <div className="grid gap-2 sm:grid-cols-2">
                {SCOPE_OPTIONS.map((scope) => {
                  const active = scopes.includes(scope.value);
                  return (
                    <button
                      key={scope.value}
                      type="button"
                      aria-pressed={active}
                      onClick={() => toggleScope(scope.value)}
                      className={cn(
                        'flex items-start gap-2 rounded-lg border p-3 text-left transition-colors',
                        active
                          ? 'border-neon/60 bg-neon/10'
                          : 'border-panel-line bg-panel hover:border-neon/40 hover:bg-panel-soft',
                      )}
                    >
                      <span
                        className={cn(
                          'mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded border',
                          active
                            ? 'border-neon bg-neon text-slate-950'
                            : 'border-panel-line bg-panel text-transparent',
                        )}
                      >
                        <Check className="h-3 w-3" aria-hidden="true" />
                      </span>
                      <span className="min-w-0">
                        <span className="block text-xs font-medium text-ink">
                          {scope.label}
                        </span>
                        <span className="mt-0.5 block font-mono text-[11px] text-neon">
                          {scope.value}
                        </span>
                        <span className="block text-[11px] text-ink-muted">
                          {scope.hint}
                        </span>
                      </span>
                    </button>
                  );
                })}
              </div>
              {!scopesValid && (
                <p className="mt-1 text-[11px] text-rose-300">
                  请至少授予一项契约权限。
                </p>
              )}
            </div>
          </div>
        )}
      </Modal>
    </section>
  );
}
