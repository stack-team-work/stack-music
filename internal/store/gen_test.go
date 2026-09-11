package store

import (
	"path/filepath"
	"testing"

	"go-music/internal/model"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// TestGenTaskCRUD 生成任务增查改删。
func TestGenTaskCRUD(t *testing.T) {
	st := openTestStore(t)

	task := &model.GenTask{
		Mode: "text", Prompt: "一首欢快的夏日电子乐", Style: "Electronic",
		Mood: "欢快", Lang: "中文", Title: "夏日电音", Provider: "minimax", Model: "music-01",
	}
	id, err := st.CreateGenTask(task)
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}
	if id <= 0 {
		t.Fatal("任务 ID 非法")
	}

	got, err := st.GenTaskByID(id)
	if err != nil || got == nil {
		t.Fatalf("查询任务失败: %v", err)
	}
	if got.Status != model.GenPending {
		t.Errorf("初始状态应为 pending，得到 %q", got.Status)
	}

	got.Status = model.GenDone
	got.AudioPath = "C:/tmp/x.wav"
	got.SongID = 42
	if err := st.UpdateGenTask(got); err != nil {
		t.Fatalf("更新任务失败: %v", err)
	}
	got2, _ := st.GenTaskByID(id)
	if got2.Status != model.GenDone || got2.AudioPath == "" || got2.SongID != 42 {
		t.Errorf("更新未生效: %+v", got2)
	}

	tasks, err := st.GenTasks()
	if err != nil || len(tasks) != 1 {
		t.Fatalf("任务列表异常: %v len=%d", err, len(tasks))
	}
	if err := st.DeleteGenTask(id); err != nil {
		t.Fatalf("删除任务失败: %v", err)
	}
	if got3, _ := st.GenTaskByID(id); got3 != nil {
		t.Error("删除后任务仍存在")
	}
}

// TestSongsSourceColumn 生成歌曲入库且 source 可读写。
func TestSongsSourceColumn(t *testing.T) {
	st := openTestStore(t)

	so := &model.Song{
		Path: filepath.Join(t.TempDir(), "gen.wav"), Title: "AI 生成曲",
		Artist: "AI 音乐", Album: "My Music", Format: "wav", Source: model.SourceGenerated,
	}
	id, err := st.UpsertSong(so)
	if err != nil {
		t.Fatalf("入库失败: %v", err)
	}
	got, err := st.SongByID(id)
	if err != nil || got == nil {
		t.Fatalf("查询失败: %v", err)
	}
	if got.Source != model.SourceGenerated {
		t.Errorf("source 应为 generated，得到 %q", got.Source)
	}
	if got.Album != "My Music" {
		t.Errorf("album 应为 My Music，得到 %q", got.Album)
	}
}
