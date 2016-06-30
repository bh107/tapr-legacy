package library

import "sync/atomic"

type DriveStatistics struct {
	// Current drive throughput in B/s
	CurrentThroughput uint64

	// Number of attached streams
	AttachedStreams uint64
}

func (stats *DriveStatistics) IncrementAttachedStreams(delta uint64) {
	atomic.AddUint64(&stats.AttachedStreams, delta)
}

func (stats *DriveStatistics) DecrementAttachedStreams(delta uint64) {
	atomic.AddUint64(&stats.AttachedStreams, ^uint64(delta-1))
}

func (stats *DriveStatistics) UpdateCurrentThroughput(to uint64) {
	atomic.StoreUint64(&stats.CurrentThroughput, to)
}
