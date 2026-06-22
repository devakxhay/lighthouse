package ssl

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Generator struct {
	CADir    string
	CertsDir string
	log      *slog.Logger
}

func NewGenerator(caDir, certsDir string, logger *slog.Logger) *Generator {
	return &Generator{
		CADir:    caDir,
		CertsDir: certsDir,
		log:      logger.With(slog.String("component", "ssl")),
	}
}

func (g *Generator) caCert() string { return filepath.Join(g.CADir, "ca.crt") }
func (g *Generator) caConf() string { return filepath.Join(g.CADir, "openssl.cnf") }

type CertPaths struct {
	CertPath  string
	KeyPath   string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type IssuedCert struct {
	Serial    string     `json:"serial"`
	Status    string     `json:"status"` // V=valid, R=revoked, E=expired
	Domain    string     `json:"domain"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at"`
}

// Generate creates a signed SSL cert for the given domain using the user's CA.
func (g *Generator) Generate(domain string) (*CertPaths, error) {
	indexPath := filepath.Join(g.CADir, "index.txt")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("CA not initialized — run install.sh first")
	}

	if err := os.MkdirAll(g.CertsDir, 0755); err != nil {
		return nil, fmt.Errorf("create certs dir: %w", err)
	}

	base := filepath.Join(g.CertsDir, domain)
	keyPath := base + ".key"
	csrPath := base + ".csr"
	certPath := base + ".crt"
	extPath := base + ".ext"

	// Check if certificate already exists, and if so, revoke it first to allow re-signing.
	if _, err := os.Stat(certPath); err == nil {
		g.log.Info("cert already exists, revoking before re-generating", "domain", domain)
		if err := g.Revoke(domain); err != nil {
			g.log.Warn("failed to revoke existing cert", "domain", domain, "error", err.Error())
		}
	}

	// 1. Generate private key
	g.log.Debug("generating key", "domain", domain, "path", keyPath)
	if err := run("openssl", "genrsa", "-out", keyPath, "2048"); err != nil {
		g.log.Error("key generation failed", "domain", domain, "error", err.Error())
		return nil, fmt.Errorf("generate key: %w", err)
	}

	// 2. Generate CSR
	g.log.Debug("generating CSR", "domain", domain)
	subj := fmt.Sprintf("/CN=%s/O=Lighthouse/OU=Local", domain)
	if err := run("openssl", "req", "-new",
		"-key", keyPath,
		"-out", csrPath,
		"-subj", subj,
	); err != nil {
		g.log.Error("CSR generation failed", "domain", domain, "error", err.Error())
		return nil, fmt.Errorf("generate csr: %w", err)
	}

	// 3. Write SAN extension file
	extContent := fmt.Sprintf(`[v3_req]
subjectAltName = DNS:%s, DNS:*.%s
basicConstraints = CA:FALSE
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
`, domain, domain)
	if err := os.WriteFile(extPath, []byte(extContent), 0644); err != nil {
		return nil, fmt.Errorf("write ext file: %w", err)
	}

	// 4. Sign with CA
	validDays := "365"
	g.log.Debug("signing cert", "ca", g.caCert(), "days", validDays)
	if err := run("openssl", "ca",
		"-config", g.caConf(),
		"-in", csrPath,
		"-out", certPath,
		"-days", validDays,
		"-extensions", "v3_req",
		"-extfile", extPath,
		"-batch",
		"-notext",
	); err != nil {
		g.log.Error("signing failed", "domain", domain, "error", err.Error())
		return nil, fmt.Errorf("sign cert: %w", err)
	}

	// Cleanup temp files
	g.log.Debug("cleanup temp files", "csr", csrPath, "ext", extPath)
	os.Remove(csrPath)
	os.Remove(extPath)

	now := time.Now()
	expiresAt := now.Add(365 * 24 * time.Hour)
	g.log.Info("cert generated", "domain", domain, "expires", expiresAt.Format("2006-01-02"))
	return &CertPaths{
		CertPath:  certPath,
		KeyPath:   keyPath,
		IssuedAt:  now,
		ExpiresAt: expiresAt,
	}, nil
}

