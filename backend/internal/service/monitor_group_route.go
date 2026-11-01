package service

import (
	"context"
	"fmt"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func fmtGroupHealthKey(id int64, model string) string { return fmt.Sprintf("%d:%s", id, model) }

var ErrMonitorFallbackModelDenied = infraerrors.NotFound("MODEL_NOT_ALLOWED", "model is not available for this fallback entry")

type MonitorGroupRouteCandidate struct {
	GroupID   int64             `json:"group_id"`
	GroupName string            `json:"group_name"`
	Status    GroupHealthStatus `json:"status"`
	Reason    string            `json:"reason"`
}
type MonitorGroupRoute struct {
	EntryGroupID    int64                        `json:"entry_group_id"`
	EntryGroupName  string                       `json:"entry_group_name"`
	TargetGroupID   *int64                       `json:"target_group_id"`
	TargetGroupName string                       `json:"target_group_name"`
	TargetHealth    GroupHealthStatus            `json:"target_health"`
	Status          string                       `json:"status"`
	Model           string                       `json:"model"`
	Candidates      []MonitorGroupRouteCandidate `json:"candidates"`
	Target          *Group                       `json:"-"`
}
type MonitorGroupRouter struct {
	groups   GroupRepository
	health   GroupHealthReader
	accounts AccountRepository
}

func NewMonitorGroupRouter(groups GroupRepository, health GroupHealthReader) *MonitorGroupRouter {
	return &MonitorGroupRouter{groups: groups, health: health}
}
func (r *MonitorGroupRouter) Resolve(ctx context.Context, entry *Group, model string) (*MonitorGroupRoute, error) {
	route := &MonitorGroupRoute{EntryGroupID: entry.ID, EntryGroupName: entry.Name, Status: "unavailable", Model: model, Candidates: []MonitorGroupRouteCandidate{}}
	if entry.GroupType != GroupTypeFallback {
		id := entry.ID
		route.TargetGroupID = &id
		route.TargetGroupName = entry.Name
		route.Target = entry
		route.Status = "direct"
		return route, nil
	}
	if entry.Status != StatusActive {
		return route, nil
	}
	visited := map[int64]bool{entry.ID: true}
	for _, id := range entry.FallbackGroupIDs {
		if id <= 0 || visited[id] {
			continue
		}
		visited[id] = true
		candidate := MonitorGroupRouteCandidate{GroupID: id, Status: GroupHealthUnknown}
		group, err := r.groups.GetByIDLite(ctx, id)
		if err != nil || group == nil {
			candidate.Reason = "missing"
			route.Candidates = append(route.Candidates, candidate)
			continue
		}
		candidate.GroupName = group.Name
		if model != "" && !group.ModelAllowlist.Allows(model) {
			candidate.Reason = "model_not_allowed"
			route.Candidates = append(route.Candidates, candidate)
			continue
		}
		if group.Status != StatusActive || group.Platform != entry.Platform || (group.GroupType != "" && group.GroupType != GroupTypeBusiness) || len(group.FallbackGroupIDs) != 0 || group.IsSubscriptionType() || group.ClaudeCodeOnly || group.FallbackGroupIDOnInvalidRequest != nil {
			candidate.Reason = "ineligible"
			route.Candidates = append(route.Candidates, candidate)
			continue
		}
		snapshot := GroupHealthSnapshot{Status: GroupHealthUnknown}
		if r.health != nil {
			snapshot, err = r.health.GetGroupHealth(ctx, id, model)
			if err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				snapshot.Status = GroupHealthUnknown
				candidate.Reason = "monitor_unavailable"
			}
		}
		candidate.Status = snapshot.Status
		if snapshot.Status == GroupHealthFailed || snapshot.Status == GroupHealthError {
			candidate.Reason = "monitor_failed"
			route.Candidates = append(route.Candidates, candidate)
			continue
		}
		if r.accounts != nil {
			accounts, listErr := r.accounts.ListSchedulableByGroupID(ctx, id)
			if listErr != nil {
				return nil, listErr
			}
			available := false
			for i := range accounts {
				account := &accounts[i]
				platformMatches := account.Platform == group.Platform || ((group.Platform == PlatformAnthropic || group.Platform == PlatformGemini) && account.Platform == PlatformAntigravity && account.IsMixedSchedulingEnabled())
				// Model compatibility is decided by the platform scheduler after
				// channel aliases are applied. Public names are not account names.
				if platformMatches && account.IsSchedulable() {
					available = true
					break
				}
			}
			if !available {
				candidate.Reason = "no_schedulable_accounts"
				route.Candidates = append(route.Candidates, candidate)
				continue
			}
		}
		if candidate.Reason == "" {
			candidate.Reason = "selected"
		}
		route.Candidates = append(route.Candidates, candidate)
		route.Target = group
		route.TargetGroupID = &group.ID
		route.TargetGroupName = group.Name
		route.TargetHealth = candidate.Status
		route.Status = "routed"
		return route, nil
	}
	return route, nil
}
func (s *APIKeyService) MonitorGroupRouter() *MonitorGroupRouter { return s.monitorGroupRouter }

// ResolveMonitorFallbackKey clones the authenticated key. Explicit admin chains
// delegate access; all prices, quotas, RPM, sticky sessions and logs use target.
func (s *APIKeyService) ResolveMonitorFallbackKey(ctx context.Context, key *APIKey, model string) (*APIKey, *MonitorGroupRoute, error) {
	if key == nil || key.Group == nil || key.Group.GroupType != GroupTypeFallback {
		return key, nil, nil
	}
	if s.monitorGroupRouter == nil {
		return nil, nil, infraerrors.ServiceUnavailable("FALLBACK_UNAVAILABLE", "monitor fallback routing is unavailable")
	}
	entry, err := s.groupRepo.GetByIDLite(ctx, key.Group.ID)
	if err != nil {
		return nil, nil, err
	}
	if entry == nil || entry.Status != StatusActive {
		return nil, nil, ErrGroupNotFound
	}
	if model != "" && !entry.ModelAllowlist.Allows(model) {
		return nil, nil, ErrMonitorFallbackModelDenied
	}
	route, err := s.monitorGroupRouter.Resolve(ctx, entry, model)
	if err != nil {
		return nil, nil, err
	}
	if route.Target == nil {
		return nil, route, infraerrors.ServiceUnavailable("FALLBACK_UNAVAILABLE", "all monitored route groups are unavailable")
	}
	copy := *key
	copy.GroupID = route.TargetGroupID
	copy.Group = route.Target
	if key.User != nil {
		user := *key.User
		user.UserGroupRPMOverride = nil
		if s.userGroupRateRepo != nil {
			rpm, err := s.userGroupRateRepo.GetRPMOverrideByUserAndGroup(ctx, user.ID, route.Target.ID)
			if err != nil {
				return nil, nil, err
			}
			user.UserGroupRPMOverride = rpm
		}
		copy.User = &user
	}
	return &copy, route, nil
}
