// Copyright 2016 Weaveworks Ltd.
// Use of this source code is governed by a Apache License Version 2.0 license
// that can be found at https://github.com/weaveworks/flux/blob/master/LICENSE

package agent

// How did this update get triggered?
type Cause struct {
	Message string
	User    string
}

// A tagged union for all (both) kinds of update. The type is just so
// we know how to decode the rest of the struct.
type Spec struct {
	Type  string      `json:"type"`
	Cause Cause       `json:"cause"`
	Spec  interface{} `json:"spec"`
}
