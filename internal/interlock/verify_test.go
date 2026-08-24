package interlock_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/wyw14/cry-96/internal/api"
	"github.com/wyw14/cry-96/internal/interlock"
	"github.com/wyw14/cry-96/internal/model"
)

func TestOpposedGateCommandsRemainExclusive(t *testing.T) {
	_ = interlock.DefaultPolicy()
	for attempt := 0; attempt < 6; attempt++ {
		runtime, err := api.NewRuntime(context.Background(), "")
		if err != nil {
			t.Fatal(err)
		}
		vessel, _ := model.NewVessel("Pilot", "PI-1", 55)
		plan, _ := model.NewConvoyPlan("chamber-1", model.DirectionUpstream, []model.Vessel{vessel}, runtime.Now())
		if _, err := runtime.Cycles.Start(context.Background(), plan); err != nil {
			t.Fatal(err)
		}
		server := httptest.NewServer(api.Router(runtime, t.TempDir()))
		start := make(chan struct{})
		var accepted atomic.Int32
		var wait sync.WaitGroup
		for index, direction := range []model.Direction{model.DirectionUpstream, model.DirectionDownstream} {
			wait.Add(1)
			go func(owner string, requested model.Direction) {
				defer wait.Done()
				<-start
				request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/chambers/chamber-1/gates/"+string(requested)+"/open", http.NoBody)
				request.Header.Set("X-Owner", owner)
				response, requestErr := server.Client().Do(request)
				if requestErr == nil && response.StatusCode == http.StatusAccepted {
					accepted.Add(1)
				}
				if response != nil {
					response.Body.Close()
				}
			}(string(rune('A'+index)), direction)
		}
		close(start)
		wait.Wait()
		server.Close()
		if got := accepted.Load(); got != 1 {
			t.Fatalf("opposed requests accepted=%d, want exactly one", got)
		}
	}
}
