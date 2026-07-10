package youtube

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/govdbot/govd/internal/config"
	"github.com/govdbot/govd/internal/database"
	"github.com/govdbot/govd/internal/models"
	"github.com/govdbot/govd/internal/util"
	"github.com/govdbot/govd/internal/util/download"

	"github.com/bytedance/sonic"
)

const (
	ytDlpBinary      = "yt-dlp"
	ytDlpCookiesPath = "private/cookies/youtube.txt"
	ytDlpNativeFormat = "bestvideo[vcodec^=avc1]+bestaudio[ext=m4a]/bestvideo+bestaudio/best"
)

func resolveProxy(ctx *models.ExtractorContext) []string {
	if ctx.Config != nil && ctx.Config.DisableProxy {
		return []string{"--proxy", ""}
	}
	proxy := ""
	if ctx.Config != nil && ctx.Config.Proxy != "" {
		proxy = ctx.Config.Proxy
	} else if config.Env != nil {
		proxy = config.Env.Proxy
	}
	if proxy == "" {
		return nil
	}
	return []string{"--proxy", proxy}
}

func GetVideoFromYtDlp(ctx *models.ExtractorContext) (*models.Media, error) {
	if _, err := exec.LookPath(ytDlpBinary); err != nil {
		return nil, fmt.Errorf("yt-dlp not found in path: %w", err)
	}

	args := []string{"--dump-single-json", "--no-warnings"}
	args = append(args, resolveProxy(ctx)...)
	if _, err := os.Stat(ytDlpCookiesPath); err == nil {
		args = append(args, "--cookies", ytDlpCookiesPath)
	}
	args = append(args, ctx.ContentURL)

	ctx.Debugf("extracting with yt-dlp")

	cmd := exec.Command(ytDlpBinary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp failed: %w: %s", err, stderr.String())
	}

	var data *YtDlpResponse
	if err := sonic.ConfigFastest.Unmarshal(stdout.Bytes(), &data); err != nil {
		return nil, fmt.Errorf("failed to decode yt-dlp response: %w", err)
	}

	formats := ParseYtDlpFormats(data)
	if len(formats) == 0 {
		return nil, fmt.Errorf("no formats found")
	}

	media := ctx.NewMedia()
	media.SetCaption(data.Title)
	item := media.NewItem()
	item.AddFormats(formats...)

	return media, nil
}

func ParseYtDlpFormats(data *YtDlpResponse) []*models.MediaFormat {
	formats := make([]*models.MediaFormat, 0, len(data.Formats))
	duration := int32(data.Duration)

	for _, format := range data.Formats {
		if format.URL == "" {
			continue
		}
		// skip storyboards and other non-media protocols
		if format.Protocol != "https" && format.Protocol != "http" {
			continue
		}

		vCodec := util.ParseVideoCodec(format.VideoCodec)
		aCodec := util.ParseAudioCodec(format.AudioCodec)

		var mediaType database.MediaType
		switch {
		case vCodec != "":
			mediaType = database.MediaTypeVideo
		case aCodec != "":
			mediaType = database.MediaTypeAudio
		default:
			continue
		}

		var settings *models.DownloadSettings
		if userAgent := format.HTTPHeaders["User-Agent"]; userAgent != "" {
			settings = &models.DownloadSettings{
				Headers: map[string]string{"User-Agent": userAgent},
				// youtube throttles the download speed
				// if chunk size is too small
				ChunkSize:   10 * 1024 * 1024, // 10 MB
				AvailableAt: format.AvailableAt,
			}
		} else {
			settings = &models.DownloadSettings{
				ChunkSize:   10 * 1024 * 1024, // 10 MB
				AvailableAt: format.AvailableAt,
			}
		}

		formats = append(formats, &models.MediaFormat{
			Type:             mediaType,
			VideoCodec:       vCodec,
			AudioCodec:       aCodec,
			FormatID:         format.FormatID,
			Width:            int32(format.Width),
			Height:           int32(format.Height),
			Bitrate:          int64(format.Tbr),
			Duration:         duration,
			URL:              []string{format.URL},
			Title:            data.Title,
			Artist:           data.Uploader,
			DownloadSettings: settings,
		})
	}
	return formats
}

func GetVideoFromYtDlpNative(ctx *models.ExtractorContext) (*models.Media, error) {
	if _, err := exec.LookPath(ytDlpBinary); err != nil {
		return nil, fmt.Errorf("yt-dlp not found in path: %w", err)
	}

	download.EnsureDownloadDir()
	outputTmpl := download.ToPath(util.RandomBase64(16) + ".%(ext)s")

	args := []string{
		"--no-warnings",
		"--no-playlist",
		"-f", ytDlpNativeFormat,
		"--merge-output-format", "mp4",
		"-o", outputTmpl,
		"--print", "after_move:%(title)s",
		"--print", "after_move:filepath",
	}
	args = append(args, resolveProxy(ctx)...)
	if _, err := os.Stat(ytDlpCookiesPath); err == nil {
		args = append(args, "--cookies", ytDlpCookiesPath)
	}
	args = append(args, ctx.ContentURL)

	ctx.Debugf("downloading with yt-dlp (native)")

	cmd := exec.Command(ytDlpBinary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("yt-dlp native failed: %w: %s", err, stderr.String())
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("yt-dlp native produced unexpected output")
	}
	title := strings.TrimSpace(lines[0])
	filePath := strings.TrimSpace(lines[len(lines)-1])
	if filePath == "" {
		return nil, fmt.Errorf("yt-dlp native produced no file")
	}
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("yt-dlp native output not found: %w", err)
	}

	media := ctx.NewMedia()
	media.SetCaption(title)
	item := media.NewItem()
	item.AddFormats(&models.MediaFormat{
		Type:          database.MediaTypeVideo,
		VideoCodec:    database.MediaCodecAvc,
		FormatID:      "yt-dlp-native",
		LocalFilePath: filePath,
		Title:         title,
	})

	return media, nil
}
