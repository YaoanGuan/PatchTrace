package marketplace

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func newStoreServer(t *testing.T, store *Store) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(NewRouter(store))
	t.Cleanup(server.Close)
	return server
}

func TestExpertOverview(t *testing.T) {
	server := newTestServer(t)

	response, raw := doJSON(t, server, http.MethodGet, "/api/expert/overview", nil)
	assertStatus(t, response, http.StatusOK)
	assertJSONContentType(t, response)

	overview := decodeBody[ExpertOverview](t, raw)
	if len(overview.Tools) != 7 {
		t.Fatalf("tool count = %d, want 7", len(overview.Tools))
	}

	categories := map[AiToolCategory]int{}
	for _, tool := range overview.Tools {
		categories[tool.Category]++
		if tool.ID == "" || tool.Name == "" || len(tool.Roles) == 0 || len(tool.Compliance) == 0 {
			t.Fatalf("incomplete tool: %+v", tool)
		}
	}
	for _, category := range []AiToolCategory{AiToolCloudComputer, AiToolCLI, AiToolIDE, AiToolChatUI} {
		if categories[category] == 0 {
			t.Fatalf("missing tool category %s (got %v)", category, categories)
		}
	}

	if overview.Validation.Generator != "Claude" || overview.Validation.Reviewer != "Codex" {
		t.Fatalf("unexpected validation agents: %+v", overview.Validation)
	}
	if overview.Validation.Consensus != VerdictPass || overview.Validation.Confidence != 0.94 {
		t.Fatalf("unexpected validation verdict: %+v", overview.Validation)
	}
	if len(overview.Compliance) != 4 {
		t.Fatalf("compliance count = %d, want 4", len(overview.Compliance))
	}
	if got := countBlocks(overview.Compliance); got != 1 {
		t.Fatalf("overview BLOCK count = %d, want 1", got)
	}
	if overview.Risk.Score != 82 || overview.Risk.Level != RiskHigh || !overview.Risk.Blocked || len(overview.Risk.Reasons) != 2 {
		t.Fatalf("unexpected risk: %+v", overview.Risk)
	}
	if len(overview.Pricing) != 3 {
		t.Fatalf("pricing count = %d, want 3", len(overview.Pricing))
	}
}

func TestTaskTrust(t *testing.T) {
	server := newTestServer(t)

	response, raw := doJSON(t, server, http.MethodGet, "/api/tasks/task_1001/trust", nil)
	assertStatus(t, response, http.StatusOK)

	trust := decodeBody[TaskTrust](t, raw)
	if trust.TaskID != "task_1001" {
		t.Fatalf("taskId = %s, want task_1001", trust.TaskID)
	}
	if len(trust.Compliance) != 4 {
		t.Fatalf("compliance count = %d, want 4", len(trust.Compliance))
	}
	if trust.Validation.Consensus != VerdictPending || trust.Validation.GeneratorVerdict != VerdictPending || trust.Validation.ReviewerVerdict != VerdictPending {
		t.Fatalf("task_1001 should be pending: %+v", trust.Validation)
	}
	if trust.Validation.Confidence != 0 || trust.Validation.Cost != 0 || trust.Validation.ElapsedMs != 0 {
		t.Fatalf("task_1001 should have zero usage: %+v", trust.Validation)
	}
	if trust.Risk.Blocked || trust.Risk.Level != RiskMedium || trust.Risk.Score != 58 {
		t.Fatalf("task_1001 should not be blocked: %+v", trust.Risk)
	}
	if got := countBlocks(trust.Compliance); got != 0 {
		t.Fatalf("task_1001 BLOCK count = %d, want 0", got)
	}

	highRisk, highRiskRaw := doJSON(t, server, http.MethodGet, "/api/tasks/task_1003/trust", nil)
	assertStatus(t, highRisk, http.StatusOK)
	highRiskTrust := decodeBody[TaskTrust](t, highRiskRaw)
	if !highRiskTrust.Risk.Blocked || highRiskTrust.Risk.Score != 82 || highRiskTrust.Risk.Level != RiskHigh {
		t.Fatalf("task_1003 should be blocked: %+v", highRiskTrust.Risk)
	}
	if got := countBlocks(highRiskTrust.Compliance); got != 1 {
		t.Fatalf("task_1003 BLOCK count = %d, want 1", got)
	}
	if highRiskTrust.Validation.Consensus != VerdictPending {
		t.Fatalf("task_1003 consensus = %s, want PENDING", highRiskTrust.Validation.Consensus)
	}

	completed, completedRaw := doJSON(t, server, http.MethodGet, "/api/tasks/task_1006/trust", nil)
	assertStatus(t, completed, http.StatusOK)
	completedTrust := decodeBody[TaskTrust](t, completedRaw)
	if completedTrust.Validation.Consensus != VerdictPass {
		t.Fatalf("task_1006 consensus = %s, want PASS", completedTrust.Validation.Consensus)
	}
	if completedTrust.Risk.Blocked {
		t.Fatalf("task_1006 should not be blocked: %+v", completedTrust.Risk)
	}

	missing, missingRaw := doJSON(t, server, http.MethodGet, "/api/tasks/task_9999/trust", nil)
	assertStatus(t, missing, http.StatusNotFound)
	if message := errorMessage(t, missingRaw); message != "task not found" {
		t.Fatalf("error = %q, want task not found", message)
	}
}

