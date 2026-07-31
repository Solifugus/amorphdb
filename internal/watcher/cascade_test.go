// Tests for watcher cascade and convergence.
//
// DEVPLAN Step 29. Two defects motivated these:
//
//  1. Cascade never happened. ExecuteWatcher collected written paths via
//     GetWrittenPaths, which reads the commit buffer — already flushed and
//     cleared by Interpret — so the list was always empty and a watcher writing
//     the path it watched fired exactly once, like one that wrote nothing.
//
//  2. An unchanged write still announced itself. Storage declines to create an
//     instance for an identical value but reported nothing back, so a watcher
//     recomputing the same value would keep re-triggering once cascade worked.
//
// The pair matters: fixing (1) without (2) would turn a silent no-op into a
// spin.
package watcher

import (
	"testing"
	"time"

	"github.com/solifugus/amorphdb/internal/storage"
)

// runRounds drives tick-like rounds by hand: trigger, execute, commit cascade,
// tick. It returns how many rounds fired at least one watcher.
func runRounds(t *testing.T, engine *WatcherEngine, maxRounds int) int {
	t.Helper()

	fired := 0
	for round := 0; round < maxRounds; round++ {
		triggered := engine.GetTriggeredWatchers()
		if len(triggered) == 0 {
			break
		}
		fired++
		for _, w := range triggered {
			if _, err := engine.ExecuteWatcher(w); err != nil {
				t.Logf("round %d: watcher %s: %v", round, w.Name, err)
			}
		}
		engine.Tick()
		engine.ClearChangeLog()
		engine.CommitCascadeChanges()
		time.Sleep(2 * time.Millisecond)
	}
	return fired
}

// TestCascadeReachesADependentWatcher is the positive half: a watcher writing a
// path that another watcher observes must wake that second watcher. Before the
// fix nothing cascaded at all.
func TestCascadeReachesADependentWatcher(t *testing.T) {
	st := createTestStorage(t)
	defer st.Close()

	engine := NewWatcherEngine(st, 1000)
	if err := engine.RegisterWatcher("first", []string{"my.source"}, `my.middle = 1`); err != nil {
		t.Fatal(err)
	}
	if err := engine.RegisterWatcher("second", []string{"my.middle"}, `my.sink = 2`); err != nil {
		t.Fatal(err)
	}

	engine.RecordChange("my.source")
	time.Sleep(2 * time.Millisecond)

	// Round 1 runs "first", which writes my.middle. That must queue "second".
	triggered := engine.GetTriggeredWatchers()
	if len(triggered) != 1 || triggered[0].Name != "first" {
		t.Fatalf("round 1 triggered %d watchers, want just 'first'", len(triggered))
	}
	if _, err := engine.ExecuteWatcher(triggered[0]); err != nil {
		t.Fatalf("execute first: %v", err)
	}
	engine.Tick()
	engine.ClearChangeLog()
	engine.CommitCascadeChanges()
	time.Sleep(2 * time.Millisecond)

	triggered = engine.GetTriggeredWatchers()
	if len(triggered) == 0 {
		t.Fatal("cascade did not reach the dependent watcher — nothing triggered in round 2")
	}
	found := false
	for _, w := range triggered {
		if w.Name == "second" {
			found = true
		}
	}
	if !found {
		t.Errorf("round 2 triggered %v, want 'second'", triggered)
	}
}

// TestUnchangedSelfWriteConverges is the negative half, and the reason cascade is
// safe to enable. A watcher that writes the same value back must settle.
func TestUnchangedSelfWriteConverges(t *testing.T) {
	st := createTestStorage(t)
	defer st.Close()

	engine := NewWatcherEngine(st, 1000)
	// Writes a constant to the very path it watches.
	if err := engine.RegisterWatcher("settler", []string{"my.data"}, `my.data = 99`); err != nil {
		t.Fatal(err)
	}

	engine.RecordChange("my.data")
	time.Sleep(2 * time.Millisecond)

	rounds := runRounds(t, engine, 12)
	if rounds > 3 {
		t.Errorf("self-writing watcher fired in %d rounds — it is not converging", rounds)
	}
	if rounds == 0 {
		t.Error("watcher never fired at all")
	}
}

// TestQuietSelfWriteDoesNotCascade covers the escape hatch: a value that differs
// every round would spin, unless written quietly.
func TestQuietSelfWriteDoesNotCascade(t *testing.T) {
	st := createTestStorage(t)
	defer st.Close()

	engine := NewWatcherEngine(st, 1000)
	if err := engine.RegisterWatcher("quiet", []string{"my.counter"},
		`my.counter = (quietly) my.counter + 1`); err != nil {
		t.Fatal(err)
	}

	engine.RecordChange("my.counter")
	time.Sleep(2 * time.Millisecond)

	rounds := runRounds(t, engine, 12)
	if rounds > 2 {
		t.Errorf("quiet self-write cascaded for %d rounds — (quietly) is not suppressing", rounds)
	}
}

