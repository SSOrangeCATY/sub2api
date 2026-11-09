package repository

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ProvideHTTPUpstream adds passive account observation to the shared upstream
// client. The constructor remains usable without persistence in transport tests.
func ProvideHTTPUpstream(cfg *config.Config, accounts service.AccountRepository) service.HTTPUpstream {
	upstream := NewHTTPUpstream(cfg).(*httpUpstreamService)
	upstream.accounts = accounts
	return upstream
}

func (s *httpUpstreamService) observeCyberPolicyResponse(ctx context.Context, accountID int64, resp *http.Response) {
	observation := service.NewCyberPolicyAccountObservation(ctx, s.accounts, s.cfg, accountID)
	if observation == nil || resp == nil || resp.Body == nil {
		return
	}
	resp.Body = &cyberPolicyResponseBody{
		ReadCloser:  resp.Body,
		observation: observation,
		errorStatus: resp.StatusCode >= http.StatusBadRequest,
		// Error responses may retain an SSE header while returning JSON or
		// plain text. Determine their format from the bytes already read.
		sse: resp.StatusCode < http.StatusBadRequest && strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream"),
	}
}

// The observer only inspects bytes the existing consumer reads. It never drains,
// rewrites or replaces a response, and passes through read/close errors exactly.
// Buffering is bounded per JSON response or SSE event, including very long lines.
const cyberPolicyObservationMaxBytes = 1 << 20

type cyberPolicyResponseBody struct {
	io.ReadCloser
	observation   *service.CyberPolicyAccountObservation
	errorStatus   bool
	sse           bool
	formatDecided bool
	mu            sync.Mutex
	buffer        []byte
	line          []byte
	overflow      bool
	lineOverflow  bool
	closeOnce     sync.Once
	closeErr      error
}

func (b *cyberPolicyResponseBody) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n, err := b.ReadCloser.Read(p)
	if b.sse {
		b.observeSSE(p[:n])
	} else if !b.overflow {
		b.buffer = appendCyberPolicyBytes(b.buffer, p[:n], &b.overflow)
		if !b.formatDecided && !b.overflow {
			prefix := bytes.TrimLeft(b.buffer, " \t\r\n")
			partial := len(prefix) == 0
			for _, marker := range [][]byte{[]byte("data:"), []byte("event:"), []byte(":"), []byte("id:"), []byte("retry:")} {
				if bytes.HasPrefix(prefix, marker) {
					b.sse = true
					buffered := b.buffer
					b.buffer = nil
					b.observeSSE(buffered)
					break
				}
				partial = partial || bytes.HasPrefix(marker, prefix)
			}
			b.formatDecided = !partial
		}
	}
	if err == io.EOF {
		b.finish()
	}
	return n, err
}

func appendCyberPolicyBytes(dst, data []byte, overflow *bool) []byte {
	if *overflow || len(dst)+len(data) > cyberPolicyObservationMaxBytes {
		*overflow = true
		return nil
	}
	return append(dst, data...)
}

func (b *cyberPolicyResponseBody) observeSSE(data []byte) {
	for len(data) > 0 {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			b.line = appendCyberPolicyBytes(b.line, data, &b.lineOverflow)
			return
		}
		b.line = appendCyberPolicyBytes(b.line, data[:i], &b.lineOverflow)
		line := bytes.TrimSuffix(b.line, []byte{'\r'})
		if b.lineOverflow {
			b.overflow = true
		} else if len(line) == 0 {
			b.flushEvent()
		} else if bytes.HasPrefix(line, []byte("data:")) {
			value := bytes.TrimPrefix(line[5:], []byte{' '})
			b.buffer = appendCyberPolicyBytes(b.buffer, value, &b.overflow)
			b.buffer = appendCyberPolicyBytes(b.buffer, []byte{'\n'}, &b.overflow)
		}
		b.line, b.lineOverflow = nil, false
		data = data[i+1:]
	}
}

func (b *cyberPolicyResponseBody) flushEvent() {
	if !b.overflow {
		b.observation.ObserveEvent(b.buffer)
	}
	b.buffer, b.overflow = nil, false
}

func (b *cyberPolicyResponseBody) finish() {
	if b.sse {
		if len(b.line) > 0 || b.lineOverflow {
			b.observeSSE([]byte{'\n'})
		}
		b.flushEvent()
	} else if !b.overflow {
		if b.errorStatus {
			b.observation.ObserveError(b.buffer)
		} else {
			b.observation.ObserveEvent(b.buffer)
		}
		b.buffer = nil
	}
}

func (b *cyberPolicyResponseBody) Close() error {
	b.closeOnce.Do(func() {
		// Close the actual transport first so concurrent blocked reads can exit.
		b.closeErr = b.ReadCloser.Close()
		b.mu.Lock()
		defer b.mu.Unlock()
		b.finish()
	})
	return b.closeErr
}
