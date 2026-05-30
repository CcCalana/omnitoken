package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type AnthropicMessagesHandler struct {
	next            http.Handler
	logger          *slog.Logger
	maxRequestBytes int64
}

type AnthropicMessagesConfig struct {
	MaxRequestBytes int64
}

func NewAnthropicMessagesHandler(next http.Handler, logger *slog.Logger, cfg AnthropicMessagesConfig) *AnthropicMessagesHandler {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.MaxRequestBytes <= 0 {
		cfg.MaxRequestBytes = DefaultMaxRequestBytes
	}
	return &AnthropicMessagesHandler{
		next:            next,
		logger:          logger,
		maxRequestBytes: cfg.MaxRequestBytes,
	}
}

func (h *AnthropicMessagesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAnthropicError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed")
		return
	}
	if h.next == nil {
		writeAnthropicError(w, http.StatusServiceUnavailable, "api_error", "upstream is not configured")
		return
	}

	reader := http.MaxBytesReader(w, r.Body, h.maxRequestBytes)
	body, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", "invalid request body")
		return
	}

	openAIBody, stream, err := anthropicToOpenAIRequest(body)
	if err != nil {
		writeAnthropicError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	req := r.Clone(r.Context())
	req.Body = io.NopCloser(bytes.NewReader(openAIBody))
	req.ContentLength = int64(len(openAIBody))
	req.Header = r.Header.Clone()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(openAIBody)))

	transform := newAnthropicResponseWriter(w, h.logger, stream)
	h.next.ServeHTTP(transform, req)
	if err := transform.finish(); err != nil {
		h.logger.Warn("anthropic response conversion failed", "err", err)
	}
}

type anthropicRequest struct {
	Model         string         `json:"model"`
	System        any            `json:"system"`
	Messages      []messageInput `json:"messages"`
	MaxTokens     any            `json:"max_tokens"`
	Stream        bool           `json:"stream"`
	Temperature   any            `json:"temperature"`
	TopP          any            `json:"top_p"`
	StopSequences any            `json:"stop_sequences"`
	Thinking      any            `json:"thinking"`
}

type messageInput struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

func anthropicToOpenAIRequest(body []byte) ([]byte, bool, error) {
	var input anthropicRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&input); err != nil {
		return nil, false, errors.New("invalid request body")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, false, errors.New("request body must contain one JSON object")
	}
	if strings.TrimSpace(input.Model) == "" {
		return nil, false, errors.New("model is required")
	}
	if len(input.Messages) == 0 {
		return nil, false, errors.New("messages must not be empty")
	}

	messages := make([]map[string]any, 0, len(input.Messages)+1)
	if system := systemContent(input.System); system != "" {
		messages = append(messages, map[string]any{
			"role":    "system",
			"content": system,
		})
	}
	for _, message := range input.Messages {
		role := strings.TrimSpace(message.Role)
		if role == "" {
			return nil, false, errors.New("message role is required")
		}
		content := messageContent(message.Content)
		messages = append(messages, map[string]any{
			"role":    role,
			"content": content,
		})
	}

	output := map[string]any{
		"model":    input.Model,
		"messages": messages,
	}
	copyIfSet(output, "max_tokens", input.MaxTokens)
	if input.Stream {
		output["stream"] = true
	}
	copyIfSet(output, "temperature", input.Temperature)
	copyIfSet(output, "top_p", input.TopP)
	copyIfSet(output, "stop", input.StopSequences)
	copyIfSet(output, "thinking", input.Thinking)

	encoded, err := json.Marshal(output)
	if err != nil {
		return nil, false, fmt.Errorf("encode request: %w", err)
	}
	return encoded, input.Stream, nil
}

func copyIfSet(output map[string]any, key string, value any) {
	if value != nil {
		output[key] = value
	}
}

