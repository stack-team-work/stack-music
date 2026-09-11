package gen

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

// localProvider 本地合成器：不依赖任何 API，按风格/情绪程序化合成一段 BGM。
// 产物为 16bit/44.1kHz 立体声 WAV，质量定位"短视频/播客背景音"级别。
type localProvider struct{}

func (p *localProvider) Name() string { return ProviderLocal }

// Submit 同步合成音频并返回本地文件路径。
func (p *localProvider) Submit(_ context.Context, req *Request) (string, string, error) {
	dir, err := os.MkdirTemp("", "gomusic-gen-*")
	if err != nil {
		return "", "", err
	}
	path := filepath.Join(dir, sanitizeFileName(req.TitleOrDefault())+".wav")
	if err := Synthesize(req, path); err != nil {
		return "", "", err
	}
	return "", path, nil
}

func (p *localProvider) Poll(context.Context, string) (string, string, string, error) {
	return "", "", "", fmt.Errorf("本地合成器无需轮询")
}

// styleProfile 风格参数：BPM、和弦进行（半音偏移，根音为 0）、音色开关。
type styleProfile struct {
	bpm        int
	prog       []int   // 和弦进行根音（相对主音的半音数）
	minor      bool    // 小调音阶
	lead       bool    // 是否有主旋律
	leadWave   float64 // 0=正弦 1=方波（方波更电子）
	hatDensity int     // 每小节 hihat 次数
	snareOn    bool
	filterCut  float64 // 低通截止系数（0-1，越小越闷）
	detune     float64 // 失谐程度（Lofi 用）
}

var styleProfiles = map[string]styleProfile{
	"Pop":        {118, []int{0, 7, 9, 5}, false, true, 0.4, 8, true, 0.9, 0.0},
	"Hip Hop":    {90, []int{0, 5, 3, 7}, true, true, 0.7, 8, true, 0.75, 0.0},
	"Rock":       {132, []int{0, 5, 7, 5}, false, true, 0.9, 8, true, 0.95, 0.0},
	"Electronic": {128, []int{0, 3, 7, 10}, true, true, 1.0, 16, true, 0.92, 0.0},
	"Acoustic":   {100, []int{0, 7, 5, 9}, false, true, 0.1, 4, true, 0.8, 0.0},
	"R&B":        {92, []int{0, 9, 5, 7}, true, true, 0.2, 8, true, 0.72, 0.0},
	"Jazz":       {106, []int{0, 5, 2, 7}, false, true, 0.15, 4, true, 0.85, 0.0},
	"Classical":  {88, []int{0, 7, 9, 5}, false, true, 0.05, 0, false, 0.95, 0.0},
	"Country":    {112, []int{0, 7, 5, 7}, false, true, 0.3, 4, true, 0.88, 0.0},
	"Lofi":       {76, []int{0, 9, 5, 4}, true, true, 0.15, 4, true, 0.45, 0.012},
}

// moodBias 情绪对音阶/温度的影响。
type moodBias struct {
	minor  bool    // 偏小调
	rootHz float64 // 主音频率
	tempo  float64 // BPM 缩放
}

var moodBiases = map[string]moodBias{
	"欢快": {false, 261.63, 1.06},
	"伤感": {true, 220.00, 0.92},
	"激昂": {false, 293.66, 1.10},
	"平静": {false, 246.94, 0.94},
	"浪漫": {false, 233.08, 0.97},
	"神秘": {true, 196.00, 0.90},
}

