package policy

import (
	"net/http"
	"time"

	"golang.org/x/net/context"
)

var DefaultPolicy = &Policy{}

type Policy struct {
	AcknowledgedWrite bool
	WriteGroup        string
	Exclusive         bool
	ExclusiveTimeout  time.Duration
}

func Construct(req *http.Request) (*Policy, error) {
	pol := &Policy{}

	var v string

	if v = req.Header.Get("Acknowledged-Write"); v == "yes" {
		pol.AcknowledgedWrite = true
	}

	pol.WriteGroup = req.Header.Get("Write-Group")

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

type key int

const policyKey key = 0

func Wrap(ctx context.Context, policy *Policy) context.Context {
	return context.WithValue(ctx, policyKey, policy)
}

func Unwrap(ctx context.Context) (*Policy, bool) {
	policy, ok := ctx.Value(policyKey).(*Policy)
	return policy, ok
}
