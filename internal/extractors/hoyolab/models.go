package hoyolab

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
	Video     *VideoDetail `json:"video"`
	CoverList []*CoverItem `json:"cover_list"`
}

type PostDetail struct {
	PostID            string `json:"post_id"`
	Subject           string `json:"subject"`
	Desc              string `json:"desc"`
	Content           string `json:"content"`
	StructuredContent string `json:"structured_content"`
	ViewType          int    `json:"view_type"`
}

type CoverItem struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
	Format string `json:"format"`
}

type VideoDetail struct {
	ID         string             `json:"id"`
	Cover      string             `json:"cover"`
	URL        string             `json:"url"`
	IsVertical bool               `json:"is_vertical"`
	Resolution []*VideoResolution `json:"resolution"`
}

type VideoResolution struct {
	Name     string `json:"name"`
	Height   string `json:"height"`
	Width    string `json:"width"`
	URL      string `json:"url"`
	Duration string `json:"duration"`
}

type StructuredContentItem struct {
	Insert     any            `json:"insert"`
	Attributes map[string]any `json:"attributes,omitempty"`
}
