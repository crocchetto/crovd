package reddit

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/govdbot/govd/internal/database"
	"github.com/govdbot/govd/internal/logger"
	"github.com/govdbot/govd/internal/models"
	"github.com/govdbot/govd/internal/util"

	"github.com/bytedance/sonic"
)

var baseHost = []string{"reddit", "redditmedia.com"}

var ShortIDExtractor = &models.Extractor{
	ID:          "reddit",
	DisplayName: "Reddit (redd.it)",

	URLPattern: regexp.MustCompile(`https?://(?:www\.)?redd\.it/(?P<id>[a-z0-9]+)`),
	Host:       []string{"redd"},

	Redirect: true,

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		return &models.ExtractorResponse{
			URL: "https://www.reddit.com/comments/" + ctx.ContentID,
		}, nil
	},
}

var ShortExtractor = &models.Extractor{
	ID:          "reddit",
	DisplayName: "Reddit (Short)",

	URLPattern: regexp.MustCompile(`https?://(?P<host>(?:\w+\.)?reddit(?:media)?\.com)/(?P<slug>(?:(?:r|u|user)/[^/]+/)?s/(?P<id>[^/?#&]+))`),
	Host:       baseHost,

	Redirect: true,

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		// follow the reddit redirect
		location, err1 := GetShortLocation(ctx)
		if err1 == nil {
			return &models.ExtractorResponse{
				URL: location,
			}, nil
		}
		// resolve the share link with vxreddit
		location, err2 := GetVxRedditLocation(ctx)
		if err2 == nil {
			return &models.ExtractorResponse{
				URL: location,
			}, nil
		}
		return nil, fmt.Errorf("all methods failed: %w; %w", err1, err2)
	},
}

func GetShortLocation(ctx *models.ExtractorContext) (string, error) {
	resp, err := ctx.Fetch(
		http.MethodGet,
		ctx.ContentURL,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	location := resp.Request.URL.String()
	if !Extractor.URLPattern.MatchString(location) {
		return "", fmt.Errorf("redirect did not lead to a post: %s", resp.Status)
	}
	return location, nil
}

var Extractor = &models.Extractor{
	ID:          "reddit",
	DisplayName: "Reddit",

	URLPattern: regexp.MustCompile(`https?://(?P<host>(?:\w+\.)?reddit(?:media)?\.com)/(?P<slug>(?:(?:r|u|user)/[^/]+/)?comments/(?P<id>[^/?#&]+))`),
	Host:       baseHost,

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		// get media from reddit API
		media, err1 := MediaFromAPI(ctx)
		if err1 == nil {
			return &models.ExtractorResponse{
				Media: media,
			}, nil
		}
		// get media from vxreddit
		media, err2 := MediaFromVxReddit(ctx)
		if err2 == nil {
			return &models.ExtractorResponse{
				Media: media,
			}, nil
		}
		return nil, fmt.Errorf("all methods failed: %w; %w", err1, err2)
	},
}

func MediaFromAPI(ctx *models.ExtractorContext) (*models.Media, error) {
	host := ctx.MatchGroups["host"]
	slug := ctx.MatchGroups["slug"]

	manifest, err := GetRedditData(ctx, host, slug, false)
	if err != nil {
		return nil, err
	}

	if len(manifest) == 0 || len(manifest[0].Data.Children) == 0 {
		return nil, fmt.Errorf("no data found in reddit response")
	}

	data := manifest[0].Data.Children[0].Data
	title := data.Title
	isNsfw := data.Over18

	media := ctx.NewMedia()
	if isNsfw {
		media.SetNSFW()
	}
	media.SetCaption(title)

	if !data.IsVideo {
		// check for single photo
		if data.Preview != nil && len(data.Preview.Images) > 0 {
			item := media.NewItem()
			image := data.Preview.Images[0]

			if data.Preview.VideoPreview != nil {
				formats, err := GetHLSFormats(
					ctx,
					data.Preview.VideoPreview.FallbackURL,
					data.Preview.VideoPreview.Duration,
				)
				if err != nil {
					return nil, err
				}
				item.AddFormats(formats...)

				return media, nil
			}

			// check for MP4 variant (animated GIF)
			if image.Variants.MP4 != nil {
				item.AddFormats(&models.MediaFormat{
					FormatID:   "gif",
					Type:       database.MediaTypeVideo,
					VideoCodec: database.MediaCodecAvc,
					AudioCodec: database.MediaCodecAac,
					URL:        []string{util.UnescapeURL(image.Variants.MP4.Source.URL)},
				})

				return media, nil
			}

			// regular photo
			item.AddFormats(&models.MediaFormat{
				FormatID: "photo",
				Type:     database.MediaTypePhoto,
				URL:      []string{util.UnescapeURL(image.Source.URL)},
			})

			return media, nil
		}

		// check for gallery/collection
		if len(data.MediaMetadata) > 0 {
			// known issue: collection is unordered
			collection := data.MediaMetadata

			for _, obj := range collection {
				item := media.NewItem()

				switch obj.Type {
				case "Image":
					item.AddFormats(&models.MediaFormat{
						FormatID: "photo",
						Type:     database.MediaTypePhoto,
						URL:      []string{util.UnescapeURL(obj.Media.URL)},
					})
				case "AnimatedImage":
					item.AddFormats(&models.MediaFormat{
						FormatID:   "video",
						Type:       database.MediaTypeVideo,
						VideoCodec: database.MediaCodecAvc,
						AudioCodec: database.MediaCodecAac,
						URL:        []string{util.UnescapeURL(obj.Media.MP4)},
					})
				}
			}

			return media, nil
		}
	} else {
		item := media.NewItem()
		var redditVideo *Video

		if data.Media != nil && data.Media.Video != nil {
			redditVideo = data.Media.Video
		} else if data.SecureMedia != nil && data.SecureMedia.Video != nil {
			redditVideo = data.SecureMedia.Video
		}

		if redditVideo != nil {
			formats, err := GetHLSFormats(
				ctx,
				redditVideo.FallbackURL,
				redditVideo.Duration,
			)
			if err != nil {
				return nil, err
			}
			item.AddFormats(formats...)

			return media, nil
		}
	}

	// no media found
	return nil, nil
}

func GetRedditData(
	ctx *models.ExtractorContext,
	host string,
	slug string,
	raise bool,
) (Response, error) {
	url := fmt.Sprintf("https://%s/%s/.json", host, slug)

	resp, err := ctx.Fetch(
		http.MethodGet,
		url, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if raise {
			return nil, fmt.Errorf("failed to get reddit data: %s", resp.Status)
		}
		// try with alternative domain
		altHost := "old.reddit.com"
		if host == "old.reddit.com" {
			altHost = "www.reddit.com"
		}

		return GetRedditData(ctx, altHost, slug, true)
	}

	// debugging
	logger.WriteFile("reddit_api_response", resp)

	var response Response
	decoder := sonic.ConfigFastest.NewDecoder(resp.Body)
	err = decoder.Decode(&response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response, nil
}
