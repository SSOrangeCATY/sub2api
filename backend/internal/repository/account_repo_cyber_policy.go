package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.CyberPolicyAccountRecorder = (*accountRepository)(nil)

const cyberPolicyDisableReasonPrefix = "Cyber Policy: accumulated upstream errors reached disable threshold"

// The account row is locked by lockAndMergeAccountProbeExtra. Preserve a
// Cyber Policy disable that happened after this edit loaded its snapshot.
// A fresh account snapshot can still be restored by the existing admin flow.
func mergeCyberPolicyAccountState(account *service.Account, extra map[string]any, raw []byte) error {
	var current struct {
		Count        json.RawMessage `json:"count"`
		Status       string          `json:"status"`
		Schedulable  bool            `json:"schedulable"`
		ErrorMessage string          `json:"error_message"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &current); err != nil {
			return err
		}
	}
	var loadedCount, currentCount int64
	loadedRaw, err := json.Marshal(account.Extra[service.CyberPolicyErrorCountExtraKey])
	if err != nil {
		return err
	}
	_ = json.Unmarshal(loadedRaw, &loadedCount)
	_ = json.Unmarshal(current.Count, &currentCount)
	if currentCount > loadedCount && current.Status == service.StatusError &&
		strings.HasPrefix(current.ErrorMessage, cyberPolicyDisableReasonPrefix) {
		account.Status = current.Status
		account.Schedulable = current.Schedulable
		account.ErrorMessage = current.ErrorMessage
	}
	delete(extra, service.CyberPolicyErrorCountExtraKey)
	value, present, err := decodeAccountExtraJSON(current.Count)
	if err != nil {
		return err
	}
	if present {
		extra[service.CyberPolicyErrorCountExtraKey] = value
	}
	return nil
}

// Keep count, disable state, and the scheduler outbox event in one statement.
// PostgreSQL serializes concurrent updates to the account row. Do not derive
// counts from the (possibly stale) account/scheduler snapshot.
const recordCyberPolicyErrorSQL = `
WITH updated AS (
 UPDATE accounts SET
  extra = COALESCE(extra, '{}'::jsonb) || jsonb_build_object('cyber_policy_error_count', (` + cyberPolicyCountSQL + `) + 1),
  status = CASE WHEN $2 > 0 AND (` + cyberPolicyCountSQL + `) + 1 >= $2 THEN $3 ELSE status END,
  schedulable = CASE WHEN $2 > 0 AND (` + cyberPolicyCountSQL + `) + 1 >= $2 THEN false ELSE schedulable END,
  error_message = CASE WHEN $2 > 0 AND (` + cyberPolicyCountSQL + `) + 1 >= $2 THEN $4 ELSE error_message END,
  updated_at = NOW()
 WHERE id = $1 AND deleted_at IS NULL
 RETURNING (extra->>'cyber_policy_error_count')::bigint AS count,
  ($2 > 0 AND (extra->>'cyber_policy_error_count')::bigint >= $2) AS disabled
), notified AS (
 INSERT INTO scheduler_outbox (event_type, account_id, group_id, payload)
 SELECT $5, $1, NULL, NULL FROM updated WHERE disabled
 RETURNING id
)
SELECT count, disabled FROM updated`

// Treat absent/malformed metadata as zero; retain all unrelated extra fields.
const cyberPolicyCountSQL = `CASE WHEN extra->>'cyber_policy_error_count' ~ '^[0-9]{1,18}$'
 THEN (extra->>'cyber_policy_error_count')::bigint ELSE 0 END`

func (r *accountRepository) RecordCyberPolicyError(ctx context.Context, id int64, threshold int) (int64, bool, error) {
	if threshold < 0 {
		return 0, false, fmt.Errorf("cyber policy disable threshold must be non-negative")
	}
	reason := fmt.Sprintf("%s (%d)", cyberPolicyDisableReasonPrefix, threshold)
	rows, err := r.sql.QueryContext(ctx, recordCyberPolicyErrorSQL, id, threshold, service.StatusError, reason, service.SchedulerOutboxEventAccountChanged)
	if err != nil {
		return 0, false, err
	}
	var count int64
	var disabled bool
	if !rows.Next() {
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return 0, false, err
		}
		return 0, false, service.ErrAccountNotFound
	}
	err = rows.Scan(&count, &disabled)
	closeErr := rows.Close()
	if err != nil {
		return 0, false, err
	}
	if closeErr != nil {
		return 0, false, closeErr
	}
	if disabled {
		// Same status/error/schedulable and cache notification as SetError.
		r.syncSchedulerAccountSnapshot(ctx, id)
	}
	return count, disabled, nil
}
