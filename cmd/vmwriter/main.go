// Provides simple load balancer for prometheus metrics
//
// Project Layout
// SEE: https://eli.thegreenplace.net/2019/simple-go-project-layout-with-modules/
//
// Author: Randall Svancara
//

package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	vmhandlers "github.dev.pages/infrastructure/vmwriter/internal/handlers"
	vmupstreams "github.dev.pages/infrastructure/vmwriter/internal/upstreams"
	utility "github.dev.pages/infrastructure/vmwriter/internal/utility"
)

const publisher = "publisher"
const receiver = "receiver"

func main() {

	var wait time.Duration

	var config utility.VConfig

	// Command flags
	flag.DurationVar(&wait, "graceful-timeout", time.Second*15, "the duration for which the server gracefully wait for existing connections to finish - e.g. 15s or 1m")
	awsRegion := flag.String("region", "us-west-2", "sets regions to look for instances")
	awsSearchTag := flag.String("clustertag", "Cluster", "Tag to look for when looking the metrics cluster.  Default - Cluster")
	awsSearchTagValue := flag.String("clustertagvalue", "victoriametrix", "Value to search for when selecting the metrics cluster. Default - victoriametrix")
	awsURITag := flag.String("clusteruritag", "ClusterVMURI", "Tag to set for upstream URI. Default - api/v1/write")
	awsPortTag := flag.String("clusterporttag", "ClusterVMPort", "Tag to search for upstream port. Default - 8428")
	httpTimeOut := flag.Int("httptimeout", 3, "Sets the http client timeout. Default 3 seconds")
	debug := flag.Bool("debug", false, "sets log level to debug")
	flag.Parse()

	// Load the configuration object
	config.AWSRegion = *awsRegion
	config.AWSSearchTag = *awsSearchTag
	config.AWSSearchTagValue = *awsSearchTagValue
	config.AWSURITag = *awsURITag
	config.AWSPortTag = *awsPortTag
	config.HTTPTimeOut = *httpTimeOut

	// Set the http client timeout to prevent lingering connections and exhaustion of our http thread pool!
	// SEE: https://medium.com/@nate510/don-t-use-go-s-default-http-client-4804cb19f779

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
		log.Error().Err(err).Msg("Quiting, could not get AWS instances")
		os.Exit(0)
	}

	if len(instances) == 0 {
		log.Error().Msg("Could not find any instances for upstreams.  Did you set up your tags for the cluster correctly?")
		os.Exit(0)
	}

	var vmUpstreams vmupstreams.VMUpstreams
	vmUpstreams.VMUpstreamsInitialize(&config)

	// Set up our handlers
	pctx := vmhandlers.PCTXHandlerContext(&vmUpstreams, &config)

	r := mux.NewRouter()

	// Handlers for the web part of this application

	// Index Page
	r.Handle(
		"/",

		http.HandlerFunc(
			pctx.HomeHandler)).Methods("GET")

	// Prometheus Metrics
	r.Handle("/metrics", promhttp.Handler()).Methods("GET")

	// API Handler
	r.Handle(
		"/api/v1/write",
		http.HandlerFunc(
			pctx.PromHandler))

	srv := &http.Server{
		Handler: r,
		Addr:    "0.0.0.0:5000",
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Error().Err(err).Str("service", receiver).Msg("Failed to create http server")
		}
	}()

	// Monitoring thread for changes in AWS
	go func() {
		if err := vmUpstreams.AWSServiceWorker(); err != nil {
			log.Error().Err(err).Str("service", receiver).Msg("Failed to create AWS Service Worker")
		}
	}()

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()

	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.

	srv.Shutdown(ctx)
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	//<-ctx.Done() //if your application should wait for other services
	// to finalize based on context cancellation.
	log.Print("shutting down")
	os.Exit(0)

}
