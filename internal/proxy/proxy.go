package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/omnitoken/omnitoken/internal/auth"
	"github.com/omnitoken/omnitoken/internal/credentials"
	"github.com/omnitoken/omnitoken/internal/httpx"
)

const (
	DefaultMaxRequestBytes      = int64(1 << 20)
	DefaultMaxCredentialRetries = 2

	CodeInvalidRequest              = "invalid_request"
	CodeUpstreamNotConfigured       = "upstream_not_configured"
	CodeUpstreamTimeout             = "upstream_timeout"
	CodeUpstreamConnectionFailed    = "upstream_connection_failed"
	CodeUpstream5xx                 = "upstream_5xx"
	CodeUpstream429                 = "upstream_429"
	CodeUpstreamInvalidResponse     = "upstream_invalid_response"
	CodeUpstreamCredentialPoolEmpty = "upstream_credential_pool_empty"
	defaultConnectTimeout           = 5 * time.Second
	defaultWriteTimeout             = 10 * time.Second
	defaultFirstByteTimeout         = 20 * time.Second
	defaultNonStreamTotalTimeout    = 60 * time.Second
	defaultSSEIdleTimeout           = 30 * time.Second
	defaultExpectContinueTimeout    = time.Second
	defaultTransportKeepAlive       = 30 * time.Second
	defaultTransportIdleConn        = 90 * time.Second
	defaultTransportMaxIdleConns    = 100
	defaultTransportMaxIdlePerHost  = 10
)

var errSSEIdleTimeout = errors.New("sse idle timeout")

type ArkChatConfig struct {
	BaseURL              string
	APIKey               string
	DefaultModel         string
	DisableThinking      bool
	MaxRequestBytes      int64
	Timeouts             TimeoutConfig
	CredentialSelector   *credentials.Selector
	MaxCredentialRetries int
	DegradeDuration      time.Duration
}

type TimeoutConfig struct {
	Connect        time.Duration
	Write          time.Duration
	FirstByte      time.Duration
	NonStreamTotal time.Duration
	SSEIdle        time.Duration
}

type ArkChatProxy struct {
	cfg    ArkChatConfig
	client *http.Client
	logger *slog.Logger
}

type errorEnvelope struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

type streamCopyResult struct {
	status      int
	code        string
	wroteHeader bool
	err         error
	final       bool
}

type readResult struct {
	n   int
	err error
}

func NewArkChatProxy(cfg ArkChatConfig, logger *slog.Logger, client *http.Client) *ArkChatProxy {
	cfg = cfg.withDefaults()
	if client == nil {
		client = NewHTTPClient(cfg.Timeouts)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ArkChatProxy{
		cfg:    cfg,
		client: client,
		logger: logger,
	}
}

func NewHTTPClient(timeouts TimeoutConfig) *http.Client {
	timeouts = timeouts.withDefaults()
	dialer := &net.Dialer{
		Timeout:   timeouts.Connect,
		KeepAlive: defaultTransportKeepAlive,
	}

	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          defaultTransportMaxIdleConns,
			MaxIdleConnsPerHost:   defaultTransportMaxIdlePerHost,
			IdleConnTimeout:       defaultTransportIdleConn,
			TLSHandshakeTimeout:   timeouts.Connect,
			ExpectContinueTimeout: defaultExpectContinueTimeout,
			ResponseHeaderTimeout: timeouts.FirstByte,
			DisableCompression:    true,
		},
	}
}

func (p *ArkChatProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	stream := false
	status := http.StatusOK
	code := ""
	defer func() {
		p.logRequest(r.Context(), status, code, stream, time.Since(start))
	}()

	if r.Method != http.MethodPost {
		status = http.StatusMethodNotAllowed
		code = CodeInvalidRequest
		writeError(w, status, "method not allowed", "invalid_request", code)
		return
	}
	if !p.configured() {
		status = http.StatusServiceUnavailable
		code = CodeUpstreamNotConfigured
		writeError(w, status, "upstream is not configured", "gateway_error", code)
		return
	}

	body, streamRequest, err := p.rewriteRequest(w, r)
	stream = streamRequest
	if err != nil {
		status = http.StatusBadRequest
		code = CodeInvalidRequest
		writeError(w, status, "invalid request body", "invalid_request", code)
		return
	}

	status, code = p.doWithRetries(w, r, body, stream)
}

func (p *ArkChatProxy) configured() bool {
	if strings.TrimSpace(p.cfg.APIKey) != "" && strings.TrimSpace(p.cfg.BaseURL) != "" {
		return true
	}
	return p.cfg.CredentialSelector != nil && p.cfg.CredentialSelector.Len() > 0
}

