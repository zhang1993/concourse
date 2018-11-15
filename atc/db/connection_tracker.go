package db

import (
	"context"
	"runtime/debug"
	"sync"

	"github.com/opentracing/opentracing-go"
)

var GlobalConnectionTracker = NewConnectionTracker()

type ConnectionTracker struct {
	sessions  map[*ConnectionSession]struct{}
	sessionsL *sync.Mutex
}

func NewConnectionTracker() *ConnectionTracker {
	return &ConnectionTracker{
		sessions:  map[*ConnectionSession]struct{}{},
		sessionsL: &sync.Mutex{},
	}
}

func (tracker *ConnectionTracker) Track(ctx context.Context) *ConnectionSession {
	// CC: Track() could take a context that we just propagate...
	span := opentracing.SpanFromContext(ctx)
	span.LogEvent("started")

	session := &ConnectionSession{
		tracker: tracker,
		stack:   string(debug.Stack()),
		ctx:     opentracing.ContextWithSpan(context.Background(), span),
	}

	tracker.sessionsL.Lock()
	tracker.sessions[session] = struct{}{}
	tracker.sessionsL.Unlock()

	return session
}

func (tracker *ConnectionTracker) Current() []string {
	stacks := []string{}

	tracker.sessionsL.Lock()

	for session := range tracker.sessions {
		stacks = append(stacks, session.stack)
	}

	tracker.sessionsL.Unlock()

	return stacks
}

func (tracker *ConnectionTracker) remove(session *ConnectionSession) {
	tracker.sessionsL.Lock()
	delete(tracker.sessions, session)
	tracker.sessionsL.Unlock()
}

type ConnectionSession struct {
	tracker *ConnectionTracker
	ctx     context.Context
	stack   string
}

func (session *ConnectionSession) Release() {
	span := opentracing.SpanFromContext(session.ctx)
	span.LogEvent("release")

	session.tracker.remove(session)

	span.Finish()
}
