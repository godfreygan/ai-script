// Package ffmpeg 是对系统 ffmpeg 二进制的薄封装,提供视频合成的常用操作。
//
// 设计目标:
//   - 不引入 CGO,通过 os/exec 调用 ffmpeg 命令行
//   - 透传 stderr 的 progress 信号,以回调形式上抛
//   - 输入支持本地路径与远程 URL(ffmpeg 原生支持 http/https)
package ffmpeg

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Binary 默认使用 PATH 中的 ffmpeg。可通过 SetBinary 指定绝对路径。
var (
	mu     sync.RWMutex
	binary = "ffmpeg"
)

// SetBinary 设置 ffmpeg 可执行文件路径
func SetBinary(p string) {
	mu.Lock()
	defer mu.Unlock()
	if p != "" {
		binary = p
	}
}

func getBinary() string {
	mu.RLock()
	defer mu.RUnlock()
	return binary
}

// ProgressFn 进度回调:percent ∈ [0,1]
type ProgressFn func(percent float64, message string)

// ConcatVideos 把多个视频按顺序拼接为一个 MP4。
//
// 使用 concat demuxer:
//
//	ffmpeg -y -f concat -safe 0 -protocol_whitelist file,http,https,tcp,tls,crypto \
//	       -i list.txt -c copy out.mp4
//
// 若编码不一致(如不同帧率/分辨率),会自动降级为重编码:
//
//	ffmpeg -y -i a.mp4 -i b.mp4 ... -filter_complex "concat=n=N:v=1:a=1" -c:v libx264 ...
func ConcatVideos(ctx context.Context, inputs []string, outPath string, totalDurationMs int, pf ProgressFn) error {
	if len(inputs) == 0 {
		return errors.New("ffmpeg: no input")
	}
	if err := ensureDir(outPath); err != nil {
		return err
	}
	// 简单策略:始终走重编码,兼容性最好;短剧片段量级小,代价可控
	args := []string{"-y", "-hide_banner", "-progress", "pipe:2"}
	for _, in := range inputs {
		args = append(args, "-i", in)
	}
	// 构造 [0:v][0:a][1:v][1:a]...concat=n=N:v=1:a=1[v][a]
	var fc strings.Builder
	for i := range inputs {
		fc.WriteString(fmt.Sprintf("[%d:v:0][%d:a:0?]", i, i))
	}
	fc.WriteString(fmt.Sprintf("concat=n=%d:v=1:a=1[v][a]", len(inputs)))
	args = append(args,
		"-filter_complex", fc.String(),
		"-map", "[v]", "-map", "[a]",
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
		"-c:a", "aac", "-b:a", "128k",
		"-pix_fmt", "yuv420p",
		outPath,
	)
	return runFFmpeg(ctx, args, totalDurationMs, pf)
}

// OverlayAudio 把视频与音频合到一起(替换原音轨)。
//
//	ffmpeg -y -i video.mp4 -i audio.mp3 -map 0:v:0 -map 1:a:0 -c:v copy -c:a aac out.mp4
func OverlayAudio(ctx context.Context, videoPath, audioPath, outPath string, totalDurationMs int, pf ProgressFn) error {
	if videoPath == "" || audioPath == "" {
		return errors.New("ffmpeg: video and audio paths required")
	}
	if err := ensureDir(outPath); err != nil {
		return err
	}
	args := []string{"-y", "-hide_banner", "-progress", "pipe:2",
		"-i", videoPath,
		"-i", audioPath,
		"-map", "0:v:0", "-map", "1:a:0",
		"-c:v", "copy", "-c:a", "aac", "-b:a", "128k",
		"-shortest",
		outPath,
	}
	return runFFmpeg(ctx, args, totalDurationMs, pf)
}

// MixAudio 把视频原音轨与新音轨混合(不替换),用于配音叠加。
//
//	ffmpeg -y -i video.mp4 -i audio.mp3 -filter_complex "[0:a][1:a]amix=inputs=2:duration=longest[a]" \
//	       -map 0:v:0 -map "[a]" -c:v copy out.mp4
func MixAudio(ctx context.Context, videoPath, audioPath, outPath string, totalDurationMs int, pf ProgressFn) error {
	if videoPath == "" || audioPath == "" {
		return errors.New("ffmpeg: video and audio paths required")
	}
	if err := ensureDir(outPath); err != nil {
		return err
	}
	args := []string{"-y", "-hide_banner", "-progress", "pipe:2",
		"-i", videoPath, "-i", audioPath,
		"-filter_complex", "[0:a][1:a]amix=inputs=2:duration=longest:dropout_transition=0[a]",
		"-map", "0:v:0", "-map", "[a]",
		"-c:v", "copy", "-c:a", "aac", "-b:a", "192k",
		outPath,
	}
	return runFFmpeg(ctx, args, totalDurationMs, pf)
}

