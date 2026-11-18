package xiaohongshu

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/govdbot/govd/internal/database"
	"github.com/govdbot/govd/internal/models"
	"github.com/govdbot/govd/internal/util"
)

var ShortExtractor = &models.Extractor{
	ID:          "xiaohongshu",
	DisplayName: "Xiaohongshu (Short Link)",

	URLPattern: regexp.MustCompile(
		`https?://xhslink\.com/(?:[a-zA-Z]/)?(?P<id>[a-zA-Z0-9]+)`,
	),
	Host:     []string{"xhslink"},
	Redirect: true,

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		redirectURL, err := ctx.FetchLocation(ctx.ContentURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to follow xhslink redirect: %w", err)
		}

		parsed, err := url.Parse(redirectURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse redirect url: %w", err)
		}

		token := parsed.Query().Get("xsec_token")

		cleanPath := strings.ReplaceAll(parsed.Path, "/discovery/item/", "/explore/")

		cleanURL := "https://www.xiaohongshu.com" + cleanPath
		if token != "" {
			cleanURL += "?xsec_token=" + url.QueryEscape(token) + "&xsec_source=pc_feed"
		}

		return &models.ExtractorResponse{
			URL: cleanURL,
		}, nil
	},
}

var Extractor = &models.Extractor{
	ID:          "xiaohongshu",
	DisplayName: "Xiaohongshu",

	URLPattern: regexp.MustCompile(
		`https?://(?:www\.)?xiaohongshu\.com/(?:explore|discovery/item)/(?P<id>[a-zA-Z0-9]+)(?:\?(?P<xsec_token>xsec_token=[^&\s]+).*)?`,
	),
	Host: []string{"xiaohongshu"},

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		media, err := GetMedia(ctx)
		if err != nil {
			return nil, err
		}
		return &models.ExtractorResponse{
			URL:   ctx.ContentURL,
			Media: media,
		}, nil
	},
}

func GetMedia(ctx *models.ExtractorContext) (*models.Media, error) {
	noteID := ctx.ContentID

	note, err := GetNoteWeb(ctx, noteID)
	if err != nil {
		return nil, fmt.Errorf("failed to get note: %w", err)
	}

	media := ctx.NewMedia()

	caption := note.Title
	if caption == "" {
		caption = note.Desc
	}
	media.SetCaption(caption)

	switch {
	case note.Type == "video" && !isSlideshow(note):
		if err := extractVideo(ctx, media, note); err != nil {
			return nil, err
		}
	default:
		if err := extractImages(media, note); err != nil {
			return nil, err
		}
	}

	return media, nil
}

func isSlideshow(note *NoteDetail) bool {
	if len(note.ImageList) == 0 || note.Video == nil || note.Video.Consumer == nil {
		return false
	}
	return strings.HasPrefix(note.Video.Consumer.OriginVideoKey, "slides_simulate_post/")
}

func extractVideo(ctx *models.ExtractorContext, media *models.Media, note *NoteDetail) error {
	if note.Video == nil || note.Video.Media == nil || note.Video.Media.Stream == nil {
		return util.ErrUnavailable
	}

	stream := note.Video.Media.Stream
	duration := note.Video.Media.Duration / 1000

	if duration == 0 {
		for _, e := range slices.Concat(stream.H265, stream.H264, stream.Av1) {
			duration = max(duration, e.Duration/1000)
		}
	}

	item := media.NewItem()

	addStreams := func(entries []*StreamEntry) {
		for _, entry := range entries {
			urls := streamURLs(entry)
			if len(urls) == 0 {
				continue
			}
			formatID := fmt.Sprintf("hevc-%s-%dx%d", entry.Quality, entry.Width, entry.Height)
			item.AddFormats(&models.MediaFormat{
				Type:       database.MediaTypeVideo,
				FormatID:   formatID,
				URL:        urls,
				VideoCodec: database.MediaCodecAvc,
				AudioCodec: database.MediaCodecAac,
				Width:      entry.Width,
				Height:     entry.Height,
				Duration:   int32(duration),
			})
		}
	}

	addStreams(stream.H265)
	addStreams(stream.Av1)

	if len(item.Formats) == 0 {
		for _, entry := range stream.H264 {
			urls := streamURLs(entry)
			if len(urls) == 0 {
				continue
			}
			formatID := fmt.Sprintf("h264-%s-%dx%d", entry.Quality, entry.Width, entry.Height)
			item.AddFormats(&models.MediaFormat{
				Type:       database.MediaTypeVideo,
				FormatID:   formatID,
				URL:        urls,
				VideoCodec: database.MediaCodecAvc,
				AudioCodec: database.MediaCodecAac,
				Width:      entry.Width,
				Height:     entry.Height,
				Duration:   int32(duration),
			})
		}
	}

	if len(item.Formats) == 0 {
		ctx.Warnf("no video streams found in note %s", note.ID)
		return util.ErrUnavailable
	}

	return nil
}

func extractImages(media *models.Media, note *NoteDetail) error {
	if len(note.ImageList) == 0 {
		return util.ErrUnavailable
	}

	for _, img := range note.ImageList {
		imgURL := bestImageURL(img)
		if imgURL == "" {
			continue
		}
		item := media.NewItem()
		item.AddFormats(&models.MediaFormat{
			Type:     database.MediaTypePhoto,
			FormatID: "image",
			URL:      []string{imgURL},
		})
		// img + clip should be better
		if img.LivePhoto && img.Stream != nil {
			addLiveVideo(media, img.Stream)
		}
	}

	return nil
}

func addLiveVideo(media *models.Media, stream *StreamInfo) {
	var item *models.MediaItem
	for _, entry := range stream.H264 {
		urls := streamURLs(entry)
		if len(urls) == 0 {
			continue
		}
		if item == nil {
			item = media.NewItem()
		}
		item.AddFormats(&models.MediaFormat{
			Type:       database.MediaTypeVideo,
			FormatID:   "live-h264-" + entry.Quality,
			URL:        urls,
			VideoCodec: database.MediaCodecAvc,
			AudioCodec: database.MediaCodecAac,
			Width:      entry.Width,
			Height:     entry.Height,
		})
	}
}
