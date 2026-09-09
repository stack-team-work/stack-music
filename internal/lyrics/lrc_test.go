package lyrics

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	content := "[ti:测试]\n[ar:某人]\n[offset:500]\n[00:01.50]第一句\n[00:03]第二句\n[01:02.250]第三句\n[00:02][00:04]重复时间戳\n"
	lines := Parse(content)
	if len(lines) != 5 {
		t.Fatalf("期望 5 行，得到 %d", len(lines))
	}
	// offset:+500ms，按时间排序后顺序：第一句(2.0s)、重复时间戳(2.5s)、第二句(3.5s)、重复时间戳(4.5s)、第三句(62.75s)
	if lines[0].Time != 2*time.Second {
		t.Errorf("第一句时间应为 2s（含 offset），得到 %v", lines[0].Time)
	}
	if lines[1].Text != "重复时间戳" || lines[1].Time != 2500*time.Millisecond {
		t.Errorf("重复时间戳第一段解析错误: %+v", lines[1])
	}
	if lines[2].Text != "第二句" || lines[2].Time != 3500*time.Millisecond {
		t.Errorf("第二句解析错误: %+v", lines[2])
	}
	if lines[3].Time != 4500*time.Millisecond {
		t.Errorf("重复时间戳第二段缺失: %+v", lines[3])
	}
	if lines[4].Text != "第三句" || lines[4].Time != 62*time.Second+750*time.Millisecond {
		t.Errorf("第三句解析错误: %+v", lines[4])
	}
}

func TestCurrentIndex(t *testing.T) {
	lines := []Line{{Time: time.Second, Text: "a"}, {Time: 3 * time.Second, Text: "b"}}
	if got := CurrentIndex(lines, 500*time.Millisecond); got != -1 {
		t.Errorf("未到首行应返回 -1，得到 %d", got)
	}
	if got := CurrentIndex(lines, 2*time.Second); got != 0 {
		t.Errorf("第二句之前应高亮第 0 行，得到 %d", got)
	}
	if got := CurrentIndex(lines, 5*time.Second); got != 1 {
		t.Errorf("应高亮第 1 行，得到 %d", got)
	}
}
