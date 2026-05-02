package completesession

type Response struct {
	UploadID  string `json:"upload_id"`
	Completed bool   `json:"completed"`
}