func TestTaskTrustInvariants(t *testing.T) {
	server := newTestServer(t)

	response, raw := doJSON(t, server, http.MethodGet, "/api/tasks?page=1", nil)
	assertStatus(t, response, http.StatusOK)
	page := decodeBody[GetTasksResponse](t, raw)

	pendingStatuses := map[TaskStatus]bool{
		StatusDraft:      true,
		StatusPublished:  true,
		StatusAccepted:   true,
		StatusInProgress: true,
		StatusDisputed:   true,
	}

	for _, summary := range page.Items {
		trustResponse, trustRaw := doJSON(t, server, http.MethodGet, "/api/tasks/"+summary.ID+"/trust", nil)
		assertStatus(t, trustResponse, http.StatusOK)
		trust := decodeBody[TaskTrust](t, trustRaw)

		hasBlock := countBlocks(trust.Compliance) > 0
		if trust.Risk.Blocked != hasBlock {
			t.Fatalf("task %s blocked=%v but hasBlock=%v", summary.ID, trust.Risk.Blocked, hasBlock)
		}
		if pendingStatuses[summary.Status] && trust.Validation.Consensus != VerdictPending {
			t.Fatalf("task %s status=%s consensus=%s, want PENDING", summary.ID, summary.Status, trust.Validation.Consensus)
		}
		if trust.TaskID != summary.ID || trust.Validation.TaskID != summary.ID {
			t.Fatalf("task %s trust taskId mismatch: %+v", summary.ID, trust)
		}
	}
}

func countBlocks(findings []ComplianceFinding) int {
	count := 0
	for _, finding := range findings {
		if finding.Status == ComplianceBlock {
			count++
		}
	}
	return count
}

func TestWalletIncludesVip(t *testing.T) {
	server := newTestServer(t)

	response, raw := doJSON(t, server, http.MethodGet, "/api/wallet", nil)
	assertStatus(t, response, http.StatusOK)

	wallet := decodeBody[WalletOverview](t, raw)
	if wallet.Vip.Level != VipFree || wallet.Vip.FeeRate != 0.15 || wallet.Vip.ExpiresAt != nil {
		t.Fatalf("unexpected initial vip: %+v", wallet.Vip)
	}
	if wallet.Balance.Available != 3200 || wallet.Balance.Frozen != 1200 {
		t.Fatalf("unexpected balance: %+v", wallet.Balance)
	}
}

func TestRechargeWallet(t *testing.T) {
	server := newTestServer(t)

	_, beforeRaw := doJSON(t, server, http.MethodGet, "/api/wallet", nil)
	before := decodeBody[WalletOverview](t, beforeRaw)
	beforeTotal := before.Balance.Available + before.Balance.Frozen

	response, raw := doJSON(t, server, http.MethodPost, "/api/wallet/recharge", RechargeInput{Amount: 500})
	assertStatus(t, response, http.StatusOK)

	after := decodeBody[WalletOverview](t, raw)
	if after.Balance.Available != before.Balance.Available+500 {
		t.Fatalf("available = %v, want %v", after.Balance.Available, before.Balance.Available+500)
	}
	if got := after.Balance.Available + after.Balance.Frozen; got != beforeTotal+500 {
		t.Fatalf("total = %v, want %v", got, beforeTotal+500)
	}
	if len(after.Transactions) != len(before.Transactions)+1 {
		t.Fatalf("transaction count = %d, want %d", len(after.Transactions), len(before.Transactions)+1)
	}
	newest := after.Transactions[0]
	if newest.Type != TxDeposit || newest.Amount != 500 || newest.Status != TxSettled {
		t.Fatalf("unexpected deposit transaction: %+v", newest)
	}

	for _, amount := range []float64{0, -10} {
		invalid, invalidRaw := doJSON(t, server, http.MethodPost, "/api/wallet/recharge", RechargeInput{Amount: amount})
		assertStatus(t, invalid, http.StatusBadRequest)
		if message := errorMessage(t, invalidRaw); message == "" {
			t.Fatalf("amount %v returned an empty error", amount)
		}
	}
}

