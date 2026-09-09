// Package player 封装 beep 音频播放引擎：
// 播放队列、四种播放模式、进度定位、音量控制，并通过事件通道向 UI/Web 推送状态。
package player

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/flac"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/vorbis"
	"github.com/gopxl/beep/wav"

	"go-music/internal/model"
)

// State 播放状态。
type State int

const (
	StateStopped State = iota
	StatePlaying
	StatePaused
)

func (s State) String() string {
	switch s {
	case StatePlaying:
		return "playing"
	case StatePaused:
		return "paused"
	}
	return "stopped"
}

// EventKind 事件类型。
type EventKind int

const (
	// EvSong 歌曲切换（含开始播放）。
	EvSong EventKind = iota
	// EvState 播放状态变化（播放/暂停/停止）。
	EvState
	// EvError 播放出错（如文件损坏无法解码）。
	EvError
)

// Event 播放器事件。
type Event struct {
	Kind    EventKind
	Song    *model.Song
	State   State
	Message string
}

const outputSampleRate = 44100

// resampleQuality 实时重采样质量（beep 建议 3-4 为性能/质量均衡区）。
const resampleQuality = 4

// Player 播放控制器。所有方法线程安全。
type Player struct {
	mu   sync.Mutex
	gain *gainStream
	ctrl *beep.Ctrl
	// resampler 源采样率≠输出采样率时启用，统一重采样到 outputSampleRate，
	// 否则 48k/96k 音源会变速变调播放。
	resampler *beep.Resampler
	stream    beep.StreamSeekCloser
	format    beep.Format
	// cancel 当前流的取消通道：切歌时关闭以让旧流从混音器退出。
	cancel  chan struct{}
	gen     int
	current *model.Song

	queue    []*model.Song
	queuePos int
	mode     model.PlayMode
	volume   float64 // 0-100
	state    State

	events chan Event
	doneCh chan doneSignal

	// OnSongPlayed 歌曲开始播放时回调（用于记录播放历史），可空。
	OnSongPlayed func(*model.Song)
}

type doneSignal struct{ gen int }

// New 创建播放器并初始化音频输出。
func New() (*Player, error) {
	if err := speaker.Init(outputSampleRate, outputSampleRate/10); err != nil {
		return nil, fmt.Errorf("初始化音频输出失败: %w", err)
	}
	p := &Player{
		mode:   model.ModeLoopList,
		volume: 100,
		state:  StateStopped,
		events: make(chan Event, 64),
		doneCh: make(chan doneSignal, 1),
	}
	go p.finishLoop()
	return p, nil
}

// Events 返回事件通道（UI 与 Web 服务订阅）。
func (p *Player) Events() <-chan Event { return p.events }

func (p *Player) emit(ev Event) {
	select {
	case p.events <- ev:
	default: // 消费者未就绪时丢弃，避免阻塞播放
	}
}

func (p *Player) setState(s State) {
	p.mu.Lock()
	p.state = s
	p.mu.Unlock()
	p.emit(Event{Kind: EvState, State: s})
}

// finishLoop 监听歌曲自然播完信号并自动切歌。
func (p *Player) finishLoop() {
	for sig := range p.doneCh {
		p.mu.Lock()
		if sig.gen != p.gen {
			p.mu.Unlock()
			continue // 过期信号（切歌后的残留回调）
		}
		n := len(p.queue)
		mode := p.mode
		idx := p.queuePos
		p.mu.Unlock()
		if n == 0 {
			continue
		}
		var next int
		switch mode {
		case model.ModeLoopOne:
			next = idx
		case model.ModeShuffle:
			if n == 1 {
				next = idx
			} else {
				next = rand.Intn(n - 1)
				if next >= idx {
					next++
				}
			}
		case model.ModeLoopList:
			next = (idx + 1) % n
		default: // ModeOrder：播完全部后停止
			if idx >= n-1 {
				p.SeekTo(0)
				p.setState(StateStopped)
				continue
			}
			next = idx + 1
		}
		p.playIndex(next)
	}
}

// PlayQueue 设置播放队列并从 start 处开始播放。
func (p *Player) PlayQueue(songs []*model.Song, start int) {
	if len(songs) == 0 {
		return
	}
	p.mu.Lock()
	p.queue = append([]*model.Song(nil), songs...)
	if start < 0 || start >= len(songs) {
		start = 0
	}
	p.queuePos = start
	p.mu.Unlock()
	p.playIndex(start)
}

