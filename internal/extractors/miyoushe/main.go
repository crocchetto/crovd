package miyoushe

import (
	"fmt"
	"regexp"

	"github.com/govdbot/govd/internal/database"
	"github.com/govdbot/govd/internal/models"
	"github.com/govdbot/govd/internal/util"
)

var Extractor = &models.Extractor{
	ID:          "miyoushe",
	DisplayName: "Miyoushe",

	URLPattern: regexp.MustCompile(
		`https?://(?:www\.|m\.)?miyoushe\.com/[^/?#]+(?:\?[^#]*)?(?:#/article/|/article/)(?P<id>\d+)`,
	),
	Host: []string{"miyoushe"},

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

	if len(wrapper.VodList) > 0 {
		if err := extractVideos(media, wrapper); err != nil {
			return nil, err
		}
		return media, nil
	}

	if err := extractImages(media, wrapper); err != nil {
		return nil, err
	}

	return media, nil
}

func extractVideos(media *models.Media, wrapper *PostWrapper) error {
	for _, vod := range wrapper.VodList {
		res := bestResolution(vod)
		if res == nil {
			continue
		}

		item := media.NewItem()
		duration := int32(vod.Duration / 1000)

		var thumbnailURL []string
		if vod.Cover != "" {
			thumbnailURL = []string{vod.Cover}
		}

		item.AddFormats(&models.MediaFormat{
			Type:         database.MediaTypeVideo,
			FormatID:     fmt.Sprintf("mp4_%s", res.Definition),
			URL:          []string{res.URL},
			VideoCodec:   database.MediaCodecAvc,
			AudioCodec:   database.MediaCodecAac,
			Width:        res.Width,
			Height:       res.Height,
			Duration:     duration,
			ThumbnailURL: thumbnailURL,
			DownloadSettings: &models.DownloadSettings{
				Headers: map[string]string{
					"Referer": "https://www.miyoushe.com/",
				},
				NumConnections: 1,
			},
		})
	}

	if len(media.Items) == 0 {
		return util.ErrUnavailable
	}

	return nil
}

func extractImages(media *models.Media, wrapper *PostWrapper) error {
	seen := make(map[string]struct{})
	var urls []string

	add := func(u string) {
		if u != "" {
			if _, dup := seen[u]; !dup {
				seen[u] = struct{}{}
				urls = append(urls, u)
			}
		}
	}

	if wrapper.Cover != nil {
		add(wrapper.Cover.URL)
	}

	inlineURLs, err := extractImagesFromStructuredContent(wrapper.Post.StructuredContent)
	if err != nil {
		return fmt.Errorf("failed to extract images: %w", err)
	}
	for _, u := range inlineURLs {
		add(u)
	}

	if len(urls) == 0 || len(inlineURLs) == 0 {
		for _, img := range wrapper.ImageList {
			add(img.URL)
		}
	}

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
