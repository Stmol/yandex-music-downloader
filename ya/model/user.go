package model

type User struct {
	UID         int    `json:"uid"`
	Login       string `json:"login"`
	DisplayName string `json:"displayName"`
	FullName    string `json:"fullName"`
	Sex         string `json:"sex"`
	Verified    bool   `json:"verified"`
}
