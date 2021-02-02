package main

//package main
//List nodes in a cluster and writes out to proxmy configuration file
//This tool can be executed as part of a cron job
//
//This tool updates the following section in the configuration
// promxy:
//  server_groups:
//  # All upstream prometheus service discovery mechanisms are supported with the same
//  # markup, all defined in https://github.com/prometheus/prometheus/blob/master/discovery/config/config.go#L33
//  - static_configs:
//	  - targets:
//		- localhost:8428
//		- localhost:8429
//

import (
	"flag"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	utility "github.dev.pages/infrastructure/vmwriter/internal/utility"
)

func main() {

	var config utility.VConfig

	// Command flags
	awsRegion := flag.String("region", "us-west-2", "sets regions to look for instances")
	awsSearchTag := flag.String("searchtag", "Cluster", "Tag to look for when looking the metrics cluster.  Default - Cluster")
	awsSearchTagValue := flag.String("value", "victoriametrix", "Value to search for when selecting the metrics cluster. Default - victoriametrix")
	debug := flag.Bool("debug", false, "sets log level to debug")
	flag.Parse()

	// Load the configuration object
	config.AWSRegion = *awsRegion
	config.AWSSearchTag = *awsSearchTag
	config.AWSSearchTagValue = *awsSearchTagValue

	// UNIX Time is faster and smaller than most timestamps
	// If you set zerolog.TimeFieldFormat to an empty string,
	// logs will write with UNIX time
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Default level for this example is info, unless debug flag is present
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	// Test that we can talk to AWS and we can find some nodes
	instances, err := utility.GetAWSInstancesByTag(&config)
	if err != nil {
		log.Error().Err(err).Msg("Could not find any AWS Instances")
		os.Exit(0)
	}

	if len(instances) == 0 {
		log.Error().Msg("Could not find any AWS Instances.  Did you set up your tags for the cluster correctly?")
		os.Exit(0)
	}

	for _, i := range instances {
		fmt.Printf("%s,%s,%s\n", i.AWSName, i.AWSInstanceID, i.AWSHost)
	}
}
