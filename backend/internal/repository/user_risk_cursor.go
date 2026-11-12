package repository

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Binding protects traversal consistency; every query independently reapplies
// its authorization and filters. This token is not an authorization credential.
// History's high-water ID excludes later allocations, not older transactions
// that commit late. Each page is a live committed read snapshot; refresh is
// required to discover records committed above a boundary already traversed.
type userRiskCursor struct {
	Version         int       `json:"v"`
	Kind            string    `json:"kind"`
	Binding         string    `json:"binding"`
	Page            int       `json:"page"`
	BeforeID        int64     `json:"before_id,omitempty"`
	UpperID         int64     `json:"upper_id,omitempty"`
	BeforeAt        time.Time `json:"before_at,omitempty"`
	BeforeUserID    int64     `json:"before_user_id,omitempty"`
	BeforePolicyKey string    `json:"before_policy_key,omitempty"`
}

func openUserRiskCursor(kind string, f service.UserRiskListFilter) (userRiskCursor, error) {
	raw, err := json.Marshal(struct {
		Kind, Policy, Search, Mode, Action string
		UserID                             int64
		WindowDays, PageSize               int
	}{kind, f.PolicyKey, f.Search, f.ReleaseMode, f.Action, f.UserID, f.WindowDays, f.PageSize})
	if err != nil {
		return userRiskCursor{}, err
	}
	digest := sha256.Sum256(raw)
	binding := hex.EncodeToString(digest[:])
	invalid := func() (userRiskCursor, error) {
		return userRiskCursor{}, infraerrors.BadRequest("INVALID_USER_RISK_CURSOR", "分页游标与筛选不匹配或已失效，请刷新列表。")
	}
	if f.Cursor == "" {
		if f.Page > 1 {
			return userRiskCursor{}, infraerrors.BadRequest("USER_RISK_CURSOR_REQUIRED", "翻页需要上一页返回的游标")
		}
		return userRiskCursor{Version: 1, Kind: kind, Binding: binding, Page: 1}, nil
	}
	if len(f.Cursor) > 2048 {
		return invalid()
	}
	decoded, err := base64.RawURLEncoding.DecodeString(f.Cursor)
	if err != nil {
		return invalid()
	}
	var cursor userRiskCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil {
		return invalid()
	}
	if cursor.Version != 1 || cursor.Kind != kind || cursor.Binding != binding || cursor.Page <= 1 {
		return invalid()
	}
	if kind == "history" && (cursor.BeforeID <= 0 || cursor.UpperID < cursor.BeforeID) {
		return invalid()
	}
	if kind == "restrictions" && (cursor.BeforeAt.IsZero() || cursor.BeforeUserID <= 0 || cursor.BeforePolicyKey == "" || len(cursor.BeforePolicyKey) > 96) {
		return invalid()
	}
	return cursor, nil
}

func encodeUserRiskCursor(cursor userRiskCursor) (string, error) {
	raw, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (r *userRiskStore) beginListSnapshot(ctx context.Context) (userRiskTransaction, time.Time, error) {
	tx, err := r.begin(ctx)
	if err != nil {
		return nil, time.Time{}, err
	}
	if _, err := tx.ExecContext(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); err != nil {
		_ = tx.Rollback()
		return nil, time.Time{}, err
	}
	var asOf time.Time
	if err := scanSingleRow(ctx, tx, "SELECT statement_timestamp()", nil, &asOf); err != nil {
		_ = tx.Rollback()
		return nil, time.Time{}, err
	}
	return tx, asOf, nil
}
