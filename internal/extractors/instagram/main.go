package instagram

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"regexp"

	"github.com/bytedance/sonic"
	"github.com/govdbot/govd/internal/database"
	"github.com/govdbot/govd/internal/logger"
	"github.com/govdbot/govd/internal/models"
	"github.com/govdbot/govd/internal/networking"
	"github.com/govdbot/govd/internal/util"
)

var instagramHost = []string{"instagram", "ddinstagram", "kkinstagram"}

var Extractor = &models.Extractor{
	ID:          "instagram",
	DisplayName: "Instagram",

	URLPattern: regexp.MustCompile(`https:\/\/(www\.)?(?:dd|kk)?instagram\.com\/(?:[a-zA-Z0-9_.]+\/)?(reels?|p|tv)\/(?P<id>[a-zA-Z0-9_-]+)`),
	Host:       instagramHost,
	Redirect:   false,

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		// method 1: get media from GQL web API
		media, err1 := GetGQLMedia(ctx)
		if err1 == nil {
			return &models.ExtractorResponse{
				Media: media,
			}, nil
		}
		// method 2: get media from embed page
		media, err2 := GetEmbedMedia(ctx)
		if err2 == nil {
			return &models.ExtractorResponse{
				Media: media,
			}, nil
		}
		// method 3: get media from 3rd party service (unlikely) (oh my god)
		media, err3 := GetIGramPost(ctx)
		if err3 == nil {
			return &models.ExtractorResponse{
				Media: media,
			}, nil
		}
		return nil, fmt.Errorf("all methods failed: %w; %w; %w", err1, err2, err3)
	},
}

var StoriesExtractor = &models.Extractor{
	ID:          "instagram",
	DisplayName: "Instagram Stories",

	URLPattern: regexp.MustCompile(`https:\/\/(www\.)?(?:dd|kk)?instagram\.com\/stories\/[a-zA-Z0-9._]+\/(?P<id>\d+)`),
	Host:       instagramHost,
	Hidden:     true,

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		media, err1 := GetStoryEmbedMedia(ctx)
		if err1 == nil {
			return &models.ExtractorResponse{
				Media: media,
			}, nil
		}
		media, err2 := GetIGramStory(ctx)
		if err2 == nil {
			return &models.ExtractorResponse{
				Media: media,
			}, nil
		}
		return nil, fmt.Errorf("all methods failed: %w; %w", err1, err2)
	},
}

func GetStoryEmbedMedia(ctx *models.ExtractorContext) (*models.Media, error) {
	shortcode, err := MediaIDToShortcode(ctx.ContentID)
	if err != nil {
		return nil, err
	}
	postCtx := *ctx
	postCtx.ContentID = shortcode
	media, err := GetEmbedMedia(&postCtx)
	if err != nil {
		return nil, err
	}
	media.ContentID = ctx.ContentID
	media.ContentURL = ctx.ContentURL
	return media, nil
}

var ShareURLExtractor = &models.Extractor{
	ID:          "instagram",
	DisplayName: "Instagram (Share)",

	URLPattern: regexp.MustCompile(`https?:\/\/(www\.)?(?:dd|kk)?instagram\.com\/share\/((reels?|video|s|p)\/)?(?P<id>[^\/\?]+)`),
	Host:       instagramHost,

	Redirect: true,

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		redirectURL, err := ctx.FetchLocation(ctx.ContentURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get url location: %w", err)
		}
		return &models.ExtractorResponse{URL: redirectURL}, nil
	},
}

func GetGQLMedia(ctx *models.ExtractorContext) (*models.Media, error) {
	graphData, err := GetGQLData(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get graph data: %w", err)
	}
	return ParseGQLMedia(ctx, graphData.ShortcodeMedia)
}