// PlaySong 播放单曲（队列仅含该曲）。
func (p *Player) PlaySong(song *model.Song) {
	p.PlayQueue([]*model.Song{song}, 0)
}

// Current 当前播放歌曲（可能为 nil）。
func (p *Player) Current() *model.Song {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.current
}

// Queue 当前播放队列的副本。
func (p *Player) Queue() []*model.Song {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]*model.Song(nil), p.queue...)
}

// QueuePos 当前在队列中的位置。
func (p *Player) QueuePos() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.queuePos
}

// State 当前播放状态。
func (p *Player) State() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// Mode 播放模式。
func (p *Player) Mode() model.PlayMode {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mode
}

// SetMode 切换播放模式。
func (p *Player) SetMode(m model.PlayMode) {
	p.mu.Lock()
	p.mode = m
	p.mu.Unlock()
}

// Play 播放/继续：暂停时恢复；停止时若队列非空则从头播放。
func (p *Player) Play() {
	p.mu.Lock()
	if p.ctrl != nil && p.state == StatePaused {
		p.ctrl.Paused = false
		p.state = StatePlaying
		p.mu.Unlock()
		p.setState(StatePlaying)
		return
	}
	hasQueue, pos := len(p.queue) > 0, p.queuePos
	state := p.state
	p.mu.Unlock()
	if state == StateStopped && hasQueue {
		p.playIndex(pos)
	}
}

// Pause 暂停。
func (p *Player) Pause() {
	p.mu.Lock()
	if p.ctrl != nil && p.state == StatePlaying {
		p.ctrl.Paused = true
		p.state = StatePaused
		p.mu.Unlock()
		p.setState(StatePaused)
		return
	}
	p.mu.Unlock()
}

// Toggle 切换播放/暂停。
func (p *Player) Toggle() {
	if p.State() == StatePlaying {
		p.Pause()
	} else {
		p.Play()
	}
}

// Next 下一首（随机模式随机取一首）。
func (p *Player) Next() { p.step(1) }

// Prev 上一首：播放超过 3 秒回到开头，否则切上一首。
func (p *Player) Prev() {
	if pos, _ := p.Position(); pos > 3*time.Second {
		p.SeekTo(0)
		return
	}
	p.step(-1)
}

func (p *Player) step(delta int) {
	p.mu.Lock()
	n := len(p.queue)
	if n == 0 {
		p.mu.Unlock()
		return
	}
	idx := p.queuePos + delta
	if p.mode == model.ModeShuffle {
		if n > 1 {
			idx = rand.Intn(n - 1)
			if idx >= p.queuePos {
				idx++
			}
		}
	} else {
		idx = ((idx % n) + n) % n // 手动切歌始终循环
	}
	p.mu.Unlock()
	p.playIndex(idx)
}

// playIndex 播放队列中第 i 首。
func (p *Player) playIndex(i int) {
	p.mu.Lock()
	if i < 0 || i >= len(p.queue) {
		p.mu.Unlock()
		return
	}
	song := p.queue[i]
	p.queuePos = i
	// 释放旧流：先解除暂停再取消，使其立即从混音器退出并关闭文件
	if p.ctrl != nil {
		p.ctrl.Paused = false
	}
	if p.cancel != nil {
		close(p.cancel)
	}
	p.gen++
	gen := p.gen
	p.mu.Unlock()

	streamer, format, err := openStream(song)
	if err != nil {
		p.emit(Event{Kind: EvError, Song: song, Message: err.Error()})
		return
	}

	cancel := make(chan struct{})
	inter := &interruptible{inner: streamer, cancel: cancel}

	// 统一采样率：speaker 固定 44100Hz，不以 44100 采样的源必须重采样，
	// 否则播放速度与音调都会偏移（如 96kHz 音频会慢 2 倍以上）。
	var playStream beep.Streamer = inter
	var resampler *beep.Resampler
	if format.SampleRate != outputSampleRate && format.SampleRate > 0 {
		resampler = beep.Resample(resampleQuality, format.SampleRate, outputSampleRate, inter)
		playStream = resampler
	}

	gain := newGain(playStream, volumeGain(p.volume))
	ctrl := &beep.Ctrl{Streamer: gain}

	p.mu.Lock()
	p.cancel = cancel
	p.gain = gain
	p.ctrl = ctrl
	p.resampler = resampler
	p.stream = streamer
	p.format = format
	p.current = song
	p.state = StatePlaying
	p.mu.Unlock()

	speaker.Play(beep.Seq(ctrl, beep.Callback(func() {
		select {
		case p.doneCh <- doneSignal{gen: gen}:
		default:
		}
	})))

	p.emit(Event{Kind: EvSong, Song: song, State: StatePlaying})
	if p.OnSongPlayed != nil {
		p.OnSongPlayed(song)
	}
}

