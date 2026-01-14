package cluster

import "time"

const(
	HeartBeatInterval = 1 * time.Second
	LeaderTimeout = 3 * time.Second
)

