import { useCallback, useEffect, useState } from 'react';
import {
  AlertCircle,
  ArrowDownLeft,
  ArrowUpRight,
  Crown,
  Landmark,
  Loader2,
  Percent,
  RefreshCw,
  Sparkles,
  TrendingUp,
  Wallet,
} from 'lucide-react';
import type {
  WalletTransaction,
  WalletTransactionStatus,
  WalletTransactionType,
} from '../../../types/domain';
import type { WalletVip } from '../../../types/api';
import { useWalletStore } from '../../../app/store/walletStore';
import { useUiStore } from '../../../app/store/uiStore';
import { Badge } from '../../../components/ui/Badge';
import { Button } from '../../../components/ui/Button';
import { Card } from '../../../components/ui/Card';
import { EmptyState } from '../../../components/ui/EmptyState';
import { Modal } from '../../../components/ui/Modal';
import { cn } from '../../../lib/cn';

type VipLevel = 'PRO' | 'ENTERPRISE';
type LoadStatus = 'idle' | 'loading' | 'ready' | 'error';

const VIP_LABELS: Record<WalletVip['level'], string> = {
  FREE: '免费玩家',
  PRO: 'PRO 月卡',
  ENTERPRISE: 'ENTERPRISE 月卡',
};

const VIP_TIER_OPTIONS: {
  level: VipLevel;
  feeRate: number;
  summary: string;
  perks: string[];
}[] = [
  {
    level: 'PRO',
    feeRate: 0.1,
    summary: '适合独行冒险者与高频接取副本',
    perks: ['平台服务费 10%', '优先副本匹配', '技能补给站标准配额'],
  },
  {
    level: 'ENTERPRISE',
    feeRate: 0.05,
    summary: '适合公会与团队级委托',
    perks: ['平台服务费 5%', '专属副本池', '技能补给站高配额与优先通道'],
  },
];

const TRANSACTION_TYPE_LABELS: Record<WalletTransactionType, string> = {
  ESCROW_FREEZE: '押金冻结',
  ESCROW_RELEASE: '赏金释放',
  REFUND: '赏金退回',
  DEPOSIT: '充值金币',
  WITHDRAW: '提现',
  FEE: '平台服务费',
};

const TRANSACTION_STATUS_LABELS: Record<WalletTransactionStatus, string> = {
  PENDING: '待结算',
  SETTLED: '已结算',
  FAILED: '失败',
};

const INCOME_TYPES: WalletTransactionType[] = [
  'DEPOSIT',
  'REFUND',
  'ESCROW_RELEASE',
];

const RECHARGE_PRESETS = [100, 500, 1000, 5000];
const MAX_RECHARGE_AMOUNT = 1_000_000;

const formatAmount = (amount: number): string =>
  Number.isFinite(amount)
    ? amount.toLocaleString('zh-CN', {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      })
    : '0.00';

const formatCurrency = (amount: number): string => `¥${formatAmount(amount)}`;

/** 兼容 0.05 与 5 两种后端写法，统一输出百分比。 */
const formatFeeRate = (rate: number): string => {
  if (!Number.isFinite(rate) || rate < 0) {
    return '--';
  }
  const percent = rate > 1 ? rate : rate * 100;
  return `${percent.toLocaleString('zh-CN', {
    minimumFractionDigits: 0,
    maximumFractionDigits: 1,
  })}%`;
};

const formatDateTime = (iso?: string | null): string => {
  if (!iso) {
    return '—';
  }
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return '—';
  }
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
};

const statusVariant = (
  status: WalletTransactionStatus,
): 'success' | 'warning' | 'danger' => {
  if (status === 'SETTLED') {
    return 'success';
  }
  if (status === 'PENDING') {
    return 'warning';
  }
  return 'danger';
};

