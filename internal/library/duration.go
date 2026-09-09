package library

import (
	"encoding/binary"
	"io"
	"os"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/vorbis"
)

// probeDuration 返回歌曲时长（秒）。策略：
//   - mp3: 优先解析首帧 Xing/Info 头；缺失则全量解码探测
//   - flac: 解析 STREAMINFO 块（快速，不解码）
//   - wav: 解析 fmt 块字节率 + data 块大小（快速）
//   - ogg: 全量解码探测
func probeDuration(format string, f *os.File) (float64, bool) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, false
	}
	switch format {
	case "mp3":
		if d, ok := mp3DurationXing(f); ok {
			return d, true
		}
		return decodeDuration(f, func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
			return mp3.Decode(r)
		})
	case "flac":
		return flacDuration(f)
	case "wav":
		return wavDuration(f)
	case "ogg":
		return decodeDuration(f, func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
			return vorbis.Decode(r)
		})
	}
	return 0, false
}

// decodeDuration 通过 beep 解码器全量读取计算时长（较慢，仅作回退）。
func decodeDuration(f *os.File, dec func(io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error)) (float64, bool) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, false
	}
	s, format, err := dec(f)
	if err != nil {
		return 0, false
	}
	defer s.Close()
	if format.SampleRate == 0 {
		return 0, false
	}
	return float64(s.Len()) / float64(format.SampleRate), true
}

// --- mp3 ---

// MPEG 版本: 3=MPEG1 2=MPEG2 0=MPEG2.5
var mp3Bitrates = map[byte][]int{
	3: {0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0},
	2: {0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
	0: {0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},
}

var mp3SampleRates = map[byte][]int{
	3: {44100, 48000, 32000, 0},
	2: {22050, 24000, 16000, 0},
	0: {11025, 12000, 8000, 0},
}

// mp3DurationXing 在文件头 64KB 内查找第一帧的 Xing/Info 标签计算时长。
func mp3DurationXing(f *os.File) (float64, bool) {
	buf := make([]byte, 65536)
	n, err := f.ReadAt(buf, 0)
	if err != nil && n == 0 {
		return 0, false
	}
	buf = buf[:n]

	for i := 0; i+4 <= n; i++ {
		if buf[i] != 0xFF || buf[i+1]&0xE0 != 0xE0 {
			continue
		}
		version := (buf[i+1] >> 3) & 3
		layer := (buf[i+1] >> 1) & 3
		if version == 1 || layer != 1 { // 跳过 MPEG2.5保留位与非 Layer III
			continue
		}
		bitrateIdx := (buf[i+2] >> 4) & 0xF
		srIdx := (buf[i+2] >> 2) & 3
		if bitrateIdx == 0 || bitrateIdx == 15 || srIdx == 3 {
			continue
		}
		bitrate := mp3Bitrates[version][bitrateIdx] * 1000
		sampleRate := mp3SampleRates[version][srIdx]
		if bitrate == 0 || sampleRate == 0 {
			continue
		}
		samplesPerFrame := 1152
		if version != 3 {
			samplesPerFrame = 576
		}
		frameLen := 144 * bitrate / sampleRate
		if version != 3 {
			frameLen = 72 * bitrate / sampleRate
		}
		if frameLen < 4 {
			continue
		}
		// Xing/Info 位于首帧末尾的 4 字节处
		xing := i + frameLen - 4
		if xing+8 > n {
			return 0, false
		}
		if string(buf[xing:xing+4]) != "Xing" && string(buf[xing:xing+4]) != "Info" {
			return 0, false
		}
		flags := binary.BigEndian.Uint32(buf[xing+4 : xing+8])
		if flags&1 == 0 { // 无帧数字段
			return 0, false
		}
		frames := binary.BigEndian.Uint32(buf[xing+8 : xing+12])
		if frames == 0 {
			return 0, false
		}
		return float64(frames) * float64(samplesPerFrame) / float64(sampleRate), true
	}
	return 0, false
}

// --- flac ---

// flacDuration 解析 STREAMINFO 块获取总采样数与采样率。
func flacDuration(f *os.File) (float64, bool) {
	var head [4]byte
	if _, err := f.ReadAt(head[:], 0); err != nil || string(head[:4]) != "fLaC" {
		return 0, false
	}
	// 元数据块头: 1 字节 (last|type) + 3 字节长度
	off := int64(4)
	for {
		var bh [4]byte
		if _, err := f.ReadAt(bh[:], off); err != nil {
			return 0, false
		}
		blockType := bh[0] & 0x7F
		length := int64(bh[1])<<16 | int64(bh[2])<<8 | int64(bh[3])
		if blockType == 0 && length >= 34 { // STREAMINFO
			var b [18]byte
			if _, err := f.ReadAt(b[:], off+4); err != nil {
				return 0, false
			}
			sampleRate := int64(b[10])<<12 | int64(b[11])<<4 | int64(b[12])>>4
			totalSamples := int64(b[13]&0x0F)<<32 | int64(b[14])<<24 |
				int64(b[15])<<16 | int64(b[16])<<8 | int64(b[17])
			if sampleRate == 0 || totalSamples == 0 {
				return 0, false
			}
			return float64(totalSamples) / float64(sampleRate), true
		}
		if bh[0]&0x80 != 0 {
			return 0, false // 已是最后一块
		}
		off += 4 + length
	}
}

// --- wav ---

// wavDuration 从 RIFF 头解析格式块字节率与数据块大小。
func wavDuration(f *os.File) (float64, bool) {
	var hdr [12]byte
	if _, err := f.ReadAt(hdr[:], 0); err != nil || string(hdr[:4]) != "RIFF" || string(hdr[8:12]) != "WAVE" {
		return 0, false
	}
	off := int64(12)
	for {
		var ch [8]byte
		if _, err := f.ReadAt(ch[:], off); err != nil {
			return 0, false
		}
		id := string(ch[:4])
		size := int64(binary.LittleEndian.Uint32(ch[4:8]))
		switch id {
		case "fmt ":
			if size < 16 {
				return 0, false
			}
			// fmt 数据：audioFormat(2) channels(2) sampleRate(4) byteRate(4) ...
			var fm [12]byte
			if _, err := f.ReadAt(fm[:], off+8); err != nil {
				return 0, false
			}
			byteRate := int64(binary.LittleEndian.Uint32(fm[8:12]))
			if byteRate == 0 {
				return 0, false
			}
			// 寻找 data 块
			doff := off + 8 + size
			for {
				var dc [8]byte
				if _, err := f.ReadAt(dc[:], doff); err != nil {
					return 0, false
				}
				did := string(dc[:4])
				dsize := int64(binary.LittleEndian.Uint32(dc[4:8]))
				if did == "data" {
					return float64(dsize) / float64(byteRate), true
				}
				doff += 8 + dsize + (dsize & 1) // 块按 2 字节对齐
			}
		}
		off += 8 + size + (size & 1)
	}
}
