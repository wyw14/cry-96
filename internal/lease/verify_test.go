package lease_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/wyw14/cry-96/internal/api"
	"github.com/wyw14/cry-96/internal/model"
)

func TestVesselReassignmentKeepsSingleLeaseOwner(t *testing.T) {
	runtime, err := api.NewRuntime(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(api.Router(runtime, t.TempDir()))
	defer server.Close()
	vesselID := uuid.New()
	assign := func(owner string, ttl int) model.Lease {
		body, _ := json.Marshal(map[string]any{
			"chamber_id": "chamber-1", "vessel_id": vesselID,
			"owner": owner, "ttl_seconds": ttl,
		})
		response, requestErr := http.Post(server.URL+"/api/leases", "application/json", bytes.NewReader(body))
		if requestErr != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("assign %s failed: response=%v err=%v", owner, response, requestErr)
		}
		defer response.Body.Close()
		var value model.Lease
		if err := json.NewDecoder(response.Body).Decode(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	oldLease := assign("primary", 1)
	newLease := assign("backup", 60)
	start := make(chan struct{})
	var accepted atomic.Int32
	var wait sync.WaitGroup
	for _, leaseValue := range []model.Lease{oldLease, newLease} {
		wait.Add(1)
		go func(value model.Lease) {
			defer wait.Done()
			<-start
			body, _ := json.Marshal(map[string]any{
				"chamber_id": "chamber-1", "owner": value.Owner,
				"token": value.FencingToken, "ttl_seconds": 60,
			})
			request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/leases/"+vesselID.String()+"/heartbeat", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response, requestErr := server.Client().Do(request)
			if requestErr == nil && response.StatusCode == http.StatusOK {
				accepted.Add(1)
			}
			if response != nil {
				response.Body.Close()
			}
		}(leaseValue)
	}
	close(start)
	wait.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("accepted %d concurrent heartbeats, want exactly one", accepted.Load())
	}
	if !runtime.Leases.Authorized(vesselID, newLease.Owner, newLease.FencingToken) {
		t.Fatal("backup owner lost the reassigned lease")
	}
}
