package bilibili

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/govdbot/govd/internal/models"
	"github.com/govdbot/govd/internal/networking"
	"github.com/govdbot/govd/internal/util"
)

const (
	viewAPIBase    = "https://api.bilibili.com/x/web-interface/view"
	playURLAPIBase = "https://api.bilibili.com/x/player/playurl"

	qualityHD = 80
)

var webHeaders = map[string]string{
	"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Accept":          "application/json, text/plain, */*",
	"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
	"Referer":         "https://www.bilibili.com/",
	"Origin":          "https://www.bilibili.com",
}

func GetVideoInfo(ctx *models.ExtractorContext, bvid string) (*VideoInfo, error) {
	reqURL := fmt.Sprintf("%s?bvid=%s", viewAPIBase, bvid)
	cookies := util.GetExtractorCookies("bilibili")

	resp, err := ctx.Fetch(
		http.MethodGet,
		reqURL,
		&networking.RequestParams{
			Headers: webHeaders,
			Cookies: cookies,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch video info: %w", err)
	}
	defer resp.Body.Close()

	var infoResp VideoInfoResponse
	if err := sonic.ConfigFastest.NewDecoder(resp.Body).Decode(&infoResp); err != nil {
		return nil, fmt.Errorf("failed to parse video info: %w", err)
	}

	if infoResp.Code != 0 {
		return nil, fmt.Errorf("API error %d: %s", infoResp.Code, infoResp.Message)
	}

	return &infoResp.Data, nil
}

func GetPlayURL(ctx *models.ExtractorContext, bvid string, cid int64) (*PlayURLData, error) {
	quality := qualityHD
	cookies := util.GetExtractorCookies("bilibili")
	if len(cookies) > 0 {
		quality = 116
	}

	reqURL := fmt.Sprintf(
		"%s?bvid=%s&cid=%d&qn=%d&fnval=0&fnver=0&fourk=0",
		playURLAPIBase, bvid, cid, quality,
	)

	resp, err := ctx.Fetch(
		http.MethodGet,
		reqURL,
		&networking.RequestParams{
			Headers: webHeaders,
			Cookies: cookies,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch play url: %w", err)
	}
	defer resp.Body.Close()

	var playResp PlayURLResponse
	if err := sonic.ConfigFastest.NewDecoder(resp.Body).Decode(&playResp); err != nil {
		return nil, fmt.Errorf("failed to parse play url response: %w", err)
	}

	if playResp.Code != 0 {
		return nil, fmt.Errorf("API error %d: %s", playResp.Code, playResp.Message)
	}

	return &playResp.Data, nil
}

func GetBangumiInfo(ctx *models.ExtractorContext, epID string) (*BangumiEpisode, error) {
	apiURL := "https://api.bilibili.com/pgc/view/web/season?ep_id=" + epID
	resp, err := ctx.Fetch(http.MethodGet, apiURL, &networking.RequestParams{
		Headers: webHeaders,
		Cookies: util.GetExtractorCookies("bilibili"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bangumi info: %w", err)
	}
	defer resp.Body.Close()

	var result BangumiInfoResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode bangumi info: %w", err)
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("bangumi info error: %s", result.Message)
	}
	for _, ep := range result.Result.Episodes {
		if strconv.Itoa(ep.ID) == epID {
			return &ep, nil
		}
	}
	return nil, util.ErrUnavailable
}

func GetBangumiPlayURL(ctx *models.ExtractorContext, epID string) (*PlayURLData, error) {
	cookies := util.GetExtractorCookies("bilibili")
	quality := qualityHD
	if len(cookies) > 0 {
		quality = 116
	}
	apiURL := fmt.Sprintf(
		"https://api.bilibili.com/pgc/player/web/playurl?ep_id=%s&qn=%d&fnval=0&fnver=0&fourk=0",
		epID, quality,
	)
	resp, err := ctx.Fetch(http.MethodGet, apiURL, &networking.RequestParams{
		Headers: webHeaders,
		Cookies: cookies,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bangumi playurl: %w", err)
	}
	defer resp.Body.Close()

	var result BangumiPlayURLResponse
	if err := sonic.ConfigDefault.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode bangumi playurl: %w", err)
	}
	if result.Code != 0 {
		if strings.Contains(result.Message, "地区") {
			return nil, util.ErrGeoRestrictedContent
		}
		return nil, fmt.Errorf("bangumi playurl error: %s", result.Message)
	}
	return result.Result, nil
}

func buildURLList(item *DURLItem) []string {
	seen := make(map[string]struct{})
	urls := make([]string, 0, 1+len(item.BackupURL))

	add := func(u string) {
		if u != "" {
			if _, dup := seen[u]; !dup {
				seen[u] = struct{}{}
				urls = append(urls, u)
			}
		}
	}

	add(item.URL)
	for _, u := range item.BackupURL {
		add(u)
	}

	return urls
}
