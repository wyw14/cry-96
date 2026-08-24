package hydraulics_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wyw14/cry-96/internal/api"
	"github.com/wyw14/cry-96/internal/model"
)

func TestCancelledRampStopsFurtherValveCommands(t *testing.T) {
	runtime, err := api.NewRuntime(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	vessel, _ := model.NewVessel("Ramp Test", "RT-1", 40)
	plan, _ := model.NewConvoyPlan("chamber-1", model.DirectionUpstream, []model.Vessel{vessel}, time.Now())
	if _, err := runtime.Cycles.Start(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(api.Router(runtime, t.TempDir()))
	defer server.Close()
	requestContext, cancel := context.WithCancel(context.Background())
	request, _ := http.NewRequestWithContext(requestContext, http.MethodPost, server.URL+"/api/chambers/chamber-1/valves/ramp", bytes.NewBufferString(`{"targets":[20,40,60]}`))
	request.Header.Set("Content-Type", "application/json")
	done := make(chan struct{})
	go func() {
		response, _ := server.Client().Do(request)
		if response != nil {
			response.Body.Close()
		}
		close(done)
	}()
	deadline := time.Now().Add(time.Second)
	for len(runtime.GateCommands.Pending("chamber-1")) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
	time.Sleep(100 * time.Millisecond)
	if got := len(runtime.GateCommands.Pending("chamber-1")); got != 1 {
		t.Fatalf("request cancellation left %d valve commands, want 1", got)
	}
}
