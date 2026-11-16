package marketplace

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(NewRouter(NewStore()))
	t.Cleanup(server.Close)
	return server
}

func doJSON(t *testing.T, server *httptest.Server, method, path string, body any) (*http.Response, []byte) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}

	request, err := http.NewRequest(method, server.URL+path, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return response, payload
}

func decodeBody[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode response %q: %v", string(raw), err)
	}
	return value
}

func assertStatus(t *testing.T, response *http.Response, want int) {
	t.Helper()
	if response.StatusCode != want {
		t.Fatalf("status = %d, want %d", response.StatusCode, want)
	}
}

func assertJSONContentType(t *testing.T, response *http.Response) {
	t.Helper()
	if got := response.Header.Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want application/json; charset=utf-8", got)
	}
}

func errorMessage(t *testing.T, raw []byte) string {
	t.Helper()
	payload := decodeBody[map[string]string](t, raw)
	message := payload["error"]
	if message == "" {
		t.Fatalf("expected non-empty error body, got %s", string(raw))
	}
	return message
}

func TestHealthz(t *testing.T) {
	server := newTestServer(t)

	response, raw := doJSON(t, server, http.MethodGet, "/api/healthz", nil)
	assertStatus(t, response, http.StatusOK)
	assertJSONContentType(t, response)

	payload := decodeBody[map[string]string](t, raw)
	if payload["status"] != "ok" {
		t.Fatalf("status payload = %q, want ok", payload["status"])
	}
}

func TestGetMe(t *testing.T) {
	server := newTestServer(t)

	response, raw := doJSON(t, server, http.MethodGet, "/api/me", nil)
	assertStatus(t, response, http.StatusOK)
	assertJSONContentType(t, response)

	user := decodeBody[User](t, raw)
	if user.ID != "usr_001" || user.DisplayName != "林知远" {
		t.Fatalf("unexpected user identity: %+v", user)
	}
	if user.ActiveRole != RoleCommander || len(user.Roles) != 2 {
		t.Fatalf("unexpected roles: active=%s roles=%v", user.ActiveRole, user.Roles)
	}
	if user.VerificationChannel != ChannelUniversity {
		t.Fatalf("verificationChannel = %q", user.VerificationChannel)
	}
	if user.Reputation.Score != 4.8 || user.Reputation.CompletedTasks != 32 {
		t.Fatalf("unexpected reputation: %+v", user.Reputation)
	}
	if len(user.Skills) != 5 || user.Skills[0].Key != SkillInstructionPrecision {
		t.Fatalf("unexpected skills: %+v", user.Skills)
	}
	if len(user.Expertise) != 3 || len(user.Badges) != 3 {
		t.Fatalf("unexpected expertise/badges: %d/%d", len(user.Expertise), len(user.Badges))
	}
	if user.AvatarURL != "/avatars/u1.png" {
		t.Fatalf("avatarUrl = %q", user.AvatarURL)
	}
}

