package repository

import (
	"errors"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/lib/pq"
)

func userRiskReleasePersistenceError(err error) error {
	var pg *pq.Error
	if errors.As(err, &pg) && pg != nil && pg.Code == "23505" &&
		pg.Constraint == "user_risk_actions_policy_key_operation_id_key" {
		return infraerrors.Conflict("USER_RISK_RELEASE_IDEMPOTENCY_CONFLICT", "该操作编号已用于不同的解封请求")
	}
	return err
}