// TestCascadeRunawayIsCapped ensures a chain that genuinely cannot converge is
// stopped and named, rather than cascading forever.
func TestCascadeRunawayIsCapped(t *testing.T) {
	st := createTestStorage(t)
	defer st.Close()

	engine := NewWatcherEngine(st, 1000)

	// Force the engine past the limit directly: driving 1000 real ticks would
	// make this test slow for no extra confidence.
	for i := 0; i <= MaxCascadeRounds; i++ {
		engine.pendingCascadePaths["my.spinner"] = true
		runaway, paths := engine.CommitCascadeChanges()
		if i < MaxCascadeRounds {
			if runaway {
				t.Fatalf("reported runaway after %d rounds, before the limit of %d", i+1, MaxCascadeRounds)
			}
			continue
		}
		if !runaway {
			t.Fatalf("no runaway reported after %d rounds", i+1)
		}
		if len(paths) == 0 {
			t.Error("runaway reported without naming the cascading paths")
		}
	}
}

// TestCascadeRoundsResetWhenQuiet checks the counter is a *consecutive* measure —
// an application that cascades briefly, often, must not accumulate toward the cap.
func TestCascadeRoundsResetWhenQuiet(t *testing.T) {
	st := createTestStorage(t)
	defer st.Close()

	engine := NewWatcherEngine(st, 1000)

	for i := 0; i < MaxCascadeRounds*2; i++ {
		engine.pendingCascadePaths["my.occasional"] = true
		if runaway, _ := engine.CommitCascadeChanges(); runaway {
			t.Fatalf("runaway reported at round %d despite quiet rounds in between", i)
		}
		// A quiet round with nothing pending must reset the counter.
		if runaway, _ := engine.CommitCascadeChanges(); runaway {
			t.Fatal("quiet round reported a runaway")
		}
	}
}

// TestWatcherMatchesDescendantChanges covers the matching rule: a watcher on a
// container sees changes beneath it. Matching used to be exact string equality,
// so `watch(my.orders)` was blind to `my.orders.17.status` — surely the main
// reason to watch a container in the first place.
func TestWatcherMatchesDescendantChanges(t *testing.T) {
	tests := []struct {
		name    string
		watched string
		changed string
		want    bool
	}{
		{"exact path", "my.orders", "my.orders", true},
		{"direct child", "my.orders", "my.orders.17", true},
		{"deep descendant", "my.orders", "my.orders.17.status", true},
		{"unrelated sibling", "my.orders", "my.invoices.3", false},
		{"ancestor does not match downward", "my.orders.17", "my.orders", false},
		// The trap a bare prefix comparison falls into.
		{"prefix but not a path boundary", "my.order", "my.orders.17", false},
		{"similar leading name", "my.o", "my.orders", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := createTestStorage(t)
			defer st.Close()

			engine := NewWatcherEngine(st, 1000)
			if err := engine.RegisterWatcher("w", []string{tt.watched}, `pass`); err != nil {
				t.Fatal(err)
			}

			engine.RecordChange(tt.changed)
			time.Sleep(2 * time.Millisecond)

			got := len(engine.GetTriggeredWatchers()) > 0
			if got != tt.want {
				t.Errorf("watching %q, changed %q: triggered=%v, want %v",
					tt.watched, tt.changed, got, tt.want)
			}
		})
	}
}

// TestConvergenceThroughTheDaemonsTree runs the convergence assertion against
// the tree the daemon actually uses.
//
// This exists because of a bug the other tests could not see. The daemon wraps
// StorageTree in a TreeAdapter (service.go: NewTreeAdapter, then
// NewWatcherEngine(extendedTree, ...)), and the adapter did not forward
// WriteReportingChange. The capability is discovered by type assertion, so it
// silently degraded: unchanged writes were reported as changes and a
// self-writing watcher would have spun in production while every test here
// passed, because they all build a bare StorageTree.
//
// Any capability found by type assertion needs a test in the wrapped
// configuration, or the wrapper can quietly drop it.
func TestConvergenceThroughTheDaemonsTree(t *testing.T) {
	st := createTestStorage(t)
	defer st.Close()

	// Exactly what the service constructs.
	adapter := storage.NewTreeAdapter(st)
	engine := NewWatcherEngine(adapter, 1000)

	if err := engine.RegisterWatcher("settler", []string{"my.data"}, `my.data = 99`); err != nil {
		t.Fatal(err)
	}

	engine.RecordChange("my.data")
	time.Sleep(2 * time.Millisecond)

	rounds := runRounds(t, engine, 12)
	if rounds == 0 {
		t.Fatal("watcher never fired through the adapter")
	}
	if rounds > 3 {
		t.Errorf("self-writing watcher fired in %d rounds through the daemon's tree — "+
			"the adapter is not forwarding the unchanged-write signal", rounds)
	}
}
