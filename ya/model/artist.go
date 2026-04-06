package model

type Artist struct {
	ID   FlexibleID `json:"id"`
	Name string     `json:"name"`
}
