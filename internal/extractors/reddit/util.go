package reddit

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"

	"github.com/PuerkitoBio/goquery"
	"github.com/govdbot/govd/internal/database"
	"github.com/govdbot/govd/internal/logger"
	"github.com/govdbot/govd/internal/models"
	"github.com/govdbot/govd/internal/networking"
	"github.com/govdbot/govd/internal/util/parser/m3u8"
)

const (
	hlsURLFormat = "https://v.redd.it/%s/HLSPlaylist.m3u8"

	vxRedditBase = "https://vxreddit.com/"
)

var (
	videoURLPattern = regexp.MustCompile(`https?://v\.redd\.it/([^/]+)`)

	vxRedditHeaders = map[string]string{
		"User-Agent": "TelegramBot (like TwitterBot)",
	}
)

func GetHLSFormats(
	ctx *models.ExtractorContext,
	videoURL string,
	duration int32,
) ([]*models.MediaFormat, error) {
	matches := videoURLPattern.FindStringSubmatch(videoURL)
	if len(matches) < 2 {
		return nil, nil
	}

	videoID := matches[1]
	hlsURL := fmt.Sprintf(hlsURLFormat, videoID)

	formats, err := m3u8.ParseM3U8FromURL(ctx, hlsURL, nil)
	if err != nil {
		return nil, err
	}

	if duration > 0 {
		for _, format := range formats {
			format.Duration = duration
		}
	}

	return formats, nil
}

// last resort
func GetVxRedditPage(ctx *models.ExtractorContext) (*goquery.Document, error) {
	resp, err := ctx.Fetch(
		http.MethodGet,
		vxRedditBase+ctx.MatchGroups["slug"]+"/",
		&networking.RequestParams{
			Headers: vxRedditHeaders,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	logger.WriteFile("vxreddit_response", resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get vxreddit page: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to parse vxreddit page: %w", err)
	}
	if doc.Find("meta[property='og:url']").Length() == 0 {
		return nil, fmt.Errorf("vxreddit page has no post data")
	}
	return doc, nil
}

func MediaFromVxReddit(ctx *models.ExtractorContext) (*models.Media, error) {
	doc, err := GetVxRedditPage(ctx)
	if err != nil {
		return nil, err
	}
	return ParseVxRedditMedia(ctx, doc)
}

func ParseVxRedditMedia(ctx *models.ExtractorContext, doc *goquery.Document) (*models.Media, error) {
	meta := func(attr, name string) string {
		return doc.Find("meta["+attr+"='"+name+"']").First().AttrOr("content", "")
	}
	card := meta("name", "twitter:card")
	if card == "" {
		return nil, fmt.Errorf("vxreddit: %s", meta("property", "og:description"))
	}

	media := ctx.NewMedia()
	caption := meta("property", "og:title")
	if caption == "vxReddit" {
		caption = meta("property", "og:description")
	}
	media.SetCaption(caption)

	videoTag := doc.Find("meta[property='og:video'], meta[property='og:video:secure_url']").First()
	if card == "player" && videoTag.Length() > 0 {
		videoURL, err := url.Parse(videoTag.AttrOr("content", ""))
		if err != nil {
			return nil, fmt.Errorf("failed to parse video url: %w", err)
		}
		formats, err := GetHLSFormats(ctx, videoURL.Query().Get("video_url"), 0)
		if err != nil {
			return nil, err
		}
		if len(formats) == 0 {
			return nil, fmt.Errorf("no reddit video found")
		}
		media.NewItem().AddFormats(formats...)
		return media, nil
	}

	seen := make(map[string]bool)
	doc.Find("meta[property='og:image']").Each(func(_ int, s *goquery.Selection) {
		imageURL := s.AttrOr("content", "")
		if imageURL == "" || seen[imageURL] {
			return
		}
		seen[imageURL] = true
		media.NewItem().AddFormats(&models.MediaFormat{
			FormatID: "photo",
			Type:     database.MediaTypePhoto,
			URL:      []string{imageURL},
		})
	})
	if len(media.Items) == 0 {
		if card == "summary" {
			return nil, nil
		}
		return nil, fmt.Errorf("no media found in vxreddit page")
	}
	return media, nil
}

func GetVxRedditLocation(ctx *models.ExtractorContext) (string, error) {
	doc, err := GetVxRedditPage(ctx)
	if err != nil {
		return "", err
	}
	location := doc.Find("meta[property='og:url']").AttrOr("content", "")
	if !Extractor.URLPattern.MatchString(location) {
		return "", fmt.Errorf("vxreddit did not resolve the share link")
	}
	return location, nil
}
