package runtime

import (
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
	DB DB
}

// Detect runs exec.LookPath for: go, java, npm, node.
// Saves results to DB. Skips entries where overridden=true in UpsertRuntime.
func (d *Detector) Detect() ([]Runtime, error) {
	targets := []string{"go", "java", "npm", "node"}
	var results []Runtime

	for _, name := range targets {
		path, err := exec.LookPath(name)
		found := err == nil
		if !found {
			path = ""
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

	return results, nil
}
