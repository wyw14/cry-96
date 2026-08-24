package interlock

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-96/internal/gate"
	"github.com/wyw14/cry-96/internal/journal"
	"github.com/wyw14/cry-96/internal/model"
)

// newCoordinator builds a real Coordinator backed by in-memory state and a
// memory journal, mirroring the production wiring in api/runtime.go.
func newCoordinator() (*Coordinator, *gate.Controller, *gate.Commands) {
	state := gate.NewState("chamber-A")
	commands := gate.NewCommands()
	recorder := journal.NewRecorder(journal.NewMemoryStore(), time.Now)
	gates := gate.NewController(state, commands, recorder, time.Now)
	coord := NewCoordinator(DefaultPolicy(), NewArbiter(), gates,
		func(string) float64 { return 1.0 }, // within [0,12] safe window
		func(string) bool { return false }, // no emergency
	)
	return coord, gates, commands
}

func cycle(direction model.Direction) model.TransitCycle {
	cycleValue, _ := model.NewTransitCycle("chamber-A", uuid.New(), direction, 1, time.Now().UTC())
	return cycleValue
}

// TestCoordinatorConcurrentTakeoverPreservesExclusivity reproduces the night
// shift scenario: the automatic cycle requests the downstream gate while the
// operator concurrently takes over and requests the upstream gate. Both used
// to return accepted and leak mutual exclusion, leaving two opposing gate
// commands pending ("interlock conflict active=upstream requested=downstream").
// After the fix, exactly one direction may be granted.
func TestCoordinatorConcurrentTakeoverPreservesExclusivity(t *testing.T) {
	t.Parallel()
	for trial := 0; trial < 64; trial++ {
		coord, _, commands := newCoordinator()
		ctx := context.Background()

		var wg sync.WaitGroup
		var downstreamOK, upstreamOK int32
		var downstreamErr, upstreamErr atomic.Value
		wg.Add(2)

		// Auto-cycle: downstream open.
		go func() {
			defer wg.Done()
			cmd, err := coord.RequestOpen(ctx, cycle(model.DirectionDownstream), model.DirectionDownstream, "auto:cycle")
			if err == nil {
				atomic.StoreInt32(&downstreamOK, 1)
			} else {
				downstreamErr.Store(err.Error())
			}
			_ = cmd
		}()

		// Operator takeover: upstream open, issued almost simultaneously.
		go func() {
			defer wg.Done()
			cmd, err := coord.RequestOpen(ctx, cycle(model.DirectionUpstream), model.DirectionUpstream, "operator:console")
			if err == nil {
				atomic.StoreInt32(&upstreamOK, 1)
			} else {
				upstreamErr.Store(err.Error())
			}
			_ = cmd
		}()

		wg.Wait()

		dOK := atomic.LoadInt32(&downstreamOK) == 1
		uOK := atomic.LoadInt32(&upstreamOK) == 1
		var dErr, uErr any
		if v := downstreamErr.Load(); v != nil {
			dErr = v
		}
		if v := upstreamErr.Load(); v != nil {
			uErr = v
		}

		// Exactly one side must succeed; the other must be rejected by the
		// opposed-direction interlock.
		successes := 0
		if dOK {
			successes++
		}
		if uOK {
			successes++
		}
		if successes != 1 {
			t.Fatalf("trial %d: expected exactly one accepted open, got downstream_ok=%v upstream_ok=%v (downstream_err=%v upstream_err=%v)",
				trial, dOK, uOK, dErr, uErr)
		}

		// Exactly one pending gate command must exist for the chamber: the
		// accepted side issued its open; the rejected side issued nothing.
		pending := commands.Pending("chamber-A")
		if len(pending) != 1 {
			t.Fatalf("trial %d: expected 1 pending command, got %d", trial, len(pending))
		}

		// The active reservation must match the winning side and lock out the
		// opposite direction until released.
		reservation, ok := coord.Reservation("chamber-A")
		if !ok {
			t.Fatalf("trial %d: expected an active reservation", trial)
		}
		wantDirection := model.DirectionUpstream
		if pending[0].Device == model.DeviceDownstreamGate {
			wantDirection = model.DirectionDownstream
		}
		if reservation.Direction != wantDirection {
			t.Fatalf("trial %d: reservation direction %s does not match pending command device %s",
				trial, reservation.Direction, pending[0].Device)
		}

		// The losing direction must still be rejected now that a reservation is held.
		if _, err := coord.RequestOpen(ctx, cycle(reservation.Direction.Opposite()), reservation.Direction.Opposite(), "retry"); err == nil {
			t.Fatalf("trial %d: opposed direction was accepted after a reservation was held", trial)
		}
	}
}

// TestArbiterConcurrentOpposingReserves verifies the core invariant directly:
// two concurrent Reserve calls for opposite directions on the same chamber
// must produce exactly one successful commit and one rejected commit.
func TestArbiterConcurrentOpposingReserves(t *testing.T) {
	t.Parallel()
	for trial := 0; trial < 128; trial++ {
		a := NewArbiter()
		var committed int32
		var rejected int32
		var wg sync.WaitGroup
		wg.Add(2)

		commit := func() error {
			atomic.AddInt32(&committed, 1)
			// A small delay widens the race window so the TOCTOU would be
			// observable without the fix.
			time.Sleep(time.Millisecond)
			return nil
		}

		go func() {
			defer wg.Done()
			if _, err := a.Reserve("chamber-A", model.DirectionUpstream, "auto", commit); err != nil {
				atomic.AddInt32(&rejected, 1)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := a.Reserve("chamber-A", model.DirectionDownstream, "operator", commit); err != nil {
				atomic.AddInt32(&rejected, 1)
			}
		}()
		wg.Wait()

		if committed := atomic.LoadInt32(&committed); committed != 1 {
			t.Fatalf("trial %d: expected exactly 1 commit, got %d", trial, committed)
		}
		if rejected := atomic.LoadInt32(&rejected); rejected != 1 {
			t.Fatalf("trial %d: expected exactly 1 rejection, got %d", trial, rejected)
		}
	}
}
