package knet

import "kinz/kmetrics"

// Metric names (Prometheus convention: counters end with _total).
const (
	mConnTotal       = "kinz_conns_total"
	mConnActive      = "kinz_conns_active"
	mConnClosed      = "kinz_conns_closed_total"
	mConnRejected    = "kinz_conns_rejected_total"
	mMsgsReceived    = "kinz_msgs_received_total"
	mMsgsSent        = "kinz_msgs_sent_total"
	mBytesIn         = "kinz_bytes_in_total"
	mBytesOut        = "kinz_bytes_out_total"
	mPanics          = "kinz_handler_panics_total"
	mQueueFull       = "kinz_queue_full_total"
	mHeartbeatMissed = "kinz_heartbeat_missed_total"
	mHandleDuration  = "kinz_msg_handle_duration_seconds"
)

// connMetrics are pre-fetched per-connection counters (hot paths avoid map
// lookups). Nil means metrics are disabled.
type connMetrics struct {
	active   *kmetrics.Gauge
	closed   *kmetrics.Counter
	bytesIn  *kmetrics.Counter
	bytesOut *kmetrics.Counter
	msgsRecv *kmetrics.Counter
	msgsSent *kmetrics.Counter
}

// handlerMetrics are the dispatch metrics.
type handlerMetrics struct {
	panics    *kmetrics.Counter
	duration  *kmetrics.Histogram
	queueFull *kmetrics.Counter
}

// heartbeatMetrics are the liveness metrics.
type heartbeatMetrics struct {
	missed *kmetrics.Counter
}

func newConnMetrics(reg *kmetrics.Registry) *connMetrics {
	if reg == nil {
		return nil
	}
	return &connMetrics{
		active:   reg.Gauge(mConnActive, "Currently active connections"),
		closed:   reg.Counter(mConnClosed, "Connections closed"),
		bytesIn:  reg.Counter(mBytesIn, "Bytes received from peers"),
		bytesOut: reg.Counter(mBytesOut, "Bytes sent to peers"),
		msgsRecv: reg.Counter(mMsgsReceived, "Messages received"),
		msgsSent: reg.Counter(mMsgsSent, "Messages sent"),
	}
}

func newHandlerMetrics(reg *kmetrics.Registry) *handlerMetrics {
	if reg == nil {
		return nil
	}
	return &handlerMetrics{
		panics:    reg.Counter(mPanics, "Panics recovered in handlers"),
		duration:  reg.Histogram(mHandleDuration, "Message handler duration in seconds", nil),
		queueFull: reg.Counter(mQueueFull, "Worker queue full (backpressure) events"),
	}
}

func newHeartbeatMetrics(reg *kmetrics.Registry) *heartbeatMetrics {
	if reg == nil {
		return nil
	}
	return &heartbeatMetrics{missed: reg.Counter(mHeartbeatMissed, "Heartbeat liveness timeouts")}
}
