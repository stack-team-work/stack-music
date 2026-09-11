package gen

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// TestSynthesize 合成器产出合法的 16bit/44.1k 立体声 WAV。
func TestSynthesize(t *testing.T) {
	out := filepath.Join(t.TempDir(), "test.wav")
	req := &Request{
		Mode: ModeText, Prompt: "夏日海滩的背景音乐",
		Style: "Electronic", Mood: "欢快", Lang: "中文", Title: "测试作品",
	}
	if err := Synthesize(req, out); err != nil {
		t.Fatalf("合成失败: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 44 {
		t.Fatal("文件过小")
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		t.Fatal("WAV 头非法")
	}
	sampleRate := binary.LittleEndian.Uint32(data[24:28])
	channels := binary.LittleEndian.Uint16(data[22:24])
	bits := binary.LittleEndian.Uint16(data[34:36])
	if sampleRate != 44100 || channels != 2 || bits != 16 {
		t.Fatalf("格式错误: sr=%d ch=%d bits=%d", sampleRate, channels, bits)
	}

	// 时长约 40 秒：44100 * 2ch * 2byte * 40 ≈ 7.06MB
	wantBytes := 44 + int(44100*2*2*40)
	if abs(len(data)-wantBytes) > 44100 {
		t.Errorf("数据量偏差过大: got=%d want≈%d", len(data), wantBytes)
	}

	// 抽查非全零样本（确实有声音内容）
	nonZero := 0
	for i := 44; i < len(data)-1; i += 2 {
		if data[i] != 0 || data[i+1] != 0 {
			nonZero++
		}
	}
	if nonZero < len(data)/10 {
		t.Errorf("样本几乎全静音: nonZero=%d/%d", nonZero, len(data)/2)
	}
}

// TestLocalProviderSubmit 本地 Provider 同步产出文件路径。
func TestLocalProviderSubmit(t *testing.T) {
	p, err := NewProvider(Config{Provider: ProviderLocal})
	if err != nil {
		t.Fatal(err)
	}
	remoteID, audioURL, err := p.Submit(nil, &Request{
		Mode: ModeInstrumental, Prompt: "relaxing background", Style: "Lofi", Mood: "平静", Title: "lofi test",
	})
	if err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	if remoteID != "" || audioURL == "" {
		t.Fatalf("本地合成器应直接返回文件路径: id=%q url=%q", remoteID, audioURL)
	}
	if fi, err := os.Stat(audioURL); err != nil || fi.Size() < 100000 {
		t.Fatalf("产物文件异常: %v", err)
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
