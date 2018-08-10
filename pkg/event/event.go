// Copyright 2016 Weaveworks Ltd.
// Use of this source code is governed by a Apache License Version 2.0 license
// that can be found at https://github.com/weaveworks/flux/blob/master/LICENSE

package event

import (
	"time"

	"github.com/choerodon/choerodon-agent/pkg/resource"
)

// These are all the types of events.
const (
	EventSync = "sync"
)

type EventID int64

type Event struct {
	// ID is a UUID for this event. Will be auto-set when saving if blank.
	ID EventID `json:"id"`

	ResourceIDs []resource.ResourceID `json:"resourceIDs"`

	// Type is the type of event, usually "release" for now, but could be other
	// things later
	Type string `json:"type"`

	// StartedAt is the time the event began.
	StartedAt time.Time `json:"startedAt"`

	// EndedAt is the time the event ended. For instantaneous events, this will
	// be the same as StartedAt.
	EndedAt time.Time `json:"endedAt"`

	// Metadata is Event.Type-specific metadata. If an event has no metadata,
	// this will be nil.
	Metadata EventMetadata `json:"metadata,omitempty"`
}

// EventMetadata is a type safety trick used to make sure that Metadata field
// of Event is always a pointer, so that consumers can cast without being
// concerned about encountering a value type instead. It works by virtue of the
// fact that the method is only defined for pointer receivers; the actual
// method chosen is entirely arbitary.
type EventMetadata interface {
	Type() string
}

type ResourceError struct {
	ID    resource.ResourceID `json:"id,omitempty"`
	Path  string `json:"path,omitempty"`
	Error string `json:"error,omitempty"`
	Commit string `json:"commit,omitempty"`
}

// Commit represents the commit information in a sync event. We could
// use git.Commit, but that would lead to an import cycle, and may
// anyway represent coupling (of an internal API to serialised data)
// that we don't want.
type Commit struct {
	Revision string `json:"revision"`
	Message  string `json:"message"`
}

type FileCommit struct {
	File string `json:"file"`
	Commit string `json:"commit"`
}


// SyncEventMetadata is the metadata for when new a commit is synced to the cluster
type SyncEventMetadata struct {
	Commit string `json:"commit,omitempty"`
	// Per-resource errors
	Errors []ResourceError `json:"errors,omitempty"`
	FileCommits []FileCommit `json:"filesCommit,omitempty"`
}

func (sem *SyncEventMetadata) Type() string {
	return EventSync
}