func (p *ArkChatProxy) rewriteRequest(w http.ResponseWriter, r *http.Request) ([]byte, bool, error) {
	reader := http.MaxBytesReader(w, r.Body, p.cfg.MaxRequestBytes)
	defer reader.Close()

	decoder := json.NewDecoder(reader)
	decoder.UseNumber()

	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return nil, false, err
	}
	if payload == nil {
		return nil, false, errors.New("request body must be an object")
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, false, errors.New("request body must contain one JSON object")
	}

	payload["model"] = p.cfg.DefaultModel
	if p.cfg.DisableThinking {
		payload["thinking"] = map[string]any{"type": "disabled"}
	}

	stream, _ := payload["stream"].(bool)
	if stream {
		streamOptions, _ := payload["stream_options"].(map[string]any)
		if streamOptions == nil {
			streamOptions = map[string]any{}
		}
		streamOptions["include_usage"] = true
		payload["stream_options"] = streamOptions
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, false, err
	}
	return body, stream, nil
}

func (p *ArkChatProxy) doWithRetries(w http.ResponseWriter, r *http.Request, body []byte, stream bool) (int, string) {
	exclude := map[string]struct{}{}
	maxAttempts := p.cfg.MaxCredentialRetries + 1
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	if p.cfg.CredentialSelector == nil {
		maxAttempts = 1
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		credential, ok := p.nextCredential(r.Context(), exclude)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "no healthy upstream credential", "gateway_error", CodeUpstreamCredentialPoolEmpty)
			return http.StatusServiceUnavailable, CodeUpstreamCredentialPoolEmpty
		}
		upstreamReq, cancel, err := p.newUpstreamRequest(r, body, stream, credential)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "upstream is not configured", "gateway_error", CodeUpstreamNotConfigured)
			return http.StatusServiceUnavailable, CodeUpstreamNotConfigured
		}

		resp, err := p.client.Do(upstreamReq)
		if err != nil {
			cancel()
			code := classifyUpstreamRequestError(err)
			if shouldRetryCode(code) && credential.ID != "" && attempt+1 < maxAttempts {
				exclude[credential.ID] = struct{}{}
				p.logCredentialRetry(r.Context(), credential.ID, 0, code, attempt+1)
				continue
			}
			writeError(w, http.StatusBadGateway, "upstream request failed", "gateway_error", code)
			return http.StatusBadGateway, code
		}

		retry, retryCode := p.retryableResponse(resp, credential, attempt, maxAttempts)
		if retry {
			cancel()
			_ = resp.Body.Close()
			exclude[credential.ID] = struct{}{}
			p.logCredentialRetry(r.Context(), credential.ID, resp.StatusCode, retryCode, attempt+1)
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			cancel()
			_ = resp.Body.Close()
			writeError(w, http.StatusServiceUnavailable, "upstream rate limited", "gateway_error", CodeUpstream429)
			return http.StatusServiceUnavailable, CodeUpstream429
		}
		if resp.StatusCode >= http.StatusInternalServerError {
			cancel()
			_ = resp.Body.Close()
			writeError(w, http.StatusBadGateway, "upstream request failed", "gateway_error", CodeUpstream5xx)
			return http.StatusBadGateway, CodeUpstream5xx
		}

		if stream && isSuccessful(resp.StatusCode) {
			result := p.copyStreamingResponse(w, r.Context(), cancel, resp)
			_ = resp.Body.Close()
			if result.err != nil && !result.wroteHeader && !result.final && credential.ID != "" && attempt+1 < maxAttempts {
				exclude[credential.ID] = struct{}{}
				p.logCredentialRetry(r.Context(), credential.ID, result.status, result.code, attempt+1)
				continue
			}
			if result.err != nil && !result.wroteHeader {
				writeError(w, http.StatusBadGateway, "upstream request failed", "gateway_error", result.code)
				return http.StatusBadGateway, result.code
			}
			return result.status, result.code
		}

		copyStatus, copyCode, copyErr := copyBufferedResponse(w, resp)
		cancel()
		_ = resp.Body.Close()
		if copyErr != nil {
			writeError(w, http.StatusBadGateway, "upstream request failed", "gateway_error", copyCode)
			return http.StatusBadGateway, copyCode
		}
		return copyStatus, copyCode
	}

	writeError(w, http.StatusServiceUnavailable, "no healthy upstream credential", "gateway_error", CodeUpstreamCredentialPoolEmpty)
	return http.StatusServiceUnavailable, CodeUpstreamCredentialPoolEmpty
}

