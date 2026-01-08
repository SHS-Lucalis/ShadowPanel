package pluginstore

import (
	"encoding/json"
	"fmt"
	"io"
)

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}

	return fmt.Sprintf("plugin store API error: HTTP %d", e.StatusCode)
}

func (e *APIError) HTTPStatus() int {
	return e.StatusCode
}

type apiErrorResponse struct {
	Message  string `json:"message"`
	Error    string `json:"error"`
	Status   string `json:"status"`
	HTTPCode int    `json:"http_code"`
}

func parseAPIError(body io.Reader, statusCode int) *APIError {
	var resp apiErrorResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return &APIError{StatusCode: statusCode}
	}

	message := resp.Message
	if message == "" {
		message = resp.Error
	}

	return &APIError{StatusCode: statusCode, Message: message}
}
