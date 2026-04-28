package bluesky

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/govdbot/govd/internal/models"
	"github.com/govdbot/govd/internal/networking"
)

const (
	apiBase = "https://public.api.bsky.app/xrpc/app.bsky.feed.getPostThread"
)

var (
	webHeaders = map[string]string{
		"Accept":          "application/json",
		"Accept-Language": "en-us,en;q=0.9",
	}
)

func GetPost(ctx *models.ExtractorContext) (*PostView, error) {
	handle := ctx.MatchGroups["handle"]
	rkey := ctx.MatchGroups["id"]

	atURI := fmt.Sprintf("at://%s/app.bsky.feed.post/%s", handle, rkey)

	reqURL := fmt.Sprintf("%s?uri=%s", apiBase, atURI)

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

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("post requires authentication")
	}

	var threadResp PostThreadResponse
	if err := sonic.ConfigFastest.NewDecoder(resp.Body).Decode(&threadResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if threadResp.Thread == nil || threadResp.Thread.Post == nil {
		return nil, fmt.Errorf("post not found")
	}

	return threadResp.Thread.Post, nil
}

func isVideoEmbed(embed *EmbedView) bool {
	return strings.HasPrefix(embed.Type, "app.bsky.embed.video")
}

func isImagesEmbed(embed *EmbedView) bool {
	return strings.HasPrefix(embed.Type, "app.bsky.embed.images")
}
