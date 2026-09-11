package store

import (
	"database/sql"

	"go-music/internal/model"
)

// --- AI 音乐生成任务 ---

const genTaskCols = "id, mode, prompt, lyrics, style, mood, lang, title, provider, model, " +
	"remote_task_id, status, audio_path, song_id, fail_reason, created_at, updated_at"

func genTaskFromRow(sc interface{ Scan(...any) error }) (*model.GenTask, error) {
	var t model.GenTask
	err := sc.Scan(&t.ID, &t.Mode, &t.Prompt, &t.Lyrics, &t.Style, &t.Mood, &t.Lang,
		&t.Title, &t.Provider, &t.Model, &t.RemoteTaskID, &t.Status, &t.AudioPath,
		&t.SongID, &t.FailReason, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateGenTask 新建生成任务，返回任务 ID。
func (s *Store) CreateGenTask(t *model.GenTask) (int64, error) {
	if t.Status == "" {
		t.Status = model.GenPending
	}
	now := nowUnix()
	if t.CreatedAt == 0 {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	res, err := s.db.Exec(`
		INSERT INTO gen_tasks (mode, prompt, lyrics, style, mood, lang, title, provider, model,
			remote_task_id, status, audio_path, song_id, fail_reason, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.Mode, t.Prompt, t.Lyrics, t.Style, t.Mood, t.Lang, t.Title, t.Provider, t.Model,
		t.RemoteTaskID, t.Status, t.AudioPath, t.SongID, t.FailReason, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateGenTask 更新任务状态字段。
func (s *Store) UpdateGenTask(t *model.GenTask) error {
	t.UpdatedAt = nowUnix()
	_, err := s.db.Exec(`
		UPDATE gen_tasks SET mode=?, prompt=?, lyrics=?, style=?, mood=?, lang=?, title=?,
			provider=?, model=?, remote_task_id=?, status=?, audio_path=?, song_id=?,
			fail_reason=?, updated_at=?
		WHERE id=?`,
		t.Mode, t.Prompt, t.Lyrics, t.Style, t.Mood, t.Lang, t.Title, t.Provider, t.Model,
		t.RemoteTaskID, t.Status, t.AudioPath, t.SongID, t.FailReason, t.UpdatedAt, t.ID)
	return err
}

// GenTaskByID 按 ID 查询任务；不存在返回 (nil, nil)。
func (s *Store) GenTaskByID(id int64) (*model.GenTask, error) {
	row := s.db.QueryRow(`SELECT `+genTaskCols+` FROM gen_tasks WHERE id = ?`, id)
	t, err := genTaskFromRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

// GenTasks 全部生成任务（新→旧）。
func (s *Store) GenTasks() ([]*model.GenTask, error) {
	rows, err := s.db.Query(`SELECT ` + genTaskCols + ` FROM gen_tasks ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.GenTask
	for rows.Next() {
		t, err := genTaskFromRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GenTasksByStatus 按状态查询任务（管理器轮询用）。
func (s *Store) GenTasksByStatus(status string) ([]*model.GenTask, error) {
	rows, err := s.db.Query(`SELECT `+genTaskCols+` FROM gen_tasks WHERE status = ? ORDER BY id`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.GenTask
	for rows.Next() {
		t, err := genTaskFromRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeleteGenTask 删除任务记录（歌曲由调用方另行处理）。
func (s *Store) DeleteGenTask(id int64) error {
	_, err := s.db.Exec(`DELETE FROM gen_tasks WHERE id = ?`, id)
	return err
}
