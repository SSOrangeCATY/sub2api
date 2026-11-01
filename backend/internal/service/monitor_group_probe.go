package service

import (
	"context"
	"errors"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// A local fallback key would probe the replacement and falsely mark its primary
// healthy. Reject this configuration both on writes and before existing probes.
func (s *ChannelMonitorService) validateIndependentProbe(ctx context.Context, plainKey, groupName string) error {
	if s.localKeys == nil || plainKey == "" {
		return nil
	}
	key, err := s.localKeys.GetByKey(ctx, plainKey)
	if errors.Is(err, ErrAPIKeyNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if key == nil || key.Group == nil {
		return nil
	}
	if key.Group.GroupType == GroupTypeFallback || (groupName != "" && key.Group.Name != groupName) {
		return infraerrors.BadRequest("MONITOR_PROBE_ROUTE_MISMATCH", "probe a business group directly with its own API key; fallback entry keys cannot be used for independent monitoring")
	}
	return nil
}