// Revoke marks a cert as revoked in index.txt and regenerates CRL
func (g *Generator) Revoke(domain string) error {
	certPath := filepath.Join(g.CertsDir, domain+".crt")
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return nil // Nothing to revoke
	}

	g.log.Info("revoking cert", "domain", domain, "path", certPath)
	// Ignore errors if the certificate is not active or already revoked in index.txt
	if err := run("openssl", "ca", "-config", g.caConf(), "-revoke", certPath, "-batch"); err != nil {
		g.log.Warn("revoke command returned error (could be already revoked)", "domain", domain, "error", err.Error())
	}

	crlPath := filepath.Join(g.CADir, "crl", "ca.crl")
	if err := run("openssl", "ca", "-config", g.caConf(), "-gencrl", "-out", crlPath); err != nil {
		g.log.Error("failed to generate CRL", "error", err.Error())
		return fmt.Errorf("generate crl: %w", err)
	}

	g.log.Info("crl generated successfully", "path", crlPath)
	return nil
}

// Remove revokes the certificate and deletes the certificate/key files.
func (g *Generator) Remove(domain string) error {
	// First revoke the certificate
	if err := g.Revoke(domain); err != nil {
		g.log.Warn("failed to revoke cert during removal", "domain", domain, "error", err.Error())
	}

	// Delete key and cert files
	base := filepath.Join(g.CertsDir, domain)
	if err := os.Remove(base + ".key"); err != nil && !os.IsNotExist(err) {
		g.log.Warn("failed to delete key file", "path", base+".key", "error", err.Error())
	}
	if err := os.Remove(base + ".crt"); err != nil && !os.IsNotExist(err) {
		g.log.Warn("failed to delete cert file", "path", base+".crt", "error", err.Error())
	}
	if err := os.Remove(base + ".csr"); err != nil && !os.IsNotExist(err) {
		g.log.Warn("failed to delete csr file", "path", base+".csr", "error", err.Error())
	}
	if err := os.Remove(base + ".ext"); err != nil && !os.IsNotExist(err) {
		g.log.Warn("failed to delete ext file", "path", base+".ext", "error", err.Error())
	}

	g.log.Info("cert files removed", "domain", domain)
	return nil
}

// ListIssued parses index.txt and returns all issued certs
func (g *Generator) ListIssued() ([]IssuedCert, error) {
	indexPath := filepath.Join(g.CADir, "index.txt")
	f, err := os.Open(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []IssuedCert{}, nil
		}
		return nil, fmt.Errorf("open index.txt: %w", err)
	}
	defer f.Close()

	var certs []IssuedCert
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 6 {
			continue
		}

		status := parts[0]
		expiryStr := parts[1]
		revokedStr := parts[2]
		serial := parts[3]
		subject := parts[5]

		expiresAt, err := parseOpenSSLTime(expiryStr)
		if err != nil {
			g.log.Warn("failed to parse certificate expiry time", "time", expiryStr, "error", err.Error())
			continue
		}

		var revokedAt *time.Time
		if revokedStr != "" {
			// Revocation format in index.txt has a suffix or is just timestamp. E.g. "YYMMDDHHMMSSZ,reason"
			// Split by comma to extract timestamp
			revTimeStr := strings.Split(revokedStr, ",")[0]
			if parsedRev, err := parseOpenSSLTime(revTimeStr); err == nil {
				revokedAt = &parsedRev
			} else {
				g.log.Warn("failed to parse certificate revocation time", "time", revTimeStr, "error", err.Error())
			}
		}

		domain := extractCN(subject)

		certs = append(certs, IssuedCert{
			Serial:    serial,
			Status:    status,
			Domain:    domain,
			ExpiresAt: expiresAt,
			RevokedAt: revokedAt,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read index.txt: %w", err)
	}

	return certs, nil
}

func parseOpenSSLTime(s string) (time.Time, error) {
	if len(s) == 13 {
		return time.Parse("060102150405Z", s)
	}
	return time.Parse("20060102150405Z", s)
}

func extractCN(subject string) string {
	parts := strings.Split(subject, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, "CN=") {
			return strings.TrimPrefix(part, "CN=")
		}
	}
	if idx := strings.Index(subject, "CN="); idx != -1 {
		cnPart := subject[idx+3:]
		if endIdx := strings.Index(cnPart, "/"); endIdx != -1 {
			return cnPart[:endIdx]
		}
		if endIdx := strings.Index(cnPart, ","); endIdx != -1 {
			return cnPart[:endIdx]
		}
		return cnPart
	}
	return subject
}

// Verify checks whether an existing cert is still valid.
func (g *Generator) Verify(certPath string) error {
	return run("openssl", "verify", "-CAfile", g.caCert(), certPath)
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err.Error(), string(out))
	}
	return nil
}
