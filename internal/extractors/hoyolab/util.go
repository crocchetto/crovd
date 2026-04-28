package hoyolab

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/bytedance/sonic"
	"github.com/govdbot/govd/internal/models"
	"github.com/govdbot/govd/internal/networking"
)

const (
	apiBase = "https://bbs-api-os.hoyolab.com/community/post/wapi/getPostFull"
)

var webHeaders = map[string]string{
	"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Accept":          "application/json, text/plain, */*",
	"Accept-Language": "en-us,en;q=0.9",
	"Origin":          "https://www.hoyolab.com",
	"Referer":         "https://www.hoyolab.com/",
}

func GetPost(ctx *models.ExtractorContext, postID string) (*PostWrapper, error) {
	reqURL := fmt.Sprintf("%s?post_id=%s", apiBase, postID)

	resp, err := ctx.Fetch(
		http.MethodGet,
		reqURL,
		&networking.RequestParams{
			Headers: webHeaders,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch post: %w", err)
	}
	defer resp.Body.Close()

	var apiResp APIResponse
	if err := sonic.ConfigFastest.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Retcode != 0 {
		return nil, fmt.Errorf("API error %d: %s", apiResp.Retcode, apiResp.Message)
	}

	return &apiResp.Data.Post, nil
}

func extractImagesFromStructuredContent(structuredContent string) ([]string, error) {
	if structuredContent == "" {
		return nil, nil
	}

	var items []StructuredContentItem
	if err := sonic.ConfigFastest.UnmarshalFromString(structuredContent, &items); err != nil {
		return nil, fmt.Errorf("failed to parse structured_content: %w", err)
	}

	var urls []string
	for _, item := range items {
		if item.Insert == nil {
			continue
		}
		insertMap, ok := item.Insert.(map[string]any)
		if !ok {
			continue
		}
		imgURL, ok := insertMap["image"].(string)
		if !ok || imgURL == "" {
			continue
		}
		urls = append(urls, imgURL)
	}

	return urls, nil
}

func bestVideoResolution(video *VideoDetail) (url string, width, height int32, duration int32) {
	if len(video.Resolution) == 0 {
		return video.URL, 0, 0, 0
	}

	best := video.Resolution[0]
	bestH, _ := strconv.Atoi(best.Height)

	for _, r := range video.Resolution[1:] {
		h, _ := strconv.Atoi(r.Height)
		if h > bestH {
			bestH = h
			best = r
		}
	}

	w, _ := strconv.Atoi(best.Width)
	dur, _ := strconv.Atoi(best.Duration)
	return best.URL, int32(w), int32(bestH), int32(dur)
}
