package bluesky

import (
	"fmt"
	"regexp"

	"github.com/govdbot/govd/internal/database"
	"github.com/govdbot/govd/internal/models"
	"github.com/govdbot/govd/internal/util"
	"github.com/govdbot/govd/internal/util/parser/m3u8"
)

var Extractor = &models.Extractor{
	ID:          "bluesky",
	DisplayName: "Bluesky",

	URLPattern: regexp.MustCompile(
	    `https?://(?:fx)?bsky\.app/profile/(?P<handle>[^/]+)/post/(?P<id>[a-zA-Z0-9]+)`,

	),
	Host: []string{"bsky", "fxbsky"},

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
	post, err := GetPost(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	if post.Embed == nil {
		return nil, util.ErrUnavailable
	}

	media := ctx.NewMedia()

	if post.Record != nil && post.Record.Text != "" {
		media.SetCaption(post.Record.Text)
	}

	switch {
	case isVideoEmbed(post.Embed):
		if err := extractVideo(ctx, media, post.Embed); err != nil {
			return nil, err
		}
	case isImagesEmbed(post.Embed):
		if err := extractImages(media, post.Embed); err != nil {
			return nil, err
		}
	default:
		return nil, util.ErrUnavailable
	}

	return media, nil
}

func extractVideo(ctx *models.ExtractorContext, media *models.Media, embed *EmbedView) error {
	if embed.Playlist == "" {
		return util.ErrUnavailable
	}

	formats, err := m3u8.ParseM3U8FromURL(ctx, embed.Playlist, nil)
	if err != nil {
		return fmt.Errorf("failed to parse m3u8: %w", err)
	}

	if len(formats) == 0 {
		return util.ErrUnavailable
	}

	item := media.NewItem()

	var thumbnailURL []string
	if embed.Thumbnail != "" {
		thumbnailURL = []string{embed.Thumbnail}
	}

	for _, format := range formats {
		format.ThumbnailURL = thumbnailURL
		item.AddFormats(format)
	}

	return nil
}

func extractImages(media *models.Media, embed *EmbedView) error {
	if len(embed.Images) == 0 {
		return util.ErrUnavailable
	}

	for _, img := range embed.Images {
		imgURL := img.Fullsize
		if imgURL == "" {
			imgURL = img.Thumb
		}
		if imgURL == "" {
			continue
		}

		item := media.NewItem()

		var width, height int32
		if img.AspectRatio != nil {
			width = img.AspectRatio.Width
			height = img.AspectRatio.Height
		}

		item.AddFormats(&models.MediaFormat{
			Type:     database.MediaTypePhoto,
			FormatID: "image",
			URL:      []string{imgURL},
			Width:    width,
			Height:   height,
		})
	}

	return nil
}
