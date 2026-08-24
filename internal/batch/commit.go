package batch

import (
	"edgelog/internal/store"
	"edgelog/internal/tailer"
)

// Commit persists a batch durably, then advances the source cursor and closes
// the dedup window. The order matters: the batch must land before the cursor
// moves and before the dedup window closes, so a delayed replay of lines
// whose cursor has not yet advanced is still recognised as a duplicate. The
// dedup window must close last: an earlier close lets a replay between close
// and commit be ingested again as new data.
func (s *Service) Commit(sourceID string, record store.BatchRecord, nextOffset int64, nextLine int) (store.BatchRecord, error) {
	staged, err := s.Stage(record)
	if err != nil {
		return store.BatchRecord{}, err
	}
	staged.State = store.BatchCommitted
	staged.UpdatedAt = s.now()
	if err := s.batches.Stage(staged); err != nil {
		return store.BatchRecord{}, err
	}
	current, err := s.advancer.Current(sourceID)
	if err != nil {
		return store.BatchRecord{}, err
	}
	next := tailer.Cursor{SourceID: sourceID, Offset: nextOffset, Line: nextLine}
	if _, err := s.advancer.Advance(current, next); err != nil {
		return store.BatchRecord{}, err
	}
	if s.dedup != nil {
		s.dedup.Close(sourceID)
	}
	if s.audit != nil {
		_, _ = s.audit.Record("engine", "batch.commit", "batch", staged.ID, staged.SourceID)
	}
	return staged, nil
}
