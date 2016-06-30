package policy

import (
	"net/http"
	"time"

	"golang.org/x/net/context"
)

var DefaultPolicy = NewDefaultPolicy()

type Policy struct {
	AcknowledgedWrite bool
	WriteGroup        string
	Exclusive         bool
	ExclusiveTimeout  time.Duration
}

func NewDefaultPolicy() *Policy {
	return &Policy{
		AcknowledgedWrite: true,
		WriteGroup:        "none",
		Exclusive:         false,
		ExclusiveTimeout:  0,
	}
}

func (pol *Policy) Parallel() bool {
	return pol.WriteGroup != "none"
}

func Construct(req *http.Request) (*Policy, error) {
	pol := NewDefaultPolicy()

	var v string

	if v = req.Header.Get("Acknowledged-Write"); v == "no" {
		pol.AcknowledgedWrite = false
	}

	if v = req.Header.Get("Write-Group"); v != "" {
		pol.WriteGroup = v
	}

	if v = req.Header.Get("Exclusive"); v == "yes" {
		pol.Exclusive = true
	}

	if v = req.Header.Get("Exclusive-Timeout"); v != "" {
		timeout, err := time.ParseDuration(v)
		if err != nil {
			return nil, err
		}

		pol.ExclusiveTimeout = timeout
	}

	return pol, nil
}

type contextKey struct {
	name string
}

func (k *contextKey) Sring() string { return "policy context value " + k.name }

var PolicyContextKey = &contextKey{"policy"}

func Wrap(ctx context.Context, policy *Policy) context.Context {
	return context.WithValue(ctx, PolicyContextKey, policy)
}

func Unwrap(ctx context.Context) (*Policy, bool) {
	policy, ok := ctx.Value(PolicyContextKey).(*Policy)
	return policy, ok
}
