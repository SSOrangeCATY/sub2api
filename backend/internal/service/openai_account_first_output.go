package service

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/tidwall/gjson"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Opt-in per-account budget; unrelated accounts and endpoints keep their behavior.
const openAIAccountFirstOutputTimeoutKey = "openai_first_output_timeout_seconds"

var errOpenAIAccountFirstOutputTimeout = errors.New("account first output timeout")

func openAIAccountFirstOutputTimeout(request *http.Request, account *Account) time.Duration {
	if request == nil || request.URL == nil || account == nil || !account.IsOpenAIApiKey() || request.Method != http.MethodPost {
		return 0
	}
	path := strings.TrimRight(request.URL.Path, "/")
	if !strings.HasSuffix(path, "/chat/completions") && !strings.HasSuffix(path, "/responses") {
		return 0
	}
	seconds, err := strconv.ParseFloat(fmt.Sprint(account.Extra[openAIAccountFirstOutputTimeoutKey]), 64)
	if err != nil || !(seconds >= 1 && seconds <= 300) {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

// Stage the entire pre-output phase before returning to a downstream handler.
// This prevents heartbeats/metadata from committing a hung account's response.
func doOpenAIWithAccountFirstOutputTimeout(request *http.Request, timeout time.Duration, send func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	if timeout <= 0 {
		return send(request)
	}
	ctx, cancel := context.WithCancelCause(request.Context())
	timer := time.AfterFunc(timeout, func() { cancel(errOpenAIAccountFirstOutputTimeout) })
	stop := func() error {
		if !timer.Stop() {
			<-ctx.Done()
		}
		if errors.Is(context.Cause(ctx), errOpenAIAccountFirstOutputTimeout) {
			return fmt.Errorf("%w after %s", errOpenAIAccountFirstOutputTimeout, timeout)
		}
		return nil
	}
	response, err := send(request.WithContext(ctx))
	if err == nil && response != nil && response.Body != nil {
		var prefix []byte
		prefix, err = stageOpenAIAccountFirstOutput(response)
		if err == nil {
			response.Body = &openAIRequestContextReadCloser{
				ReadCloser: &openAIAccountPrefixedBody{Reader: io.MultiReader(bytes.NewReader(prefix), response.Body), body: response.Body},
				cleanup:    func() { cancel(nil) },
			}
		}
	}
	if timeoutErr := stop(); timeoutErr != nil {
		err = timeoutErr
	}
	if err != nil {
		cancel(nil)
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		return nil, err
	}
	if response == nil || response.Body == nil {
		cancel(nil)
		return nil, errors.New("upstream returned no response body")
	}
	return response, nil
}

type openAIAccountPrefixedBody struct {
	io.Reader
	body io.ReadCloser
}

func (b *openAIAccountPrefixedBody) Close() error { return b.body.Close() }

// Keep exact wire bytes, including CRLF and multiline SSE data. Replay all
// bytes read ahead by bufio. Enforce a bounded prefix even for endless lines.
func stageOpenAIAccountFirstOutput(response *http.Response) ([]byte, error) {
	const maxBytes = openAIFirstOutputStageMaxBytes
	if !strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "text/event-stream") {
		body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
		if len(body) > maxBytes {
			return nil, errOpenAIFirstOutputStageLimit
		}
		return body, err
	}
	reader := bufio.NewReader(response.Body)
	var prefix, event, line bytes.Buffer
	for {
		part, err := reader.ReadSlice('\n')
		if prefix.Len()+len(part)+reader.Buffered() > maxBytes {
			return nil, errOpenAIFirstOutputStageLimit
		}
		_, _ = prefix.Write(part)
		_, _ = line.Write(part)
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		text := strings.TrimRight(line.String(), "\r\n")
		line.Reset()
		if strings.HasPrefix(text, "data:") {
			if event.Len() > 0 {
				_ = event.WriteByte('\n')
			}
			_, _ = event.WriteString(strings.TrimPrefix(strings.TrimPrefix(text, "data:"), " "))
		}
		if text == "" || err != nil {
			if openAIAccountEventCommitsAttempt(event.String()) {
				if reader.Buffered() > 0 {
					buffered, _ := reader.Peek(reader.Buffered())
					_, _ = prefix.Write(buffered)
				}
				return prefix.Bytes(), nil
			}
			event.Reset()
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}
	}
}

func openAIAccountEventCommitsAttempt(payload string) bool {
	payload = strings.TrimSpace(payload)
	if payload == "[DONE]" {
		return true
	}
	if !gjson.Valid(payload) {
		return false
	}
	root := gjson.Parse(payload)
	if root.Get("error").Exists() {
		return true
	}
	if eventType := root.Get("type").String(); eventType != "" {
		// Tool starts commit an attempt even before text, avoiding unsafe replay.
		if eventType == "response.output_item.added" {
			switch root.Get("item.type").String() {
			case "function_call", "custom_tool_call":
				return true
			}
		}
		return openAIStreamDataStartsClientOutput(payload, eventType) ||
			eventType == "response.completed" || eventType == "response.done" ||
			eventType == "response.failed" || eventType == "error"
	}
	for _, choice := range root.Get("choices").Array() {
		delta := choice.Get("delta")
		if choice.Get("finish_reason").Type != gjson.Null ||
			delta.Get("content").String() != "" || delta.Get("reasoning_content").String() != "" ||
			delta.Get("reasoning").String() != "" || delta.Get("refusal").String() != "" ||
			len(delta.Get("tool_calls").Array()) > 0 || delta.Get("function_call").Exists() ||
			choice.Get("message").Exists() {
			return true
		}
	}
	return false
}
