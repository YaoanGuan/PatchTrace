import { create } from 'zustand';
import type {
  WalletBalance,
  WalletTransaction,
  WalletTransactionType,
} from '../../types/domain';
import { getWallet } from '../../lib/api/client';

export interface WalletState {
  balance: WalletBalance;
  transactions: WalletTransaction[];
  refresh(): Promise<void>;
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

export const useWalletStore = create<WalletState>((set, get) => ({
  balance: { available: 0, frozen: 0, currency: 'CNY' },
  transactions: [],

  refresh: async () => {
    const response = await getWallet();
    set({
      balance: response.balance,
      transactions: response.transactions,
    });
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
