package enrollsetup

import (
	"net/http"
	"strings"

	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/internal/enrollment"
	"github.com/gameap/gameap/pkg/api"
	"github.com/pkg/errors"
)

type Handler struct {
	enrollmentSvc *enrollment.Service
	responder     base.Responder
	grpcExtHost   string
	grpcPort      uint16
	grpcExtPort   uint16
	panelHost     string
}

func NewHandler(
	enrollmentSvc *enrollment.Service,
	responder base.Responder,
	panelHost string,
	grpcExtHost string,
	grpcPort uint16,
	grpcExtPort uint16,
) *Handler {
	return &Handler{
		enrollmentSvc: enrollmentSvc,
		responder:     responder,
		panelHost:     panelHost,
		grpcExtHost:   grpcExtHost,
		grpcPort:      grpcPort,
		grpcExtPort:   grpcExtPort,
	}
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.enrollmentSvc == nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.New("enrollment is not available, gRPC is disabled"),
			http.StatusServiceUnavailable,
		))

		return
	}

	key, err := api.NewInputReader(r).ReadString("key")
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "invalid key"),
			http.StatusBadRequest,
		))

		return
	}

	if err := h.enrollmentSvc.SetupKeyManager().Validate(ctx, key); err != nil {
		if errors.Is(err, enrollment.ErrInvalidSetupKey) || errors.Is(err, enrollment.ErrSetupKeyNotConfigured) {
			h.responder.WriteError(ctx, rw, api.WrapHTTPError(
				errors.WithMessage(err, "invalid setup key"),
				http.StatusForbidden,
			))

			return
		}

		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to validate setup key"))

		return
	}

	grpcHost := h.resolveGRPCHost(r)
	grpcPort := h.grpcPort
	if h.grpcExtPort > 0 {
		grpcPort = h.grpcExtPort
	}

	connectURL := enrollment.FormatConnectURL(grpcHost, grpcPort, key)

	queryReader := api.NewQueryReader(r)
	config, _ := queryReader.ReadString("config")
	github, _ := queryReader.ReadString("github")
	branch, _ := queryReader.ReadString("branch")

	script := h.buildSetupScript(connectURL, config, github == "true", branch)

	rw.Header().Set("Content-Type", "text/plain")
	_, _ = rw.Write([]byte(script))
}

func (h *Handler) resolveGRPCHost(r *http.Request) string {
	if h.grpcExtHost != "" {
		return h.grpcExtHost
	}

	host := h.panelHost
	if host == "" {
		host = r.Header.Get("X-Forwarded-Host")
		if host == "" {
			host = r.Host
		}
	}

	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")

	if idx := strings.IndexByte(host, ':'); idx != -1 {
		host = host[:idx]
	}

	return host
}

func (h *Handler) buildSetupScript(connectURL, config string, github bool, branch string) string {
	sb := strings.Builder{}
	sb.Grow(1024)

	sb.WriteString("#!/bin/bash\nset -e\n\n")
	sb.WriteString("cleanup() { rm -f /tmp/gameapctl /tmp/gameapctl.tar.gz; }\n")
	sb.WriteString("trap cleanup EXIT\n\n")

	sb.WriteString("CONNECT_URL=\"")
	sb.WriteString(connectURL)
	sb.WriteString("\"\n\n")

	installCmd := h.buildInstallCmd(config, github, branch)

	// Check if gameapctl is already installed
	sb.WriteString("if command -v gameapctl >/dev/null 2>&1; then\n")
	sb.WriteString("  echo \"gameapctl found, updating...\"\n")
	sb.WriteString("  gameapctl self-update || true\n")
	sb.WriteString("  ")
	sb.WriteString(strings.Replace(installCmd, "/tmp/gameapctl", "gameapctl", 1))
	sb.WriteString("\n  exit 0\nfi\n\n")

	// Check required utilities
	sb.WriteString("for cmd in curl tar; do\n")
	sb.WriteString("  if ! command -v \"$cmd\" >/dev/null 2>&1; then\n")
	sb.WriteString("    echo \"Error: '$cmd' is required but not installed.\"\n")
	sb.WriteString("    echo \"Install it with:\"\n")
	sb.WriteString("    echo \"  apt-get install $cmd  (Debian/Ubuntu)\"\n")
	sb.WriteString("    echo \"  yum install $cmd      (RHEL/CentOS)\"\n")
	sb.WriteString("    exit 1\n")
	sb.WriteString("  fi\n")
	sb.WriteString("done\n\n")

	sb.WriteString("OS=$(uname -s | tr '[:upper:]' '[:lower:]')\n")
	sb.WriteString("ARCH=$(uname -m)\n")
	sb.WriteString("case \"$ARCH\" in\n")
	sb.WriteString("  x86_64|amd64) ARCH=\"amd64\" ;;\n")
	sb.WriteString("  aarch64|arm64) ARCH=\"arm64\" ;;\n")
	sb.WriteString("  *) echo \"Unsupported architecture: $ARCH\"; exit 1 ;;\n")
	sb.WriteString("esac\n\n")

	sb.WriteString("echo \"Downloading gameapctl...\"\n")
	sb.WriteString("VERSION=$(curl -sL ")
	sb.WriteString("https://api.github.com/repos/gameap/gameapctl/releases")
	sb.WriteString(" | grep -m1 '\"tag_name\"' | sed 's/.*\"tag_name\": *\"//;s/\".*//')\n")
	sb.WriteString("if [ -z \"$VERSION\" ]; then\n")
	sb.WriteString("  echo \"Failed to detect latest gameapctl version\"\n")
	sb.WriteString("  exit 1\nfi\n\n")
	sb.WriteString("ARCHIVE=\"gameapctl-${VERSION}-${OS}-${ARCH}.tar.gz\"\n")
	sb.WriteString("DOWNLOAD_URL=\"https://github.com/gameap/gameapctl/releases/")
	sb.WriteString("download/${VERSION}/${ARCHIVE}\"\n")
	sb.WriteString("echo \"Downloading ${DOWNLOAD_URL}\"\n")
	sb.WriteString("curl -sLf -o /tmp/gameapctl.tar.gz \"$DOWNLOAD_URL\"\n")
	sb.WriteString("tar -xzf /tmp/gameapctl.tar.gz -C /tmp gameapctl\n")
	sb.WriteString("chmod +x /tmp/gameapctl\n")
	sb.WriteString("rm -f /tmp/gameapctl.tar.gz\n\n")

	sb.WriteString(installCmd)
	sb.WriteString("\n")

	return sb.String()
}

func (h *Handler) buildInstallCmd(
	config string, github bool, branch string,
) string {
	sb := strings.Builder{}

	sb.WriteString("/tmp/gameapctl daemon install --connect=\"$CONNECT_URL\"")

	if config != "" {
		sb.WriteString(" --config=")
		sb.WriteString(shellEscape(config))
	}

	if github {
		sb.WriteString(" --github")
	}

	if branch != "" {
		sb.WriteString(" --branch=")
		sb.WriteString(shellEscape(branch))
	}

	return sb.String()
}

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
