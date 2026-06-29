package bilibili

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/govdbot/govd/internal/database"
	"github.com/govdbot/govd/internal/models"
	"github.com/govdbot/govd/internal/util"
)

var ShortExtractor = &models.Extractor{
	ID:          "bilibili",
	DisplayName: "Bilibili (Short Link)",

	URLPattern: regexp.MustCompile(
		`https?://b23\.tv/(?P<id>[a-zA-Z0-9]+)`,
	),
	Host:     []string{"b23"},
	Redirect: true,

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		redirectURL, err := ctx.FetchLocation(ctx.ContentURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to follow b23.tv redirect: %w", err)
		}
		redirectURL = strings.ReplaceAll(redirectURL, "m.bilibili.com", "www.bilibili.com")
		return &models.ExtractorResponse{URL: redirectURL}, nil
	},
}

var Extractor = &models.Extractor{
	ID:          "bilibili",
	DisplayName: "Bilibili",

	URLPattern: regexp.MustCompile(
		`https?://(?:www\.|m\.)?bilibili\.com/video/(?P<id>BV[a-zA-Z0-9]+)`,
	),
	Host: []string{"bilibili"},

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

var BangumiExtractor = &models.Extractor{
	ID:          "bilibili",
	DisplayName: "Bilibili Bangumi",

	URLPattern: regexp.MustCompile(
		`https?://(?:www\.)?bilibili\.com/bangumi/play/ep(?P<id>\d+)`,
	),
	Host: []string{"bilibili"},

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		media, err := GetBangumiMedia(ctx)
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
	bvid := ctx.ContentID

	info, err := GetVideoInfo(ctx, bvid)
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	playData, err := GetPlayURL(ctx, bvid, info.CID)
	if err != nil {
		return nil, fmt.Errorf("failed to get play url: %w", err)
	}

	if len(playData.DURL) == 0 {
		return nil, util.ErrUnavailable
	}

	media := ctx.NewMedia()
	media.SetCaption(info.Title)

	return buildMedia(media, playData, []string{info.Pic})
}

func GetBangumiMedia(ctx *models.ExtractorContext) (*models.Media, error) {
	epID := ctx.ContentID

	ep, err := GetBangumiInfo(ctx, epID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bangumi info: %w", err)
	}

	playData, err := GetBangumiPlayURL(ctx, epID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bangumi play url: %w", err)
	}

	if len(playData.DURL) == 0 {
		return nil, util.ErrUnavailable
	}

	media := ctx.NewMedia()

	title := ep.LongTitle
	if title == "" {
		title = ep.Title
	}
	media.SetCaption(title)

	return buildMedia(media, playData, []string{ep.Cover})
}

func buildMedia(media *models.Media, playData *PlayURLData, thumbnailURLs []string) (*models.Media, error) {
	item := media.NewItem()
	duration := int32(playData.Timelength / 1000)

	var thumbnailURL []string
	for _, u := range thumbnailURLs {
		if u != "" {
			thumbnailURL = append(thumbnailURL, u)
		}
	}

	for _, durl := range playData.DURL {
		urls := buildURLList(durl)
		if len(urls) == 0 {
			continue
		}
		item.AddFormats(&models.MediaFormat{
			Type:         database.MediaTypeVideo,
			FormatID:     fmt.Sprintf("mp4_%d", playData.Quality),
			URL:          urls,
			VideoCodec:   database.MediaCodecAvc,
			AudioCodec:   database.MediaCodecAac,
			Duration:     duration,
			ThumbnailURL: thumbnailURL,
			DownloadSettings: &models.DownloadSettings{
				Headers: map[string]string{
					"Referer":    "https://www.bilibili.com/",
					"Origin":     "https://www.bilibili.com",
					"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
				},
				Cookies: util.GetExtractorCookies("bilibili"),
			},
		})
	}

	if len(item.Formats) == 0 {
		return nil, util.ErrUnavailable
	}

	return media, nil
}
