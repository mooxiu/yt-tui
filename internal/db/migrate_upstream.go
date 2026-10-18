package db

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
)

//go:embed upstream_playlists.sql
var upstreamPlaylistsSQL string

// adoptUnversionedSchema upgrades the pre-runner upstream schema atomically.
// Unlike a fresh database it already has the core tables, but it may predate
// collection unification. Keep the baseline strict for normal migrations; only
// this explicitly detected legacy path creates missing objects conditionally.
func (d *DB) adoptUnversionedSchema(ctx context.Context, ms []migration) error {
	return d.withTx(ctx, "adopt legacy schema", func(tx *sql.Tx) error {
		baseline := strings.NewReplacer(
			"CREATE TABLE ", "CREATE TABLE IF NOT EXISTS ",
			"CREATE INDEX ", "CREATE INDEX IF NOT EXISTS ",
			"CREATE UNIQUE INDEX ", "CREATE UNIQUE INDEX IF NOT EXISTS ",
		).Replace(ms[0].sql)
		if _, err := tx.ExecContext(ctx, baseline); err != nil {
			return fmt.Errorf("adopt legacy baseline: %w", err)
		}
		var oldPlaylists int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='playlists'`,
		).Scan(&oldPlaylists); err != nil {
			return fmt.Errorf("detect upstream playlists: %w", err)
		}
		if oldPlaylists > 0 {
			if _, err := tx.ExecContext(ctx, upstreamPlaylistsSQL); err != nil {
				return fmt.Errorf("import upstream playlists: %w", err)
			}
		}
		// Prefer the authoritative video_positions row when one already exists.
		var oldPosition int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM pragma_table_info('local_videos') WHERE name='last_position_ms'`,
		).Scan(&oldPosition); err != nil {
			return fmt.Errorf("detect upstream resume positions: %w", err)
		}
		if oldPosition > 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO video_positions (video_id, position_ms)
    SELECT id, last_position_ms FROM local_videos WHERE last_position_ms > 0
    ON CONFLICT(video_id) DO NOTHING`); err != nil {
				return fmt.Errorf("import upstream resume positions: %w", err)
			}
		}
		// Reconcile the known legacy column drift and stamp only after all copies
		// succeed. A failure rolls back the baseline, imports, and reconciliation.
		if _, err := tx.ExecContext(ctx, ms[1].sql); err != nil {
			return fmt.Errorf("reconcile upstream schema: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `PRAGMA user_version = 2`); err != nil {
			return fmt.Errorf("stamp upstream schema: %w", err)
		}
		return nil
	})
}
