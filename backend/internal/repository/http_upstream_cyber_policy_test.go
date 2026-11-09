package repository

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type passiveCyberTestRepo struct {
	service.AccountRepository
	counts    map[int64]int64
	threshold int
	fail      bool
}

func (r *passiveCyberTestRepo) RecordCyberPolicyError(_ context.Context, id int64, threshold int) (int64, bool, error) {
	if r.fail {
		return 0, false, errors.New("database offline")
	}
	r.threshold = threshold
	r.counts[id]++
	return r.counts[id], threshold > 0 && r.counts[id] >= int64(threshold), nil
}

func TestHTTPUpstreamPassiveCyberPolicyPreservesResponseAndCountsEachAttempt(t *testing.T) {
	body := `{"error":{"type":"cyber_policy","message":"blocked"}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Upstream-Test", "untouched")
		w.WriteHeader(400)
		_, _ = io.WriteString(w, body)
	}))
	defer server.Close()
	repo := &passiveCyberTestRepo{counts: map[int64]int64{}}
	cfg := &config.Config{Gateway: config.GatewayConfig{CyberPolicyDisableThreshold: 10}}
	upstream := ProvideHTTPUpstream(cfg, repo)
	for _, accountID := range []int64{11, 11, 12} {
		req, err := http.NewRequestWithContext(t.Context(), "POST", server.URL, nil)
		require.NoError(t, err)
		resp, err := upstream.Do(req, "", accountID, 1)
		require.NoError(t, err)
		got, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, body, string(got))
		require.Equal(t, 400, resp.StatusCode)
		require.Equal(t, "untouched", resp.Header.Get("X-Upstream-Test"))
	}
	require.Equal(t, map[int64]int64{11: 2, 12: 1}, repo.counts)
	require.Equal(t, 10, repo.threshold)
}

func TestPassiveCyberPolicyResponseBodyShapesAndChunkBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		sse        bool
		count      int64
	}{
		{"bare SSE error", "data: {\"error\":{\"type\":\"cyber_policy\"}}\n\n", 200, true, 1},
		{"SSE without content type", "data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"code\":\"cyber_policy\"}}}\n\n", 200, false, 1},
		{"SSE without content type and comment", ": keepalive\r\nevent: error\r\ndata: {\"error\":{\"type\":\"cyber_policy\"}}\r\n\r\n", 200, false, 1},
		{"ordinary error", `{"error":{"message":"cyber connection timeout"}}`, 500, false, 0},
		{"JSON error with SSE header", `{"error":{"code":"cyber_policy"}}`, 400, true, 1},
		{"plain error with SSE header", "This content was flagged for possible cybersecurity risk", 400, true, 1},
		{"SSE error status", "data: {\"error\":{\"code\":\"cyber_policy\"}}\n\n", 400, true, 1},
		{"success echo", `{"message":"This content was flagged for possible cybersecurity risk"}`, 200, false, 0},
		{"non-stream failed", `{"status":"failed","error":{"type":"cyber_policy"}}`, 200, false, 1},
		{"SSE paired events", "data: {\"type\":\"error\",\"error\":{\"type\":\"cyber_policy\"}}\r\n\r\ndata: {\"type\":\"response.failed\",\"response\":{\"error\":{\"code\":\"cyber_policy\"}}}\n\n", 200, true, 1},
		{"SSE full phrase", "data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"message\":\"This content was flagged for possible cybersecurity risk\"}}}\n\n", 200, true, 1},
		{"SSE success text", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"This content was flagged for possible cybersecurity risk\"}\n\n", 200, true, 0},
		{"SSE multiline data", "data: {\"type\":\"error\",\n" + "data: \"error\":{\"code\":\"cyber_policy\"}}\n\n", 200, true, 1},
		{"SSE oversize then failure", "data: " + strings.Repeat("x", cyberPolicyObservationMaxBytes+1) + "\n\ndata: {\"type\":\"error\",\"error\":{\"code\":\"cyber_policy\"}}\n\n", 200, true, 1},
		{"SSE terminal without blank line", "data: {\"type\":\"error\",\"error\":{\"code\":\"cyber_policy\"}}", 200, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &passiveCyberTestRepo{counts: map[int64]int64{}}
			resp := &http.Response{StatusCode: tc.status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tc.body))}
			if tc.sse {
				resp.Header.Set("Content-Type", "text/event-stream")
			}
			upstream := &httpUpstreamService{cfg: &config.Config{}, accounts: repo}
			upstream.observeCyberPolicyResponse(t.Context(), 1, resp)
			b := resp.Body.(*cyberPolicyResponseBody)
			var got strings.Builder
			// Split JSON fields, CRLF and event boundaries across reads.
			buffer := make([]byte, 17)
			for {
				n, err := b.Read(buffer)
				got.Write(buffer[:n])
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
			}
			require.NoError(t, b.Close())
			require.Equal(t, tc.body, got.String())
			require.Equal(t, tc.count, repo.counts[1])
			require.LessOrEqual(t, len(b.buffer)+len(b.line), 2*cyberPolicyObservationMaxBytes)
		})
	}
}

type cyberPolicyErrorBody struct{ readErr, closeErr error }

func (b cyberPolicyErrorBody) Read([]byte) (int, error) { return 0, b.readErr }
func (b cyberPolicyErrorBody) Close() error             { return b.closeErr }

func TestPassiveCyberPolicyResponseBodyPreservesIOErrorsAndStorageFailure(t *testing.T) {
	readErr, closeErr := errors.New("upstream read failed"), errors.New("upstream close failed")
	b := &cyberPolicyResponseBody{ReadCloser: cyberPolicyErrorBody{readErr, closeErr}}
	_, err := b.Read(make([]byte, 1))
	require.ErrorIs(t, err, readErr)
	require.ErrorIs(t, b.Close(), closeErr)
	repo := &passiveCyberTestRepo{fail: true}
	body := `{"error":{"code":"cyber_policy"}}`
	b = &cyberPolicyResponseBody{ReadCloser: io.NopCloser(strings.NewReader(body)), observation: service.NewCyberPolicyAccountObservation(t.Context(), repo, &config.Config{}, 1), errorStatus: true}
	got, err := io.ReadAll(b)
	require.NoError(t, err)
	require.Equal(t, body, string(got))
	require.NoError(t, b.Close())
}

func TestPassiveCyberPolicyTruncatedEchoDoesNotCount(t *testing.T) {
	repo := &passiveCyberTestRepo{counts: map[int64]int64{}}
	prefix := `{"error":{"message":"invalid input"},"request":{"message":"This content was flagged for possible cybersecurity risk"},`
	body := prefix + `"padding":"unread"}`
	b := &cyberPolicyResponseBody{
		ReadCloser: io.NopCloser(strings.NewReader(body)), errorStatus: true,
		observation: service.NewCyberPolicyAccountObservation(t.Context(), repo, &config.Config{}, 1),
	}
	got := make([]byte, len(prefix))
	_, err := io.ReadFull(b, got)
	require.NoError(t, err)
	require.Equal(t, prefix, string(got))
	require.NoError(t, b.Close())
	require.Empty(t, repo.counts)
}

type closingCyberPolicyBody struct {
	started chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func (b *closingCyberPolicyBody) Read(p []byte) (int, error) {
	close(b.started)
	<-b.closed
	return copy(p, `{"error":{"code":"cyber_policy"}}`), nil
}

func (b *closingCyberPolicyBody) Close() error {
	b.once.Do(func() { close(b.closed) })
	return nil
}

func TestPassiveCyberPolicyCloseUnblocksAndFinishesConcurrentRead(t *testing.T) {
	repo := &passiveCyberTestRepo{counts: map[int64]int64{}}
	inner := &closingCyberPolicyBody{started: make(chan struct{}), closed: make(chan struct{})}
	b := &cyberPolicyResponseBody{
		ReadCloser: inner, errorStatus: true,
		observation: service.NewCyberPolicyAccountObservation(t.Context(), repo, &config.Config{}, 1),
	}
	done := make(chan struct{})
	go func() { defer close(done); _, _ = b.Read(make([]byte, 128)) }()
	<-inner.started
	require.NoError(t, b.Close())
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close did not release the pending Read")
	}
	require.EqualValues(t, 1, repo.counts[1])
}
