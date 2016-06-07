package policy

import (
	"net/http"
	"strconv"

	"golang.org/x/net/context"
)

type Policy struct {
	AcknowledgedWrite bool

	ParallelWrite struct {
		Level int
	}

	Exclusive bool
}

func Construct(req *http.Request) (*Policy, error) {
	pol := &Policy{}

	var v string

	if v = req.Header.Get("Acknowledged-Write"); v == "yes" {
		pol.AcknowledgedWrite = true
	}

	v = req.Header.Get("Parallel-Write")
	n, err := strconv.Atoi(v)
	if err != nil {
		return nil, err
	}

	pol.ParallelWrite.Level = n

	if v = req.Header.Get("Exclusive"); v == "yes" {
		pol.Exclusive = true
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