func TestListTasks(t *testing.T) {
	server := newTestServer(t)

	t.Run("default page", func(t *testing.T) {
		response, raw := doJSON(t, server, http.MethodGet, "/api/tasks", nil)
		assertStatus(t, response, http.StatusOK)
		assertJSONContentType(t, response)

		page := decodeBody[GetTasksResponse](t, raw)
		if page.Total != 7 || len(page.Items) != 7 || page.Page != 1 || page.PageSize != 20 {
			t.Fatalf("unexpected page: total=%d len=%d page=%d size=%d", page.Total, len(page.Items), page.Page, page.PageSize)
		}
		if page.Items[0].ID != "task_1001" {
			t.Fatalf("first task = %s, want task_1001", page.Items[0].ID)
		}
	})

	t.Run("status filter", func(t *testing.T) {
		response, raw := doJSON(t, server, http.MethodGet, "/api/tasks?status=PUBLISHED", nil)
		assertStatus(t, response, http.StatusOK)
		page := decodeBody[GetTasksResponse](t, raw)
		if page.Total != 1 || page.Items[0].ID != "task_1001" {
			t.Fatalf("PUBLISHED filter: total=%d items=%+v", page.Total, page.Items)
		}
	})

	t.Run("status ALL is ignored", func(t *testing.T) {
		response, raw := doJSON(t, server, http.MethodGet, "/api/tasks?status=ALL", nil)
		assertStatus(t, response, http.StatusOK)
		if page := decodeBody[GetTasksResponse](t, raw); page.Total != 7 {
			t.Fatalf("ALL filter total = %d, want 7", page.Total)
		}
	})

	t.Run("keyword matches title", func(t *testing.T) {
		response, raw := doJSON(t, server, http.MethodGet, "/api/tasks?keyword=%E5%86%85%E6%A0%B8", nil)
		assertStatus(t, response, http.StatusOK)
		page := decodeBody[GetTasksResponse](t, raw)
		if page.Total != 1 || page.Items[0].ID != "task_1004" {
			t.Fatalf("keyword filter: total=%d items=%+v", page.Total, page.Items)
		}
	})

	t.Run("keyword matches category case-insensitively", func(t *testing.T) {
		response, raw := doJSON(t, server, http.MethodGet, "/api/tasks?keyword=security", nil)
		assertStatus(t, response, http.StatusOK)
		if page := decodeBody[GetTasksResponse](t, raw); page.Total != 3 {
			t.Fatalf("keyword security total = %d, want 3", page.Total)
		}
	})

	t.Run("keyword matches publisher", func(t *testing.T) {
		response, raw := doJSON(t, server, http.MethodGet, "/api/tasks?keyword=%E6%9F%90%E5%AE%89%E5%85%A8%E5%AE%9E%E9%AA%8C%E5%AE%A4", nil)
		assertStatus(t, response, http.StatusOK)
		if page := decodeBody[GetTasksResponse](t, raw); page.Total != 2 {
			t.Fatalf("publisher keyword total = %d, want 2", page.Total)
		}
	})

	t.Run("reward range is inclusive", func(t *testing.T) {
		response, raw := doJSON(t, server, http.MethodGet, "/api/tasks?rewardMin=1500&rewardMax=2400", nil)
		assertStatus(t, response, http.StatusOK)
		page := decodeBody[GetTasksResponse](t, raw)
		if page.Total != 2 {
			t.Fatalf("reward range total = %d, want 2", page.Total)
		}
		for _, item := range page.Items {
			if item.Reward.Amount < 1500 || item.Reward.Amount > 2400 {
				t.Fatalf("task %s reward %v outside range", item.ID, item.Reward.Amount)
			}
		}
	})

	t.Run("page beyond range is empty", func(t *testing.T) {
		response, raw := doJSON(t, server, http.MethodGet, "/api/tasks?page=2", nil)
		assertStatus(t, response, http.StatusOK)
		page := decodeBody[GetTasksResponse](t, raw)
		if page.Total != 7 || len(page.Items) != 0 || page.Page != 2 {
			t.Fatalf("page 2: total=%d len=%d page=%d", page.Total, len(page.Items), page.Page)
		}
	})

	t.Run("pageSize pagination", func(t *testing.T) {
		response, raw := doJSON(t, server, http.MethodGet, "/api/tasks?page=2&pageSize=2", nil)
		assertStatus(t, response, http.StatusOK)
		page := decodeBody[GetTasksResponse](t, raw)
		if page.Total != 7 || page.Page != 2 || page.PageSize != 2 || len(page.Items) != 2 {
			t.Fatalf("pageSize page: total=%d page=%d size=%d len=%d", page.Total, page.Page, page.PageSize, len(page.Items))
		}
		if page.Items[0].ID != "task_1003" || page.Items[1].ID != "task_1004" {
			t.Fatalf("unexpected pageSize slice: %s, %s", page.Items[0].ID, page.Items[1].ID)
		}
	})

	invalid := []string{
		"/api/tasks?rewardMin=abc",
		"/api/tasks?rewardMax=abc",
		"/api/tasks?status=BOGUS",
		"/api/tasks?page=0",
		"/api/tasks?page=abc",
		"/api/tasks?pageSize=0",
		"/api/tasks?pageSize=abc",
	}
	for _, path := range invalid {
		t.Run("invalid "+path, func(t *testing.T) {
			response, raw := doJSON(t, server, http.MethodGet, path, nil)
			assertStatus(t, response, http.StatusBadRequest)
			errorMessage(t, raw)
		})
	}
}

