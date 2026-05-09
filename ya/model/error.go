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

	ResultError struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	} `json:"result"`
}

func (e *ErrorResponse) IsError() bool {
	return e.APIError.Name != "" || e.ResultError.Name != ""
}

func (e *ErrorResponse) Error() string {
	if e.APIError.Message != "" {
		return e.APIError.Message
	}
	if e.APIError.Name != "" {
		return e.APIError.Name
	}
	if e.ResultError.Message != "" {
		return e.ResultError.Message
	}
	return e.ResultError.Name
}
