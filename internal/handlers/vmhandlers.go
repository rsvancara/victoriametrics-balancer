// Package vmhandlers for providing handlers
package vmhandlers

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog/log"

	vmupstreams "github.dev.pages/infrastructure/vmwriter/internal/upstreams"
	utility "github.dev.pages/infrastructure/vmwriter/internal/utility"
)

// HTTP Thread pool
var pClient *http.Client

// PromHTTPHandlerContext provides context for passing global values to handlers
// such as http thread pools or database handlers
//
// SEE: https://drstearns.github.io/tutorials/gohandlerctx/
type PromHTTPHandlerContext struct {
	pUpstream *vmupstreams.VMUpstreams
	pConfig   *utility.VConfig
}

// Prometheus Metrics
var (
	eventsTotalProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "vmwriter_events_total",
		Help: "The total number of processed events",
	})

	eventsFailedProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "vmwriter_events_failed_total",
		Help: "The total number of processed events failed",
	})

	eventsSucceedProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "vmwriter_events_succeed_total",
		Help: "The total number of processed events succeed",
	})

	requestDurationTimer = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "vm_writer_request_duration_seconds",
		Help:    "Upstream latency in seconds.",
		Buckets: prometheus.LinearBuckets(0.01, 0.01, 10),
	})

	eventsFailedTimeouts = promauto.NewCounter(prometheus.CounterOpts{
		Name: "vmwriter_events_failed_timeout",
		Help: "Total events timedout",
	})
)

const publisher = "publisher"
const receiver = "receiver"

//PCTXHandlerContext constructs a new HandlerContext,
//ensuring that the dependencies are valid values
func PCTXHandlerContext(upstreams *vmupstreams.VMUpstreams, config *utility.VConfig) *PromHTTPHandlerContext {

	if upstreams.UList == nil && len(upstreams.UList) == 0 {
		log.Error().Str("service", receiver).Msg("Could not find a list of upstreams to connect too")
	}

	return &PromHTTPHandlerContext{upstreams, config}
}

// HomeHandler displays home page at /
func (ctx *PromHTTPHandlerContext) HomeHandler(w http.ResponseWriter, r *http.Request) {

	w.Write([]byte("Prometheus Load Balancer - Load Balancing for Everyone!"))
}

// PromHandler handles prometheus metrics at /api/v1/write
func (ctx *PromHTTPHandlerContext) PromHandler(w http.ResponseWriter, r *http.Request) {

	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Str("service", receiver).Msg("Error creating request body")
		eventsFailedProcessed.Inc()
		w.WriteHeader(http.StatusExpectationFailed)
		return
	}

	//log.Debug().Str("service", receiver).Msg("Getting host list")

	hostList, err := ctx.pUpstream.GetActiveHostList()
	if err != nil {
		log.Error().Err(err).Str("service", receiver).Msg("Error getting host list")
	}

	var httpforwards []HTTPForward

	for _, host := range hostList {

		// Forwards to use for upstreams
		var httpforward HTTPForward
		httpforward.URL = host
		httpforward.ReqBody = reqBody
		httpforwards = append(httpforwards, httpforward)
	}

	// Asyncronously send the requests to the upstreams and then
	// wait for the results
	results := asyncHTTPPost(httpforwards)

	for _, result := range results {
		if result != nil && result.response != nil {
			log.Debug().Msgf("Received the following status: %s", result.response.Status)
		}
	}

	w.WriteHeader(http.StatusOK)

}

//HTTPResponse response type
type HTTPResponse struct {
	url      string
	response *http.Response
	err      error
}

//HTTPForward forwarding http type
type HTTPForward struct {
	URL     string
	ReqBody []byte
}

// asyncHttpPost
// SEE: https://matt.aimonetti.net/posts/2012-11-real-life-concurrency-in-go/
func asyncHTTPPost(forwards []HTTPForward) []*HTTPResponse {
	eventsTotalProcessed.Inc()
	ch := make(chan *HTTPResponse)
	responses := []*HTTPResponse{}
	client := http.Client{}
	for _, forward := range forwards {
		go func(forward HTTPForward) {
			requestDurationTimer := prometheus.NewTimer(requestDurationTimer)
			requestDurationTimer.ObserveDuration()
			log.Debug().Msgf("Fetching %s", forward.URL)
			req, err := http.NewRequest(http.MethodGet, forward.URL, bytes.NewBuffer(forward.ReqBody))
			resp, err := client.Do(req)
			requestDurationTimer.ObserveDuration()
			ch <- &HTTPResponse{forward.URL, resp, err}

			if err != nil {
				log.Error().Err(err).Msg("Error processing http client event")
				eventsFailedProcessed.Inc()
			} else {
				if resp != nil {
					// Victoriametrics returns a 204 on success, everything else is a fail
					if resp.StatusCode == 204 {
						eventsSucceedProcessed.Inc()
					} else {
						eventsFailedProcessed.Inc()
					}
					if resp.StatusCode == http.StatusOK {
						err := resp.Body.Close()
						if err != nil {
							log.Error().Err(err).Msg("Error closing response body")
						}
					}
				} else {
					log.Error().Msg("Empty response returned for request")
				}
			}
		}(forward)
	}

	for {
		select {
		case r := <-ch:
			log.Debug().Msgf("%s was fetched", r.url)
			if r.err != nil {
				log.Error().Err(r.err).Msgf("Error with request %s failed", r.url)
			}
			responses = append(responses, r)
			if len(responses) == len(forwards) {
				return responses
			}
		case <-time.After(500 * time.Millisecond):
			log.Info().Msg("forwards are taking too long, please check the upstream systems to ensure they are accepting requests")
			eventsFailedTimeouts.Inc()
		}
	}
	return responses
}
