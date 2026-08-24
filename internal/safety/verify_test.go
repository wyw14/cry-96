package safety_test

import (
	"context"
	"testing"
	"time"

	"github.com/wyw14/cry-96/internal/gate"
	"github.com/wyw14/cry-96/internal/journal"
	"github.com/wyw14/cry-96/internal/model"
	"github.com/wyw14/cry-96/internal/safety"
)

func TestEmergencyStopWaitsForStableEquipment(t *testing.T) {
	store := journal.NewMemoryStore()
	recorder := journal.NewRecorder(store, time.Now)
	barrier := safety.NewBarrier()
	barrier.Add("chamber-1", 1)
	coordinator := safety.NewCoordinator(gate.NewCommands(), barrier, recorder, time.Now)
	done := make(chan error, 1)
	go func() {
		_, err := coordinator.Stop(context.Background(), "chamber-1")
		done <- err
	}()
	deadline := time.Now().Add(time.Second)
	for {
		events, _ := store.Events(context.Background(), "chamber-1", 0)
		if len(events) > 0 {
			for _, event := range events {
				if event.Type == model.EventEmergencySettled {
					t.Fatal("emergency completed before actuator barrier settled")
				}
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("emergency start was not recorded")
		}
		time.Sleep(time.Millisecond)
	}
	barrier.Settled("chamber-1")
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	events, _ := store.Events(context.Background(), "chamber-1", 0)
	if len(events) < 2 || events[len(events)-1].Type != model.EventEmergencySettled {
		t.Fatalf("stable completion event missing: %+v", events)
	}
}
