//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type plazaDiscoveryAccounts struct {
	AccountRepository
	accounts []Account
	err      error
	calls    int
}

func (r *plazaDiscoveryAccounts) ListModelAvailabilityCandidates(_ context.Context, groupID *int64, platforms []string, includeGrouped bool) ([]Account, error) {
	r.calls++
	if groupID != nil || !includeGrouped || len(platforms) == 0 {
		panic("catalog must read one global persistent snapshot")
	}
	return r.accounts, r.err
}

func TestListPlazaGroups_DiscoversDirectAccountsWithoutChannelOrPriceEdits(t *testing.T) {
	groups := []Group{{ID: 30, Platform: PlatformOpenAI}, {ID: 12, Platform: PlatformGemini}, {ID: 99, Platform: PlatformOpenAI}}
	s := newPlazaService(nil, groups, nil)
	repo := &plazaDiscoveryAccounts{accounts: []Account{
		{Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, GroupIDs: []int64{30}, Credentials: map[string]any{"model_mapping": map[string]any{"deepseek-flash": "private-upstream-id", "deepseek-v4-pro": "private-pro", "hidden-*": "private-wildcard", "disabled-alias": ""}}},
		{Platform: PlatformGemini, Status: StatusActive, Schedulable: true, GroupIDs: []int64{12, 30}, Credentials: map[string]any{"model_mapping": map[string]any{"gemini-configured": "private-target"}}},
		{Platform: PlatformOpenAI, Status: StatusActive, Schedulable: false, GroupIDs: []int64{30}, Credentials: map[string]any{"model_mapping": map[string]any{"disabled-account-model": "private"}}},
	}}
	s.accountRepo = repo
	out, err := s.ListGroups(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, repo.calls)
	byID := map[int64]PlazaGroup{}
	for _, g := range out {
		byID[g.ID] = g
	}
	require.Len(t, byID, 3)
	require.Equal(t, []string{"deepseek-flash", "deepseek-v4-pro"}, []string{byID[30].Models[0].Name, byID[30].Models[1].Name})
	require.Len(t, byID[30].Models, 2)
	require.Len(t, byID[12].Models, 1)
	require.Equal(t, "gemini-configured", byID[12].Models[0].Name)
	require.Empty(t, byID[99].Models, "empty groups remain selectable with an honest empty state")
	require.Nil(t, byID[30].Models[0].Pricing, "enumeration must not fabricate prices")
	limited, err := s.ListGroups(context.Background(), 30)
	require.NoError(t, err)
	require.Len(t, limited, 1)
	require.Equal(t, int64(30), limited[0].ID)
}

func TestListPlazaGroups_RestrictedChannelCatalogRemainsAuthoritative(t *testing.T) {
	channel := plazaPricedChannel(1, "configured", []int64{1}, PlatformOpenAI, "allowed-model")
	channel.RestrictModels = true
	s := newPlazaService([]Channel{channel}, []Group{{ID: 1, Platform: PlatformOpenAI}}, nil)
	repo := &plazaDiscoveryAccounts{err: errors.New("must not query")}
	s.accountRepo = repo
	out, err := s.ListGroups(context.Background())
	require.NoError(t, err)
	require.Zero(t, repo.calls)
	require.Len(t, out[0].Models, 1)
	require.Equal(t, "allowed-model", out[0].Models[0].Name)
}

func TestListPlazaGroups_NativeDiscoveryErrorDoesNotSilentlyOmitGroups(t *testing.T) {
	s := newPlazaService(nil, []Group{{ID: 1, Platform: PlatformOpenAI}}, nil)
	sentinel := errors.New("candidate snapshot failed")
	s.accountRepo = &plazaDiscoveryAccounts{err: sentinel}
	_, err := s.ListGroups(context.Background())
	require.ErrorIs(t, err, sentinel)
}

func TestListPlazaGroups_NativeMixedAnthropicAndContextSuffix(t *testing.T) {
	s := newPlazaService(nil, []Group{{ID: 1, Platform: PlatformAnthropic}, {ID: 2, Platform: PlatformOpenAI}, {ID: 3, Platform: PlatformAnthropic}}, nil)
	s.accountRepo = &plazaDiscoveryAccounts{accounts: []Account{
		{Platform: PlatformAntigravity, Status: StatusActive, Schedulable: true, GroupIDs: []int64{1}, Extra: map[string]any{"mixed_scheduling": true}, Credentials: map[string]any{"model_mapping": map[string]any{"claude-sonnet-4-5": "claude-sonnet-4-5", "public-sonnet": "claude-sonnet-4-5", "gemini-3-flash": "gemini-3-flash"}}},
		{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, GroupIDs: []int64{2}, Credentials: map[string]any{"model_mapping": map[string]any{"deepseek-flash[1m]": "deepseek-flash"}}},
		{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Status: StatusActive, Schedulable: true, GroupIDs: []int64{3}, Credentials: map[string]any{"aws_region": "us-east-1"}},
	}}
	out, err := s.ListGroups(context.Background())
	require.NoError(t, err)
	byID := map[int64]PlazaGroup{}
	for _, g := range out {
		byID[g.ID] = g
	}
	require.NotEmpty(t, byID[1].Models)
	for _, m := range byID[1].Models {
		require.Equal(t, PlatformAnthropic, m.Platform)
		require.NotEqual(t, "gemini-3-flash", m.Name)
	}
	require.Contains(t, plazaModelsByName(byID[1].Models), "public-sonnet")
	require.Len(t, byID[2].Models, 1)
	require.Equal(t, "deepseek-flash[1m]", byID[2].Models[0].Name)
	require.NotEmpty(t, byID[3].Models)
	for _, m := range byID[3].Models {
		_, supported := ResolveBedrockModelID(&Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Credentials: map[string]any{"aws_region": "us-east-1"}}, m.Name)
		require.True(t, supported)
	}
}
