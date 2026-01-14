package config


import (
	"encoding/json"
	"os"
)

type Config struct{
	NodeId string

	DataDir string

	Role string

	ListenAddr string
	Peers []string

	EnableMetrics bool

	SnapshotIntervalSeconds int
}

func LoadFromFile(path string)(*Config,error){
	data,err := os.ReadFile(path)

	if err != nil{
		return nil,err
	}

	var cfg Config

	if err := json.Unmarshal(data,&cfg);err != nil{
		return nil,err
	}

	return &cfg,nil
}

func (c *Config) ApplyEnvOverrides() {
	if v := os.Getenv("NODE_ID"); v != "" {
		c.NodeId = v
	}
	if v := os.Getenv("DATADIR"); v != "" {
		c.DataDir = v
	}
	if v := os.Getenv("ROLE"); v != "" {
		c.Role = v
	}
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		c.ListenAddr = v
	}
}
