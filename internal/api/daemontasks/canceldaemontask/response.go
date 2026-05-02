package canceldaemontask

type cancelDaemonTaskResponse struct {
	Message string `json:"message"`
}

func newCancelDaemonTaskResponse() *cancelDaemonTaskResponse {
	return &cancelDaemonTaskResponse{
		Message: "success",
	}
}
