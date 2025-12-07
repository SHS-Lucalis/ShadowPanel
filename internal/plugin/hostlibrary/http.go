package hostlibrary

import (
	"bytes"
	"context"
	"io"
	stdhttp "net/http"
	"time"

	"github.com/gameap/gameap/pkg/plugin/sdk/http"
	"github.com/samber/lo"
	"github.com/tetratelabs/wazero"
)

const (
	defaultTimeout = 30 * time.Second
	maxBodySize    = 10 * 1024 * 1024 // 10 MB
)

type HTTPServiceImpl struct {
	client *stdhttp.Client
}

func NewHTTPService() *HTTPServiceImpl {
	return &HTTPServiceImpl{
		client: &stdhttp.Client{
			Timeout: defaultTimeout,
		},
	}
}

func (s *HTTPServiceImpl) Fetch(
	ctx context.Context,
	req *http.HTTPFetchRequest,
) (*http.HTTPFetchResponse, error) {
	timeout := defaultTimeout
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds) * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var body io.Reader
	if req.Body != nil {
		body = bytes.NewReader(req.Body)
	}

	httpReq, err := stdhttp.NewRequestWithContext(ctx, req.Method, req.Url, body)
	if err != nil {
		return &http.HTTPFetchResponse{
			Error: lo.ToPtr(err.Error()),
		}, nil
	}

	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return &http.HTTPFetchResponse{
			Error: lo.ToPtr(err.Error()),
		}, nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return &http.HTTPFetchResponse{
			StatusCode: int32(resp.StatusCode), //nolint:gosec
			Error:      lo.ToPtr(err.Error()),
		}, nil
	}

	headers := make(map[string]string)
	for key := range resp.Header {
		headers[key] = resp.Header.Get(key)
	}

	return &http.HTTPFetchResponse{
		StatusCode: int32(resp.StatusCode), //nolint:gosec
		Headers:    headers,
		Body:       respBody,
	}, nil
}

type HTTPHostLibrary struct {
	impl *HTTPServiceImpl
}

func NewHTTPHostLibrary() *HTTPHostLibrary {
	return &HTTPHostLibrary{
		impl: NewHTTPService(),
	}
}

func (l *HTTPHostLibrary) Instantiate(ctx context.Context, r wazero.Runtime) error {
	return http.Instantiate(ctx, r, l.impl)
}
