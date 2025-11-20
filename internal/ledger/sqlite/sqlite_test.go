package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/ledger"
)

func TestStoreRecordAndSummary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.db")
	store, err := New(path, 100, 10, 60, 10)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	record := func(direction ledger.Direction, prompt, completion int64) {
		if err := store.Record(ctx, ledger.Entry{
			UserID:           42,
			ServiceID:        1,
			PromptTokens:     prompt,
			CompletionTokens: completion,
			Direction:        direction,
			Memo:             "test",
		}); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	record(ledger.DirectionConsume, 100, 50)
	record(ledger.DirectionSupply, 60, 20)

	summary, err := store.Summary(ctx, 42)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if summary.ConsumedTokens != 150 {
		t.Fatalf("expected consumed 150, got %d", summary.ConsumedTokens)
	}
	if summary.SuppliedTokens != 80 {
		t.Fatalf("expected supplied 80, got %d", summary.SuppliedTokens)
	}
	if summary.NetTokens != -70 {
		t.Fatalf("unexpected net %d", summary.NetTokens)
	}
}

func TestListRecentOrdering(t *testing.T) {
	dir := t.TempDir()
	store, err := New(filepath.Join(dir, "ledger.db"), 100, 10, 60, 10)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	entries := []ledger.Entry{
		{UserID: 7, ServiceID: 1, PromptTokens: 1, CompletionTokens: 1, Direction: ledger.DirectionConsume, CreatedAt: time.Now().Add(-2 * time.Hour)},
		{UserID: 7, ServiceID: 1, PromptTokens: 2, CompletionTokens: 2, Direction: ledger.DirectionConsume, CreatedAt: time.Now().Add(-1 * time.Hour)},
		{UserID: 7, ServiceID: 1, PromptTokens: 3, CompletionTokens: 3, Direction: ledger.DirectionSupply, CreatedAt: time.Now()},
	}

	for _, e := range entries {
		if err := store.Record(ctx, e); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}

	recent, err := store.ListRecent(ctx, 7, 2)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(recent))
	}
	if recent[0].PromptTokens != 3 || recent[1].PromptTokens != 2 {
		t.Fatalf("unexpected ordering %#v", recent)
	}
}

func TestRecordValidation(t *testing.T) {
	dir := t.TempDir()
	store, err := New(filepath.Join(dir, "ledger.db"), 100, 10, 60, 10)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	err = store.Record(context.Background(), ledger.Entry{UserID: 0, Direction: ledger.DirectionConsume})
	if err == nil {
		t.Fatalf("expected error for missing user id")
	}

	err = store.Record(context.Background(), ledger.Entry{UserID: 1, Direction: "unexpected"})
	if err == nil {
		t.Fatalf("expected error for invalid direction")
	}
}
