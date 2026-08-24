package cycle_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wyw14/cry-96/internal/api"
	"github.com/wyw14/cry-96/internal/model"
)

func TestCyclePlanSwitchKeepsGateStateSafe(t *testing.T) {
	ctx := context.Background()
	runtime, err := api.NewRuntime(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	vessel, _ := model.NewVessel("Atlas", "AT-1", 80)
	firstPlan, _ := model.NewConvoyPlan("chamber-1", model.DirectionUpstream, []model.Vessel{vessel}, time.Now())
	first, err := runtime.Cycles.Start(ctx, firstPlan)
	if err != nil {
		t.Fatal(err)
	}
	oldCommand, err := runtime.Gates.Request(ctx, first, model.DirectionUpstream, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Cycles.Cancel(ctx, first.ChamberID, "operator switch"); err != nil {
		t.Fatal(err)
	}
	secondPlan, _ := model.NewConvoyPlan("chamber-1", model.DirectionDownstream, []model.Vessel{vessel}, time.Now())
	second, err := runtime.Cycles.Start(ctx, secondPlan)
	if err != nil {
		t.Fatal(err)
	}
	ack := model.DeviceAck{
		CommandID: oldCommand.ID, CycleID: oldCommand.CycleID, ChamberID: oldCommand.ChamberID,
		Generation: oldCommand.Generation, Epoch: oldCommand.Epoch, Device: oldCommand.Device,
		Settled: true, ObservedAt: time.Now(),
	}
	server := httptest.NewServer(api.Router(runtime, t.TempDir()))
	defer server.Close()
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/chambers/chamber-1/commands/"+ack.CommandID.String()+"/ack", http.NoBody)
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	current, ok := runtime.Cycles.Current("chamber-1")
	if !ok {
		t.Fatal("new transit disappeared")
	}
	if current.ID != second.ID || current.Stage != model.StagePlanned {
		t.Fatalf("late ack changed new transit: id=%s stage=%s", current.ID, current.Stage)
	}
}
