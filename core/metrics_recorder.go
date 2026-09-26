package core

// MetricsRecorder records one finished HTTP response.
//
// The prerouter metrics middleware calls Record after the next handler
// returns, with the ResponseRecorder holding the status code, the bytes
// written and the request duration. The collectors and the daemon that serve
// the values live outside the framework, in restinpieces-metrics.
type MetricsRecorder interface {
	Record(rec *ResponseRecorder)
}
