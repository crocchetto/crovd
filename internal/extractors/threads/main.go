package threads

import (
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/govdbot/govd/internal/logger"
	"github.com/govdbot/govd/internal/models"
	"github.com/govdbot/govd/internal/networking"
)

var ShareExtractor = &models.Extractor{
	ID:          "threads",
	DisplayName: "Threads (Share)",

	URLPattern: regexp.MustCompile(`https:\/\/(www\.)?threads\.[^\/]+\/share\/(?P<id>[a-zA-Z0-9_-]+)`),
	Host:       []string{"threads"},

	Redirect: true,

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		finalURL, err := ctx.FetchLocation(
			ctx.ContentURL,
			&networking.RequestParams{Headers: shareHeaders},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to follow share redirect: %w", err)
		}
		return &models.ExtractorResponse{URL: finalURL}, nil
	},
}

var Extractor = &models.Extractor{
	ID:          "threads",
	DisplayName: "Threads",

	URLPattern: regexp.MustCompile(`https:\/\/(www\.)?threads\.[^\/]+\/(?:(?:@[^\/]+)\/)?p(?:ost)?\/(?P<id>[a-zA-Z0-9_-]+)`),
	Host:       []string{"threads"},

	GetFunc: func(ctx *models.ExtractorContext) (*models.ExtractorResponse, error) {
		media, err := GetEmbedMedia(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get media: %w", err)
		}
		return &models.ExtractorResponse{Media: media}, nil
	},
}

func GetEmbedMedia(ctx *models.ExtractorContext) (*models.Media, error) {
	embedURL := fmt.Sprintf(
		"https://www.threads.net/@_/post/%s/embed",
		ctx.ContentID,
	)
	resp, err := ctx.Fetch(
		http.MethodGet,
		embedURL,
		&networking.RequestParams{
			Headers: headers,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	logger.WriteFile("threads_embed", resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get embed media: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return ParseEmbedMedia(ctx, body)
}
