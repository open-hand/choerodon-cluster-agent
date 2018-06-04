package model

import "fmt"

type Response struct {
	Key     string `json:"key,omitempty"`
	Type    string `json:"type,omitempty"`
	Payload string `json:"payload,omitempty"`
}


func (r *Response) String() string {
	return fmt.Sprintf("{key: %s, type: %s}: %s", r.Key, r.Type, r.Payload)
}
