package bins

import "context"

// Status represents the detection status of a binary.
type Status struct {
	Found   bool
	BinPath string
	Version string
}

// DB defines the database operations required by the installers.
type DB interface {
	SetRuntimeOverride(name, path string) error
}

// Installer defines the interface that each runtime installer must implement.
type Installer interface {
	Name() string                                                 // "go", "java", "node"
	Label() string                                                // Human-readable label
	Detect() Status                                               // Detect if binary is already installed
	Install(ctx context.Context, logger LogFunc) (Status, error) // Install the binary and return status
}
type LogFunc func(msg string)