func GetEmbedMedia(ctx *models.ExtractorContext) (*models.Media, error) {
	embedURL := fmt.Sprintf(
		"https://www.instagram.com/p/%s/embed/captioned",
		ctx.ContentID,
	)
	var body []byte
	var err error
	for attempt := range embedRetries {
		ctx.Debugf("getting embed page (attempt %d/%d)", attempt+1, embedRetries)
		body, err = getEmbedPage(ctx, embedURL)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	graphData, err := ParseEmbedGQL(body)
	if err == nil {
		return ParseGQLMedia(ctx, graphData)
	}
	if !errors.Is(err, errEmbedNoContext) {
		return nil, fmt.Errorf("failed to parse embed page: %w", err)
	}
	media, htmlErr := ParseEmbedHTML(ctx, body)
	if htmlErr == nil {
		return media, nil
	}
	media, err = GetPostPageMedia(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to parse embed html: %w; %w", htmlErr, err)
	}
	return media, nil
}

func getEmbedPage(ctx *models.ExtractorContext, embedURL string) ([]byte, error) {
	resp, err := ctx.Fetch(
		http.MethodGet,
		embedURL,
		&networking.RequestParams{
			Headers: webHeaders,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	logger.WriteFile("ig_embed_response", resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get embed page: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return body, nil
}

func GetPostPageMedia(ctx *models.ExtractorContext) (*models.Media, error) {
	resp, err := ctx.Fetch(
		http.MethodGet,
		"https://www.instagram.com/p/"+ctx.ContentID+"/",
		&networking.RequestParams{
			Headers: webHeaders,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	logger.WriteFile("ig_post_page", resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get post page: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	polaris, err := ParsePostPage(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse post page: %w", err)
	}
	if polaris.IfNotGatedLoggedOut == nil {
		return nil, fmt.Errorf("media not available in post page (%s)", polaris.Typename)
	}
	return ParseV1Media(ctx, polaris.IfNotGatedLoggedOut)
}

func GetIGramPost(ctx *models.ExtractorContext) (*models.Media, error) {
	details, err := GetPostFromIGram(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get post: %w", err)
	}

	media := ctx.NewMedia()
	for _, obj := range details.Items {
		item := media.NewItem()
		if len(obj.URL) == 0 {
			return nil, fmt.Errorf("no media url found")
		}
		urlObj := obj.URL[0]
		contentURL, err := GetCDNURL(urlObj.URL)
		if err != nil {
			return nil, err
		}
		thumbnailURL, err := GetCDNURL(obj.Thumb)
		if err != nil {
			return nil, err
		}
		fileExt := urlObj.Ext
		formatID := urlObj.Type
		switch fileExt {
		case "mp4":
			item.AddFormats(&models.MediaFormat{
				FormatID:     formatID,
				Type:         database.MediaTypeVideo,
				URL:          []string{contentURL},
				VideoCodec:   database.MediaCodecAvc,
				AudioCodec:   database.MediaCodecAac,
				ThumbnailURL: []string{thumbnailURL},
			},
			)
		case "jpg", "png", "webp", "heic", "jpeg":
			item.AddFormats(&models.MediaFormat{
				Type:     database.MediaTypePhoto,
				FormatID: formatID,
				URL:      []string{contentURL},
			})
		default:
			return nil, fmt.Errorf("unknown format: %s", fileExt)
		}
	}

	if len(media.Items) == 0 {
		return nil, fmt.Errorf("no media found")
	}

	return media, nil
}

func GetIGramStory(ctx *models.ExtractorContext) (*models.Media, error) {
	details, err := GetStoryFromIGram(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get story: %w", err)
	}

	if len(details.Result) == 0 {
		return nil, util.ErrUnavailable
	}
	result := details.Result[0]
	isVideo := len(result.VideoVersions) > 0

	media := ctx.NewMedia()
	item := media.NewItem()
	if isVideo {
		video := GetBestVideoVersion(result.VideoVersions)
		item.AddFormats(&models.MediaFormat{
			FormatID:   "video",
			Type:       database.MediaTypeVideo,
			URL:        []string{video.URL},
			VideoCodec: database.MediaCodecAvc,
			AudioCodec: database.MediaCodecAac,
		})
	} else {
		image := GetBestCandidate(result.ImageVersions.Candidates)
		item.AddFormats(&models.MediaFormat{
			Type:     database.MediaTypePhoto,
			FormatID: "photo",
			URL:      []string{image.URL},
		})
	}

	if len(media.Items) == 0 {
		return nil, fmt.Errorf("no media found")
	}

	return media, nil
}

func GetPostFromIGram(ctx *models.ExtractorContext) (*IGramResponse, error) {
	contentURL := "https://www.instagram.com/p/" + ctx.ContentID + "/"
	apiURL := fmt.Sprintf("https://%s/api/convert", igramHostname)
	payload, err := IGramBodyFromURL(contentURL)
	if err != nil {
		return nil, fmt.Errorf("failed to build signed payload: %w", err)
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	maps.Copy(headers, igramHeaders)

	resp, err := igramFetch(
		ctx, apiURL,
		&networking.RequestParams{
			Body:    payload,
			Headers: headers,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	logger.WriteFile("ig_3party_response", resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get response: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	response, err := ParseIGramResponse(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return response, nil
}

func GetStoryFromIGram(ctx *models.ExtractorContext) (*IGramStoryResponse, error) {
	apiURL := fmt.Sprintf("https://%s/api/v1/instagram/story", igramHostname)
	payload, err := IGramBodyFromParams(map[string]string{
		"url": ctx.ContentURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build signed payload: %w", err)
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}
	maps.Copy(headers, igramHeaders)

	resp, err := igramFetch(
		ctx, apiURL,
		&networking.RequestParams{
			Body:    payload,
			Headers: headers,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	logger.WriteFile("ig_story_3party_response", resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get response: %s", resp.Status)
	}

	var story IGramStoryResponse
	decoder := sonic.ConfigFastest.NewDecoder(resp.Body)
	err = decoder.Decode(&story)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &story, nil
}

// no cookies for you (why wasn't this implemented sooner?)
func igramFetch(
	ctx *models.ExtractorContext,
	url string,
	params *networking.RequestParams,
) (*http.Response, error) {
	client := *ctx.HTTPClient
	client.Cookies = nil
	return client.FetchWithContext(
		ctx.Context, http.MethodPost,
		url, params,
	)
}
