package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

const upstreamPlaylistSchema = `
DROP TABLE collection_videos;
DROP TABLE collections;
CREATE TABLE playlists (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE,
 created_at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE playlist_videos (playlist_id INTEGER NOT NULL, video_id TEXT NOT NULL,
 position INTEGER NOT NULL, added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (playlist_id, video_id));
CREATE TABLE yt_playlists (id TEXT PRIMARY KEY, title TEXT NOT NULL,
 updated_at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE yt_playlist_videos (playlist_id TEXT NOT NULL, video_id TEXT NOT NULL,
 position INTEGER NOT NULL, fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (playlist_id, video_id));
CREATE TABLE watch_later (video_id TEXT PRIMARY KEY, title TEXT NOT NULL,
 channel TEXT, url TEXT, added_at DATETIME DEFAULT CURRENT_TIMESTAMP);
INSERT INTO videos (id,title) VALUES ('vid2','Second');
INSERT INTO playlists (id,name) VALUES (7,'Favorites');
INSERT INTO playlist_videos (playlist_id,video_id,position) VALUES (7,'vid2',0),(7,'vid1',1);
INSERT INTO yt_playlists (id,title) VALUES ('PL1','YouTube list');
INSERT INTO yt_playlist_videos (playlist_id,video_id,position) VALUES ('PL1','vid1',0);
INSERT INTO watch_later (video_id,title,channel,url) VALUES ('queued','Queued video','Channel','https://example.com/queued');
INSERT INTO video_positions (video_id,position_ms) VALUES ('vid2',12000);
INSERT INTO history (video_id,event_type) VALUES ('vid1','playAudio');
UPDATE activity_log SET playlist_local_id=7;
PRAGMA user_version=0;
`

func newUpstreamDataDir(t *testing.T) string {
	t.Helper()
	dir := newLegacyDataDir(t)
	raw, err := sql.Open("sqlite", filepath.Join(dir, "yt-tui.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = raw.Close() }()
	if _, err := raw.ExecContext(t.Context(), upstreamPlaylistSchema); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestMigrateUnversionedUpstream(t *testing.T) {
	dir := newUpstreamDataDir(t)
	db, err := New(dir, false, 90)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	ctx := t.Context()
	assertObjectAbsent(t, db, "table", "playlists")
	assertObjectAbsent(t, db, "table", "playlist_videos")
	assertCount(t, db, `SELECT COUNT(*) FROM collections WHERE id='local:7' AND name='Favorites'`, 1)
	assertCount(t, db, `SELECT COUNT(*) FROM collections WHERE id='PL1' AND kind='yt'`, 1)
	assertCount(t, db, `SELECT COUNT(*) FROM collection_videos WHERE collection_id='local:7'`, 2)
	assertCount(t, db, `SELECT COUNT(*) FROM collection_videos WHERE collection_id='PL1' AND video_id='vid1'`, 1)
	assertCount(t, db, `SELECT COUNT(*) FROM activity_log WHERE playlist_local_id='local:7'`, 1)
	assertCount(t, db, `SELECT COUNT(*) FROM video_positions WHERE video_id='vid1' AND position_ms=5000`, 1)
	assertCount(t, db, `SELECT COUNT(*) FROM video_positions WHERE video_id='vid2' AND position_ms=12000`, 1)
	assertCount(t, db, `SELECT COUNT(*) FROM history WHERE video_id='vid1'`, 1)
	assertCount(t, db, `SELECT COUNT(*) FROM local_videos WHERE id='vid1'`, 1)
	assertCount(t, db, `SELECT COUNT(*) FROM feed_cache WHERE video_id='vid1'`, 1)
	videos, err := db.PlaylistVideos(ctx, "local:7")
	if err != nil {
		t.Fatal(err)
	}
	if len(videos) != 2 || videos[0].ID != "vid2" || videos[1].ID != "vid1" {
		t.Fatalf("playlist order changed: %+v", videos)
	}
	in, err := db.InWatchLater(ctx, "queued", "WL", "Watch Later")
	if err != nil || !in {
		t.Fatalf("Watch Later missing: %v, %v", in, err)
	}
	version, err := db.SchemaVersion(ctx)
	if err != nil || version != 2 {
		t.Fatalf("version=%d, err=%v", version, err)
	}
	if err := db.migrate(); err != nil {
		t.Fatal(err)
	}
	// Reopening through the public startup path must also be idempotent.
	reopened, err := New(dir, false, 90)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reopened.Close() }()
	assertCount(t, reopened, `SELECT COUNT(*) FROM collections`, 3)
}

func TestAdoptUpstreamRollsBackOnImportFailure(t *testing.T) {
	dir := newUpstreamDataDir(t)
	raw, err := sql.Open("sqlite", filepath.Join(dir, "yt-tui.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = raw.Close() }()
	if _, err := raw.ExecContext(t.Context(), `DROP TABLE watch_later`); err != nil {
		t.Fatal(err)
	}
	db := &DB{sql: raw}
	if err := db.migrate(); err == nil {
		t.Fatal("expected unsupported schema to fail")
	}
	version, err := db.SchemaVersion(t.Context())
	if err != nil || version != 0 {
		t.Fatalf("failed migration stamped version %d: %v", version, err)
	}
	assertObjectAbsent(t, db, "table", "collections")
	assertCount(t, db, `SELECT COUNT(*) FROM playlists WHERE id=7`, 1)
	assertCount(t, db, `SELECT COUNT(*) FROM local_videos WHERE last_position_ms=5000`, 1)
}

func TestMigrateUnversionedUnifiedLegacy(t *testing.T) {
	dir := newLegacyDataDir(t)
	raw, err := sql.Open("sqlite", filepath.Join(dir, "yt-tui.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(t.Context(), `PRAGMA user_version=0`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := New(dir, false, 90)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	assertCount(t, db, `SELECT COUNT(*) FROM local_videos WHERE id='vid1'`, 1)
}
