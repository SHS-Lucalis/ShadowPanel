package chmod

type chmodRequest struct {
	Disk  string      `json:"disk"`
	Mode  int         `json:"mode"`
	Items []chmodItem `json:"items"`
}

type chmodItem struct {
	Path string `json:"path"`
}