// BurnSubtitles 将 SRT 字幕烧录进视频画面。
//
//	ffmpeg -y -i in.mp4 -vf subtitles=in.srt:force_style='FontSize=22' out.mp4
//
// 注意:Windows 下 subtitles= 滤镜对路径转义比较敏感,这里把反斜杠/冒号转义。
func BurnSubtitles(ctx context.Context, videoPath, srtPath, outPath string, totalDurationMs int, pf ProgressFn) error {
	if videoPath == "" || srtPath == "" {
		return errors.New("ffmpeg: video and srt paths required")
	}
	if err := ensureDir(outPath); err != nil {
		return err
	}
	esc := escapeSubtitlePath(srtPath)
	args := []string{"-y", "-hide_banner", "-progress", "pipe:2",
		"-i", videoPath,
		"-vf", fmt.Sprintf("subtitles=%s:force_style='FontSize=22,PrimaryColour=&H00FFFFFF,OutlineColour=&H00000000,Outline=1'", esc),
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23",
		"-c:a", "copy",
		outPath,
	}
	return runFFmpeg(ctx, args, totalDurationMs, pf)
}

// ExtractThumb 抽取视频第 1 秒画面为封面。
func ExtractThumb(ctx context.Context, videoPath, outPath string) error {
	if err := ensureDir(outPath); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, getBinary(),
		"-y", "-hide_banner",
		"-ss", "00:00:01.000",
		"-i", videoPath,
		"-frames:v", "1",
		"-q:v", "2",
		outPath,
	)
	return cmd.Run()
}

// Probe 返回视频的元信息(只解析时长和分辨率)。
type ProbeInfo struct {
	DurationMs int
	Width      int
	Height     int
}

func Probe(ctx context.Context, videoPath string) (*ProbeInfo, error) {
	probe := strings.Replace(getBinary(), "ffmpeg", "ffprobe", 1)
	cmd := exec.CommandContext(ctx, probe,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height:format=duration",
		"-of", "default=noprint_wrappers=1:nokey=0",
		videoPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	info := &ProbeInfo{}
	for _, line := range strings.Split(string(out), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch k {
		case "width":
			info.Width, _ = strconv.Atoi(v)
		case "height":
			info.Height, _ = strconv.Atoi(v)
		case "duration":
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				info.DurationMs = int(f * 1000)
			}
		}
	}
	return info, nil
}

// ============= internal =============

var timeOutRe = regexp.MustCompile(`out_time_ms=(\d+)`)

// runFFmpeg 运行 ffmpeg 命令并把 stderr 中的进度上抛。
// totalDurationMs 为 0 时仅以阶段 message 上抛,不计算百分比。
func runFFmpeg(ctx context.Context, args []string, totalDurationMs int, pf ProgressFn) error {
	bin := getBinary()
	cmd := exec.CommandContext(ctx, bin, args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)
		reader := bufio.NewReader(stderr)
		for {
			line, err := reader.ReadString('\n')
			if line != "" {
				handleFFmpegLine(line, totalDurationMs, pf)
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				return
			}
		}
	}()

	waitErr := cmd.Wait()
	<-doneCh
	if waitErr != nil {
		return fmt.Errorf("ffmpeg: %w", waitErr)
	}
	if pf != nil {
		pf(1.0, "ffmpeg done")
	}
	return nil
}

func handleFFmpegLine(line string, totalDurationMs int, pf ProgressFn) {
	if pf == nil {
		return
	}
	if m := timeOutRe.FindStringSubmatch(line); len(m) == 2 {
		if totalDurationMs <= 0 {
			return
		}
		us, _ := strconv.ParseInt(m[1], 10, 64)
		curMs := int(us / 1000)
		pct := float64(curMs) / float64(totalDurationMs)
		if pct > 1 {
			pct = 1
		}
		if pct < 0 {
			pct = 0
		}
		pf(pct, fmt.Sprintf("ffmpeg %.0f%%", pct*100))
	}
}

func escapeSubtitlePath(p string) string {
	// Windows 路径 C:\path\to.srt → C\:/path/to.srt (subtitles filter 接受 forward slash)
	s := filepath.ToSlash(p)
	if len(s) >= 2 && s[1] == ':' {
		s = string(s[0]) + `\:` + s[2:]
	}
	return "'" + s + "'"
}

func ensureDir(p string) error {
	return os.MkdirAll(filepath.Dir(p), 0o755)
}

// Available 检测 ffmpeg 是否可用,用于启动期 self-check。
func Available() bool {
	cmd := exec.Command(getBinary(), "-version")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// 通过 ms 计算的 ffmpeg 内部秒数;保持 time 引用,避免 unused 包警告
var _ = time.Second