func (p *ArkChatProxy) nextCredential(ctx context.Context, exclude map[string]struct{}) (credentials.Credential, bool) {
	if p.cfg.CredentialSelector != nil {
		return p.cfg.CredentialSelector.Next(ctx, exclude)
	}
	return credentials.Credential{
		BaseURL: p.cfg.BaseURL,
		Secret:  strings.TrimSpace(p.cfg.APIKey),
	}, strings.TrimSpace(p.cfg.APIKey) != "" && strings.TrimSpace(p.cfg.BaseURL) != ""
}

func (p *ArkChatProxy) newUpstreamRequest(r *http.Request, body []byte, stream bool, credential credentials.Credential) (*http.Request, context.CancelFunc, error) {
	if credential.ID != "" {
		httpx.SetUpstreamCredentialID(r.Context(), credential.ID)
	}
	target, err := chatCompletionsURL(credential.BaseURL)
	if err != nil {
		return nil, func() {}, err
	}

	upstreamCtx := r.Context()
	cancel := func() {}
	if stream {
		upstreamCtx, cancel = context.WithCancel(r.Context())
	} else if p.cfg.Timeouts.NonStreamTotal > 0 {
		upstreamCtx, cancel = context.WithTimeout(r.Context(), p.cfg.Timeouts.NonStreamTotal)
	}

	req, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(credential.Secret))
	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	if requestID := httpx.RequestIDFromContext(r.Context()); requestID != "" {
		req.Header.Set(httpx.RequestIDHeader, requestID)
	}

	return req, cancel, nil
}

func (p *ArkChatProxy) retryableResponse(resp *http.Response, credential credentials.Credential, attempt int, maxAttempts int) (bool, string) {
	if resp.StatusCode == http.StatusTooManyRequests {
		if credential.ID != "" && p.cfg.CredentialSelector != nil {
			duration := p.cfg.DegradeDuration
			if duration <= 0 {
				duration = credentials.DefaultDegradeDuration
			}
			p.cfg.CredentialSelector.MarkDegraded(credential.ID, duration)
		}
		return credential.ID != "" && attempt+1 < maxAttempts, CodeUpstream429
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return credential.ID != "" && attempt+1 < maxAttempts, CodeUpstream5xx
	}
	return false, ""
}

func (p *ArkChatProxy) copyStreamingResponse(w http.ResponseWriter, ctx context.Context, cancel context.CancelFunc, resp *http.Response) streamCopyResult {
	result := streamCopyResult{status: resp.StatusCode}
	if !isEventStream(resp.Header.Get("Content-Type")) {
		result.status = http.StatusBadGateway
		result.code = CodeUpstreamInvalidResponse
		result.err = errors.New("upstream response is not event-stream")
		return result
	}

	buf := make([]byte, 32*1024)
	n, err := readWithIdle(ctx, cancel, resp.Body, buf, p.cfg.Timeouts.SSEIdle)
	if err != nil && n == 0 {
		result.status = http.StatusBadGateway
		result.code = classifyStreamingReadError(err)
		result.err = err
		return result
	}
	if n > 0 {
		result.final = true
	}

	copyAllowedResponseHeaders(w.Header(), resp.Header, true)
	w.WriteHeader(resp.StatusCode)
	result.wroteHeader = true

	for {
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				cancel()
				result.code = CodeUpstreamInvalidResponse
				result.err = writeErr
				return result
			}
			flush(w)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return result
			}
			cancel()
			result.code = classifyStreamingReadError(err)
			result.err = err
			return result
		}

		n, err = readWithIdle(ctx, cancel, resp.Body, buf, p.cfg.Timeouts.SSEIdle)
	}
}

func copyBufferedResponse(w http.ResponseWriter, resp *http.Response) (int, string, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return http.StatusBadGateway, CodeUpstreamInvalidResponse, err
	}

	copyAllowedResponseHeaders(w.Header(), resp.Header, false)
	w.WriteHeader(resp.StatusCode)
	if len(body) > 0 {
		if _, err := w.Write(body); err != nil {
			return resp.StatusCode, CodeUpstreamInvalidResponse, err
		}
	}
	return resp.StatusCode, "", nil
}

func readWithIdle(ctx context.Context, cancel context.CancelFunc, body io.ReadCloser, buf []byte, idle time.Duration) (int, error) {
	select {
	case <-ctx.Done():
		_ = body.Close()
		return 0, ctx.Err()
	default:
	}

	if idle <= 0 {
		return body.Read(buf)
	}

	resultc := make(chan readResult, 1)
	go func() {
		n, err := body.Read(buf)
		resultc <- readResult{n: n, err: err}
	}()

	timer := time.NewTimer(idle)
	defer timer.Stop()

	select {
	case result := <-resultc:
		return result.n, result.err
	case <-ctx.Done():
		_ = body.Close()
		return 0, ctx.Err()
	case <-timer.C:
		cancel()
		_ = body.Close()
		return 0, errSSEIdleTimeout
	}
}

