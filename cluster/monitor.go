package cluster

import "time"

func (n *Node) startLeaderMonitor(){
	if n.IsLeader(){
		return
	}

	go func ()  {
		ticker := time.NewTicker(HeartBeatInterval)

		defer ticker.Stop()

		for{
			select{
			case <- ticker.C:
				if time.Since(n.lastHeartbeat)>LeaderTimeout{
					n.startElection()
					return
				}
			case <-n.stopCh:
				return

			}
		}
	}()
}