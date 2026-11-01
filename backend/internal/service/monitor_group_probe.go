package service

import (
	"context"
	"errors"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"strings"
)

// A local fallback key would probe the replacement and falsely mark its primary
// healthy. Reject this configuration both on writes and before existing probes.
func (s *ChannelMonitorService) validateIndependentProbe(ctx context.Context, plainKey, groupName string, extraHeaders ...map[string]string) error {
	if s.localKeys == nil {
		return nil
	}
	// Custom headers can replace the provider credential. Validate those too so
	// a fallback entry cannot be hidden behind an otherwise independent API key.
	for _, headers := range extraHeaders {
		for name, value := range headers {
			switch strings.ToLower(strings.TrimSpace(name)) {
			case "authorization", "x-api-key", "x-goog-api-key":
				value = strings.TrimSpace(value)
				if parts := strings.Fields(value); len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
					value = parts[1]
				}
				if err := s.validateIndependentProbe(ctx, value, groupName); err != nil {
					return err
				}
			}
		}
	}
	if plainKey == "" {
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
