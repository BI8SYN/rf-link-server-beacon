package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestServePreparesIdentityAndHonorsCancellationBeforeAnySend(t *testing.T) {
	_, s := newStationRig(t)
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	phases := []string{}
	e := s.Serve(ctx, func(CycleResult) { t.Error("unexpected send") }, func(phase string) {
		phases = append(phases, phase)
		if phase == "ready" {
			cancel()
		}
	})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if s.state.Fingerprint == "" {
		t.Fatal("daemon did not prepare same-identity SAS export")
	}
	files, _ := filepath.Glob(filepath.Join(s.cfg.StateDir, "sas-config-*.json"))
	if len(files) != 1 {
		t.Fatal("missing private SAS candidate")
	}
	st, _ := os.Stat(files[0])
	if st.Mode().Perm() != 0600 {
		t.Fatal("SAS export not private")
	}
	if len(phases) == 0 || phases[len(phases)-1] != "ready" {
		t.Fatal("ready phase missing")
	}
	e = s.Serve(ctx, func(CycleResult) { t.Error("unexpected send") }, func(string) {})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	files, _ = filepath.Glob(filepath.Join(s.cfg.StateDir, "sas-config-*.json"))
	if len(files) != 1 {
		t.Fatal("daemon recreated export on restart")
	}
	if s.state.LastAttempt != 0 || s.state.LastPublished != 0 {
		t.Fatal("canceled daemon broadcast")
	}
}