func validPublishInput() PublishTaskInput {
	return PublishTaskInput{
		Title:              "新增测试任务",
		Description:        "用于验证发布冻结逻辑",
		Category:           "SECURITY",
		Reward:             Money{Amount: 500, Currency: "CNY"},
		Deadline:           "2026-12-01T00:00:00Z",
		AcceptanceCriteria: []string{"可复现脚本"},
		RequiredSkills:     []string{"AGENT_ORCHESTRATION"},
		ComplianceAck:      true,
	}
}

func TestPublishTask(t *testing.T) {
	server := newTestServer(t)
	input := validPublishInput()

	response, raw := doJSON(t, server, http.MethodPost, "/api/tasks", input)
	assertStatus(t, response, http.StatusCreated)
	assertJSONContentType(t, response)

	detail := decodeBody[TaskDetail](t, raw)
	if detail.ID != "task_1008" || detail.Status != StatusPublished {
		t.Fatalf("unexpected published task: id=%s status=%s", detail.ID, detail.Status)
	}
	if detail.Publisher.ID != "usr_001" || detail.Assignee != nil {
		t.Fatalf("unexpected publisher/assignee: %+v / %+v", detail.Publisher, detail.Assignee)
	}
	if detail.Escrow.State != EscrowFrozen || detail.Escrow.Amount != 500 || detail.Escrow.FrozenAt == nil {
		t.Fatalf("unexpected escrow: %+v", detail.Escrow)
	}
	if len(detail.Pipeline.Stages) != 4 || detail.Pipeline.CurrentStage != PipelineStageAIGeneration {
		t.Fatalf("unexpected pipeline: %+v", detail.Pipeline)
	}
	if detail.Deadline != input.Deadline || detail.CreatedAt == "" {
		t.Fatalf("unexpected timestamps: created=%s deadline=%s", detail.CreatedAt, detail.Deadline)
	}

	// The new task must appear first in the list.
	_, listRaw := doJSON(t, server, http.MethodGet, "/api/tasks", nil)
	page := decodeBody[GetTasksResponse](t, listRaw)
	if page.Total != 8 || page.Items[0].ID != "task_1008" {
		t.Fatalf("list after publish: total=%d first=%s", page.Total, page.Items[0].ID)
	}

	// The reward must be frozen into escrow.
	_, walletRaw := doJSON(t, server, http.MethodGet, "/api/wallet", nil)
	wallet := decodeBody[GetWalletResponse](t, walletRaw)
	if wallet.Balance.Available != 2700 || wallet.Balance.Frozen != 1700 {
		t.Fatalf("unexpected balance after freeze: %+v", wallet.Balance)
	}
	if len(wallet.Transactions) != 7 {
		t.Fatalf("transaction count = %d, want 7", len(wallet.Transactions))
	}
	newest := wallet.Transactions[0]
	if newest.ID != "tx_007" || newest.Type != TxEscrowFreeze || newest.TaskID != "task_1008" || newest.Amount != 500 {
		t.Fatalf("unexpected newest transaction: %+v", newest)
	}
}

func TestPublishTaskValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*PublishTaskInput)
	}{
		{"complianceAck false", func(input *PublishTaskInput) { input.ComplianceAck = false }},
		{"reward zero", func(input *PublishTaskInput) { input.Reward.Amount = 0 }},
		{"reward negative", func(input *PublishTaskInput) { input.Reward.Amount = -10 }},
		{"empty title", func(input *PublishTaskInput) { input.Title = "   " }},
		{"empty description", func(input *PublishTaskInput) { input.Description = "  " }},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := newTestServer(t)
			input := validPublishInput()
			testCase.mutate(&input)

			response, raw := doJSON(t, server, http.MethodPost, "/api/tasks", input)
			assertStatus(t, response, http.StatusBadRequest)
			errorMessage(t, raw)

			// A rejected publish must not freeze funds or create a task.
			_, listRaw := doJSON(t, server, http.MethodGet, "/api/tasks", nil)
			if page := decodeBody[GetTasksResponse](t, listRaw); page.Total != 7 {
				t.Fatalf("task count changed after invalid publish: %d", page.Total)
			}
			_, walletRaw := doJSON(t, server, http.MethodGet, "/api/wallet", nil)
			if wallet := decodeBody[GetWalletResponse](t, walletRaw); wallet.Balance.Frozen != 1200 {
				t.Fatalf("frozen balance changed after invalid publish: %+v", wallet.Balance)
			}
		})
	}

	t.Run("malformed json", func(t *testing.T) {
		server := newTestServer(t)
		request, err := http.NewRequest(http.MethodPost, server.URL+"/api/tasks", strings.NewReader("{"))
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatalf("do request: %v", err)
		}
		defer response.Body.Close()
		raw, _ := io.ReadAll(response.Body)
		assertStatus(t, response, http.StatusBadRequest)
		errorMessage(t, raw)
	})
}

func TestGetTask(t *testing.T) {
	server := newTestServer(t)

	response, raw := doJSON(t, server, http.MethodGet, "/api/tasks/task_1001", nil)
	assertStatus(t, response, http.StatusOK)
	assertJSONContentType(t, response)

	detail := decodeBody[TaskDetail](t, raw)
	if detail.ID != "task_1001" || detail.Assignee != nil {
		t.Fatalf("unexpected task_1001: %+v", detail)
	}
	if detail.Escrow.State != EscrowFrozen || detail.Escrow.Amount != 1200 {
		t.Fatalf("unexpected escrow: %+v", detail.Escrow)
	}
	if len(detail.AcceptanceCriteria) != 3 || len(detail.RequiredSkills) != 2 {
		t.Fatalf("unexpected criteria/skills: %v / %v", detail.AcceptanceCriteria, detail.RequiredSkills)
	}

	_, raw = doJSON(t, server, http.MethodGet, "/api/tasks/task_1002", nil)
	assigned := decodeBody[TaskDetail](t, raw)
	if assigned.Assignee == nil || assigned.Assignee.ID != "usr_001" {
		t.Fatalf("unexpected assignee: %+v", assigned.Assignee)
	}

	response, raw = doJSON(t, server, http.MethodGet, "/api/tasks/task_unknown", nil)
	assertStatus(t, response, http.StatusNotFound)
	errorMessage(t, raw)
}

func TestAcceptTask(t *testing.T) {
	server := newTestServer(t)

	before := time.Now().UTC()
	response, raw := doJSON(t, server, http.MethodPost, "/api/tasks/task_1001/accept", nil)
	assertStatus(t, response, http.StatusOK)
	assertJSONContentType(t, response)

	accepted := decodeBody[AcceptTaskResponse](t, raw)
	if accepted.TaskID != "task_1001" || accepted.Status != StatusAccepted {
		t.Fatalf("unexpected accept response: %+v", accepted)
	}
	lease, err := time.Parse(time.RFC3339, accepted.LeaseExpiresAt)
	if err != nil {
		t.Fatalf("parse leaseExpiresAt: %v", err)
	}
	expected := before.Add(2 * time.Hour)
	if delta := lease.Sub(expected); delta < -time.Minute || delta > time.Minute {
		t.Fatalf("lease expires at %s, expected about %s", lease, expected)
	}
	if !strings.HasSuffix(accepted.LeaseExpiresAt, "Z") {
		t.Fatalf("leaseExpiresAt is not UTC: %s", accepted.LeaseExpiresAt)
	}

	// Accepting must persist the assignee and status.
	_, taskRaw := doJSON(t, server, http.MethodGet, "/api/tasks/task_1001", nil)
	detail := decodeBody[TaskDetail](t, taskRaw)
	if detail.Status != StatusAccepted || detail.Assignee == nil || detail.Assignee.ID != "usr_001" {
		t.Fatalf("task did not reflect acceptance: %+v", detail)
	}

	// A second accept is an illegal transition.
	response, raw = doJSON(t, server, http.MethodPost, "/api/tasks/task_1001/accept", nil)
	assertStatus(t, response, http.StatusConflict)
	errorMessage(t, raw)

	// A task in a non-PUBLISHED state cannot be accepted.
	response, raw = doJSON(t, server, http.MethodPost, "/api/tasks/task_1002/accept", nil)
	assertStatus(t, response, http.StatusConflict)
	errorMessage(t, raw)

	// Unknown task.
	response, raw = doJSON(t, server, http.MethodPost, "/api/tasks/task_unknown/accept", nil)
	assertStatus(t, response, http.StatusNotFound)
	errorMessage(t, raw)
}

