package runtime

import (
	"fmt"
	"log/slog"
	"os/exec"
	"github.com/devakxhay/lighthouse/internal/config"
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
	cfg *config.Config
}

func NewDetector(database DB, cfg *config.Config, logger *slog.Logger) *Detector {
	return &Detector{
		DB:  database,
		cfg: cfg,
		log: logger.With(slog.String("component", "runtime")),
	}
}

// Detect runs exec.LookPath for: go, java, npm, node.
// Saves results to DB. Skips entries where overridden=true in UpsertRuntime.
func (d *Detector) Detect() ([]Runtime, error) {
	d.log.Info("detecting runtimes...")
	targets := []string{"go", "java", "npm", "node", "npx"}
	var results []Runtime
	foundCount := 0
	missingCount := 0

	for _, name := range targets {
		var path string
		var found bool
		// Prefer a user‑defined path from configuration if present
		if custom, ok := d.cfg.RuntimePaths[name]; ok && custom != "" {
			path = custom
			found = true
		} else {
			var err error
			path, err = exec.LookPath(name)
			found = err == nil
		}
		if !found {
			path = ""
			d.log.Warn(fmt.Sprintf("%s not found in PATH or config", name))
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

