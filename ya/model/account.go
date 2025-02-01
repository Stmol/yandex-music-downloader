package model

import "time"

type Account struct {
	Birthday         string    `json:"birthday,omitempty"`
	DisplayName      string    `json:"displayName,omitempty"`
	FirstName        string    `json:"firstName,omitempty"`
	FullName         string    `json:"fullName,omitempty"`
	HostedUser       bool      `json:"hostedUser,omitempty"`
	Login            string    `json:"login,omitempty"`
	Now              time.Time `json:"now"`
	Region           int       `json:"region,omitempty"`
	RegisteredAt     time.Time `json:"registeredAt,omitempty"`
	SecondName       string    `json:"secondName,omitempty"`
	ServiceAvailable bool      `json:"serviceAvailable"`
	Uid              int       `json:"uid,omitempty"`
}
