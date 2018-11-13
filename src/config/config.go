package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

var Config *config

type domain struct {
	key   string
	value string
}

type config struct {
	HttpPort     uint16 `json:"http_port"`
	K8sMasterURL string `json:"k8s_master_url"`
	K8sConfig    string `json:"k8s_config"`
	BaseDomain   string `json:"base_domain"`
	HostedZoneID string `json:"hosted_zone_id"`
}

func LoadConfig(path string) *config {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("LoadConfig:Error: Failed reading configuration from %s\nError: %s\n", path, err.Error())
	}

	s := &config{}
	err = json.Unmarshal(data, s)
	if err != nil {
		log.Fatalf("LoadConfig:Error: Failed unmarshalling json from config file Error: %s\nRaw data: %v\n", err.Error(),
			string(data))
	}
	return s
}
