package xiaohongshu

type InitialState struct {
	Note *NoteState `json:"note"`
}

type NoteState struct {
	NoteDetailMap map[string]*NoteDetailWrapper `json:"noteDetailMap"`
}

type NoteDetailWrapper struct {
	Note *NoteDetail `json:"note"`
}

type NoteDetail struct {
	ID        string       `json:"id"`
	Title     string       `json:"title"`
	Desc      string       `json:"desc"`
	Type      string       `json:"type"`
	Video     *VideoInfo   `json:"video"`
	ImageList []*ImageItem `json:"imageList"`
}

type VideoInfo struct {
	Media *VideoMedia `json:"media"`
}

type VideoMedia struct {
	Stream   *StreamInfo `json:"stream"`
	Duration int64       `json:"duration"`
}

type StreamInfo struct {
	H264 []*StreamEntry `json:"h264"`
	H265 []*StreamEntry `json:"h265"`
	Av1  []*StreamEntry `json:"av1"`
}

type StreamEntry struct {
	MasterURL  string   `json:"masterUrl"`
	BackupURLs []string `json:"backupUrls"`
	VideoCodec string   `json:"videoCodec"`
	AudioCodec string   `json:"audioCodec"`
	Width      int32    `json:"width"`
	Height     int32    `json:"height"`
	Bitrate    int64    `json:"videoBitrate"`
	Size       int64    `json:"size"`
	Quality    string   `json:"qualityType"`
	Format     string   `json:"format"`
}

type ImageItem struct {
	InfoList []*ImageInfo `json:"infoList"`
}

type ImageInfo struct {
	ImageScene string `json:"imageScene"`
	URL        string `json:"url"`
}
