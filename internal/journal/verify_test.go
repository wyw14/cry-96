package journal_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-96/internal/journal"
	"github.com/wyw14/cry-96/internal/model"
)

func TestReplayKeepsChambersIndependent(t *testing.T) {
	ctx := context.Background()
	store := journal.NewMemoryStore()
	for _, chamberID := range []string{"chamber-1", "chamber-2"} {
		for index := 0; index < 5; index++ {
			event, _ := model.NewEvent(chamberID, model.EventCycleAdvanced, uuid.New(), 1, map[string]int{"index": index}, time.Now())
			if _, err := store.Append(ctx, event); err != nil {
				t.Fatal(err)
			}
		}
		if err := store.SaveSnapshot(ctx, model.ChamberSnapshot{ChamberID: chamberID, Sequence: 3, SavedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	states, err := journal.NewReplayer(store).ReplayAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, chamberID := range []string{"chamber-1", "chamber-2"} {
		state := states[chamberID]
		if state.Cursor != 5 || len(state.Applied) != 2 {
			t.Fatalf("%s replay cursor=%d applied=%d", chamberID, state.Cursor, len(state.Applied))
		}
		for _, event := range state.Applied {
			if event.ChamberID != chamberID {
				t.Fatalf("%s applied event from %s", chamberID, event.ChamberID)
			}
		}
	}
}