func systemContent(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := blockText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func messageContent(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := blockText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func blockText(value any) string {
	block, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	if blockType, _ := block["type"].(string); blockType != "" && blockType != "text" {
		return ""
	}
	text, _ := block["text"].(string)
	return text
}

type anthropicResponseWriter struct {
	dst         http.ResponseWriter
	header      http.Header
	logger      *slog.Logger
	stream      bool
	status      int
	wroteHeader bool
	sentHeader  bool
	buffer      bytes.Buffer
	converter   *anthropicStreamConverter
}

func newAnthropicResponseWriter(dst http.ResponseWriter, logger *slog.Logger, stream bool) *anthropicResponseWriter {
	return &anthropicResponseWriter{
		dst:       dst,
		header:    make(http.Header),
		logger:    logger,
		stream:    stream,
		status:    http.StatusOK,
		converter: newAnthropicStreamConverter(logger),
	}
}

func (w *anthropicResponseWriter) Header() http.Header {
	return w.header
}

func (w *anthropicResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = status
}

func (w *anthropicResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.shouldStream() {
		if err := w.ensureStreamHeader(); err != nil {
			return 0, err
		}
		if err := w.converter.Write(p, w.dst); err != nil {
			return 0, err
		}
		flush(w.dst)
		return len(p), nil
	}
	w.buffer.Write(p)
	return len(p), nil
}

func (w *anthropicResponseWriter) Flush() {
	if w.shouldStream() {
		_ = w.ensureStreamHeader()
	}
	flush(w.dst)
}

func (w *anthropicResponseWriter) Unwrap() http.ResponseWriter {
	return w.dst
}

func (w *anthropicResponseWriter) finish() error {
	if w.shouldStream() {
		if err := w.ensureStreamHeader(); err != nil {
			return err
		}
		if err := w.converter.Flush(w.dst); err != nil {
			return err
		}
		flush(w.dst)
		return nil
	}
	if w.sentHeader {
		return nil
	}

	body, status := w.convertBufferedBody()
	copyAnthropicHeaders(w.dst.Header(), false)
	w.dst.WriteHeader(status)
	w.sentHeader = true
	if len(body) == 0 {
		return nil
	}
	_, err := w.dst.Write(body)
	return err
}

func (w *anthropicResponseWriter) shouldStream() bool {
	return w.status >= http.StatusOK && w.status < http.StatusMultipleChoices && (w.stream || isEventStream(w.header.Get("Content-Type")))
}

func (w *anthropicResponseWriter) ensureStreamHeader() error {
	if w.sentHeader {
		return nil
	}
	copyAnthropicHeaders(w.dst.Header(), true)
	w.dst.WriteHeader(w.status)
	w.sentHeader = true
	return nil
}

func (w *anthropicResponseWriter) convertBufferedBody() ([]byte, int) {
	if w.status < http.StatusOK || w.status >= http.StatusMultipleChoices {
		return encodeAnthropicError(w.status, anthropicErrorType(w.status), openAIErrorMessage(w.buffer.Bytes()))
	}
	body, err := openAIToAnthropicMessage(w.buffer.Bytes())
	if err != nil {
		return encodeAnthropicError(http.StatusBadGateway, "api_error", "invalid upstream response")
	}
	return body, w.status
}

func copyAnthropicHeaders(header http.Header, stream bool) {
	header.Del("Content-Length")
	header.Set("X-Content-Type-Options", "nosniff")
	if stream {
		header.Set("Content-Type", "text/event-stream")
		header.Set("Cache-Control", "no-cache")
		return
	}
	header.Set("Content-Type", "application/json")
}

type openAIResponse struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   *openAIUsage   `json:"usage"`
}

type openAIUsage struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
}

type openAIChoice struct {
	Index        int           `json:"index"`
	FinishReason string        `json:"finish_reason"`
	Message      openAIMessage `json:"message"`
	Delta        openAIMessage `json:"delta"`
}

type openAIMessage struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content"`
}

type anthropicMessage struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Role       string                 `json:"role"`
	Model      string                 `json:"model"`
	Content    []map[string]string    `json:"content"`
	StopReason string                 `json:"stop_reason"`
	Usage      anthropicUsageResponse `json:"usage"`
}

type anthropicUsageResponse struct {
	InputTokens          int `json:"input_tokens"`
	OutputTokens         int `json:"output_tokens"`
	CacheReadInputTokens int `json:"cache_read_input_tokens"`
}

