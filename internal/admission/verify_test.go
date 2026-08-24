package admission_test

import (
	"context"
	"testing"
	"time"

	"github.com/wyw14/cry-96/internal/admission"
	"github.com/wyw14/cry-96/internal/gate"
	"github.com/wyw14/cry-96/internal/model"
	"github.com/wyw14/cry-96/internal/signal"
)

func TestAdmissionWaitsForSealedChamber(t *testing.T) {
	queue := admission.NewQueue()
	gates := gate.NewState("chamber-1")
	signals := signal.NewStore()
	controller := admission.NewController(queue, gates, signals, time.Now)
	vessel, _ := model.NewVessel("Harbor Light", "HL-7", 74)
	plan, _ := model.NewConvoyPlan("chamber-1", model.DirectionUpstream, []model.Vessel{vessel}, time.Now())
	if err := queue.Add(plan.ChamberID, plan.ID, plan.Vessels); err != nil {
		t.Fatal(err)
	}
	cycleValue, _ := model.NewTransitCycle(plan.ChamberID, plan.ID, plan.Direction, 1, time.Now())
	if err := gates.Observe(plan.ChamberID, model.DirectionUpstream, model.GateClosing, time.Now()); err != nil {
		t.Fatal(err)
	}
	decision, err := controller.Evaluate(context.Background(), cycleValue)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed {
		t.Fatal("admission was released while a gate was still moving")
	}
	entry, ok := queue.Peek(plan.ChamberID)
	if !ok || entry.Released {
		t.Fatal("waiting vessel was not preserved in the queue")
	}
}
