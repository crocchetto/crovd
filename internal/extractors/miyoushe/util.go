package miyoushe

import (
	"fmt"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/govdbot/govd/internal/models"
	"github.com/govdbot/govd/internal/networking"
)

const (
	apiBase = "https://bbs-api.mihoyo.com/post/wapi/getPostFull"
)

var webHeaders = map[string]string{
	"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Accept":          "application/json, text/plain, */*",
	"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
	"Referer":         "https://www.miyoushe.com/",
	"Origin":          "https://www.miyoushe.com",
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

func bestResolution(vod *VodItem) *Resolution {
	if len(vod.Resolutions) == 0 {
		return nil
	}

	best := vod.Resolutions[0]
	for _, r := range vod.Resolutions[1:] {
		if r.Height > best.Height {
			best = r
		}
	}
	return best
}

func extractImagesFromStructuredContent(structuredContent string) ([]string, error) {
	if structuredContent == "" {
		return nil, nil
	}

	var items []StructuredContentItem
	if err := sonic.ConfigFastest.UnmarshalFromString(structuredContent, &items); err != nil {
		return nil, fmt.Errorf("failed to parse structured_content: %w", err)
	}

	urls := make([]string, 0, len(items))
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
