-- Import the original, unversioned upstream playlist stores into collections.
INSERT INTO collections (id, kind, name, created_at, updated_at)
    SELECT 'local:' || id, 'local', name, created_at, created_at FROM playlists;
INSERT INTO collection_videos (collection_id, video_id, added_at)
    SELECT 'local:' || playlist_id, video_id, added_at
    FROM playlist_videos ORDER BY playlist_id, position, rowid;
INSERT INTO collections (id, kind, name, synced, updated_at)
    SELECT id, 'yt', title, 1, updated_at FROM yt_playlists;
INSERT INTO collection_videos (collection_id, video_id, added_at)
    SELECT playlist_id, video_id, fetched_at
    FROM yt_playlist_videos ORDER BY playlist_id, position, rowid;
UPDATE activity_log SET playlist_local_id = 'local:' || playlist_local_id
    WHERE playlist_local_id IS NOT NULL AND typeof(playlist_local_id) = 'integer';

-- The old standalone Watch Later queue is a local collection now. Insert its
-- video metadata first, as those videos need not have been cached elsewhere.
INSERT INTO videos (id, title, channel, url, added_at)
    SELECT video_id, COALESCE(title, ''), channel, url, added_at FROM watch_later WHERE true
    ON CONFLICT(id) DO NOTHING;
INSERT INTO collections (id, kind, name)
    SELECT 'local:' || (SELECT COALESCE(MAX(CAST(SUBSTR(id, 7) AS INTEGER)), 0) + 1
                       FROM collections WHERE kind='local'), 'local', 'Watch Later'
    WHERE EXISTS (SELECT 1 FROM watch_later)
      AND NOT EXISTS (SELECT 1 FROM collections WHERE kind='local' AND name='Watch Later');
INSERT INTO collection_videos (collection_id, video_id, added_at)
    SELECT c.id, wl.video_id, wl.added_at FROM watch_later wl
    JOIN collections c ON c.kind='local' AND c.name='Watch Later'
    ORDER BY wl.rowid
    ON CONFLICT(collection_id, video_id) DO NOTHING;

-- Remove the obsolete stores after copying successfully. Retaining their video
-- foreign keys would prevent later video deletions even after a user removes
-- every membership from the new collections.
DROP TABLE playlist_videos;
DROP TABLE playlists;
DROP TABLE yt_playlist_videos;
DROP TABLE yt_playlists;
DROP TABLE watch_later;
