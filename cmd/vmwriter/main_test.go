package main

import (
	"testing"

	vmupstreams "github.dev.pages/infrastructure/vmwriter/internal/upstreams"
	utility "github.dev.pages/infrastructure/vmwriter/internal/utility"
)

func TestAbc(t *testing.T) {

	//t.Error() // to indicate test failed
}

func TestConfig(t *testing.T) {
	var config utility.VConfig

	config.AWSPollingIntervalSeconds = 4
	config.AWSPortTag = "test"
	config.AWSRegion = "us-west-2"
	config.AWSSearchTag = "Cluster"
	config.AWSSearchTagValue = "victoriametrix"
	config.AWSURITag = "/api/v1/writer"
	config.HTTPTimeOut = 5
	config.ServicePollingSeconds = 30

	_, err := utility.GetAWSInstancesByTag(&config)
	if err != nil {
		t.Error()
	}

}

func TestVMWriter(t *testing.T) {
	var config utility.VConfig

	config.AWSPollingIntervalSeconds = 4
	config.AWSPortTag = "test"
	config.AWSRegion = "us-west-2"
	config.AWSSearchTag = "Cluster"
	config.AWSSearchTagValue = "victoriametrix"
	config.AWSURITag = "/api/v1/writer"
	config.HTTPTimeOut = 5
	config.ServicePollingSeconds = 30

	var vmUpstreams vmupstreams.VMUpstreams
	vmUpstreams.VMUpstreamsInitialize(&config)

	activeHosts, err := vmUpstreams.GetActiveHostList()
	if err != nil {
		t.Error()
	}

	if len(activeHosts) == 0 {
		t.Error()
	}

}
