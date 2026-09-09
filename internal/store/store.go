// Package store 封装 SQLite 持久化（曲库、歌单、收藏、历史）。
// 使用纯 Go 的 modernc.org/sqlite 驱动，无 CGO 依赖。
package store

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"go-music/internal/model"
)

const schema = `
CREATE TABLE IF NOT EXISTS songs (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	path      TEXT    NOT NULL UNIQUE,
	title     TEXT    NOT NULL DEFAULT '',
	artist    TEXT    NOT NULL DEFAULT '',
	album     TEXT    NOT NULL DEFAULT '',
	genre     TEXT    NOT NULL DEFAULT '',
	year      INTEGER NOT NULL DEFAULT 0,
	track     INTEGER NOT NULL DEFAULT 0,
	duration  REAL    NOT NULL DEFAULT 0,
	format    TEXT    NOT NULL DEFAULT '',
	size      INTEGER NOT NULL DEFAULT 0,
	mod_time  INTEGER NOT NULL DEFAULT 0,
	hash      TEXT    NOT NULL DEFAULT '',
	has_cover INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_songs_title ON songs(title);
CREATE INDEX IF NOT EXISTS idx_songs_artist ON songs(artist);

CREATE TABLE IF NOT EXISTS favorites (
	song_id  INTEGER PRIMARY KEY REFERENCES songs(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS playlists (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	name       TEXT    NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS playlist_songs (
	playlist_id INTEGER NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
	song_id     INTEGER NOT NULL REFERENCES songs(id) ON DELETE CASCADE,
	position    INTEGER NOT NULL,
	PRIMARY KEY (playlist_id, position)
);

CREATE TABLE IF NOT EXISTS history (
	id        INTEGER PRIMARY KEY AUTOINCREMENT,
	song_id   INTEGER NOT NULL REFERENCES songs(id) ON DELETE CASCADE,
	played_at INTEGER NOT NULL
);
`

// Store 数据库访问对象。
type Store struct {
	db *sql.DB
}

// Open 打开（或创建）dataDir/music.db 并初始化表结构。
func Open(dataDir string) (*Store, error) {
	path := filepath.Join(dataDir, "music.db")
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close 关闭数据库。
func (s *Store) Close() error { return s.db.Close() }

func songFromRow(sc interface{ Scan(...any) error }) (*model.Song, error) {
	var so model.Song
	var hasCover int
	err := sc.Scan(&so.ID, &so.Path, &so.Title, &so.Artist, &so.Album, &so.Genre,
		&so.Year, &so.Track, &so.Duration, &so.Format, &so.Size, &so.ModTime,
		&so.Hash, &hasCover)
	if err != nil {
		return nil, err
	}
	so.HasCover = hasCover != 0
	return &so, nil
}

const songCols = "id, path, title, artist, album, genre, year, track, duration, format, size, mod_time, hash, has_cover"

// songColsPrefixed join 查询使用的带别名列，避免列名歧义。
func songColsPrefixed(a string) string {
	return a + ".id, " + a + ".path, " + a + ".title, " + a + ".artist, " +
		a + ".album, " + a + ".genre, " + a + ".year, " + a + ".track, " +
		a + ".duration, " + a + ".format, " + a + ".size, " + a + ".mod_time, " +
		a + ".hash, " + a + ".has_cover"
}

func songArgs(so *model.Song) []any {
	hasCover := 0
	if so.HasCover {
		hasCover = 1
	}
	return []any{so.Path, so.Title, so.Artist, so.Album, so.Genre, so.Year,
		so.Track, so.Duration, so.Format, so.Size, so.ModTime, so.Hash, hasCover}
}

// UpsertSong 插入或更新歌曲（按 path 唯一）。返回歌曲 ID。
func (s *Store) UpsertSong(so *model.Song) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO songs (path, title, artist, album, genre, year, track, duration, format, size, mod_time, hash, has_cover)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(path) DO UPDATE SET
			title=excluded.title, artist=excluded.artist, album=excluded.album,
			genre=excluded.genre, year=excluded.year, track=excluded.track,
			duration=excluded.duration, format=excluded.format, size=excluded.size,
			mod_time=excluded.mod_time, hash=excluded.hash, has_cover=excluded.has_cover`,
		songArgs(so)...)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// DeleteSong 从曲库删除歌曲（级联清理歌单/收藏/历史引用）。
func (s *Store) DeleteSong(id int64) error {
	_, err := s.db.Exec(`DELETE FROM songs WHERE id = ?`, id)
	return err
}

// SongByPath 按路径查询；不存在返回 (nil, nil)。
func (s *Store) SongByPath(path string) (*model.Song, error) {
	row := s.db.QueryRow(`SELECT `+songCols+` FROM songs WHERE path = ?`, path)
	so, err := songFromRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return so, err
}

// SongByID 按 ID 查询；不存在返回 (nil, nil)。
func (s *Store) SongByID(id int64) (*model.Song, error) {
	row := s.db.QueryRow(`SELECT `+songCols+` FROM songs WHERE id = ?`, id)
	so, err := songFromRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return so, err
}

// Songs 返回全部歌曲，可选按关键字过滤（标题/艺术家/专辑）。
func (s *Store) Songs(q string) ([]*model.Song, error) {
	query := `SELECT ` + songCols + ` FROM songs`
	args := []any{}
	if q = strings.TrimSpace(q); q != "" {
		like := "%" + q + "%"
		query += ` WHERE title LIKE ? OR artist LIKE ? OR album LIKE ?`
		args = append(args, like, like, like)
	}
	query += ` ORDER BY artist, album, track, title`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Song
	for rows.Next() {
		so, err := songFromRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, so)
	}
	return out, rows.Err()
}

// SongCount 曲库歌曲总数。
func (s *Store) SongCount() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM songs`).Scan(&n)
	return n, err
}