// Position 返回当前播放位置与歌曲总时长。
func (p *Player) Position() (pos, total time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stream == nil {
		return 0, 0
	}
	speaker.Lock()
	sp := p.stream.Position()
	ln := p.stream.Len()
	sr := p.format.SampleRate
	speaker.Unlock()
	if sr == 0 || ln <= 0 {
		return 0, 0
	}
	total = time.Duration(float64(ln) / float64(sr) * float64(time.Second))
	pos = time.Duration(float64(sp) / float64(sr) * float64(time.Second))
	if pos > total {
		pos = total
	}
	return pos, total
}

// SeekTo 跳转到指定秒数。
// 注意：重采样器无 Reset API，seek 后其内部缓冲残留不足 1ms 的旧样本，人耳不可闻。
func (p *Player) SeekTo(seconds float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stream == nil {
		return
	}
	speaker.Lock()
	defer speaker.Unlock()
	_ = p.stream.Seek(int(seconds * float64(p.format.SampleRate)))
}

// Volume 当前音量 0-100。
func (p *Player) Volume() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.volume
}

// SetVolume 设置音量 0-100。
func (p *Player) SetVolume(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	p.mu.Lock()
	p.volume = v
	if p.gain != nil {
		p.gain.set(volumeGain(v))
	}
	p.mu.Unlock()
}

// Close 关闭音频输出。
func (p *Player) Close() { speaker.Close() }

// volumeGain 将 0-100 音量映射为增益系数（100 为原始音量，0 为静音）。
func volumeGain(v float64) float64 {
	g := v / 100
	if g <= 0 {
		return 0
	}
	// 指数曲线让低音量段更柔和
	return g * g
}

// gainStream 原子安全的音量增益包装。
type gainStream struct {
	inner beep.Streamer
	gain  atomic.Uint64 // math.Float64bits 编码
}

func (g *gainStream) Stream(samples [][2]float64) (int, bool) {
	n, ok := g.inner.Stream(samples)
	gn := math.Float64frombits(g.gain.Load())
	for i := range samples[:n] {
		samples[i][0] *= gn
		samples[i][1] *= gn
	}
	return n, ok
}

func (g *gainStream) Err() error { return g.inner.Err() }

func (g *gainStream) set(v float64) { g.gain.Store(math.Float64bits(v)) }

func newGain(inner beep.Streamer, v float64) *gainStream {
	g := &gainStream{}
	g.inner = inner
	g.set(v)
	return g
}

// openStream 打开并解码音频文件；返回的 StreamSeekCloser 负责关闭文件。
func openStream(song *model.Song) (beep.StreamSeekCloser, beep.Format, error) {
	f, err := os.Open(song.Path)
	if err != nil {
		return nil, beep.Format{}, fmt.Errorf("无法打开文件: %w", err)
	}
	switch song.Format {
	case "mp3":
		return mp3.Decode(f)
	case "flac":
		return flac.Decode(f)
	case "wav":
		return wav.Decode(f)
	case "ogg":
		return vorbis.Decode(f)
	}
	f.Close()
	return nil, beep.Format{}, fmt.Errorf("不支持格式: %s", song.Format)
}

// interruptible 包装流：cancel 被关闭后立即返回 EOF，
// 使切歌时旧流自动从混音器移除并释放文件句柄。
type interruptible struct {
	inner  beep.StreamSeekCloser
	cancel <-chan struct{}
}

func (s *interruptible) Stream(samples [][2]float64) (int, bool) {
	select {
	case <-s.cancel:
		return 0, false
	default:
	}
	return s.inner.Stream(samples)
}

func (s *interruptible) Err() error       { return s.inner.Err() }
func (s *interruptible) Close() error     { return s.inner.Close() }
func (s *interruptible) Len() int         { return s.inner.Len() }
func (s *interruptible) Position() int    { return s.inner.Position() }
func (s *interruptible) Seek(p int) error { return s.inner.Seek(p) }
