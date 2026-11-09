package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestIsCyberPolicyError(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{"code", `{"error":{"code":"cyber_policy"}}`, true},
		{"type", `{"error":{"type":"cyber_policy","message":"blocked"}}`, true},
		{"nested failed", `{"type":"response.failed","response":{"error":{"type":"Cyber_Policy"}}}`, true},
		{"bare", `{"type":"cyber_policy","message":"blocked"}`, true},
		{"message", `{"error":{"type":"invalid_request_error","message":"This content was flagged for possible cybersecurity risk. Please try another prompt."}}`, true},
		{"plain text", "This content was flagged for possible cybersecurity risk", true},
		{"detail", `{"detail":"This content was flagged for possible cybersecurity risk"}`, true},
		{"ordinary cyber", `{"error":{"type":"api_error","message":"cyber upstream connection failed"}}`, false},
		{"ordinary plain cyber", "cyber server unavailable", false},
		{"content policy", `{"error":{"code":"content_policy","message":"blocked"}}`, false},
		{"similar code", `{"error":{"type":"cyber_policy_warning"}}`, false},
		{"echoed request", `{"error":{"message":"invalid input"},"request":{"message":"This content was flagged for possible cybersecurity risk","type":"cyber_policy"}}`, false},
		{"successful output", `{"output":[{"content":[{"text":"This content was flagged for possible cybersecurity risk"}]}]}`, false},
		{"empty", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) { require.Equal(t, tc.want, IsCyberPolicyError([]byte(tc.body))) })
	}
}

type cyberPolicyAccountTestRepo struct {
	AccountRepository
	mu          sync.Mutex
	counts      map[int64]int64
	accounts    map[int64]*Account
	fail        bool
	sawCanceled bool
}

func (r *cyberPolicyAccountTestRepo) RecordCyberPolicyError(ctx context.Context, id int64, threshold int) (int64, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sawCanceled = ctx.Err() != nil
	if r.fail {
		return 0, false, errors.New("database unavailable")
	}
	r.counts[id]++
	disabled := threshold > 0 && r.counts[id] >= int64(threshold)
	if a := r.accounts[id]; a != nil && disabled {
		a.Status, a.Schedulable = StatusError, false
	}
	return r.counts[id], disabled, nil
}

func TestCyberPolicyAccountObservation_CountsPerAccountAndDisablesAtConfiguredThreshold(t *testing.T) {
	cfg := &config.Config{Gateway: config.GatewayConfig{CyberPolicyDisableThreshold: 10}}
	a := &Account{ID: 1, Status: StatusActive, Schedulable: true}
	b := &Account{ID: 2, Status: StatusActive, Schedulable: true}
	repo := &cyberPolicyAccountTestRepo{counts: map[int64]int64{}, accounts: map[int64]*Account{1: a, 2: b}}
	for i := 0; i < 9; i++ {
		observation := NewCyberPolicyAccountObservation(context.Background(), repo, cfg, 1)
		observation.ObserveError([]byte(`{"error":{"type":"cyber_policy"}}`))
		observation.ObserveEvent([]byte(`{"type":"response.failed","response":{"error":{"code":"cyber_policy"}}}`))
	}
	require.EqualValues(t, 9, repo.counts[1], "paired failure events count once per attempt")
	require.True(t, a.IsSchedulable())
	NewCyberPolicyAccountObservation(context.Background(), repo, cfg, 2).ObserveError([]byte(`{"error":{"code":"cyber_policy"}}`))
	require.EqualValues(t, 1, repo.counts[2])
	NewCyberPolicyAccountObservation(context.Background(), repo, cfg, 1).ObserveError([]byte(`{"error":{"message":"This content was flagged for possible cybersecurity risk"}}`))
	require.EqualValues(t, 10, repo.counts[1])
	require.False(t, a.IsSchedulable())
	require.Equal(t, StatusError, a.Status)
	require.True(t, b.IsSchedulable())
}

func TestCyberPolicyAccountObservation_IgnoresOrdinaryErrorsAndSuccessfulEvents(t *testing.T) {
	repo := &cyberPolicyAccountTestRepo{counts: map[int64]int64{}}
	o := NewCyberPolicyAccountObservation(context.Background(), repo, &config.Config{}, 1)
	o.ObserveError([]byte(`{"error":{"message":"cyber API timed out"}}`))
	o.ObserveEvent([]byte(`{"type":"response.completed","response":{"error":{"code":"cyber_policy"}}}`))
	o.ObserveEvent([]byte(`{"type":"response.done","response":{"status":"completed","error":{"code":"cyber_policy"}}}`))
	o.ObserveEvent([]byte(`{"type":"response.output_text.delta","delta":"This content was flagged for possible cybersecurity risk"}`))
	require.Empty(t, repo.counts)
}

func TestCyberPolicyAccountObservation_FailedDoneEvent(t *testing.T) {
	repo := &cyberPolicyAccountTestRepo{counts: map[int64]int64{}}
	o := NewCyberPolicyAccountObservation(t.Context(), repo, &config.Config{}, 1)
	o.ObserveEvent([]byte(`{"type":"response.done","response":{"status":"failed","error":{"code":"cyber_policy"}}}`))
	require.EqualValues(t, 1, repo.counts[1])
}

func TestCyberPolicyAccountObservation_ConcurrentAttemptsAndCanceledClient(t *testing.T) {
	repo := &cyberPolicyAccountTestRepo{counts: map[int64]int64{}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			NewCyberPolicyAccountObservation(ctx, repo, &config.Config{}, 1).ObserveError([]byte(`{"error":{"code":"cyber_policy"}}`))
		}()
	}
	wg.Wait()
	require.EqualValues(t, 40, repo.counts[1])
	require.False(t, repo.sawCanceled)
}

func TestCyberPolicyAccountObservation_MissingOptionalDependenciesAndStorageFailure(t *testing.T) {
	NewCyberPolicyAccountObservation(context.Background(), nil, &config.Config{}, 1).ObserveError(nil)
	repo := &cyberPolicyAccountTestRepo{counts: map[int64]int64{}, fail: true}
	NewCyberPolicyAccountObservation(context.Background(), repo, &config.Config{}, 1).ObserveError([]byte(`{"error":{"code":"cyber_policy"}}`))
	require.Empty(t, repo.counts)
}

func TestDuplicateAccountDoesNotCopyCyberPolicyCount(t *testing.T) {
	extra, err := duplicateAccountExtra(map[string]any{CyberPolicyErrorCountExtraKey: 9, "configuration": "keep"})
	require.NoError(t, err)
	require.NotContains(t, extra, CyberPolicyErrorCountExtraKey)
	require.Equal(t, "keep", extra["configuration"])
}
