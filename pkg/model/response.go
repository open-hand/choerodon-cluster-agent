package model

type Response struct {
	Key     string `json:"key,omitempty"`
	Type    string `json:"type,omitempty"`
	Payload string `json:"payload,omitempty"`
}
