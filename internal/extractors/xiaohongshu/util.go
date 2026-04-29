package xiaohongshu

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/govdbot/govd/internal/logger"
	"github.com/govdbot/govd/internal/models"
	"github.com/govdbot/govd/internal/networking"
	"github.com/govdbot/govd/internal/util"
)

var (
	initialStatePattern = regexp.MustCompile(
		`window\.__INITIAL_STATE__\s*=\s*({.*?})\s*(?:</script>|;)`,
	)

	webHeaders = map[string]string{
		"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"Referer":         "https://www.xiaohongshu.com/",
		"Sec-Fetch-Dest":  "document",
		"Sec-Fetch-Mode":  "navigate",
		"Sec-Fetch-Site":  "none",
	}
)

func GetNoteWeb(ctx *models.ExtractorContext, noteID string) (*NoteDetail, error) {
	pageURL := ctx.ContentURL
	pageURL = strings.ReplaceAll(pageURL, "/discovery/item/", "/explore/")

	cookies := util.GetExtractorCookies("xiaohongshu")
	if len(cookies) == 0 {
		ctx.Warnf("no cookies found in private/cookies/xiaohongshu.txt — requests may fail")
	}

	resp, err := ctx.Fetch(
		http.MethodGet,
		pageURL,
		&networking.RequestParams{
			Headers: webHeaders,
			Cookies: cookies,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch note page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return ParseInitialState(body, noteID)
}

func ParseInitialState(body []byte, noteID string) (*NoteDetail, error) {
	matches := initialStatePattern.FindSubmatch(body)
	if len(matches) < 2 {
		return nil, fmt.Errorf("initial state not found in page HTML")
	}

	cleaned := strings.ReplaceAll(string(matches[1]), "undefined", "null")

	var state InitialState
	if err := sonic.ConfigFastest.Unmarshal([]byte(cleaned), &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal initial state: %w", err)
	}
	logger.WriteFile("xhs_initial_state", state)

	if state.Note == nil || len(state.Note.NoteDetailMap) == 0 {
		return nil, util.ErrUnavailable
	}

	wrapper, ok := state.Note.NoteDetailMap[noteID]
	if !ok {
		for _, v := range state.Note.NoteDetailMap {
			wrapper = v
			break
		}
	}

	if wrapper == nil || wrapper.Note == nil {
		return nil, util.ErrUnavailable
	}

	logger.WriteFile("xhs_note_detail", wrapper.Note)
	return wrapper.Note, nil
}

func removeImageWatermark(u string) string {
	if idx := strings.Index(u, "!"); idx != -1 {
		u = u[:idx]
	}

	const marker1 = "notes_pre_post/"
	if idx := strings.Index(u, marker1); idx != -1 {
		hash := u[idx+len(marker1):]
		return "https://sns-img-hw.xhscdn.com/notes_pre_post/" + hash + "?imageView2/2/w/0/format/jpg"
	}

	const marker2 = "spectrum/"
	if idx := strings.Index(u, marker2); idx != -1 {
		hash := u[idx+len(marker2):]
		return "https://sns-img-hw.xhscdn.com/spectrum/" + hash + "?imageView2/2/w/0/format/jpg"
	}

	parts := strings.Split(u, "/")
	if len(parts) > 0 {
		hash := parts[len(parts)-1]
		if hash != "" {
			return "https://sns-img-hw.xhscdn.com/" + hash + "?imageView2/2/w/0/format/jpg"
		}
	}

	return u
}

func bestImageURL(item *ImageItem) string {
	if item == nil || len(item.InfoList) == 0 {
		return ""
	}

	priority := map[string]int{
		"MFULL_HD": 0,
		"WB_DFT":   1,
		"WB_PRV":   2,
	}

	best := item.InfoList[0]
	bestPrio := 999

	for _, info := range item.InfoList {
		if p, ok := priority[info.ImageScene]; ok && p < bestPrio {
			bestPrio = p
			best = info
		}
	}

	return removeImageWatermark(best.URL)
}

func streamURLs(entry *StreamEntry) []string {
	seen := make(map[string]struct{})
	var urls []string

	add := func(u string) {
		if idx := strings.Index(u, "?"); idx != -1 {
			u = u[:idx]
		}
		if u != "" {
			if _, dup := seen[u]; !dup {
				seen[u] = struct{}{}
				urls = append(urls, u)
			}
		}
	}

	for _, u := range entry.BackupURLs {
		add(u)
	}
	add(entry.MasterURL)

	return urls
}
