package collector

import "nvtop-web/internal/model"

// FilterPID removes the process with the given PID from every GPU's process
// list. nvtop opens the DRM device to read metrics, so the supervised child
// reports itself as a "graphic" process — self-monitoring noise the dashboard
// shouldn't show.
func FilterPID(s *model.Snapshot, pid int) {
	if pid <= 0 {
		return
	}
	for i := range s.GPUs {
		g := &s.GPUs[i]
		kept := g.Processes[:0]
		for _, p := range g.Processes {
			if p.PID == nil || *p.PID != uint64(pid) {
				kept = append(kept, p)
			}
		}
		g.Processes = kept
	}
}
