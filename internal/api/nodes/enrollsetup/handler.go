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

	config, _ := api.NewQueryReader(r).ReadString("config")

	script := h.buildSetupScript(connectURL, config)

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

func (h *Handler) buildSetupScript(connectURL, config string) string {
	sb := strings.Builder{}
	sb.Grow(512)

	sb.WriteString("#!/bin/bash\nset -e\n\n")
	sb.WriteString("cleanup() { rm -f /tmp/gameapctl; }\n")
	sb.WriteString("trap cleanup EXIT\n\n")

	sb.WriteString("CONNECT_URL=\"")
	sb.WriteString(connectURL)
	sb.WriteString("\"\n\n")

	sb.WriteString("OS=$(uname -s | tr '[:upper:]' '[:lower:]')\n")
	sb.WriteString("ARCH=$(uname -m)\n")
	sb.WriteString("case \"$ARCH\" in\n")
	sb.WriteString("  x86_64|amd64) ARCH=\"amd64\" ;;\n")
	sb.WriteString("  aarch64|arm64) ARCH=\"arm64\" ;;\n")
	sb.WriteString("  *) echo \"Unsupported architecture: $ARCH\"; exit 1 ;;\n")
	sb.WriteString("esac\n\n")

	sb.WriteString("GAMEAPCTL_URL=\"https://github.com/gameap/gameapctl/releases/")
	sb.WriteString("latest/download/gameapctl_${OS}_${ARCH}\"\n")
	sb.WriteString("echo \"Downloading gameapctl...\"\n")
	sb.WriteString("curl -sLf -o /tmp/gameapctl \"$GAMEAPCTL_URL\"\n")
	sb.WriteString("chmod +x /tmp/gameapctl\n\n")

	sb.WriteString("/tmp/gameapctl daemon install --connect=\"$CONNECT_URL\"")

	if config != "" {
		sb.WriteString(" --config=\"")
		sb.WriteString(config)
		sb.WriteString("\"")
	}

	sb.WriteString("\n")

	return sb.String()
}
