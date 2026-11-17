package marketplace

import (
	"database/sql"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func openTempStore(t *testing.T) (*Store, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "marketplace.db")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore(%q): %v", path, err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, path
}

func persistPublishInput(title string) PublishTaskInput {
	return PublishTaskInput{
		Title:              title,
		Description:        "用于验证 SQLite 持久化的任务",
		Category:           "SECURITY",
		Reward:             Money{Amount: 500, Currency: "CNY"},
		Deadline:           "2026-12-01T00:00:00Z",
		AcceptanceCriteria: []string{"可复现脚本"},
		RequiredSkills:     []string{"AGENT_ORCHESTRATION"},
		ComplianceAck:      true,
	}
}

func TestOpenStoreSeedsData(t *testing.T) {
	store, _ := openTempStore(t)

	page := store.ListTasks(TaskQuery{Page: 1})
	if page.Total != 7 || len(page.Items) != 7 {
		t.Fatalf("seeded task count = %d/%d, want 7", page.Total, len(page.Items))
	}
	if page.Items[0].ID != "task_1001" {
		t.Fatalf("first seeded task = %s, want task_1001", page.Items[0].ID)
	}

	if got := store.Wallet().Balance; got.Available != 3200 || got.Frozen != 1200 {
		t.Fatalf("seeded balance = %+v", got)
	}
	if got := len(store.Reputation("usr_001").Ratings); got != 3 {
		t.Fatalf("seeded ratings = %d, want 3", got)
	}
}

