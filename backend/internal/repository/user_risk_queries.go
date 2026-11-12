package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func normalizeUserRiskListFilter(f service.UserRiskListFilter) (service.UserRiskListFilter, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.Page > 1000000 {
		f.Page = 1000000
	}
	if f.PageSize <= 0 {
		f.PageSize = 20
	}
	if f.PageSize > 100 {
		f.PageSize = 100
	}
	if f.WindowDays == 0 {
		f.WindowDays = 7
	}
	if f.UserID < 0 || f.WindowDays < 1 || f.WindowDays > 30 ||
		len(f.PolicyKey) > 96 || strings.TrimSpace(f.PolicyKey) != f.PolicyKey || strings.ContainsRune(f.PolicyKey, 0) ||
		len(f.Action) > 64 || strings.ContainsRune(f.Action, 0) ||
		(f.ReleaseMode != "" && f.ReleaseMode != "manual" && f.ReleaseMode != "temporary") {
		return f, infraerrors.BadRequest("INVALID_USER_RISK_FILTER", "用户限制查询条件无效")
	}
	f.Search = userRiskAuditText(strings.TrimSpace(f.Search), 256)
	return f, nil
}

func userRiskPageCount(total int64, size int) int {
	pages := int((total + int64(size) - 1) / int64(size))
	if pages == 0 {
		return 1
	}
	return pages
}

const userRiskRestrictionWhere = `
 WHERE u.deleted_at IS NULL AND r.cleared_at IS NULL AND (r.admin_release OR r.expires_at>$5)
 AND ($1='' OR r.policy_key=$1) AND ($2::bigint=0 OR r.user_id=$2)
 AND ($3='' OR strpos(lower(u.email||' '||COALESCE(u.username,'')||' '||u.id::text),lower($3))>0)
 AND ($4='' OR ($4='manual' AND r.admin_release) OR ($4='temporary' AND NOT r.admin_release))
`

func (r *contentModerationRepository) ListUserRiskRestrictions(ctx context.Context, filter service.UserRiskListFilter) (*service.UserRiskRestrictionPage, error) {
	return r.userRiskStore().listRestrictions(ctx, filter)
}

