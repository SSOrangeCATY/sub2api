package handler

import (
	"context"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Gateway tests observe the boundary between forwarding and durable decisions.
// Real policy counting, release and locking are covered by PostgreSQL tests.
func (r *contentModerationHandlerTestRepo) RecordUserRiskEvent(ctx context.Context, input service.UserRiskEventInput) (*service.UserRiskDecision, error) {
	r.mu.Lock()
	r.riskEvents = append(r.riskEvents, input)
	record := r.riskRecord
	r.mu.Unlock()
	if record != nil {
		return record(ctx, input)
	}
	count := 1
	if input.ExemptReason != "" {
		count = 0
	}
	return &service.UserRiskDecision{
		UserID: input.Admission.UserID, EventID: input.Admission.EventID,
		PolicyKey: input.Admission.PolicyKey, Scope: input.Policy.Scope,
		Generation: input.Admission.Generation, PolicyRevision: input.Policy.Revision,
		HitCount: count, IgnoredReason: input.ExemptReason, RecordedAt: time.Now(),
	}, nil
}

func (r *contentModerationHandlerTestRepo) BeginUserRiskRequest(ctx context.Context, userID int64, policy, scope string) (service.UserRiskAdmission, error) {
	return (&gatewayRiskTestUserRepo{}).BeginUserRiskRequest(ctx, userID, policy, scope)
}

func (r *contentModerationHandlerTestRepo) ReleaseUserRiskRestriction(context.Context, service.UserRiskReleaseInput) (*service.UserRiskReleaseResult, error) {
	return nil, errors.ErrUnsupported
}

func (r *contentModerationHandlerTestRepo) ListUserRiskRestrictions(context.Context, service.UserRiskListFilter) (*service.UserRiskRestrictionPage, error) {
	return nil, errors.ErrUnsupported
}

func (r *contentModerationHandlerTestRepo) ListUserRiskActions(context.Context, service.UserRiskListFilter) (*service.UserRiskActionPage, error) {
	return nil, errors.ErrUnsupported
}

func (r *contentModerationHandlerTestRepo) ExpireUserRiskRestrictions(context.Context, int) ([]int64, error) {
	return nil, errors.ErrUnsupported
}

func (r *contentModerationHandlerTestRepo) setRiskRecord(record func(context.Context, service.UserRiskEventInput) (*service.UserRiskDecision, error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.riskRecord = record
}

func (r *contentModerationHandlerTestRepo) riskEventSnapshot() []service.UserRiskEventInput {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]service.UserRiskEventInput(nil), r.riskEvents...)
}