func TestOpenStorePersistsAcrossRestart(t *testing.T) {
	store, path := openTempStore(t)

	published, err := store.PublishTask(persistPublishInput("重启后仍应存在的唯一任务"))
	if err != nil {
		t.Fatalf("PublishTask: %v", err)
	}
	if published.ID != "task_1008" {
		t.Fatalf("published id = %s, want task_1008", published.ID)
	}

	accepted, err := store.AcceptTask("task_1001")
	if err != nil {
		t.Fatalf("AcceptTask: %v", err)
	}
	if accepted.Status != StatusAccepted {
		t.Fatalf("accept status = %s", accepted.Status)
	}

	rating, err := store.SubmitRating(SubmitRatingInput{
		TaskID:    published.ID,
		Direction: RatingEmployerToCommander,
		Score:     4.5,
		Tags:      []string{"证据完整"},
		Comment:   "持久化评分",
	})
	if err != nil {
		t.Fatalf("SubmitRating: %v", err)
	}
	if rating.ID != "rat_004" {
		t.Fatalf("rating id = %s, want rat_004", rating.ID)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer reopened.Close()

	detail, ok := reopened.GetTask(published.ID)
	if !ok {
		t.Fatalf("task %s missing after restart", published.ID)
	}
	if detail.Title != published.Title || detail.Description != published.Description {
		t.Fatalf("task content changed: %+v", detail)
	}
	if detail.Reward != published.Reward || detail.Status != StatusPublished {
		t.Fatalf("task reward/status changed: %+v", detail)
	}
	if detail.Escrow.State != EscrowFrozen || detail.Escrow.Amount != 500 {
		t.Fatalf("escrow changed: %+v", detail.Escrow)
	}
	if len(detail.Pipeline.Stages) != 4 || detail.Pipeline.CurrentStage != PipelineStageAIGeneration {
		t.Fatalf("pipeline changed: %+v", detail.Pipeline)
	}

	acceptedDetail, ok := reopened.GetTask("task_1001")
	if !ok {
		t.Fatal("task_1001 missing after restart")
	}
	if acceptedDetail.Status != StatusAccepted {
		t.Fatalf("task_1001 status = %s, want ACCEPTED", acceptedDetail.Status)
	}
	if acceptedDetail.Assignee == nil || acceptedDetail.Assignee.ID != "usr_001" {
		t.Fatalf("task_1001 assignee not persisted: %+v", acceptedDetail.Assignee)
	}

	summary := reopened.Reputation("usr_001")
	if len(summary.Ratings) != 4 {
		t.Fatalf("rating count after restart = %d, want 4", len(summary.Ratings))
	}
	if summary.Ratings[0].ID != "rat_004" || summary.Ratings[0].Comment != "持久化评分" {
		t.Fatalf("rating not persisted: %+v", summary.Ratings[0])
	}

	// Counters must also survive so new IDs never collide with reloaded rows.
	next, err := reopened.PublishTask(persistPublishInput("重启后继续发布"))
	if err != nil {
		t.Fatalf("publish after restart: %v", err)
	}
	if next.ID != "task_1009" {
		t.Fatalf("next task id = %s, want task_1009", next.ID)
	}

	page := reopened.ListTasks(TaskQuery{Page: 1})
	if page.Total != 9 {
		t.Fatalf("task count after restart + publish = %d, want 9", page.Total)
	}
}

func TestOpenStoreRejectsUnsupportedVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "marketplace.db")

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
		VALUES (1, 99, '{"me":{"id":"usr_001"}}', '2026-11-16T00:00:00Z');
	`); err != nil {
		_ = db.Close()
		t.Fatalf("seed wrong version: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	store, err := OpenStore(path)
	if err == nil {
		_ = store.Close()
		t.Fatal("OpenStore accepted unsupported version 99")
	}
	if !strings.Contains(err.Error(), "version") {
		t.Fatalf("error does not mention version: %v", err)
	}

	// The rejected database must be left intact, not reset to seed data.
	check, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("reopen raw db: %v", err)
	}
	defer check.Close()

	var (
		version int
		payload []byte
	)
	if err := check.QueryRow(`SELECT version, payload FROM marketplace_state WHERE id = 1`).Scan(&version, &payload); err != nil {
		t.Fatalf("read preserved row: %v", err)
	}
	if version != 99 {
		t.Fatalf("version after rejected open = %d, want 99", version)
	}
	if string(payload) != `{"me":{"id":"usr_001"}}` {
		t.Fatalf("payload was modified: %s", payload)
	}
}

func TestOpenStoreRejectsCorruptPayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "marketplace.db")

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
		VALUES (1, 1, '{"me":', '2026-11-16T00:00:00Z');
	`); err != nil {
		_ = db.Close()
		t.Fatalf("seed corrupt payload: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	store, err := OpenStore(path)
	if err == nil {
		_ = store.Close()
		t.Fatal("OpenStore accepted a corrupt payload")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Fatalf("error does not mention decode: %v", err)
	}
}

func TestOpenStoreConcurrentWrites(t *testing.T) {
	store, path := openTempStore(t)

	const (
		publishes = 10
		ratings   = 10
	)

	var wg sync.WaitGroup
	errCh := make(chan error, publishes+ratings)

	for index := 0; index < publishes; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			input := persistPublishInput("并发发布任务")
			input.Reward.Amount = float64(100 + index)
			if _, err := store.PublishTask(input); err != nil {
				errCh <- err
			}
		}(index)
	}
	for index := 0; index < ratings; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			if _, err := store.SubmitRating(SubmitRatingInput{
				TaskID:    "task_1001",
				Direction: RatingCommanderToEmployer,
				Score:     4,
				Comment:   "并发评分",
			}); err != nil {
				errCh <- err
			}
		}(index)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent write: %v", err)
	}

	page := store.ListTasks(TaskQuery{Page: 1})
	if page.Total != 7+publishes {
		t.Fatalf("task total after concurrent writes = %d, want %d", page.Total, 7+publishes)
	}
	if got := len(store.Reputation("usr_001").Ratings); got != 3+ratings {
		t.Fatalf("rating count after concurrent writes = %d, want %d", got, 3+ratings)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := OpenStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer reopened.Close()

	if page := reopened.ListTasks(TaskQuery{Page: 1}); page.Total != 7+publishes {
		t.Fatalf("task total after reopen = %d, want %d", page.Total, 7+publishes)
	}
	summary := reopened.Reputation("usr_001")
	if len(summary.Ratings) != 3+ratings {
		t.Fatalf("rating count after reopen = %d, want %d", len(summary.Ratings), 3+ratings)
	}
	if summary.Ratings[0].ID != "rat_013" {
		t.Fatalf("newest rating after reopen = %s, want rat_013", summary.Ratings[0].ID)
	}

	next, err := reopened.PublishTask(persistPublishInput("并发写入后的新任务"))
	if err != nil {
		t.Fatalf("publish after reopen: %v", err)
	}
	if next.ID != "task_1018" {
		t.Fatalf("next task id = %s, want task_1018", next.ID)
	}
}