func TestSubscribeVip(t *testing.T) {
	server := newTestServer(t)

	_, beforeRaw := doJSON(t, server, http.MethodGet, "/api/wallet", nil)
	before := decodeBody[WalletOverview](t, beforeRaw)

	response, raw := doJSON(t, server, http.MethodPost, "/api/wallet/vip", SubscribeVipInput{Level: "PRO"})
	assertStatus(t, response, http.StatusOK)

	after := decodeBody[WalletOverview](t, raw)
	if after.Vip.Level != VipPro || after.Vip.FeeRate != 0.10 {
		t.Fatalf("unexpected vip after subscribe: %+v", after.Vip)
	}
	if after.Vip.ExpiresAt == nil || *after.Vip.ExpiresAt == "" {
		t.Fatalf("vip expiry not set: %+v", after.Vip)
	}
	if after.Balance.Available != before.Balance.Available-99 {
		t.Fatalf("available = %v, want %v", after.Balance.Available, before.Balance.Available-99)
	}
	if newest := after.Transactions[0]; newest.Type != TxFee || newest.Amount != 99 {
		t.Fatalf("unexpected fee transaction: %+v", newest)
	}

	invalid, invalidRaw := doJSON(t, server, http.MethodPost, "/api/wallet/vip", SubscribeVipInput{Level: "GOLD"})
	assertStatus(t, invalid, http.StatusBadRequest)
	if message := errorMessage(t, invalidRaw); message == "" {
		t.Fatal("invalid tier returned an empty error")
	}
}

func TestSubscribeVipInsufficientBalance(t *testing.T) {
	store := NewStore()
	store.wallet.Balance.Available = 10
	server := newStoreServer(t, store)

	response, raw := doJSON(t, server, http.MethodPost, "/api/wallet/vip", SubscribeVipInput{Level: "ENTERPRISE"})
	assertStatus(t, response, http.StatusBadRequest)
	if message := errorMessage(t, raw); message != "insufficient balance" {
		t.Fatalf("error = %q, want insufficient balance", message)
	}
	if store.Wallet().Vip.Level != VipFree {
		t.Fatalf("vip changed despite insufficient balance: %+v", store.Wallet().Vip)
	}
}

func TestGatewayOverviewAndKeys(t *testing.T) {
	server := newTestServer(t)

	response, raw := doJSON(t, server, http.MethodGet, "/api/gateway", nil)
	assertStatus(t, response, http.StatusOK)

	overview := decodeBody[ApiGatewayOverview](t, raw)
	if len(overview.Models) != 8 {
		t.Fatalf("model count = %d, want 8", len(overview.Models))
	}
	if len(overview.Keys) != 1 || overview.Keys[0].MaskedKey != "sk-pt-****-7f3a" {
		t.Fatalf("unexpected seeded keys: %+v", overview.Keys)
	}
	if overview.Usage.Period == "" || overview.Usage.TokensIn != 1284000 || overview.Usage.Requests != 3912 {
		t.Fatalf("unexpected usage: %+v", overview.Usage)
	}

	created, createdRaw := doJSON(t, server, http.MethodPost, "/api/gateway/keys", CreateApiKeyInput{
		Label:  "CI 流水线",
		Scopes: []string{"tasks:read", "tasks:write"},
	})
	assertStatus(t, created, http.StatusCreated)

	result := decodeBody[CreateApiKeyResponse](t, createdRaw)
	if !strings.HasPrefix(result.PlaintextKey, "sk-pt-") {
		t.Fatalf("plaintextKey = %q, want sk-pt- prefix", result.PlaintextKey)
	}
	if len(result.PlaintextKey) != len("sk-pt-")+8 {
		t.Fatalf("plaintextKey length = %d, want 14", len(result.PlaintextKey))
	}
	if !strings.HasPrefix(result.Key.MaskedKey, "sk-pt-****-") {
		t.Fatalf("maskedKey = %q", result.Key.MaskedKey)
	}
	if result.Key.Status != ApiKeyActive || len(result.Key.Scopes) != 2 || result.Key.ID == "" {
		t.Fatalf("unexpected created key: %+v", result.Key)
	}

	invalidBodies := []CreateApiKeyInput{
		{Label: "空 scope", Scopes: []string{}},
		{Label: "非法 scope", Scopes: []string{"tasks:delete"}},
		{Label: "   ", Scopes: []string{"tasks:read"}},
	}
	for _, input := range invalidBodies {
		bad, badRaw := doJSON(t, server, http.MethodPost, "/api/gateway/keys", input)
		assertStatus(t, bad, http.StatusBadRequest)
		if message := errorMessage(t, badRaw); message == "" {
			t.Fatalf("input %+v returned an empty error", input)
		}
	}

	revoked, revokedRaw := doJSON(t, server, http.MethodPost, "/api/gateway/keys/"+result.Key.ID+"/revoke", nil)
	assertStatus(t, revoked, http.StatusOK)
	record := decodeBody[ApiKeyRecord](t, revokedRaw)
	if record.Status != ApiKeyRevoked || record.ID != result.Key.ID {
		t.Fatalf("unexpected revoked record: %+v", record)
	}

	missing, missingRaw := doJSON(t, server, http.MethodPost, "/api/gateway/keys/key_999/revoke", nil)
	assertStatus(t, missing, http.StatusNotFound)
	if message := errorMessage(t, missingRaw); message == "" {
		t.Fatal("unknown key returned an empty error")
	}
}

