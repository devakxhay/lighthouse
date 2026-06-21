package templates

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

type AppData struct {
	Name       string
	Domain     string
	Port       int
	BinaryPath string
	AppDir     string
	EnvFile    string
	CertPath   string
	KeyPath    string
	JavaBin    string
	NpmBin     string
	GoBin      string
	PathEnv    string
}

//go:embed units/*.service
var unitsFS embed.FS

//go:embed nginx/app.conf
var nginxFS embed.FS

// ---- Systemd Unit Templates ----

var springBootUnit, _ = unitsFS.ReadFile("units/0001_spring_boot.service")
var nextjsUnit, _ = unitsFS.ReadFile("units/0002_next_js.service")
var goUnit, _ = unitsFS.ReadFile("units/0003_go.service")

// ---- Nginx Config Template ----

var nginxConf, _ = nginxFS.ReadFile("nginx/app.conf")

// ---- Renderers ----

func RenderSystemdUnit(appType string, data AppData) (string, error) {
	var raw string
	switch appType {
	case "spring-boot":
		raw = string(springBootUnit)
	case "nextjs":
		raw = string(nextjsUnit)
	case "go":
		raw = string(goUnit)
	default:
		return "", fmt.Errorf("unknown app type: %s", appType)
	}
	return render(raw, data)
}

func RenderNginxConf(data AppData) (string, error) {
	return render(string(nginxConf), data)
}

func render(tmpl string, data AppData) (string, error) {
	t, err := template.New("").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
