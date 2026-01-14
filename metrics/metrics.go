package metrics

import "sync/atomic"

type Metrics struct{
	Writes uint64
	Reads uint64
	Deletes uint64
	ReplicationFailures uint64
}

func (m *Metrics) IncWrites(){
	atomic.AddUint64(&m.Writes,1)
}

func (m *Metrics) IncReads(){
	atomic.AddUint64(&m.Reads,1)
}

func (m *Metrics) IncDeletes(){
	atomic.AddUint64(&m.Deletes,1)
}

func (m *Metrics) IncReplicationFailures(){
	atomic.AddUint64(&m.ReplicationFailures,1)
}

func (m *Metrics) Snapshot() map[string]uint64{
	return map[string]uint64{
		"writes":atomic.LoadUint64(&m.Writes),
		"reads":atomic.LoadUint64(&m.Reads),
		"deletes":atomic.LoadUint64(&m.Deletes),
		"replication_failures":atomic.LoadUint64(&m.ReplicationFailures),
	}
}