// AllPathsModTimes 返回 (path, mod_time) 映射，用于增量扫描比对。
func (s *Store) AllPathsModTimes() (map[string]int64, error) {
	rows, err := s.db.Query(`SELECT path, mod_time FROM songs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int64)
	for rows.Next() {
		var p string
		var mt int64
		if err := rows.Scan(&p, &mt); err != nil {
			return nil, err
		}
		out[p] = mt
	}
	return out, rows.Err()
}

// --- 收藏 ---

// SetFavorite 设置/取消收藏。
func (s *Store) SetFavorite(songID int64, fav bool) error {
	if fav {
		_, err := s.db.Exec(`INSERT OR IGNORE INTO favorites (song_id) VALUES (?)`, songID)
		return err
	}
	_, err := s.db.Exec(`DELETE FROM favorites WHERE song_id = ?`, songID)
	return err
}

// IsFavorite 查询是否已收藏。
func (s *Store) IsFavorite(songID int64) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM favorites WHERE song_id = ?`, songID).Scan(&n)
	return n > 0, err
}

// Favorites 返回收藏的歌曲列表。
func (s *Store) Favorites() ([]*model.Song, error) {
	rows, err := s.db.Query(`
		SELECT ` + songColsPrefixed("s") + ` FROM songs s
		JOIN favorites f ON f.song_id = s.id
		ORDER BY s.artist, s.album, s.track`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Song
	for rows.Next() {
		so, err := songFromRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, so)
	}
	return out, rows.Err()
}

// --- 歌单 ---

