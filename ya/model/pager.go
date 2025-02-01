package model

type Pager struct {
	Total   int `json:"total"`
	Page    int `json:"page"`
	PerPage int `json:"perPage"`
}
