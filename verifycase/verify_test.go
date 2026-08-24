package verifycase

import (
	"os"
	"testing"

	"edgelog/internal/batch"
	"edgelog/internal/dedup"
	"edgelog/internal/store"
	"edgelog/internal/tailer"
)

// TestCheckpointFollowsBatchDurable verifies recovery writes the checkpoint
// only after the staged batches are re-affirmed durably.
func TestCheckpointFollowsBatchDurable(t *testing.T) {
	root := t.TempDir()
	st, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	checkpoints := store.NewCheckpointStore(st.FS)
	batchesStore := store.NewBatchStore(st.FS)
	tailerSvc := tailer.NewService(st.FS, checkpoints)
	batchSvc := batch.NewService(st.FS, batchesStore, checkpoints, dedup.NewDeduper(60), tailerSvc)

	record, err := batchSvc.Assemble("src", "topic", []string{"x"}, 99)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := batchSvc.Stage(record); err != nil {
		t.Fatal(err)
	}
	block := batchesStore.Path(record.ID)
	if err := os.Chmod(block, 0o444); err != nil {
		t.Fatal(err)
	}
	if _, err := batchSvc.Restore("src"); err == nil {
		t.Fatal("restore should fail when a staged batch cannot be re-affirmed")
	}
	if _, err := checkpoints.Load("src"); err != store.ErrNotFound {
		t.Fatalf("checkpoint advanced before the staged batch was durable: %v", err)
	}
}
