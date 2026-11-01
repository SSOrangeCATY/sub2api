package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestFallbackCandidatesVisibleOnlyToAdmin(t *testing.T) {
	group := &service.Group{ID: 7, GroupType: "fallback", FallbackGroupIDs: []int64{2, 3}}
	for name, mapped := range map[string]any{
		"user":    GroupFromService(group),
		"shallow": GroupFromServiceShallow(group),
	} {
		t.Run(name, func(t *testing.T) {
			fields := marshalToMap(t, mapped)
			require.NotContains(t, fields, "fallback_group_ids")
			require.Equal(t, "fallback", fields["group_type"])
		})
	}
	admin := marshalToMap(t, GroupFromServiceAdmin(group))
	require.Equal(t, []any{float64(2), float64(3)}, admin["fallback_group_ids"])
}
