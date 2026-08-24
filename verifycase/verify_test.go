package verifycase

import (
	"os"
	"testing"

	"edgelog/internal/batch"
	"edgelog/internal/dedup"
	"edgelog/internal/store"
	"edgelog/internal/tailer"
)

// TestDedupWindowHoldsUntilCommit verifies the dedup window stays open until
// the batch commit closes it.
func TestDedupWindowHoldsUntilCommit(t *testing.T) {
	root := t.TempDir()
	st, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	checkpoints := store.NewCheckpointStore(st.FS)
	batchesStore := store.NewBatchStore(st.FS)
	tailerSvc := tailer.NewService(st.FS, checkpoints)
	deduper := dedup.NewDeduper(60)
	batchSvc := batch.NewService(st.FS, batchesStore, checkpoints, deduper, tailerSvc)

	if !deduper.Accept("src", "line") {
		t.Fatal("first occurrence should be accepted")
	}
	record, err := batchSvc.Assemble("src", "topic", []string{"line"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	block := batchesStore.Path(record.ID)
	if err := os.WriteFile(block, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(block, 0o444); err != nil {
		t.Fatal(err)
	}
	if _, err := batchSvc.Commit("src", record, 10, 1); err == nil {
		t.Fatal("commit should fail while the staged batch write is blocked")
	}
	if deduper.Accept("src", "line") {
		t.Fatalf("dedup window closed before the commit boundary")
	}
}
