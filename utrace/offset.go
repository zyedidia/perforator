package utrace

// A PieOffsetter can determine the PIE offset for a given PID.
type PieOffsetter interface {
	// PieOffset returns the PIE offset for a given process. The result should
	// be 0 if ASLR/PIE is not enabled.
	PieOffset(pid int) (uint64, error)
}
