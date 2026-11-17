package marketplace

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
)

type vipPlan struct {
	fee     float64
	feeRate float64
}

var vipPlans = map[WalletVipLevel]vipPlan{
	VipPro:        {fee: 99, feeRate: 0.10},
	VipEnterprise: {fee: 999, feeRate: 0.05},
}

// Recharge adds funds to the available balance and records a DEPOSIT entry.
func (s *Store) Recharge(amount float64) (WalletOverview, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
		return WalletOverview{}, newError(http.StatusBadRequest, "amount must be greater than 0")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := nowString()
	s.wallet.Balance.Available += amount
	transaction := WalletTransaction{
		ID:     fmt.Sprintf("tx_%03d", s.nextTxID),
		Type:   TxDeposit,
		Amount: amount,
		At:     now,
		Status: TxSettled,
	}
	s.nextTxID++
	s.wallet.Transactions = append([]WalletTransaction{transaction}, s.wallet.Transactions...)

	if err := s.persistLocked(); err != nil {
		return WalletOverview{}, newError(http.StatusInternalServerError, "persist recharge: %v", err)
	}
	return cloneWallet(s.wallet), nil
}

// SubscribeVip charges the plan fee and updates the VIP level, expiry and fee
// rate. An insufficient available balance yields a 400 domain error.
func (s *Store) SubscribeVip(level string) (WalletOverview, error) {
	tier := WalletVipLevel(strings.ToUpper(strings.TrimSpace(level)))
	plan, ok := vipPlans[tier]
	if !ok {
		return WalletOverview{}, newError(http.StatusBadRequest, "invalid vip level")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.wallet.Balance.Available < plan.fee {
		return WalletOverview{}, newError(http.StatusBadRequest, "insufficient balance")
	}

	now := time.Now().UTC()
	s.wallet.Balance.Available -= plan.fee
	s.wallet.Vip = WalletVip{
		Level:     tier,
		ExpiresAt: stringPtr(now.AddDate(1, 0, 0).Format(time.RFC3339)),
		FeeRate:   plan.feeRate,
	}
	transaction := WalletTransaction{
		ID:     fmt.Sprintf("tx_%03d", s.nextTxID),
		Type:   TxFee,
		Amount: plan.fee,
		At:     now.Format(time.RFC3339),
		Status: TxSettled,
	}
	s.nextTxID++
	s.wallet.Transactions = append([]WalletTransaction{transaction}, s.wallet.Transactions...)

	if err := s.persistLocked(); err != nil {
		return WalletOverview{}, newError(http.StatusInternalServerError, "persist vip subscription: %v", err)
	}
	return cloneWallet(s.wallet), nil
}