func TestGetTrace(t *testing.T) {
	server := newTestServer(t)

	response, raw := doJSON(t, server, http.MethodGet, "/api/tasks/task_1001/trace", nil)
	assertStatus(t, response, http.StatusOK)
	assertJSONContentType(t, response)

	trace := decodeBody[GetTraceResponse](t, raw)
	if trace.TaskID != "task_1001" || trace.HeadHash == "" {
		t.Fatalf("unexpected trace header: %+v", trace)
	}
	if len(trace.Nodes) != 6 {
		t.Fatalf("task_1001 node count = %d, want 6", len(trace.Nodes))
	}
	if trace.Nodes[0].PrevHash != nil || trace.Nodes[0].Type != TraceRequirement {
		t.Fatalf("unexpected first node: %+v", trace.Nodes[0])
	}
	if trace.Nodes[5].Hash != trace.HeadHash || trace.Nodes[5].Type != TraceValidation {
		t.Fatalf("head hash does not match last node: %+v", trace.Nodes[5])
	}
	for index := 1; index < len(trace.Nodes); index++ {
		previous := trace.Nodes[index-1].Hash
		if trace.Nodes[index].PrevHash == nil || *trace.Nodes[index].PrevHash != previous {
			t.Fatalf("broken hash chain at index %d", index)
		}
	}

	_, raw = doJSON(t, server, http.MethodGet, "/api/tasks/task_1004/trace", nil)
	four := decodeBody[GetTraceResponse](t, raw)
	if len(four.Nodes) != 4 {
		t.Fatalf("task_1004 node count = %d, want 4", len(four.Nodes))
	}

	response, raw = doJSON(t, server, http.MethodGet, "/api/tasks/task_1003/trace", nil)
	assertStatus(t, response, http.StatusOK)
	if !strings.Contains(string(raw), `"nodes":[]`) {
		t.Fatalf("empty chain must serialize as [], got %s", string(raw))
	}
	if empty := decodeBody[GetTraceResponse](t, raw); len(empty.Nodes) != 0 {
		t.Fatalf("task_1003 node count = %d, want 0", len(empty.Nodes))
	}

	response, raw = doJSON(t, server, http.MethodGet, "/api/tasks/task_unknown/trace", nil)
	assertStatus(t, response, http.StatusNotFound)
	errorMessage(t, raw)
}

func TestWallet(t *testing.T) {
	server := newTestServer(t)

	response, raw := doJSON(t, server, http.MethodGet, "/api/wallet", nil)
	assertStatus(t, response, http.StatusOK)
	assertJSONContentType(t, response)

	wallet := decodeBody[GetWalletResponse](t, raw)
	if wallet.Balance.Available != 3200 || wallet.Balance.Frozen != 1200 || wallet.Balance.Currency != "CNY" {
		t.Fatalf("unexpected balance: %+v", wallet.Balance)
	}
	if len(wallet.Transactions) != 6 {
		t.Fatalf("transaction count = %d, want 6", len(wallet.Transactions))
	}
	if wallet.Transactions[0].ID != "tx_001" || wallet.Transactions[0].Type != TxEscrowFreeze {
		t.Fatalf("unexpected first transaction: %+v", wallet.Transactions[0])
	}
}