func openAIToAnthropicMessage(body []byte) ([]byte, error) {
	var input openAIResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&input); err != nil {
		return nil, err
	}
	output := anthropicMessage{
		ID:      firstNonEmptyString(input.ID, "msg_"+fmt.Sprintf("%d", time.Now().UnixNano())),
		Type:    "message",
		Role:    "assistant",
		Model:   input.Model,
		Content: []map[string]string{},
		Usage:   anthropicUsage(input.Usage),
	}
	if len(input.Choices) > 0 {
		if len(input.Choices) > 1 {
			slog.Warn("anthropic response converter ignoring additional choices", "choice_count", len(input.Choices))
		}
		choice := input.Choices[0]
		if choice.Message.ReasoningContent != "" {
			output.Content = append(output.Content, map[string]string{"type": "thinking", "thinking": choice.Message.ReasoningContent})
		}
		output.Content = append(output.Content, map[string]string{"type": "text", "text": choice.Message.Content})
		output.StopReason = anthropicStopReason(choice.FinishReason)
	}
	if len(output.Content) == 0 {
		output.Content = append(output.Content, map[string]string{"type": "text", "text": ""})
	}
	if output.StopReason == "" {
		output.StopReason = "end_turn"
	}
	return json.Marshal(output)
}

func anthropicUsage(usage *openAIUsage) anthropicUsageResponse {
	if usage == nil {
		return anthropicUsageResponse{}
	}
	return anthropicUsageResponse{
		InputTokens:          usage.PromptTokens,
		OutputTokens:         usage.CompletionTokens,
		CacheReadInputTokens: usage.PromptTokensDetails.CachedTokens,
	}
}

func anthropicStopReason(reason string) string {
	switch reason {
	case "stop", "":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	default:
		return "end_turn"
	}
}

type anthropicStreamConverter struct {
	logger     *slog.Logger
	frame      bytes.Buffer
	messageID  string
	model      string
	started    bool
	ended      bool
	nextIndex  int
	openKind   string
	openIndex  int
	finalUsage anthropicUsageResponse
	stopReason string
	deltaSent  bool
}

func newAnthropicStreamConverter(logger *slog.Logger) *anthropicStreamConverter {
	return &anthropicStreamConverter{logger: logger, openIndex: -1}
}

func (c *anthropicStreamConverter) Write(p []byte, dst io.Writer) error {
	c.frame.Write(p)
	for {
		data := c.frame.Bytes()
		idx := bytes.Index(data, []byte("\n\n"))
		sepLen := 2
		if idx < 0 {
			idx = bytes.Index(data, []byte("\r\n\r\n"))
			sepLen = 4
		}
		if idx < 0 {
			return nil
		}
		frame := append([]byte(nil), data[:idx]...)
		c.frame.Next(idx + sepLen)
		if err := c.handleFrame(frame, dst); err != nil {
			return err
		}
	}
}

func (c *anthropicStreamConverter) Flush(dst io.Writer) error {
	if c.frame.Len() > 0 {
		if err := c.handleFrame(c.frame.Bytes(), dst); err != nil {
			return err
		}
		c.frame.Reset()
	}
	return nil
}

func (c *anthropicStreamConverter) handleFrame(frame []byte, dst io.Writer) error {
	if c.ended {
		c.warn("anthropic stream ignoring trailing frame after done")
		return nil
	}
	payload := sseDataPayload(frame)
	if strings.TrimSpace(payload) == "" {
		return nil
	}
	if strings.TrimSpace(payload) == "[DONE]" {
		if c.openKind != "" {
			if err := writeAnthropicSSE(dst, "content_block_stop", map[string]any{"type": "content_block_stop", "index": c.openIndex}); err != nil {
				return err
			}
			c.openKind = ""
			c.openIndex = -1
		}
		if !c.started {
			if err := c.emitMessageStart(dst); err != nil {
				return err
			}
		}
		if c.stopReason != "" && !c.deltaSent {
			if err := c.emitMessageDelta(dst); err != nil {
				return err
			}
		}
		if err := writeAnthropicSSE(dst, "message_stop", map[string]any{"type": "message_stop"}); err != nil {
			return err
		}
		c.ended = true
		return nil
	}

	var chunk openAIResponse
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&chunk); err != nil {
		c.warn("anthropic stream skipping malformed upstream frame")
		return nil
	}
	if chunk.ID != "" {
		c.messageID = chunk.ID
	}
	if chunk.Model != "" {
		c.model = chunk.Model
	}
	if chunk.Usage != nil {
		c.finalUsage = anthropicUsage(chunk.Usage)
		if c.stopReason != "" && !c.deltaSent {
			return c.emitMessageDelta(dst)
		}
	}
	if !c.started {
		if err := c.emitMessageStart(dst); err != nil {
			return err
		}
	}
	if len(chunk.Choices) == 0 {
		return nil
	}
	if len(chunk.Choices) > 1 {
		c.warn("anthropic stream converter ignoring additional choices")
	}
	choice := chunk.Choices[0]
	if choice.Delta.ReasoningContent != "" {
		if err := c.emitDelta(dst, "thinking", choice.Delta.ReasoningContent); err != nil {
			return err
		}
	}
	if choice.Delta.Content != "" {
		if err := c.emitDelta(dst, "text", choice.Delta.Content); err != nil {
			return err
		}
	}
	if choice.FinishReason != "" {
		if c.openKind != "" {
			if err := writeAnthropicSSE(dst, "content_block_stop", map[string]any{"type": "content_block_stop", "index": c.openIndex}); err != nil {
				return err
			}
			c.openKind = ""
			c.openIndex = -1
		}
		c.stopReason = anthropicStopReason(choice.FinishReason)
		if c.finalUsage.OutputTokens > 0 {
			return c.emitMessageDelta(dst)
		}
	}
	return nil
}