func TestNewFeaturesPersistAcrossRestart(t *testing.T) {
	store, path := openTempStore(t)

	if _, err := store.Recharge(500); err != nil {
		t.Fatalf("Recharge: %v", err)
	}
	created, err := store.CreateApiKey(CreateApiKeyInput{Label: "持久化 key", Scopes: []string{"tasks:read"}})
	if err != nil {
		t.Fatalf("CreateApiKey: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer reopened.Close()

	wallet := reopened.Wallet()
	if wallet.Balance.Available != 3700 {
		t.Fatalf("available after restart = %v, want 3700", wallet.Balance.Available)
	}
	foundDeposit := false
	for _, transaction := range wallet.Transactions {
		if transaction.Type == TxDeposit && transaction.Amount == 500 {
			foundDeposit = true
		}
	}
	if !foundDeposit {
		t.Fatalf("recharge transaction missing after restart: %+v", wallet.Transactions)
	}

	overview := reopened.Gateway()
	foundKey := false
	for _, key := range overview.Keys {
		if key.ID == created.Key.ID {
			foundKey = true
			if key.Status != ApiKeyActive || key.Scopes[0] != "tasks:read" {
				t.Fatalf("key changed after restart: %+v", key)
			}
		}
	}
	if !foundKey {
		t.Fatalf("key %s missing after restart", created.Key.ID)
	}
}

// TestOpenStoreOverwritesStaleExpertSeed writes a version-1 snapshot that still
// carries the previous release's expert seed (one BLOCK finding but
// risk=58/MEDIUM/blocked=false) plus recognizable task and wallet data. The
// expert overview is read-only demo data, so OpenStore must replace it with the
// current seed while leaving tasks and balances untouched.
func TestOpenStoreOverwritesStaleExpertSeed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "marketplace.db")

	snapshot := NewStore().snapshotLocked()

	// Recognizable task data that must survive the upgrade unchanged.
	snapshot.Tasks = snapshot.Tasks[:5]
	snapshot.Tasks[0].Detail.Title = "旧快照任务标题"

	// Recognizable wallet data that must not be reset to the seed.
	snapshot.Wallet.Balance.Available = 4321
	snapshot.Wallet.Balance.Frozen = 111
	walletTransactions := len(snapshot.Wallet.Transactions)

	// Stale expert seed from the previous release: BLOCK exists but the risk
	// section was never recomputed, so blocked=false contradicts compliance.
	snapshot.Expert = ExpertState{
		Compliance: []ComplianceFinding{
			{ID: "cmp_001", Rule: "PCPD-ART-13 个人信息跨境传输", Severity: ComplianceHigh, Status: ComplianceBlock, Detail: "旧快照遗留的阻断项。"},
		},
		Risk: RiskAssessment{
			Score:     58,
			Level:     RiskMedium,
			Threshold: 70,
			Blocked:   false,
			Reasons:   []string{"旧快照遗留的风险原因"},
		},
	}

	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal stale snapshot: %v", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE marketplace_state (
			id         INTEGER PRIMARY KEY CHECK(id=1),
			version    INTEGER NOT NULL,
			payload    BLOB NOT NULL,
			updated_at TEXT NOT NULL
		);
		INSERT INTO marketplace_state (id, version, payload, updated_at)
		VALUES (1, 1, ?, '2026-11-16T00:00:00Z');
	`, payload); err != nil {
		_ = db.Close()
		t.Fatalf("seed stale snapshot: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore stale snapshot: %v", err)
	}
	defer store.Close()

	overview := store.ExpertOverview()
	if overview.Risk.Score != 82 || overview.Risk.Level != RiskHigh || overview.Risk.Threshold != 70 || !overview.Risk.Blocked {
		t.Fatalf("stale expert risk survived OpenStore: %+v", overview.Risk)
	}
	if got := countBlocks(overview.Compliance); got != 1 {
		t.Fatalf("seeded BLOCK count = %d, want 1", got)
	}
	if overview.Risk.Blocked != hasBlockFinding(overview.Compliance) {
		t.Fatalf("invariant broken: blocked=%v hasBlock=%v", overview.Risk.Blocked, hasBlockFinding(overview.Compliance))
	}
	if len(overview.Tools) != 7 || len(overview.Pricing) != 3 {
		t.Fatalf("expert overview not fully reseeded: %d tools / %d pricing", len(overview.Tools), len(overview.Pricing))
	}

	tasks := store.ListTasks(TaskQuery{Page: 1})
	if tasks.Total != 5 {
		t.Fatalf("task count after upgrade = %d, want 5 (snapshot value)", tasks.Total)
	}
	detail, ok := store.GetTask("task_1001")
	if !ok {
		t.Fatal("task_1001 missing after upgrade")
	}
	if detail.Title != "旧快照任务标题" {
		t.Fatalf("task content was reset: %q", detail.Title)
	}

	wallet := store.Wallet()
	if wallet.Balance.Available != 4321 || wallet.Balance.Frozen != 111 {
		t.Fatalf("wallet balance was reset: %+v", wallet.Balance)
	}
	if len(wallet.Transactions) != walletTransactions {
		t.Fatalf("wallet ledger modified: %d transactions, want %d", len(wallet.Transactions), walletTransactions)
	}
}

// TestOpenStoreUpgradesLegacySnapshot simulates a version-1 snapshot written
// before the trust layer / gateway / VIP fields existed. Opening it must seed
// the new sections while preserving tasks, balances and ledgers.
func TestOpenStoreUpgradesLegacySnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "marketplace.db")

	legacy := NewStore().snapshotLocked()
	legacy.Expert = ExpertState{}
	legacy.Gateway = GatewayState{}
	legacy.Wallet.Vip = WalletVip{}
	legacy.NextApiKeyID = 0

	payload, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy snapshot: %v", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE marketplace_state (
			id         INTEGER PRIMARY KEY CHECK(id=1),
			version    INTEGER NOT NULL,
			payload    BLOB NOT NULL,
			updated_at TEXT NOT NULL
		);
		INSERT INTO marketplace_state (id, version, payload, updated_at)
		VALUES (1, 1, ?, '2026-11-16T00:00:00Z');
	`, payload); err != nil {
		_ = db.Close()
		t.Fatalf("seed legacy snapshot: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore legacy snapshot: %v", err)
	}
	defer store.Close()

	if got := len(store.ExpertOverview().Tools); got != 7 {
		t.Fatalf("seeded tools = %d, want 7", got)
	}
	gateway := store.Gateway()
	if len(gateway.Models) != 8 || len(gateway.Keys) != 1 {
		t.Fatalf("seeded gateway = %d models / %d keys", len(gateway.Models), len(gateway.Keys))
	}
	wallet := store.Wallet()
	if wallet.Vip.Level != VipFree || wallet.Vip.FeeRate != 0.15 {
		t.Fatalf("seeded vip = %+v", wallet.Vip)
	}
	if wallet.Balance.Available != 3200 || wallet.Balance.Frozen != 1200 {
		t.Fatalf("legacy balance was modified: %+v", wallet.Balance)
	}
	if got := store.ListTasks(TaskQuery{Page: 1}).Total; got != 7 {
		t.Fatalf("legacy task count = %d, want 7", got)
	}

	// A new key must not collide with the seeded key_001.
	created, err := store.CreateApiKey(CreateApiKeyInput{Label: "legacy upgrade", Scopes: []string{"validate"}})
	if err != nil {
		t.Fatalf("CreateApiKey after upgrade: %v", err)
	}
	if created.Key.ID != "key_002" {
		t.Fatalf("new key id = %s, want key_002", created.Key.ID)
	}
}
