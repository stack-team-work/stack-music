// Package lyrics 负责 .lrc 歌词文件的解析与查找。
package lyrics

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"stack-music/internal/model"
)

// Line 一行歌词。
type Line struct {
	Time time.Duration // 该行开始时间
	Text string
}

var lrcTagRe = regexp.MustCompile(`\[(\d{1,3}):(\d{1,2})(?:[.:](\d{1,3}))?\]`)
var metaRe = regexp.MustCompile(`^\[(ti|ar|al|by|offset|re):(.+)\]$`)

// Parse 解析 LRC 文本，按时间排序返回歌词行。
func Parse(content string) []Line {
	var lines []Line
	offset := time.Duration(0)
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimRight(raw, "\r")
		if m := metaRe.FindStringSubmatch(line); m != nil {
			if m[1] == "offset" {
				if v, err := strconv.ParseFloat(strings.TrimSpace(m[2]), 64); err == nil {
					offset = time.Duration(v * float64(time.Millisecond))
				}
			}
			continue
		}
		matches := lrcTagRe.FindAllStringSubmatchIndex(line, -1)
		if len(matches) == 0 {
			continue
		}
		text := strings.TrimSpace(line[matches[len(matches)-1][1]:])
		if text == "" {
			continue
		}
		for _, m := range matches {
			min, _ := strconv.Atoi(line[m[2]:m[3]])
			sec, _ := strconv.Atoi(line[m[4]:m[5]])
			ms := 0
			if m[6] >= 0 {
				frac := line[m[6]:m[7]]
				switch len(frac) {
				case 1:
					ms, _ = strconv.Atoi(frac + "00")
				case 2:
					ms, _ = strconv.Atoi(frac + "0")
				case 3:
					ms, _ = strconv.Atoi(frac)
				}
			}
			t := time.Duration(min)*time.Minute + time.Duration(sec)*time.Second + time.Duration(ms)*time.Millisecond + offset
			if t < 0 {
				t = 0
			}
			lines = append(lines, Line{Time: t, Text: text})
		}
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Time < lines[j].Time })
	return lines
}

// Find 尝试为歌曲定位 .lrc 文件。候选顺序：
//  1. 与音频同目录同名（如 song.mp3 -> song.lrc）
//  2. 艺术家 - 标题.lrc、标题 - 艺术家.lrc、标题.lrc
//
// 返回文件路径；不存在返回空字符串。
func Find(song *model.Song) string {
	if song == nil || song.Path == "" {
		return ""
	}
	dir := filepath.Dir(song.Path)
	base := strings.TrimSuffix(filepath.Base(song.Path), filepath.Ext(song.Path))
	title := strings.TrimSpace(song.Title)
	artist := strings.TrimSpace(song.Artist)

	candidates := []string{filepath.Join(dir, base+".lrc")}
	if title != "" {
		candidates = append(candidates, filepath.Join(dir, title+".lrc"))
		if artist != "" {
			candidates = append(candidates,
				filepath.Join(dir, artist+" - "+title+".lrc"),
				filepath.Join(dir, title+" - "+artist+".lrc"))
		}
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}

// Load 读取并解析歌曲的 LRC 文件；无歌词文件返回 nil。
func Load(song *model.Song) []Line {
	path := Find(song)
	if path == "" {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return Parse(string(b))
}

// CurrentIndex 返回 pos 时刻应高亮的歌词行下标（可能为 -1，表示未到第一行）。
func CurrentIndex(lines []Line, pos time.Duration) int {
	idx := -1
	for i, l := range lines {
		if l.Time <= pos {
			idx = i
		} else {
			break
		}
	}
	return idx
}