function TransactionRow({ transaction }: { transaction: WalletTransaction }) {
  const income = INCOME_TYPES.includes(transaction.type);
  return (
    <li className="flex items-center justify-between gap-3 py-2.5">
      <div className="flex min-w-0 items-center gap-2.5">
        <span
          className={cn(
            'flex h-8 w-8 shrink-0 items-center justify-center rounded-full',
            income
              ? 'bg-emerald-50 text-emerald-600'
              : 'bg-rose-50 text-rose-600',
          )}
        >
          {income ? (
            <ArrowDownLeft className="h-4 w-4" aria-hidden="true" />
          ) : (
            <ArrowUpRight className="h-4 w-4" aria-hidden="true" />
          )}
        </span>
        <div className="min-w-0">
          <p className="truncate text-sm font-medium text-ink">
            {TRANSACTION_TYPE_LABELS[transaction.type] ?? transaction.type}
            {transaction.taskId && (
              <span className="ml-1.5 font-mono text-xs font-normal text-ink-dim">
                {transaction.taskId}
              </span>
            )}
          </p>
          <p className="font-mono text-[11px] text-ink-dim">
            {formatDateTime(transaction.at)}
          </p>
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <Badge variant={statusVariant(transaction.status)}>
          {TRANSACTION_STATUS_LABELS[transaction.status] ?? transaction.status}
        </Badge>
        <span
          className={cn(
            'font-mono text-sm font-semibold tabular-nums',
            income ? 'text-emerald-300' : 'text-rose-300',
          )}
        >
          {income ? '+' : '-'}
          {formatCurrency(transaction.amount)}
        </span>
      </div>
    </li>
  );
}

/**
 * 金币袋：可用金币 / 月卡、三条金币流说明、近 10 笔金币流水，
 * 以及充值金币、购买月卡两个弹窗。数据来自 walletStore（mock / HTTP 同构）。
 */