func (c *anthropicStreamConverter) emitMessageStart(dst io.Writer) error {
	c.started = true
	return writeAnthropicSSE(dst, "message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            firstNonEmptyString(c.messageID, "msg_"+fmt.Sprintf("%d", time.Now().UnixNano())),
			"type":          "message",
			"role":          "assistant",
			"model":         c.model,
			"content":       []any{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]int{
				"input_tokens": c.finalUsage.InputTokens,
			},
		},
	})
}

func (c *anthropicStreamConverter) emitDelta(dst io.Writer, kind string, text string) error {
	if c.openKind != kind {
		if c.openKind != "" {
			if err := writeAnthropicSSE(dst, "content_block_stop", map[string]any{"type": "content_block_stop", "index": c.openIndex}); err != nil {
				return err
			}
		}
		c.openKind = kind
		c.openIndex = c.nextIndex
		c.nextIndex++
		block := map[string]any{"type": kind}
		if kind == "text" {
			block["text"] = ""
		} else {
			block["thinking"] = ""
		}
		if err := writeAnthropicSSE(dst, "content_block_start", map[string]any{"type": "content_block_start", "index": c.openIndex, "content_block": block}); err != nil {
			return err
		}
	}
	delta := map[string]any{}
	if kind == "text" {
		delta["type"] = "text_delta"
		delta["text"] = text
	} else {
		delta["type"] = "thinking_delta"
		delta["thinking"] = text
	}
	return writeAnthropicSSE(dst, "content_block_delta", map[string]any{"type": "content_block_delta", "index": c.openIndex, "delta": delta})
}

func (c *anthropicStreamConverter) emitMessageDelta(dst io.Writer) error {
	c.deltaSent = true
	return writeAnthropicSSE(dst, "message_delta", map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason": c.stopReason,
		},
		"usage": map[string]int{
			"output_tokens": c.finalUsage.OutputTokens,
		},
	})
}

func (c *anthropicStreamConverter) warn(message string) {
	if c.logger != nil {
		c.logger.Warn(message)
	}
}

func sseDataPayload(frame []byte) string {
	lines := []string{}
	for _, raw := range strings.Split(string(frame), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if strings.HasPrefix(line, "data:") {
			lines = append(lines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	return strings.Join(lines, "\n")
}

func writeAnthropicSSE(dst io.Writer, event string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(dst, "event: %s\ndata: %s\n\n", event, body)
	return err
}

func writeAnthropicError(w http.ResponseWriter, status int, errorType string, message string) {
	body, status := encodeAnthropicError(status, errorType, message)
	copyAnthropicHeaders(w.Header(), false)
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func encodeAnthropicError(status int, errorType string, message string) ([]byte, int) {
	if strings.TrimSpace(message) == "" {
		message = http.StatusText(status)
	}
	body, _ := json.Marshal(map[string]any{
		"type": "error",
		"error": map[string]string{
			"type":    errorType,
			"message": message,
		},
	})
	return body, status
}

func anthropicErrorType(status int) string {
	switch status {
	case http.StatusBadRequest, http.StatusMethodNotAllowed:
		return "invalid_request_error"
	case http.StatusUnauthorized, http.StatusForbidden:
		return "authentication_error"
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return "rate_limit_error"
	default:
		if status >= http.StatusInternalServerError {
			return "api_error"
		}
		return "api_error"
	}
}

func openAIErrorMessage(body []byte) string {
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && strings.TrimSpace(payload.Error.Message) != "" {
		return payload.Error.Message
	}
	return "upstream request failed"
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
