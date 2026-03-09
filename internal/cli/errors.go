package cli

// RunFailureError is returned by the run command when tests fail or error.
// It carries the desired exit code so Execute() can exit appropriately
// without the run command calling os.Exit() directly inside RunE.
type RunFailureError struct {
	ExitCode int
	Msg      string
}

func (e *RunFailureError) Error() string { return e.Msg }
