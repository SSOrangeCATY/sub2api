//go:build unit

package repository

import (
	"fmt"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/lib/pq"
	"testing"
)

func TestUserRiskReleasePersistenceErrorMapsOnlyItsOperationConstraint(t *testing.T) {
	matching := &pq.Error{Code: "23505", Constraint: "user_risk_actions_policy_key_operation_id_key"}
	for _, err := range []error{matching, fmt.Errorf("wrapped: %w", matching)} {
		result := userRiskReleasePersistenceError(err)
		if infraerrors.Code(result) != 409 || infraerrors.Reason(result) != "USER_RISK_RELEASE_IDEMPOTENCY_CONFLICT" {
			t.Fatal("operation collision did not produce the documented conflict")
		}
	}
	var typedNil *pq.Error
	for _, err := range []error{
		&pq.Error{Code: "23505", Constraint: "different_unique_constraint"},
		&pq.Error{Code: "23503", Constraint: "user_risk_actions_policy_key_operation_id_key"},
		fmt.Errorf("unrelated database error"), typedNil,
	} {
		if result := userRiskReleasePersistenceError(err); result != err {
			t.Fatal("an unrelated database error was reclassified")
		}
	}
}
