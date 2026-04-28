package hoyolab

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/govdbot/govd/internal/database"
	"github.com/govdbot/govd/internal/models"
	"github.com/govdbot/govd/internal/util"
)

var ShortExtractor = &models.Extractor{
	ID:          "hoyolab",
	DisplayName: "HoYoLAB (Short Link)",

	URLPattern: regexp.MustCompile(
		`https?://hoyo\.link/(?P<id>[a-zA-Z0-9]+)(?:\?q=(?P<q>[^\s&]+))?`,
	),
	Host:     []string{"hoyo"},
	Redirect: true,

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		redirectURL, err := ctx.FetchLocation(ctx.ContentURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to follow hoyo.link redirect: %w", err)
		}

		parsed, err := url.Parse(redirectURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse redirect url: %w", err)
		}

		if innerURL := parsed.Query().Get("url"); innerURL != "" {
			innerURL = strings.ReplaceAll(innerURL, "m.hoyolab.com/#/article/", "www.hoyolab.com/article/")
			if idx := strings.Index(innerURL, "/?"); idx != -1 {
				innerURL = innerURL[:idx]
			}
			innerURL = strings.TrimRight(innerURL, "/")
			return &models.ExtractorResponse{URL: innerURL}, nil
		}

		return &models.ExtractorResponse{URL: redirectURL}, nil
	},
}

var Extractor = &models.Extractor{
	ID:          "hoyolab",
	DisplayName: "HoYoLAB",

	URLPattern: regexp.MustCompile(
		`https?://(?:www\.)?hoyolab\.com/article/(?P<id>\d+)`,
	),
	Host: []string{"hoyolab"},

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
	postID := ctx.ContentID

	wrapper, err := GetPost(ctx, postID)
	if err != nil {
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	post := wrapper.Post
	media := ctx.NewMedia()
	media.SetCaption(post.Subject)

	switch post.ViewType {
	case 5:
		if err := extractVideo(media, wrapper); err != nil {
			return nil, err
		}
	default:
		if err := extractImages(media, wrapper); err != nil {
			return nil, err
		}
	}

	return media, nil
}

func extractVideo(media *models.Media, wrapper *PostWrapper) error {
	if wrapper.Video == nil || wrapper.Video.URL == "" {
		return util.ErrUnavailable
	}

	video := wrapper.Video
	videoURL, width, height, duration := bestVideoResolution(video)

	item := media.NewItem()
	item.AddFormats(&models.MediaFormat{
		Type:         database.MediaTypeVideo,
		FormatID:     "mp4",
		URL:          []string{videoURL},
		VideoCodec:   database.MediaCodecAvc,
		AudioCodec:   database.MediaCodecAac,
		Width:        width,
		Height:       height,
		Duration:     duration,
		ThumbnailURL: []string{video.Cover},
	})

	return nil
}

func extractImages(media *models.Media, wrapper *PostWrapper) error {
	var urls []string

	for _, cover := range wrapper.CoverList {
		if cover.URL != "" {
			urls = append(urls, cover.URL)
		}
	}

	inlineURLs, err := extractImagesFromStructuredContent(wrapper.Post.StructuredContent)
	if err != nil {
		return fmt.Errorf("failed to extract images: %w", err)
	}
	urls = append(urls, inlineURLs...)

	if len(urls) == 0 {
		return util.ErrUnavailable
	}

	for _, imgURL := range urls {
		item := media.NewItem()
		item.AddFormats(&models.MediaFormat{
			Type:     database.MediaTypePhoto,
			FormatID: "image",
			URL:      []string{imgURL},
		})
	}

	return nil
}
