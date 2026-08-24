package hydraulics_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-96/internal/hydraulics"
	"github.com/wyw14/cry-96/internal/model"
)

func TestLevelDecisionUsesCurrentTransit(t *testing.T) {
	window, err := hydraulics.NewLevelWindow(4)
	if err != nil {
		t.Fatal(err)
	}
	oldID, newID := uuid.New(), uuid.New()
	window.Begin(oldID, 4)
	for index := 0; index < 3; index++ {
		if err := window.Add(model.LevelSample{CycleID: oldID, Generation: 4, Meters: 7.25, ObservedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	window.Begin(newID, 5)
	for index := 0; index < 2; index++ {
		if err := window.Add(model.LevelSample{CycleID: newID, Generation: 5, Meters: 7.25, ObservedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	if window.Stable(0.05) {
		t.Fatal("new transit reached stability with an incomplete sample window")
	}
	if samples := window.Samples(); len(samples) != 2 {
		t.Fatalf("current window contains %d samples, want 2", len(samples))
	}
}
