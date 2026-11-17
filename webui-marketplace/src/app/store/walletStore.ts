import { create } from 'zustand';
import type {
  WalletBalance,
  WalletTransaction,
  WalletTransactionType,
} from '../../types/domain';
import type { WalletOverview, WalletVip } from '../../types/api';
import {
  getWallet,
  rechargeWallet,
  subscribeVip as subscribeVipApi,
} from '../../lib/api/client';

export interface WalletState {
  balance: WalletBalance;
  transactions: WalletTransaction[];
  vip: WalletVip | null;
  refresh(): Promise<void>;
  recharge(amount: number): Promise<void>;
  subscribeVip(level: 'PRO' | 'ENTERPRISE'): Promise<void>;
  freezeForTask(taskId: string, amount: number): Promise<void>;
  settleTask(taskId: string): Promise<void>;
}

let localTransactionSeq = 0;

const localTransactionId = (): string => {
  localTransactionSeq += 1;
  return `tx_local_${Date.now()}_${localTransactionSeq}`;
};

const makeTransaction = (
  type: WalletTransactionType,
  amount: number,
  taskId?: string,
): WalletTransaction => ({
  id: localTransactionId(),
  type,
  amount,
  taskId,
  at: new Date().toISOString(),
  status: 'SETTLED',
});

/** 把钱包接口返回值一次性写入 state，充值 / VIP / 刷新共用。 */
const overviewPatch = (response: WalletOverview) => ({
  balance: response.balance,
  transactions: response.transactions,
  vip: response.vip ?? null,
});

export const useWalletStore = create<WalletState>((set, get) => ({
  balance: { available: 0, frozen: 0, currency: 'CNY' },
  transactions: [],
  vip: null,

  refresh: async () => {
    const response = await getWallet();
    set(overviewPatch(response));
  },

  recharge: async (amount) => {
    const response = await rechargeWallet({ amount });
    set(overviewPatch(response));
  },

  subscribeVip: async (level) => {
    const response = await subscribeVipApi({ level });
    set(overviewPatch(response));
  },

  freezeForTask: async (taskId, amount) => {
    const { balance, transactions } = get();
    if (amount <= 0) {
      throw new Error('Escrow amount must be positive');
    }
    if (amount > balance.available) {
      throw new Error('Insufficient available balance');
    }
    set({
      balance: {
        ...balance,
        available: balance.available - amount,
        frozen: balance.frozen + amount,
      },
      transactions: [
        makeTransaction('ESCROW_FREEZE', amount, taskId),
        ...transactions,
      ],
    });
  },

  settleTask: async (taskId) => {
    const { balance, transactions } = get();
    const alreadyReleased = transactions.some(
      (transaction) =>
        transaction.taskId === taskId && transaction.type === 'ESCROW_RELEASE',
    );
    if (alreadyReleased) {
      return;
    }

    const freeze = transactions.find(
      (transaction) =>
        transaction.taskId === taskId && transaction.type === 'ESCROW_FREEZE',
    );
    const amount = freeze?.amount ?? 0;
    if (amount <= 0) {
      return;
    }

    set({
      balance: {
        ...balance,
        frozen: Math.max(0, balance.frozen - amount),
      },
      transactions: [
        makeTransaction('ESCROW_RELEASE', amount, taskId),
        ...transactions,
      ],
    });
  },
}));
