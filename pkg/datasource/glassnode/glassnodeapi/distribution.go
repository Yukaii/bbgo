package glassnodeapi

import "github.com/c9s/requestgen"

//go:generate requestgen -method GET -type DistributionRequest -url "/v1/metrics/distribution/:metric" -responseType Response
type DistributionRequest struct {
	Client requestgen.AuthenticatedAPIClient

	Asset           string   `param:"a,required,query"`
	Since           int64    `param:"s,query"`
	Until           int64    `param:"u,query"`
	Interval        Interval `param:"i,query"`
	Format          Format   `param:"f,query"`
	TimestampFormat string   `param:"timestamp_format,query"`

	Metric string `param:"metric,slug"`
}
