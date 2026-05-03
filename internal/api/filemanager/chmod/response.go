package chmod

type chmodResponse struct {
	Result resultResponse `json:"result"`
}

type resultResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func newChmodResponse() chmodResponse {
	return chmodResponse{
		Result: resultResponse{
			Status:  "success",
			Message: "Permissions changed!",
		},
	}
}