func TestReputation(t *testing.T) {
	server := newTestServer(t)

	response, raw := doJSON(t, server, http.MethodGet, "/api/reputation/usr_001", nil)
	assertStatus(t, response, http.StatusOK)
	assertJSONContentType(t, response)

	summary := decodeBody[ReputationSummary](t, raw)
	if summary.UserID != "usr_001" || summary.Score != 4.8 {
		t.Fatalf("unexpected reputation: %+v", summary)
	}
	if len(summary.Dimensions) != 5 || summary.Dimensions[0].Key != string(SkillInstructionPrecision) {
		t.Fatalf("unexpected dimensions: %+v", summary.Dimensions)
	}
	if len(summary.Ratings) != 3 || summary.Ratings[0].ID != "rat_001" {
		t.Fatalf("unexpected ratings: %+v", summary.Ratings)
	}

	_, raw = doJSON(t, server, http.MethodGet, "/api/reputation/usr_002", nil)
	second := decodeBody[ReputationSummary](t, raw)
	if second.Score != 4.6 || len(second.Ratings) != 1 || second.Ratings[0].ID != "rat_101" {
		t.Fatalf("unexpected usr_002 reputation: %+v", second)
	}

	_, raw = doJSON(t, server, http.MethodGet, "/api/reputation/usr_unknown", nil)
	unknown := decodeBody[ReputationSummary](t, raw)
	if unknown.Score != 4.5 || len(unknown.Ratings) != 0 || len(unknown.Dimensions) != 5 {
		t.Fatalf("unexpected default reputation: %+v", unknown)
	}
}

func TestSubmitRating(t *testing.T) {
	server := newTestServer(t)

	input := SubmitRatingInput{
		TaskID:    "task_1001",
		Direction: RatingEmployerToCommander,
		Score:     4.5,
		Tags:      []string{"证据完整"},
		Comment:   "测试评分",
	}
	response, raw := doJSON(t, server, http.MethodPost, "/api/ratings", input)
	assertStatus(t, response, http.StatusOK)
	assertJSONContentType(t, response)

	rating := decodeBody[Rating](t, raw)
	if rating.ID != "rat_004" || rating.TaskID != "task_1001" || rating.Score != 4.5 {
		t.Fatalf("unexpected rating: %+v", rating)
	}
	if _, err := time.Parse(time.RFC3339, rating.CreatedAt); err != nil {
		t.Fatalf("rating createdAt is not RFC3339: %v", err)
	}

	_, reputationRaw := doJSON(t, server, http.MethodGet, "/api/reputation/usr_001", nil)
	summary := decodeBody[ReputationSummary](t, reputationRaw)
	if len(summary.Ratings) != 4 || summary.Ratings[0].ID != "rat_004" {
		t.Fatalf("rating was not appended to history: %+v", summary.Ratings)
	}

	invalid := []SubmitRatingInput{
		{TaskID: "", Direction: RatingEmployerToCommander, Score: 4},
		{TaskID: "task_1001", Direction: "SIDEWAYS", Score: 4},
		{TaskID: "task_1001", Direction: RatingEmployerToCommander, Score: 6},
	}
	for _, failed := range invalid {
		response, raw := doJSON(t, server, http.MethodPost, "/api/ratings", failed)
		assertStatus(t, response, http.StatusBadRequest)
		errorMessage(t, raw)
	}
}

func TestRegisterChannel(t *testing.T) {
	server := newTestServer(t)

	response, raw := doJSON(t, server, http.MethodPost, "/api/auth/register/UNIVERSITY", RegisterChannelInput{
		Evidence: RegisterEvidence{SchoolEmail: stringPtr("student@example.edu")},
	})
	assertStatus(t, response, http.StatusOK)
	verified := decodeBody[RegisterChannelResponse](t, raw)
	if verified.Channel != ChannelUniversity || verified.Status != "VERIFIED" {
		t.Fatalf("unexpected verified response: %+v", verified)
	}

	response, raw = doJSON(t, server, http.MethodPost, "/api/auth/register/ENTERPRISE", RegisterChannelInput{})
	assertStatus(t, response, http.StatusOK)
	pending := decodeBody[RegisterChannelResponse](t, raw)
	if pending.Channel != ChannelEnterprise || pending.Status != "PENDING" {
		t.Fatalf("unexpected pending response: %+v", pending)
	}

	response, raw = doJSON(t, server, http.MethodPost, "/api/auth/register/BOGUS", RegisterChannelInput{})
	assertStatus(t, response, http.StatusBadRequest)
	errorMessage(t, raw)
}

