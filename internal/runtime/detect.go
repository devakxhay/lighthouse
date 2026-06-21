package runtime

import (
	"fmt"
	"log/slog"
	"os/exec"
)

type Runtime struct {
	Name    string
	BinPath string
	Found   bool
}

type DB interface {
	UpsertRuntime(name, path string, overridden bool) error
}

type Detector struct {
	DB  DB
	log *slog.Logger
}

func NewDetector(database DB, logger *slog.Logger) *Detector {
	return &Detector{
		DB:  database,
		log: logger.With(slog.String("component", "runtime")),
	}
}

// Detect runs exec.LookPath for: go, java, npm, node.
// Saves results to DB. Skips entries where overridden=true in UpsertRuntime.
func (d *Detector) Detect() ([]Runtime, error) {
	d.log.Info("detecting runtimes...")
	targets := []string{"go", "java", "npm", "node"}
	var results []Runtime
	foundCount := 0
	missingCount := 0

	for _, name := range targets {
		path, err := exec.LookPath(name)
		found := err == nil
		if !found {
			path = ""
			d.log.Warn(fmt.Sprintf("%s not found in PATH", name))
			missingCount++
		} else {
			d.log.Debug(fmt.Sprintf("found %s=%s", name, path))
			foundCount++
		}

		results = append(results, Runtime{
			Name:    name,
			BinPath: path,
			Found:   found,
		})

		if err := d.DB.UpsertRuntime(name, path, false); err != nil {
			return nil, err
		}
	}

	d.log.Info("detection complete", "found", foundCount, "missing", missingCount)
	return results, nil
}

