package bilibili

type VideoInfoResponse struct {
	Code    int       `json:"code"`
	Message string    `json:"message"`
	Data    VideoInfo `json:"data"`
}

type VideoInfo struct {
	Aid   int64  `json:"aid"`
	BVid  string `json:"bvid"`
	CID   int64  `json:"cid"`
	Title string `json:"title"`
	Desc  string `json:"desc"`
	Pic   string `json:"pic"`
}

type PlayURLResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    PlayURLData `json:"data"`
}

type PlayURLData struct {
	Quality        int             `json:"quality"`
	Format         string          `json:"format"`
	Timelength     int64           `json:"timelength"`
	AcceptQuality  []int           `json:"accept_quality"`
	SupportFormats []SupportFormat `json:"support_formats"`
	DURL           []*DURLItem     `json:"durl"`
}

type SupportFormat struct {
	Quality     int    `json:"quality"`
	Format      string `json:"format"`
	DisplayDesc string `json:"display_desc"`
}

type DURLItem struct {
	Order     int      `json:"order"`
	Length    int64    `json:"length"`
	Size      int64    `json:"size"`
	URL       string   `json:"url"`
	BackupURL []string `json:"backup_url"`
}

type BangumiEpisode struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	LongTitle string `json:"long_title"`
	Cover     string `json:"cover"`
	Aid       int64  `json:"aid"`
}

type BangumiInfoResult struct {
	Episodes []BangumiEpisode `json:"episodes"`
}

type BangumiInfoResponse struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Result  BangumiInfoResult `json:"result"`
}

type BangumiPlayURLResponse struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Result  *PlayURLData `json:"result"`
}