export function WalletPanel() {
  const balance = useWalletStore((state) => state.balance);
  const transactions = useWalletStore((state) => state.transactions);
  const vip = useWalletStore((state) => state.vip);
  const refresh = useWalletStore((state) => state.refresh);
  const recharge = useWalletStore((state) => state.recharge);
  const subscribeVip = useWalletStore((state) => state.subscribeVip);
  const pushToast = useUiStore((state) => state.pushToast);

  const [status, setStatus] = useState<LoadStatus>('idle');
  const [rechargeOpen, setRechargeOpen] = useState(false);
  const [preset, setPreset] = useState<number | null>(100);
  const [customAmount, setCustomAmount] = useState('');
  const [submittingRecharge, setSubmittingRecharge] = useState(false);
  const [vipOpen, setVipOpen] = useState(false);
  const [vipLevel, setVipLevel] = useState<VipLevel>('PRO');
  const [submittingVip, setSubmittingVip] = useState(false);

  const load = useCallback(async () => {
    setStatus('loading');
    try {
      await refresh();
      setStatus('ready');
    } catch {
      setStatus('error');
    }
  }, [refresh]);

  useEffect(() => {
    void load();
  }, [load]);

  const currentVip = vip;
  const vipLevelLabel = VIP_LABELS[currentVip?.level ?? 'FREE'];
  const vipFeeRate = currentVip?.feeRate ?? 0;
  const currentFeeLabel = formatFeeRate(vipFeeRate);
  const recentTransactions = transactions.slice(0, 10);

  const parsedCustom = Number(customAmount);
  const amount = preset ?? parsedCustom;
  const amountValid =
    Number.isFinite(amount) &&
    amount > 0 &&
    amount <= MAX_RECHARGE_AMOUNT &&
    /^\d*(\.\d{0,2})?$/.test(customAmount || '0');
  const showAmountError =
    preset === null &&
    customAmount.trim().length > 0 &&
    !amountValid;

  const closeRecharge = () => {
    setRechargeOpen(false);
    setPreset(100);
    setCustomAmount('');
  };

  const closeVip = () => {
    setVipOpen(false);
    setVipLevel('PRO');
  };

  const handleRecharge = async () => {
    if (!amountValid) {
      pushToast({
        id: `wallet-recharge-invalid-${Date.now()}`,
        title: '金币金额不合法',
        description: `请输入 0 到 ${MAX_RECHARGE_AMOUNT.toLocaleString('zh-CN')} 之间的金币数量，最多两位小数。`,
        variant: 'warning',
      });
      return;
    }
    setSubmittingRecharge(true);
    try {
      await recharge(amount);
      pushToast({
        id: `wallet-recharge-success-${Date.now()}`,
        title: '金币已入袋',
        description: `已充值 ${formatCurrency(amount)}，可用金币已更新。`,
        variant: 'success',
      });
      closeRecharge();
    } catch (error) {
      pushToast({
        id: `wallet-recharge-error-${Date.now()}`,
        title: '充值金币失败',
        description:
          error instanceof Error && error.message
            ? error.message
            : '请稍后重试。',
        variant: 'error',
      });
    } finally {
      setSubmittingRecharge(false);
    }
  };

  const handleSubscribeVip = async () => {
    setSubmittingVip(true);
    try {
      await subscribeVip(vipLevel);
      const tier = VIP_TIER_OPTIONS.find((option) => option.level === vipLevel);
      pushToast({
        id: `wallet-vip-success-${Date.now()}`,
        title: '月卡已激活',
        description: `已购买 ${VIP_LABELS[vipLevel]}${
          tier ? `，平台服务费 ${formatFeeRate(tier.feeRate)}` : ''
        }。`,
        variant: 'success',
      });
      closeVip();
    } catch (error) {
      pushToast({
        id: `wallet-vip-error-${Date.now()}`,
        title: '购买月卡失败',
        description:
          error instanceof Error && error.message
            ? error.message
            : '请稍后重试。',
        variant: 'error',
      });
    } finally {
      setSubmittingVip(false);
    }
  };

  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="space-y-1">
          <h2 className="flex items-center gap-1.5 text-sm font-semibold text-ink">
            <Wallet className="h-4 w-4 text-gold" aria-hidden="true" />
            金币袋
          </h2>
          <p className="text-xs text-ink-muted">
            押金托管、赏金结算与月卡服务费一屏掌握
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
            title="金币袋数据加载失败"
            description="无法读取金币余额与金币流水，请检查网络或稍后重试。"
            action={
              <Button variant="outline" size="sm" onClick={() => void load()}>
                <RefreshCw className="h-4 w-4" aria-hidden="true" />
                重试
              </Button>
            }
          />
        </Card>
      ) : status === 'loading' && transactions.length === 0 ? (
        <Card className="flex items-center justify-center gap-2 p-10 text-sm text-ink-muted">
          <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
          正在读取金币袋数据…
        </Card>
      ) : (
        <Card className="overflow-hidden">
          {/* 金币袋余额卡 */}
          <div className="relative overflow-hidden border-b border-panel-line bg-gradient-to-br from-panel-soft via-panel to-abyss p-6">
            <div
              className="pointer-events-none absolute -right-10 -top-16 h-48 w-48 rounded-full bg-gold/10 blur-2xl"
              aria-hidden="true"
            />
            <div
              className="pointer-events-none absolute -bottom-20 right-16 h-40 w-40 rounded-full bg-neon/5 blur-2xl"
              aria-hidden="true"
            />

            <div className="relative flex flex-wrap items-start justify-between gap-4">
              <div className="space-y-1">
                <p className="text-xs font-medium uppercase tracking-wide text-ink-dim">
                  可用金币（CNY）
                </p>
                <p className="font-mono text-4xl font-semibold tabular-nums tracking-tight text-gold">
                  {formatCurrency(balance.available)}
                </p>
                <p className="text-xs text-ink-muted">
                  押金冻结{' '}
                  <span className="font-mono font-medium text-ink">
                    {formatCurrency(balance.frozen)}
                  </span>
                </p>
              </div>

              <div className="space-y-2 text-right">
                <Badge className="border-gold/40 bg-gold/10 text-gold">
                  <Crown className="h-3.5 w-3.5" aria-hidden="true" />
                  {vipLevelLabel}
                </Badge>
                <p className="text-[11px] text-ink-muted">
                  {currentVip && currentVip.level !== 'FREE'
                    ? `月卡到期 ${formatDateTime(currentVip.expiresAt)}`
                    : '尚未购买月卡'}
                </p>
                {currentVip && currentVip.level !== 'FREE' && (
                  <p className="text-[11px] text-ink-muted">
                    平台服务费{' '}
                    <span className="font-mono">{currentFeeLabel}</span>
                  </p>
                )}
                <div className="flex flex-wrap justify-end gap-2 pt-1">
                  <Button
                    size="sm"
                    variant="primary"
                    onClick={() => setRechargeOpen(true)}
                  >
                    充值金币
                  </Button>
                  <Button
                    size="sm"
                    variant="secondary"
                    className="border-arcane/40 text-arcane hover:border-arcane/70 hover:text-arcane"
                    onClick={() => setVipOpen(true)}
                  >
                    <Crown className="h-3.5 w-3.5" aria-hidden="true" />
                    购买月卡
                  </Button>
                </div>
              </div>
            </div>
          </div>

          {/* 三条金币流说明 */}
          <div className="grid gap-3 border-b border-panel-line p-5 md:grid-cols-3">
            <div className="rounded-lg bg-panel-soft p-3">
              <p className="flex items-center gap-1.5 text-xs font-semibold text-ink">
                <Landmark className="h-4 w-4 text-gold" aria-hidden="true" />
                雇主押金托管
              </p>
              <p className="mt-1 text-[11px] leading-5 text-ink-muted">
                副本发布时冻结押金至托管账户，裁判验收通过后按里程碑释放给冒险者。
              </p>
            </div>
            <div className="rounded-lg bg-panel-soft p-3">
              <p className="flex items-center gap-1.5 text-xs font-semibold text-ink">
                <TrendingUp
                  className="h-4 w-4 text-clear"
                  aria-hidden="true"
                />
                冒险者副本赏金
              </p>
              <p className="mt-1 text-[11px] leading-5 text-ink-muted">
                副本通关并通过 AI 裁判 / 规则扫描后，赏金计入可用金币。
              </p>
            </div>
            <div className="rounded-lg bg-panel-soft p-3">
              <p className="flex items-center gap-1.5 text-xs font-semibold text-ink">
                <Percent className="h-4 w-4 text-arcane" aria-hidden="true" />
                平台 AI 增值服务费
              </p>
              <p className="mt-1 text-[11px] leading-5 text-ink-muted">
                按副本赏金抽成，当前费率{' '}
                <span className="font-mono font-semibold text-gold">
                  {currentFeeLabel}
                </span>
                {currentVip?.level === 'ENTERPRISE'
                  ? '（ENTERPRISE 月卡）'
                  : currentVip?.level === 'PRO'
                    ? '（PRO 月卡）'
                    : '（免费玩家）'}
                。
              </p>
            </div>
          </div>

          {/* 近 10 笔金币流水 */}
          <div className="p-5">
            <div className="mb-2 flex items-center justify-between">
              <h3 className="text-xs font-semibold text-ink">
                近 10 笔金币流水
              </h3>
              <span className="font-mono text-[11px] text-ink-dim">
                共 {transactions.length} 笔
              </span>
            </div>
            {recentTransactions.length === 0 ? (
              <EmptyState
                className="py-8"
                icon={<Sparkles className="h-7 w-7 text-gold/60" />}
                title="暂无金币流水"
                description="发布或通关首个副本后，这里会显示押金与结算记录。"
              />
            ) : (
              <ul className="divide-y divide-panel-line">
                {recentTransactions.map((transaction) => (
                  <TransactionRow
                    key={transaction.id}
                    transaction={transaction}
                  />
                ))}
              </ul>
            )}
          </div>
        </Card>
      )}

      {/* 充值金币弹窗 */}
      <Modal
        open={rechargeOpen}
        onClose={closeRecharge}
        title="充值金币"
        description="充值金币将进入金币袋，可用于副本押金托管。"
        footer={
          <div className="flex items-center justify-end gap-2">
            <Button variant="secondary" onClick={closeRecharge}>
              取消
            </Button>
            <Button
              onClick={() => void handleRecharge()}
              disabled={!amountValid || submittingRecharge}
            >
              {submittingRecharge && (
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
              )}
              确认充值金币
            </Button>
          </div>
        }
      >
        <div className="space-y-4">
          <div>
            <p className="mb-2 text-xs font-medium text-ink-muted">快捷金币</p>
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
              {RECHARGE_PRESETS.map((value) => {
                const active = preset === value;
                return (
                  <button
                    key={value}
                    type="button"
                    aria-pressed={active}
                    onClick={() => {
                      setPreset(value);
                      setCustomAmount('');
                    }}
                    className={cn(
                      'rounded-lg border px-3 py-2 font-mono text-sm font-semibold tabular-nums transition-colors',
                      active
                        ? 'border-gold/60 bg-gold/10 text-gold'
                        : 'border-panel-line bg-panel text-ink-muted hover:border-gold/40 hover:bg-panel-soft',
                    )}
                  >
                    ¥{value.toLocaleString('zh-CN')}
                  </button>
                );
              })}
            </div>
          </div>

          <div>
            <label
              htmlFor="wallet-recharge-custom"
              className="mb-1.5 block text-xs font-medium text-ink-muted"
            >
              自定义金币
            </label>
            <div className="relative">
              <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 font-mono text-sm text-gold">
                ¥
              </span>
              <input
                id="wallet-recharge-custom"
                type="number"
                min="0.01"
                step="0.01"
                inputMode="decimal"
                placeholder="输入充值金币数量"
                value={customAmount}
                onChange={(event) => {
                  setPreset(null);
                  setCustomAmount(event.target.value);
                }}
                className="h-10 w-full rounded-lg border border-panel-line bg-panel-soft pl-7 pr-3 font-mono text-sm text-ink outline-none transition-colors focus:border-gold/60 focus:ring-2 focus:ring-gold/20"
              />
            </div>
            {showAmountError ? (
              <p className="mt-1 text-[11px] text-rose-300">
                请输入大于 0 且不超过{' '}
                {MAX_RECHARGE_AMOUNT.toLocaleString('zh-CN')}{' '}
                的金币数量，最多两位小数。
              </p>
            ) : (
              <p className="mt-1 font-mono text-[11px] text-ink-dim">
                当前充值金币：{amountValid ? formatCurrency(amount) : '—'}
              </p>
            )}
          </div>
        </div>
      </Modal>

      {/* 月卡弹窗 */}
      <Modal
        open={vipOpen}
        onClose={closeVip}
        title="购买月卡"
        description="购买后立即生效，平台服务费按新月卡费率结算。"
        footer={
          <div className="flex items-center justify-end gap-2">
            <Button variant="secondary" onClick={closeVip}>
              取消
            </Button>
            <Button
              onClick={() => void handleSubscribeVip()}
              disabled={submittingVip}
            >
              {submittingVip && (
                <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
              )}
              确认购买 {VIP_LABELS[vipLevel]}
            </Button>
          </div>
        }
      >
        <div className="space-y-3">
          {VIP_TIER_OPTIONS.map((option) => {
            const active = vipLevel === option.level;
            const isCurrent = currentVip?.level === option.level;
            return (
              <button
                key={option.level}
                type="button"
                aria-pressed={active}
                onClick={() => setVipLevel(option.level)}
                className={cn(
                  'w-full rounded-xl border p-4 text-left transition-colors',
                  active
                    ? 'border-arcane/60 bg-arcane/10'
                    : 'border-panel-line bg-panel hover:border-arcane/40 hover:bg-panel-soft',
                )}
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span className="flex items-center gap-2 text-sm font-semibold text-ink">
                    <Crown
                      className={cn(
                        'h-4 w-4',
                        active ? 'text-arcane' : 'text-ink-dim',
                      )}
                      aria-hidden="true"
                    />
                    {VIP_LABELS[option.level]}
                    {isCurrent && <Badge variant="success">当前月卡</Badge>}
                  </span>
                  <span className="font-mono text-lg font-semibold tabular-nums text-gold">
                    {formatFeeRate(option.feeRate)}
                    <span className="ml-1 text-[11px] font-normal text-ink-muted">
                      服务费
                    </span>
                  </span>
                </div>
                <p className="mt-1 text-xs text-ink-muted">{option.summary}</p>
                <ul className="mt-2 space-y-1">
                  {option.perks.map((perk) => (
                    <li
                      key={perk}
                      className="flex items-center gap-1.5 text-[11px] text-ink-muted"
                    >
                      <Sparkles
                        className="h-3 w-3 text-arcane"
                        aria-hidden="true"
                      />
                      {perk}
                    </li>
                  ))}
                </ul>
              </button>
            );
          })}
          <p className="rounded-lg bg-amber-50 px-3 py-2 text-[11px] leading-5 text-amber-700">
            当前费率：{currentFeeLabel}（{vipLevelLabel}）。购买{' '}
            {VIP_LABELS[vipLevel]} 后，新副本的平台服务费按{' '}
            {formatFeeRate(
              VIP_TIER_OPTIONS.find((option) => option.level === vipLevel)
                ?.feeRate ?? 0,
            )}{' '}
            结算。
          </p>
        </div>
      </Modal>
    </section>
  );
}
