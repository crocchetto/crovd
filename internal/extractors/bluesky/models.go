package bluesky

type PostThreadResponse struct {
	Thread *ThreadView `json:"thread"`
}

type ThreadView struct {
	Post *PostView `json:"post"`
}

type PostView struct {
	URI    string      `json:"uri"`
	Author *Author     `json:"author"`
	Record *PostRecord `json:"record"`
	Embed  *EmbedView  `json:"embed"`
}

type Author struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
}

type PostRecord struct {
	Text string `json:"text"`
}

type EmbedView struct {
	Type         string       `json:"$type"`
	Images       []*ImageView `json:"images"`
	Playlist     string       `json:"playlist"`
	Thumbnail    string       `json:"thumbnail"`
	AspectRatio  *AspectRatio `json:"aspectRatio"`
	Presentation string       `json:"presentation"`
}

type ImageView struct {
	Fullsize    string       `json:"fullsize"`
	Thumb       string       `json:"thumb"`
	Alt         string       `json:"alt"`
	AspectRatio *AspectRatio `json:"aspectRatio"`
}

type AspectRatio struct {
	Width  int32 `json:"width"`
	Height int32 `json:"height"`
}