func TestSignContract(t *testing.T) {
	server := newTestServer(t)

	input := SignContractInput{SignerRole: RoleEmployer, ClausesAccepted: []bool{true, true, true}}
	response, raw := doJSON(t, server, http.MethodPost, "/api/tasks/task_1001/contracts/sign", input)
	assertStatus(t, response, http.StatusOK)
	assertJSONContentType(t, response)

	first := decodeBody[SignContractResponse](t, raw)
	if first.TaskID != "task_1001" || first.SignerRole != RoleEmployer {
		t.Fatalf("unexpected signature response: %+v", first)
	}
	if !strings.HasPrefix(first.SignatureHash, "sha256:") {
		t.Fatalf("signatureHash missing sha256 prefix: %s", first.SignatureHash)
	}
	digest, err := hex.DecodeString(strings.TrimPrefix(first.SignatureHash, "sha256:"))
	if err != nil || len(digest) != sha256Size {
		t.Fatalf("signatureHash is not 64 hex chars: %s (%v)", first.SignatureHash, err)
	}
	if !strings.HasSuffix(first.SignedAt, "Z") {
		t.Fatalf("signedAt is not UTC: %s", first.SignedAt)
	}
	if _, err := time.Parse(time.RFC3339, first.SignedAt); err != nil {
		t.Fatalf("signedAt is not RFC3339: %v", err)
	}

	// Deterministic for identical inputs.
	_, secondRaw := doJSON(t, server, http.MethodPost, "/api/tasks/task_1001/contracts/sign", input)
	second := decodeBody[SignContractResponse](t, secondRaw)
	if second.SignatureHash != first.SignatureHash {
		t.Fatalf("signature is not deterministic: %s vs %s", first.SignatureHash, second.SignatureHash)
	}

	// Different clauses produce a different digest.
	different := SignContractInput{SignerRole: RoleEmployer, ClausesAccepted: []bool{true, false, true}}
	_, thirdRaw := doJSON(t, server, http.MethodPost, "/api/tasks/task_1001/contracts/sign", different)
	if third := decodeBody[SignContractResponse](t, thirdRaw); third.SignatureHash == first.SignatureHash {
		t.Fatalf("signature did not change with different clauses")
	}

	response, raw = doJSON(t, server, http.MethodPost, "/api/tasks/task_unknown/contracts/sign", input)
	assertStatus(t, response, http.StatusNotFound)
	errorMessage(t, raw)

	badRole := SignContractInput{SignerRole: "OWNER", ClausesAccepted: []bool{true}}
	response, raw = doJSON(t, server, http.MethodPost, "/api/tasks/task_1001/contracts/sign", badRole)
	assertStatus(t, response, http.StatusBadRequest)
	errorMessage(t, raw)
}

const sha256Size = 32

func TestCORS(t *testing.T) {
	server := newTestServer(t)

	request, err := http.NewRequest(http.MethodOptions, server.URL+"/api/tasks", nil)
	if err != nil {
		t.Fatalf("build preflight: %v", err)
	}
	request.Header.Set("Origin", "http://localhost:5174")
	request.Header.Set("Access-Control-Request-Method", "POST")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	defer response.Body.Close()

	assertStatus(t, response, http.StatusNoContent)
	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5174" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
	methods := response.Header.Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, "GET") || !strings.Contains(methods, "POST") || !strings.Contains(methods, "OPTIONS") {
		t.Fatalf("Access-Control-Allow-Methods = %q", methods)
	}
	headers := response.Header.Get("Access-Control-Allow-Headers")
	if !strings.Contains(headers, "Content-Type") || !strings.Contains(headers, "Authorization") {
		t.Fatalf("Access-Control-Allow-Headers = %q", headers)
	}

	// The second allowed origin is accepted too.
	request, _ = http.NewRequest(http.MethodOptions, server.URL+"/api/tasks", nil)
	request.Header.Set("Origin", "http://localhost:4180")
	response, err = server.Client().Do(request)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	response.Body.Close()
	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:4180" {
		t.Fatalf("second origin not allowed: %q", got)
	}

	// A disallowed origin gets no allow header.
	request, _ = http.NewRequest(http.MethodOptions, server.URL+"/api/tasks", nil)
	request.Header.Set("Origin", "http://evil.example")
	response, err = server.Client().Do(request)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	response.Body.Close()
	assertStatus(t, response, http.StatusNoContent)
	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("disallowed origin received allow header: %q", got)
	}

	// Actual requests carry the allow header for allowed origins.
	request, _ = http.NewRequest(http.MethodGet, server.URL+"/api/me", nil)
	request.Header.Set("Origin", "http://localhost:5174")
	response, err = server.Client().Do(request)
	if err != nil {
		t.Fatalf("get me: %v", err)
	}
	response.Body.Close()
	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5174" {
		t.Fatalf("GET Access-Control-Allow-Origin = %q", got)
	}
}

