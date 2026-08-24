package signal_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-96/internal/model"
	"github.com/wyw14/cry-96/internal/signal"
)

type transitLookup struct {
	current model.TransitCycle
	applied int
}

func (l *transitLookup) Current(string) (model.TransitCycle, bool) { return l.current, true }
func (l *transitLookup) ApplySignalHandoff(context.Context, uuid.UUID, uint64) error {
	l.applied++
	return nil
}

func TestCompletedTransitIgnoresLateSignal(t *testing.T) {
	cycleID := uuid.New()
	lookup := &transitLookup{current: model.TransitCycle{
		ID: cycleID, ChamberID: "chamber-1", Generation: 8,
		Stage: model.StageCompleted,
	}}
	service := signal.NewService(signal.NewStore())
	service.BindTransitLookup(lookup)
	err := service.ConfirmHandoff(context.Background(), signal.Handoff{
		ChamberID: "chamber-1", CycleID: cycleID, Generation: 8, ObservedAt: time.Now(),
	})
	if err == nil {
		t.Fatal("completed transit accepted a late signal handoff")
	}
	if lookup.applied != 0 || len(service.History()) != 0 {
		t.Fatalf("late handoff changed state: applied=%d history=%d", lookup.applied, len(service.History()))
	}
}
