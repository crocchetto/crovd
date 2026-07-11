package miyoushe

type APIResponse struct {
	Retcode int     `json:"retcode"`
	Message string  `json:"message"`
	Data    APIData `json:"data"`
}

type APIData struct {
	Post PostWrapper `json:"post"`
}

type PostWrapper struct {
	Post      PostDetail   `json:"post"`
	Cover     *CoverItem   `json:"cover"`
	ImageList []*ImageItem `json:"image_list"`
	VodList   []*VodItem   `json:"vod_list"`
}

type PostDetail struct {
	PostID            string `json:"post_id"`
	Subject           string `json:"subject"`
	StructuredContent string `json:"structured_content"`
	ViewType          int    `json:"view_type"`
}

type CoverItem struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
	Format string `json:"format"`
}

type ImageItem struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
	Format string `json:"format"`
}

type VodItem struct {
	ID          string        `json:"id"`
	Duration    int64         `json:"duration"`
	Cover       string        `json:"cover"`
	Resolutions []*Resolution `json:"resolutions"`
}

type Resolution struct {
	URL        string `json:"url"`
	Definition string `json:"definition"`
	Height     int32  `json:"height"`
	Width      int32  `json:"width"`
	Bitrate    int64  `json:"bitrate"`
}

type StructuredContentItem struct {
	Insert     any            `json:"insert"`
	Attributes map[string]any `json:"attributes,omitempty"`
}
