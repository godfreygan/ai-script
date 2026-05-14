// Package subtitle 提供 SRT/VTT 字幕生成与解析能力。
//
// 主要面向 storyboard.dialogue + duration_sec 自动生成对话字幕,
// 供 ffmpeg subtitles= 滤镜烧录使用。
package subtitle

import (
	"fmt"
	"strings"
	"time"
)

// Cue 字幕条目
type Cue struct {
	Index   int           // 1-based 序号
	Start   time.Duration // 相对视频起点的开始时间
	End     time.Duration // 相对视频起点的结束时间
	Text    string        // 字幕文本(可含换行,会被压缩为 max 2 行)
	Speaker string        // 可选:角色名;若存在会显示为 "<speaker>: <text>"
}

// Builder 顺序追加字幕条目
type Builder struct {
	cues []Cue
	t    time.Duration // 当前累计时间游标
}

func NewBuilder() *Builder { return &Builder{cues: make([]Cue, 0, 32)} }

// Append 追加一条字幕,持续 dur,文本若为空跳过(但游标仍前进)
func (b *Builder) Append(text, speaker string, dur time.Duration) {
	if dur <= 0 {
		dur = 3 * time.Second
	}
	start := b.t
	end := start + dur
	b.t = end
	if strings.TrimSpace(text) == "" {
		return
	}
	b.cues = append(b.cues, Cue{
		Index:   len(b.cues) + 1,
		Start:   start,
		End:     end,
		Text:    wrapText(text, 28),
		Speaker: speaker,
	})
}

// AppendCue 直接追加自定义 cue(高级用法,start/end 由调用者负责)
func (b *Builder) AppendCue(c Cue) {
	if c.Index <= 0 {
		c.Index = len(b.cues) + 1
	}
	b.cues = append(b.cues, c)
	if c.End > b.t {
		b.t = c.End
	}
}

// Cues 返回当前所有字幕
func (b *Builder) Cues() []Cue { return b.cues }

// TotalDuration 返回当前累计游标
func (b *Builder) TotalDuration() time.Duration { return b.t }

// SRT 生成 SRT 文本
func (b *Builder) SRT() string {
	var sb strings.Builder
	for _, c := range b.cues {
		sb.WriteString(fmt.Sprintf("%d\n", c.Index))
		sb.WriteString(fmt.Sprintf("%s --> %s\n", formatSRT(c.Start), formatSRT(c.End)))
		if c.Speaker != "" {
			sb.WriteString(c.Speaker + ": " + c.Text + "\n")
		} else {
			sb.WriteString(c.Text + "\n")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// VTT 生成 WebVTT 文本
func (b *Builder) VTT() string {
	var sb strings.Builder
	sb.WriteString("WEBVTT\n\n")
	for _, c := range b.cues {
		sb.WriteString(fmt.Sprintf("%d\n", c.Index))
		sb.WriteString(fmt.Sprintf("%s --> %s\n", formatVTT(c.Start), formatVTT(c.End)))
		if c.Speaker != "" {
			sb.WriteString("<v " + c.Speaker + ">" + c.Text + "\n")
		} else {
			sb.WriteString(c.Text + "\n")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func formatSRT(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	h := int(d / time.Hour)
	d -= time.Duration(h) * time.Hour
	m := int(d / time.Minute)
	d -= time.Duration(m) * time.Minute
	s := int(d / time.Second)
	d -= time.Duration(s) * time.Second
	ms := int(d / time.Millisecond)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

func formatVTT(d time.Duration) string {
	s := formatSRT(d)
	return strings.Replace(s, ",", ".", 1)
}

// wrapText 简单的按字符数换行(中文按字符宽度近似)。
func wrapText(s string, width int) string {
	s = strings.TrimSpace(s)
	if width <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	var sb strings.Builder
	w := 0
	for _, r := range runes {
		sb.WriteRune(r)
		w++
		if w >= width && (r == ' ' || r == ',' || r == '，' || r == '。' || r == '.') {
			sb.WriteRune('\n')
			w = 0
		}
	}
	return sb.String()
}
