package service

import (
	"context"
	"time"
)

const (
	UserRiskPolicyCyber  = "cyber_policy"
	UserRiskScopeAPI     = "api"
	UserRiskAdmissionKey = "user_risk_admission"
)

type UserRiskActionRule struct {
	Threshold     int  `json:"threshold"`
	DurationHours int  `json:"duration_hours"`
	AdminRelease  bool `json:"admin_release"`
}

// Admission is minted by the server before an upstream logical request. Its
// generation prevents a late result from punishing a newly released cycle.
type UserRiskAdmission struct {
	Scope      string
	EventID    string
	UserID     int64
	PolicyKey  string
	Generation int64
	AdmittedAt time.Time
}

// Only the applicable policy is snapshotted; moderation API credentials and
// unrelated site settings must never enter the risk ledger.
type UserRiskPolicySnapshot struct {
	Revision   int64                `json:"revision"`
	Scope      string               `json:"scope"`
	WindowDays int                  `json:"window_days"`
	Actions    []UserRiskActionRule `json:"actions"`
}

type UserRiskDecision struct {
	Released           bool       `json:"released,omitempty"`
	BaseStatus         string     `json:"base_status,omitempty"`
	ActorID            int64      `json:"actor_id,omitempty"`
	ActorLabel         string     `json:"actor_label,omitempty"`
	Duplicate          bool       `json:"duplicate,omitempty"`
	Scope              string     `json:"scope"`
	PolicyRevision     int64      `json:"policy_revision"`
	EventID            string     `json:"event_id"`
	UserID             int64      `json:"user_id"`
	PolicyKey          string     `json:"policy_key"`
	Generation         int64      `json:"generation"`
	HitCount           int        `json:"hit_count"`
	Threshold          int        `json:"threshold"`
	Restricted         bool       `json:"restricted"`
	AdminRelease       bool       `json:"admin_release"`
	NewlyApplied       bool       `json:"newly_applied"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
	RestrictionVersion int64      `json:"restriction_version"`
	IgnoredReason      string     `json:"ignored_reason,omitempty"`
	RecordedAt         time.Time  `json:"recorded_at"`
}

type UserRiskEventInput struct {
	ExemptReason string
	Admission    UserRiskAdmission
	RequestID    string
	GroupID      *int64
	Model        string
	Endpoint     string
	Policy       UserRiskPolicySnapshot
}

type UserRiskReleaseInput struct {
	UserID          int64
	PolicyKey       string
	ActorID         int64
	OperationID     string
	Reason          string
	ExpectedVersion int64
}

type UserRiskReleaseResult struct {
	Released   bool   `json:"released"`
	UserID     int64  `json:"user_id"`
	PolicyKey  string `json:"policy_key"`
	Generation int64  `json:"generation"`
	Version    int64  `json:"version"`
	BaseStatus string `json:"base_status"`
}

type UserRiskRestriction struct {
	UserID           int64      `json:"user_id"`
	Email            string     `json:"email"`
	Username         string     `json:"username"`
	BaseStatus       string     `json:"base_status"`
	PolicyKey        string     `json:"policy_key"`
	Scope            string     `json:"scope"`
	Generation       int64      `json:"generation"`
	Version          int64      `json:"version"`
	WindowHitCount   int        `json:"window_hit_count"`
	DecisionHitCount int        `json:"decision_hit_count"`
	Threshold        int        `json:"threshold"`
	AdminRelease     bool       `json:"admin_release"`
	RestrictedAt     time.Time  `json:"restricted_at"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	PolicyRevision   int64      `json:"policy_revision"`
}

type UserRiskListFilter struct {
	Cursor      string
	Action      string
	PolicyKey   string
	UserID      int64
	Search      string
	ReleaseMode string
	WindowDays  int
	Page        int
	PageSize    int
}

type UserRiskRestrictionPage struct {
	NextCursor     string                `json:"next_cursor"`
	HasMore        bool                  `json:"has_more"`
	AsOf           time.Time             `json:"as_of"`
	WindowDays     int                   `json:"window_days"`
	PolicyRevision int64                 `json:"policy_revision"`
	Items          []UserRiskRestriction `json:"items"`
	Total          int64                 `json:"total"`
	Page           int                   `json:"page"`
	PageSize       int                   `json:"page_size"`
	Pages          int                   `json:"pages"`
}

type UserRiskAction struct {
	Email      string           `json:"email"`
	Username   string           `json:"username"`
	ID         int64            `json:"id"`
	UserID     int64            `json:"user_id"`
	PolicyKey  string           `json:"policy_key"`
	Generation int64            `json:"generation"`
	Action     string           `json:"action"`
	ActorID    *int64           `json:"actor_id,omitempty"`
	Reason     string           `json:"reason"`
	Decision   UserRiskDecision `json:"decision"`
	CreatedAt  time.Time        `json:"created_at"`
}

type UserRiskActionPage struct {
	NextCursor string           `json:"next_cursor"`
	HasMore    bool             `json:"has_more"`
	AsOf       time.Time        `json:"as_of"`
	Items      []UserRiskAction `json:"items"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	Pages      int              `json:"pages"`
}

type UserRiskPolicyRepository interface {
	BeginUserRiskRequest(context.Context, int64, string, string) (UserRiskAdmission, error)
	RecordUserRiskEvent(context.Context, UserRiskEventInput) (*UserRiskDecision, error)
	ReleaseUserRiskRestriction(context.Context, UserRiskReleaseInput) (*UserRiskReleaseResult, error)
	ListUserRiskRestrictions(context.Context, UserRiskListFilter) (*UserRiskRestrictionPage, error)
	ListUserRiskActions(context.Context, UserRiskListFilter) (*UserRiskActionPage, error)
	ExpireUserRiskRestrictions(context.Context, int) ([]int64, error)
}

// UserRiskActionForCount selects the highest applicable threshold from a
// validated policy without modifying its shared rule snapshot.
func UserRiskActionForCount(actions []UserRiskActionRule, count int) (UserRiskActionRule, bool) {
	selected := UserRiskActionRule{}
	for _, action := range actions {
		if count >= action.Threshold && action.Threshold > selected.Threshold {
			selected = action
		}
	}
	return selected, selected.Threshold > 0
}
