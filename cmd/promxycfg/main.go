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
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	utility "github.dev.pages/infrastructure/vmwriter/internal/utility"
)

func main() {

	var config utility.VConfig

	// Command flags
	awsRegion := flag.String("region", "us-west-2", "sets regions to look for instances")
	awsSearchTag := flag.String("clustertag", "Cluster", "Tag to look for when looking the metrics cluster.  Default - Cluster")
	awsSearchTagValue := flag.String("clustertagvalue", "victoriametrix", "Value to search for when selecting the metrics cluster. Default - victoriametrix")
	vmreadport := flag.String("vmreadport", "8428", "Tag to search for upstream port. Default - 8428")
	configpath := flag.String("config", "config.yaml", "Promxy configuration file to update")
	debug := flag.Bool("debug", false, "sets log level to debug")
	flag.Parse()

	// Load the configuration object
	config.AWSRegion = *awsRegion
	config.AWSSearchTag = *awsSearchTag
	config.AWSSearchTagValue = *awsSearchTagValue

	configPath := *configpath
	vmReadPort := *vmreadport

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

	// Open file for reading
	_, err = os.Stat(configPath)

	if err != nil {
		log.Error().Err(err).Msgf("Could not find path %s", configPath)
		os.Exit(0)
	}

	file, err := os.Open(configPath)
	if err != nil {
		log.Error().Err(err).Msgf("Could not find path %s", configPath)
		os.Exit(0)
	}
	found := false
	var out strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		matched, err := regexp.MatchString(`\#\sbegin\s\#`, line)
		if err != nil {
			log.Error().Err(err).Msgf("Error looking for match for start string %s", line)
		}
		if matched == true {
			found = true
			fmt.Fprintln(&out, line)
		}

		matched, err = regexp.MatchString(`\#\send\s\#`, line)
		if err != nil {
			log.Error().Err(err).Msgf("Error looking for match for start string %s", line)
		}
		if matched == true {
			found = false
			for _, i := range instances {
				fmt.Fprintf(&out, "          - %s:%s\n", i.AWSHost, vmReadPort)
			}
		}

		if found == false {
			fmt.Fprintln(&out, line)
		}
	}
	err = file.Close()
	if err != nil {
		log.Error().Err(err).Msgf("Error closing file %s", configPath)
		os.Exit(0)
	}

	tempfile := configPath + ".tmp"
	outfile, err := os.Create(tempfile)
	if err != nil {
		log.Error().Err(err).Msgf("Could not find path %s", tempfile)
		os.Exit(0)
	}

	w := bufio.NewWriter(outfile)

	_, err = w.WriteString(out.String())
	if err != nil {
		log.Error().Err(err).Msgf("Writing contents to file %s", tempfile)
		os.Exit(0)
	}

	err = w.Flush()
	if err != nil {
		log.Error().Err(err).Msgf("Error flushing contents to file %s", tempfile)
		os.Exit(0)
	}

	outfile.Close()
	if err != nil {
		log.Error().Err(err).Msgf("could not close out file %s", tempfile)
		os.Exit(0)
	}

	_, err = copy(configPath, configPath+"-bak")
	if err != nil {
		log.Error().Err(err).Msgf("backing up orginal configuration %s", configPath)
		os.Exit(0)
	}

	_, err = copy(tempfile, configPath)
	if err != nil {
		log.Error().Err(err).Msgf("copying the configuration file %s", configPath)
		os.Exit(0)
	}
}

func copy(src string, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}