func TestEmptyArraysSerializeAsArrays(t *testing.T) {
	server := newTestServer(t)

	input := validPublishInput()
	input.RequiredSkills = nil
	input.AcceptanceCriteria = nil
	_, raw := doJSON(t, server, http.MethodPost, "/api/tasks", input)
	if !strings.Contains(string(raw), `"requiredSkills":[]`) {
		t.Fatalf("requiredSkills did not serialize as []: %s", string(raw))
	}
	if !strings.Contains(string(raw), `"acceptanceCriteria":[]`) {
		t.Fatalf("acceptanceCriteria did not serialize as []: %s", string(raw))
	}

	signInput := SignContractInput{SignerRole: RoleCommander, ClausesAccepted: []bool{}}
	_, raw = doJSON(t, server, http.MethodPost, "/api/tasks/task_1001/contracts/sign", signInput)
	if !strings.Contains(string(raw), `"clausesAccepted":[]`) {
		t.Fatalf("clausesAccepted did not serialize as []: %s", string(raw))
	}

	ratingInput := SubmitRatingInput{TaskID: "task_1001", Direction: RatingCommanderToEmployer, Score: 5}
	_, raw = doJSON(t, server, http.MethodPost, "/api/ratings", ratingInput)
	if !strings.Contains(string(raw), `"tags":[]`) {
		t.Fatalf("tags did not serialize as []: %s", string(raw))
	}
}

func TestUnknownRoute(t *testing.T) {
	server := newTestServer(t)

	response, raw := doJSON(t, server, http.MethodGet, "/api/nope", nil)
	assertStatus(t, response, http.StatusNotFound)
	errorMessage(t, raw)
}

func TestConcurrentAccess(t *testing.T) {
	server := newTestServer(t)

	var wg sync.WaitGroup
	errCh := make(chan error, 128)

	for index := 0; index < 30; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := http.Get(server.URL + "/api/tasks")
			if err != nil {
				errCh <- err
				return
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				errCh <- fmt.Errorf("GET /api/tasks status %d", response.StatusCode)
			}
			_, _ = io.Copy(io.Discard, response.Body)
		}()
	}

	for index := 0; index < 20; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response, err := http.Get(server.URL + "/api/tasks/task_1001/trace")
			if err != nil {
				errCh <- err
				return
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				errCh <- fmt.Errorf("GET trace status %d", response.StatusCode)
			}
			_, _ = io.Copy(io.Discard, response.Body)
		}()
	}

	for index := 0; index < 10; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			payload, _ := json.Marshal(validPublishInput())
			response, err := http.Post(server.URL+"/api/tasks", "application/json", bytes.NewReader(payload))
			if err != nil {
				errCh <- err
				return
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusCreated {
				errCh <- fmt.Errorf("POST /api/tasks status %d", response.StatusCode)
			}
			_, _ = io.Copy(io.Discard, response.Body)
		}(index)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}

	_, raw := doJSON(t, server, http.MethodGet, "/api/tasks", nil)
	page := decodeBody[GetTasksResponse](t, raw)
	if page.Total != 17 {
		t.Fatalf("total after concurrent publishes = %d, want 17", page.Total)
	}
}
