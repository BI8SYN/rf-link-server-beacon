package app

import (
	"context"
	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
	"path/filepath"
)

// Serve is the supervisor entry point. It waits for manual identity approval,
// preserves credentials and exports a new SAS candidate only on identity change.
// Exporting never applies configuration to an external SAS server.
func (s *Service) Serve(ctx context.Context, report func(CycleResult), phase func(string)) error {
	info, e := s.AwaitIdentity(ctx, phase)
	if e != nil {
		return e
	}
	if s.state.Fingerprint != info.Fingerprint {
		id, e := signer.NewID()
		if e != nil {
			return e
		}
		if e = s.ExportSAS(ctx, filepath.Join(s.cfg.StateDir, "sas-config-"+id+".json")); e != nil {
			return e
		}
		phase("sas_configuration_exported_not_applied")
	}
	phase("ready")
	return s.Run(ctx, report)
}