// Synthesize 程序化合成一段音乐并写出 WAV 文件。
// 结构：4 小节前奏（鼓）→ 主段落（和弦+鼓+旋律）→ 收尾。
func Synthesize(req *Request, outPath string) error {
	const sr = 44100
	const totalSec = 40

	prof, ok := styleProfiles[req.Style]
	if !ok {
		prof = styleProfiles["Pop"]
	}
	bias, ok := moodBiases[req.Mood]
	if !ok {
		bias = moodBiases["平静"]
	}
	bpm := float64(prof.bpm) * bias.tempo
	if bpm < 60 {
		bpm = 60
	}
	minor := prof.minor || bias.minor
	beat := 60.0 / bpm      // 一拍秒数
	barSec := beat * 4      // 4/4 一小节
	root := bias.rootHz / 2 // 低八度做和弦

	// 音阶：自然大调 / 自然小调（半音表）
	scale := []int{0, 2, 4, 5, 7, 9, 11}
	if minor {
		scale = []int{0, 2, 3, 5, 7, 8, 10}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	n := sr * totalSec
	left := make([]float64, n)
	right := make([]float64, n)

	// --- 和弦垫（每小节一个和弦，三音叠加，柔和包络）---
	for bar := 0; ; bar++ {
		start := int(float64(bar) * barSec * sr)
		if start >= n {
			break
		}
		deg := prof.prog[bar%len(prof.prog)]
		// 三和弦：根音 + 三度 + 五度（按音阶内取）
		chord := []int{deg, deg + (map[bool]int{false: 4, true: 3}[minor]), deg + 7}
		for _, semi := range chord {
			freq := root * math.Pow(2, float64(semi)/12)
			addTone(left, right, start, barSec*sr, freq, 0.075, sr)
		}
	}

	// --- 鼓组 ---
	hatInterval := barSec / float64(max(prof.hatDensity, 1))
	if prof.hatDensity > 0 {
		for t := 0.0; t < totalSec; t += hatInterval {
			addHat(left, right, int(t*sr), 0.05*(1-0.5*rng.Float64()), sr)
		}
	}
	if prof.snareOn {
		for bar := 0; ; bar++ {
			base := float64(bar) * barSec
			if base >= totalSec {
				break
			}
			// 2、4 拍军鼓
			addSnare(left, right, int((base+beat)*sr), 0.16, sr)
			addSnare(left, right, int((base+3*beat)*sr), 0.16, sr)
		}
	}
	for t := 0.0; t < totalSec; t += beat {
		// 每拍底鼓（1、3 拍加重）
		amp := 0.5
		if math.Mod(math.Round(t/beat), 4) == 0 {
			amp = 0.62
		}
		addKick(left, right, int(t*sr), amp, sr)
	}

	// --- 主旋律（五声音阶随机游走，前 4 小节留白）---
	if prof.lead {
		notes := scale
		if minor {
			notes = []int{0, 3, 5, 7, 10} // 小调五声
		} else {
			notes = []int{0, 2, 4, 7, 9} // 大调五声
		}
		pos := 4 // 第 5 小节进入
		idx := rng.Intn(len(notes))
		start := float64(pos) * barSec
		leadRoot := bias.rootHz
		for start < totalSec-1 {
			dur := beat * []float64{0.5, 1, 1, 2}[rng.Intn(4)]
			if start+dur > totalSec {
				break
			}
			idx += rng.Intn(3) - 1
			if idx < 0 {
				idx += len(notes)
			}
			if idx >= len(notes) {
				idx -= len(notes)
			}
			semi := notes[idx]
			freq := leadRoot * math.Pow(2, float64(semi)/12)
			amp := 0.10 + 0.04*rng.Float64()
			if prof.leadWave > 0.6 {
				addSquare(left, right, int(start*sr), dur*sr, freq, amp, sr)
			} else {
				addTone(left, right, int(start*sr), dur*sr, freq, amp, sr)
			}
			start += dur
		}
	}

	// --- 低通滤波 + 轻微失谐（Lofi 质感）---
	if prof.detune > 0 {
		for i := range left {
			left[i] += prof.detune * math.Sin(2*math.Pi*0.3*float64(i)/sr)
		}
	}
	lowpassStereo(left, right, prof.filterCut)

	// --- 整体淡入淡出 + 归一化 ---
	fade := 1.0 * sr
	for i := 0; i < int(fade) && i < n; i++ {
		g := float64(i) / fade
		left[i] *= g
		right[i] *= g
	}
	for i := n - int(fade); i < n; i++ {
		if i < 0 {
			continue
		}
		g := float64(n-i) / fade
		left[i] *= g
		right[i] *= g
	}
	peak := 0.0
	for i := 0; i < n; i++ {
		peak = math.Max(peak, math.Max(math.Abs(left[i]), math.Abs(right[i])))
	}
	if peak > 0 {
		scale := 0.85 / peak
		for i := 0; i < n; i++ {
			left[i] *= scale
			right[i] *= scale
		}
	}

	return writeWAV(outPath, left, right, sr)
}

// --- 合成基元 ---

// addTone 叠加一个带包络的正弦音（失谐轻微摆动做 pad）。
func addTone(l, r []float64, start int, lenSamples float64, freq, amp float64, sr int) {
	n := int(lenSamples)
	atk := int(0.02 * float64(sr))
	rel := int(0.25 * float64(sr))
	for i := 0; i < n; i++ {
		j := start + i
		if j < 0 || j >= len(l) {
			continue
		}
		t := float64(i) / float64(sr)
		env := 1.0
		if i < atk {
			env = float64(i) / float64(atk)
		} else if i > n-rel && rel > 0 {
			env = math.Max(0, float64(n-i)/float64(rel))
		}
		v := math.Sin(2*math.Pi*freq*t) * env * amp
		l[j] += v
		r[j] += v * 0.96 // 轻微声道差
	}
}

// addSquare 方波音（电子感）。
func addSquare(l, r []float64, start int, lenSamples float64, freq, amp float64, sr int) {
	n := int(lenSamples)
	rel := int(0.08 * float64(sr))
	for i := 0; i < n; i++ {
		j := start + i
		if j < 0 || j >= len(l) {
			continue
		}
		t := float64(i) / float64(sr)
		s := math.Sin(2 * math.Pi * freq * t)
		v := 1.0
		if s < 0 {
			v = -1.0
		}
		v *= 0.5 * (1 + math.Sin(2*math.Pi*freq*2*t)) // 占空比调制，柔化方波
		env := 1.0
		if i > n-rel {
			env = math.Max(0, float64(n-i)/float64(rel))
		}
		v *= env * amp
		l[j] += v
		r[j] += v
	}
}

func addKick(l, r []float64, start int, amp float64, sr int) {
	n := int(0.18 * float64(sr))
	for i := 0; i < n; i++ {
		j := start + i
		if j < 0 || j >= len(l) {
			continue
		}
		t := float64(i) / float64(sr)
		f := 120 * math.Exp(-18*t) // 音高快速下滑
		env := math.Exp(-9 * t)
		v := math.Sin(2*math.Pi*f*t) * env * amp
		l[j] += v
		r[j] += v
	}
}

func addSnare(l, r []float64, start int, amp float64, sr int) {
	n := int(0.14 * float64(sr))
	for i := 0; i < n; i++ {
		j := start + i
		if j < 0 || j >= len(l) {
			continue
		}
		t := float64(i) / float64(sr)
		env := math.Exp(-26 * t)
		v := (rand.Float64()*2 - 1) * env * amp
		l[j] += v
		r[j] += v * 0.9
	}
}

func addHat(l, r []float64, start int, amp float64, sr int) {
	n := int(0.04 * float64(sr))
	prev := 0.0
	for i := 0; i < n; i++ {
		j := start + i
		if j < 0 || j >= len(l) {
			continue
		}
		t := float64(i) / float64(sr)
		env := math.Exp(-70 * t)
		noise := rand.Float64()*2 - 1
		hp := noise - prev // 一阶高通
		prev = noise
		v := hp * env * amp
		l[j] += v
		r[j] += v * 0.85
	}
}

// lowpassStereo 一阶低通滤波（cut 越小平滑越强）。
func lowpassStereo(l, r []float64, cut float64) {
	if cut >= 1 {
		return
	}
	alpha := 1 - cut
	yl, yr := 0.0, 0.0
	for i := range l {
		yl += alpha * (l[i] - yl)
		yr += alpha * (r[i] - yr)
		l[i] = yl
		r[i] = yr
	}
}

// writeWAV 写出 16bit 立体声 PCM WAV。
func writeWAV(path string, left, right []float64, sr int) error {
	n := len(left)
	dataSize := n * 2 * 2
	buf := make([]byte, 44+dataSize)
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+dataSize))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(buf[22:24], 2) // stereo
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sr))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(sr*2*2)) // byte rate
	binary.LittleEndian.PutUint16(buf[32:34], 4)              // block align
	binary.LittleEndian.PutUint16(buf[34:36], 16)             // bits
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataSize))

	off := 44
	for i := 0; i < n; i++ {
		l := int16(clampF(left[i]) * 32767)
		r := int16(clampF(right[i]) * 32767)
		binary.LittleEndian.PutUint16(buf[off:off+2], uint16(l))
		binary.LittleEndian.PutUint16(buf[off+2:off+4], uint16(r))
		off += 4
	}
	return os.WriteFile(path, buf, 0o644)
}

func clampF(v float64) float64 {
	if v > 1 {
		return 1
	}
	if v < -1 {
		return -1
	}
	return v
}

// sanitizeFileName 过滤文件名非法字符。
func sanitizeFileName(name string) string {
	out := make([]rune, 0, len(name))
	for _, ch := range name {
		switch ch {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*', '\n', '\r', '\t':
			out = append(out, '_')
		default:
			out = append(out, ch)
		}
	}
	s := string(out)
	if len([]rune(s)) > 40 {
		s = string([]rune(s)[:40])
	}
	if s == "" {
		s = "untitled"
	}
	return s
}