func shouldRetryCode(code string) bool {
	return code == CodeUpstreamTimeout || code == CodeUpstreamConnectionFailed || code == CodeUpstreamInvalidResponse
}

func classifyUpstreamRequestError(err error) string {
	if isTimeoutError(err) {
		return CodeUpstreamTimeout
	}
	return CodeUpstreamConnectionFailed
}

func classifyStreamingReadError(err error) string {
	if errors.Is(err, io.EOF) {
		return ""
	}
	if isTimeoutError(err) {
		return CodeUpstreamTimeout
	}
	return CodeUpstreamInvalidResponse
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, errSSEIdleTimeout) || os.IsTimeout(err) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func chatCompletionsURL(baseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("upstream base URL requires scheme and host")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/chat/completions"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func copyAllowedResponseHeaders(dst http.Header, src http.Header, stream bool) {
	for key, values := range src {
		if blockedResponseHeader(key, stream) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func blockedResponseHeader(key string, stream bool) bool {
	switch http.CanonicalHeaderKey(key) {
	case "Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade",
		"Server",
		"X-Powered-By",
		"Set-Cookie",
		"Authorization",
		"Content-Length":
		return true
	}
	return false
}

func isEventStream(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		return strings.EqualFold(mediaType, "text/event-stream")
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "text/event-stream")
}

func isSuccessful(status int) bool {
	return status >= http.StatusOK && status < http.StatusMultipleChoices
}

func (p *ArkChatProxy) logCredentialRetry(ctx context.Context, credentialID string, upstreamStatus int, code string, attempt int) {
	attrs := []any{
		"request_id", httpx.RequestIDFromContext(ctx),
		"credential_id", credentialID,
		"attempt", attempt,
		"code", code,
	}
	if upstreamStatus != 0 {
		attrs = append(attrs, "upstream_status", upstreamStatus)
	}
	p.logger.Warn("upstream credential retry", attrs...)
}

func flush(w http.ResponseWriter) {
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func writeError(w http.ResponseWriter, status int, message string, typ string, code string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{
		Error: errorDetail{
			Message: message,
			Type:    typ,
			Code:    code,
		},
	})
}

func (p *ArkChatProxy) logRequest(ctx context.Context, status int, code string, stream bool, duration time.Duration) {
	attrs := []any{
		"request_id", httpx.RequestIDFromContext(ctx),
		"status", status,
		"stream", stream,
		"duration_us", duration.Microseconds(),
	}
	if code != "" {
		attrs = append(attrs, "code", code)
	}
	if subject, ok := auth.SubjectFromContext(ctx); ok {
		attrs = append(attrs,
			"user_id", subject.UserID.String(),
			"org_id", subject.OrgID.String(),
			"api_key_id", subject.APIKeyID.String(),
		)
	}
	p.logger.Info("ark chat proxy", attrs...)
}

func (cfg ArkChatConfig) withDefaults() ArkChatConfig {
	if cfg.MaxRequestBytes <= 0 {
		cfg.MaxRequestBytes = DefaultMaxRequestBytes
	}
	if cfg.MaxCredentialRetries < 0 {
		cfg.MaxCredentialRetries = 0
	} else if cfg.MaxCredentialRetries == 0 && cfg.CredentialSelector != nil {
		cfg.MaxCredentialRetries = DefaultMaxCredentialRetries
	}
	if cfg.DegradeDuration <= 0 {
		cfg.DegradeDuration = credentials.DefaultDegradeDuration
	}
	cfg.Timeouts = cfg.Timeouts.withDefaults()
	return cfg
}

func (cfg TimeoutConfig) withDefaults() TimeoutConfig {
	if cfg.Connect <= 0 {
		cfg.Connect = defaultConnectTimeout
	}
	if cfg.Write <= 0 {
		cfg.Write = defaultWriteTimeout
	}
	if cfg.FirstByte <= 0 {
		cfg.FirstByte = defaultFirstByteTimeout
	}
	if cfg.NonStreamTotal <= 0 {
		cfg.NonStreamTotal = defaultNonStreamTotalTimeout
	}
	if cfg.SSEIdle <= 0 {
		cfg.SSEIdle = defaultSSEIdleTimeout
	}
	return cfg
}
