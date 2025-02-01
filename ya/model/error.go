package model

type ErrorResponse struct {
	InvocationInfo struct {
		ReqID              string `json:"req-id"`
		Hostname           string `json:"hostname"`
		ExecDurationMillis int    `json:"exec-duration-millis"`
	} `json:"invocationInfo"`

	APIError struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	} `json:"error"`
}

func (e *ErrorResponse) IsError() bool {
	return e.APIError.Name != ""
}

func (e *ErrorResponse) Error() string {
	if e.APIError.Message != "" {
		return e.APIError.Message
	}
	return e.APIError.Name
}
