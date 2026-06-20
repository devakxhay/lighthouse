package ssl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Generator struct {
	CACertPath string
	CAKeyPath  string
	CertsDir   string
}

type CertPaths struct {
	CertPath  string
	KeyPath   string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// Generate creates a signed SSL cert for the given domain using the user's CA.
func (g *Generator) Generate(domain string) (*CertPaths, error) {
	if err := os.MkdirAll(g.CertsDir, 0755); err != nil {
		return nil, fmt.Errorf("create certs dir: %w", err)
	}

	base := filepath.Join(g.CertsDir, domain)
	keyPath := base + ".key"
	csrPath := base + ".csr"
	certPath := base + ".crt"
	extPath := base + ".ext"

	// 1. Generate private key
	if err := run("openssl", "genrsa", "-out", keyPath, "2048"); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	// 2. Generate CSR
	subj := fmt.Sprintf("/CN=%s/O=Lighthouse/OU=Local", domain)
	if err := run("openssl", "req", "-new",
		"-key", keyPath,
		"-out", csrPath,
		"-subj", subj,
	); err != nil {
		return nil, fmt.Errorf("generate csr: %w", err)
	}

	// 3. Write SAN extension file
	extContent := fmt.Sprintf(`[v3_req]
subjectAltName = DNS:%s, DNS:*.%s
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
`, domain, domain)
	if err := os.WriteFile(extPath, []byte(extContent), 0644); err != nil {
		return nil, fmt.Errorf("write ext file: %w", err)
	}

	// 4. Sign with CA
	validDays := "365"
	if err := run("openssl", "x509", "-req",
		"-in", csrPath,
		"-CA", g.CACertPath,
		"-CAkey", g.CAKeyPath,
		"-CAcreateserial",
		"-out", certPath,
		"-days", validDays,
		"-extensions", "v3_req",
		"-extfile", extPath,
	); err != nil {
		return nil, fmt.Errorf("sign cert: %w", err)
	}

	// Cleanup temp files
	os.Remove(csrPath)
	os.Remove(extPath)

	now := time.Now()
	return &CertPaths{
		CertPath:  certPath,
		KeyPath:   keyPath,
		IssuedAt:  now,
		ExpiresAt: now.Add(365 * 24 * time.Hour),
	}, nil
}

// Verify checks whether an existing cert is still valid.
func (g *Generator) Verify(certPath string) error {
	return run("openssl", "verify", "-CAfile", g.CACertPath, certPath)
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err.Error(), string(out))
	}
	return nil
}