func (r *userRiskStore) listRestrictions(ctx context.Context, filter service.UserRiskListFilter) (*service.UserRiskRestrictionPage, error) {
	f, err := normalizeUserRiskListFilter(filter)
	if err != nil {
		return nil, err
	}
	cursor, err := openUserRiskCursor("restrictions", f)
	if err != nil {
		return nil, err
	}
	f.Page = cursor.Page
	tx, asOf, err := r.beginListSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	page := &service.UserRiskRestrictionPage{Items: []service.UserRiskRestriction{}, Page: f.Page, PageSize: f.PageSize, WindowDays: f.WindowDays, AsOf: asOf}
	args := []any{f.PolicyKey, f.UserID, f.Search, f.ReleaseMode, asOf}
	if err := scanSingleRow(ctx, tx,
		"SELECT COUNT(*) FROM user_risk_restrictions r JOIN users u ON u.id=r.user_id"+userRiskRestrictionWhere, args, &page.Total); err != nil {
		return nil, err
	}
	page.Pages = userRiskPageCount(page.Total, f.PageSize)
	var beforeAt any
	if !cursor.BeforeAt.IsZero() {
		beforeAt = cursor.BeforeAt
	}
	rows, err := tx.QueryContext(ctx, `
 SELECT r.user_id,u.email,COALESCE(u.username,''),u.status,r.policy_key,r.scope,r.generation,r.version,
 (SELECT COUNT(*) FROM user_risk_events e JOIN user_risk_cycles c ON c.user_id=e.user_id AND c.policy_key=e.policy_key
 WHERE e.user_id=r.user_id AND e.policy_key=r.policy_key AND e.generation=c.generation AND e.counted
 AND e.recorded_at>$5::timestamptz-($6::int*INTERVAL '1 day') AND e.recorded_at <= $5),
 r.decision_hit_count,r.threshold,r.admin_release,r.restricted_at,r.expires_at,r.policy_revision
 FROM user_risk_restrictions r JOIN users u ON u.id=r.user_id
`+userRiskRestrictionWhere+`
 AND ($7::timestamptz IS NULL OR r.restricted_at < $7
 OR (r.restricted_at=$7 AND r.user_id < $8)
 OR (r.restricted_at=$7 AND r.user_id=$8 AND r.policy_key>$9))
 ORDER BY r.restricted_at DESC,r.user_id DESC,r.policy_key LIMIT $10
`, append(args, f.WindowDays, beforeAt, cursor.BeforeUserID, cursor.BeforePolicyKey, f.PageSize+1)...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var item service.UserRiskRestriction
		var expiry sql.NullTime
		if err := rows.Scan(&item.UserID, &item.Email, &item.Username, &item.BaseStatus, &item.PolicyKey, &item.Scope,
			&item.Generation, &item.Version, &item.WindowHitCount, &item.DecisionHitCount, &item.Threshold,
			&item.AdminRelease, &item.RestrictedAt, &expiry, &item.PolicyRevision); err != nil {
			return nil, err
		}
		if expiry.Valid {
			value := expiry.Time
			item.ExpiresAt = &value
		}
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(page.Items) > f.PageSize {
		page.HasMore = true
		page.Items = page.Items[:f.PageSize]
		last := page.Items[len(page.Items)-1]
		cursor.Page++
		cursor.BeforeAt, cursor.BeforeUserID, cursor.BeforePolicyKey = last.RestrictedAt, last.UserID, last.PolicyKey
		page.NextCursor, err = encodeUserRiskCursor(cursor)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return page, nil
}

const userRiskHistoryWhere = `
 WHERE ($1='' OR a.policy_key=$1) AND ($2::bigint=0 OR a.user_id=$2)
 AND ($3='' OR strpos(lower(u.email||' '||COALESCE(u.username,'')||' '||u.id::text),lower($3))>0)
 AND ($4='' OR a.action=$4) AND a.id <= $5
`

func (r *contentModerationRepository) ListUserRiskActions(ctx context.Context, filter service.UserRiskListFilter) (*service.UserRiskActionPage, error) {
	return r.userRiskStore().listActions(ctx, filter)
}

func (r *userRiskStore) listActions(ctx context.Context, filter service.UserRiskListFilter) (*service.UserRiskActionPage, error) {
	f, err := normalizeUserRiskListFilter(filter)
	if err != nil {
		return nil, err
	}
	cursor, err := openUserRiskCursor("history", f)
	if err != nil {
		return nil, err
	}
	f.Page = cursor.Page
	tx, asOf, err := r.beginListSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if f.Cursor == "" {
		if err := scanSingleRow(ctx, tx, "SELECT COALESCE(MAX(id),0) FROM user_risk_actions", nil, &cursor.UpperID); err != nil {
			return nil, err
		}
	}
	page := &service.UserRiskActionPage{Items: []service.UserRiskAction{}, Page: f.Page, PageSize: f.PageSize, AsOf: asOf}
	args := []any{f.PolicyKey, f.UserID, f.Search, f.Action, cursor.UpperID}
	if err := scanSingleRow(ctx, tx,
		"SELECT COUNT(*) FROM user_risk_actions a JOIN users u ON u.id=a.user_id"+userRiskHistoryWhere, args, &page.Total); err != nil {
		return nil, err
	}
	page.Pages = userRiskPageCount(page.Total, f.PageSize)
	rows, err := tx.QueryContext(ctx,
		"SELECT a.id,a.user_id,a.policy_key,a.generation,a.action,a.actor_id,a.reason,a.decision,a.created_at,u.email,COALESCE(u.username,'') FROM user_risk_actions a JOIN users u ON u.id=a.user_id"+
			userRiskHistoryWhere+" AND ($6::bigint=0 OR a.id < $6) ORDER BY a.id DESC LIMIT $7",
		append(args, cursor.BeforeID, f.PageSize+1)...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var item service.UserRiskAction
		var actor sql.NullInt64
		var raw []byte
		if err := rows.Scan(&item.ID, &item.UserID, &item.PolicyKey, &item.Generation, &item.Action, &actor, &item.Reason, &raw, &item.CreatedAt, &item.Email, &item.Username); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &item.Decision); err != nil {
			return nil, err
		}
		if actor.Valid {
			value := actor.Int64
			item.ActorID = &value
		} else if item.Decision.ActorID > 0 {
			value := item.Decision.ActorID
			item.ActorID = &value
		}
		page.Items = append(page.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(page.Items) > f.PageSize {
		page.HasMore = true
		page.Items = page.Items[:f.PageSize]
		cursor.Page++
		cursor.BeforeID = page.Items[len(page.Items)-1].ID
		page.NextCursor, err = encodeUserRiskCursor(cursor)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return page, nil
}
