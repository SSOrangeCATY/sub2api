package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// CompareAndSetContentModerationConfig owns its transaction so success means
// committed persistence. Groups are locked before the setting row.
func (r *settingRepository) CompareAndSetContentModerationConfig(ctx context.Context, value string, groupIDs []int64, expectedRevision int64) (bool, error) {
	candidateRevision, err := service.ContentModerationConfigRevision(value)
	if err != nil {
		return false, err
	}
	if candidateRevision != expectedRevision+1 {
		return false, fmt.Errorf("configuration revision must advance exactly once")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return false, fmt.Errorf("versioned configuration save requires its own transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockLiveGroups(ctx, tx.Client(), groupIDs); err != nil {
		return false, err
	}
	var stored string
	err = scanSingleRow(ctx, tx, "SELECT value FROM settings WHERE key=$1 FOR UPDATE",
		[]any{service.SettingKeyContentModerationConfig}, &stored)
	var result sql.Result
	if errors.Is(err, sql.ErrNoRows) {
		if expectedRevision != 1 {
			return false, nil
		}
		result, err = tx.ExecContext(ctx,
			"INSERT INTO settings (key,value,updated_at) VALUES ($1,$2,NOW()) ON CONFLICT (key) DO NOTHING",
			service.SettingKeyContentModerationConfig, value)
	} else if err != nil {
		return false, err
	} else {
		currentRevision, readErr := service.ContentModerationConfigRevision(stored)
		if readErr != nil {
			return false, readErr
		}
		if currentRevision != expectedRevision {
			return false, nil
		}
		result, err = tx.ExecContext(ctx, "UPDATE settings SET value=$1,updated_at=NOW() WHERE key=$2",
			value, service.SettingKeyContentModerationConfig)
	}
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if affected != 1 {
		return false, nil
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