// CreatePlaylist 新建歌单，返回 ID。
func (s *Store) CreatePlaylist(name string) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO playlists (name, created_at) VALUES (?, ?)`,
		strings.TrimSpace(name), nowUnix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// DeletePlaylist 删除歌单。
func (s *Store) DeletePlaylist(id int64) error {
	_, err := s.db.Exec(`DELETE FROM playlists WHERE id = ?`, id)
	return err
}

// Playlists 列出全部歌单（含歌曲数）。
func (s *Store) Playlists() ([]*model.Playlist, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.name, p.created_at, COUNT(ps.song_id)
		FROM playlists p LEFT JOIN playlist_songs ps ON ps.playlist_id = p.id
		GROUP BY p.id ORDER BY p.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Playlist
	for rows.Next() {
		var pl model.Playlist
		if err := rows.Scan(&pl.ID, &pl.Name, &pl.CreatedAt, &pl.SongCount); err != nil {
			return nil, err
		}
		out = append(out, &pl)
	}
	return out, rows.Err()
}

// PlaylistByID 按 ID 查询歌单。
func (s *Store) PlaylistByID(id int64) (*model.Playlist, error) {
	var pl model.Playlist
	err := s.db.QueryRow(`SELECT id, name, created_at FROM playlists WHERE id = ?`, id).
		Scan(&pl.ID, &pl.Name, &pl.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pl, nil
}

// AddSongToPlaylist 将歌曲追加到歌单末尾；已存在则跳过。
func (s *Store) AddSongToPlaylist(playlistID, songID int64) error {
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM playlist_songs WHERE playlist_id = ? AND song_id = ?`,
		playlistID, songID).Scan(&n)
	if n > 0 {
		return nil
	}
	var maxPos sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(position) FROM playlist_songs WHERE playlist_id = ?`,
		playlistID).Scan(&maxPos); err != nil {
		return err
	}
	pos := int64(0)
	if maxPos.Valid {
		pos = maxPos.Int64 + 1
	}
	_, err := s.db.Exec(`INSERT INTO playlist_songs (playlist_id, song_id, position) VALUES (?, ?, ?)`,
		playlistID, songID, pos)
	return err
}

// RemoveSongFromPlaylist 从歌单移除歌曲并重排 position。
func (s *Store) RemoveSongFromPlaylist(playlistID, songID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM playlist_songs WHERE playlist_id = ? AND song_id = ?`,
		playlistID, songID); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		UPDATE playlist_songs SET position = (
			SELECT COUNT(*) FROM playlist_songs p2
			WHERE p2.playlist_id = playlist_songs.playlist_id AND p2.position <= playlist_songs.position
		) WHERE playlist_id = ?`, playlistID); err != nil {
		return err
	}
	return tx.Commit()
}

// PlaylistSongs 返回歌单内按 position 排序的歌曲。
func (s *Store) PlaylistSongs(playlistID int64) ([]*model.Song, error) {
	rows, err := s.db.Query(`
		SELECT `+songColsPrefixed("so")+` FROM songs so
		JOIN playlist_songs ps ON ps.song_id = so.id
		WHERE ps.playlist_id = ?
		ORDER BY ps.position`, playlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Song
	for rows.Next() {
		so, err := songFromRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, so)
	}
	return out, rows.Err()
}

// --- 播放历史 ---

// AddHistory 记录一次播放（保留最近 200 条）。
func (s *Store) AddHistory(songID int64) error {
	if _, err := s.db.Exec(`INSERT INTO history (song_id, played_at) VALUES (?, ?)`,
		songID, nowUnix()); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM history WHERE id NOT IN (
		SELECT id FROM history ORDER BY id DESC LIMIT 200)`)
	return err
}

// History 返回最近播放记录（去重，按时间倒序）。
func (s *Store) History(limit int) ([]*model.Song, error) {
	rows, err := s.db.Query(`
		SELECT `+songColsPrefixed("so")+` FROM songs so
		JOIN history h ON h.song_id = so.id
		WHERE h.id = (SELECT MAX(h2.id) FROM history h2 WHERE h2.song_id = so.id)
		ORDER BY h.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Song
	for rows.Next() {
		so, err := songFromRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, so)
	}
	return out, rows.Err()
}

// CoverPath 返回封面缓存文件路径：dataDir/covers/{hash}.{ext}。
func CoverPath(dataDir, hash, ext string) string {
	if ext == "" {
		ext = "jpg"
	}
	return filepath.Join(dataDir, "covers", fmt.Sprintf("%s.%s", hash, ext))
}

func nowUnix() int64 { return time.Now().Unix() }
