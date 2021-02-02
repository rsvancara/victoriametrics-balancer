//Package vmupstreams This package provides methods associated with housekeeping for vm upstreams
package vmupstreams

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/rs/zerolog/log"
	utility "github.dev.pages/infrastructure/vmwriter/internal/utility"
)

var (
	currentUpstreams = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "current_upstreams",
		Help: "current available upstreams",
	})
)

//VMUpstream upstream for prometheus compatible instance
type VMUpstream struct {
	Host   string
	Status bool
	Port   int
	URI    string
}

//VMUpstreams list of prometheus compatible upstreams
type VMUpstreams struct {
	Mu     sync.RWMutex // RW Mutex
	UList  []VMUpstream // UList list of Upstream objects
	Config utility.VConfig
}

const watcher = "watcher"

//VMUpstreamsInitialize initializes the list of upstreams
func (v *VMUpstreams) VMUpstreamsInitialize(config *utility.VConfig) error {

	log.Debug().Str("service", watcher).Msg("Initializing upstreams")

	// Load the current configuration
	v.Config = *config

	// Load the upstreams
	err := v.LoadUpstreams()
	if err != nil {
		return err
	}

	return nil
}

//UpstreamList obtains the current list of upstreams and returns a thread safe copy
func (v *VMUpstreams) UpstreamList() ([]VMUpstream, error) {
	v.Mu.RLock()
	defer v.Mu.RUnlock()
	// Return a copy
	us := v.UList

	return us, nil
}

//AddUpstream adds an upstream to the list
func (v *VMUpstreams) AddUpstream(upstream VMUpstream) error {
	v.Mu.Lock()
	defer v.Mu.Unlock()
	v.UList = append(v.UList, upstream)

	return nil
}

//UpdateUpstreamByHost updates the status of an upstream keyed by
func (v *VMUpstreams) UpdateUpstreamByHost(upstream VMUpstream) error {
	v.Mu.Lock()
	defer v.Mu.Unlock()
	for i, ulist := range v.UList {
		if ulist.Host == upstream.Host {
			// Reference the object
			u := &v.UList[i]
			u.Port = upstream.Port
			u.Status = upstream.Status
			u.URI = upstream.URI
		}
	}

	return nil
}

//DeleteUpstreamByHost removes upstream from the list (Thread Safe)
func (v *VMUpstreams) DeleteUpstreamByHost(host string) error {
	v.Mu.Lock()
	defer v.Mu.Unlock()

	// Value to delete
	idx := -1

	// Find Index
	for i, ulist := range v.UList {
		if ulist.Host == host {
			idx = i
		}
	}

	// Copy last element to idx
	v.UList[idx] = v.UList[len(v.UList)-1]

	// Erase the last element from the slice
	v.UList = v.UList[:len(v.UList)-1]

	return nil
}

//GetActiveHostList get a list of hosts (Thread Safe)
func (v *VMUpstreams) GetActiveHostList() ([]string, error) {

	v.Mu.RLock()
	defer v.Mu.RUnlock()
	var retList []string
	for _, upstream := range v.UList {
		if upstream.Status == true {
			retList = append(retList, fmt.Sprintf("http://%s:%d%s", upstream.Host, upstream.Port, upstream.URI))
		}
	}

	return retList, nil
}

//LoadUpstreams Loads the current list of upstreams and removes any upstreams not found.
func (v *VMUpstreams) LoadUpstreams() error {

	instances, err := utility.GetAWSInstancesByTag(&v.Config)
	if err != nil {
		return err
	}

	// Get current thread safe list of upstreams
	uslist, err := v.UpstreamList()
	if err != nil {
		return err
	}

	for _, inst := range instances {

		var n VMUpstream

		n.Host = inst.AWSHost
		n.Port = inst.AWSPort
		n.URI = inst.AWSURI
		n.Status = true

		// See if the existing upstream in the list, if not then add it.
		f := false
		for _, h := range uslist {
			if h.CEqual(n) {
				f = true
			}
		}
		if f == false {
			// Not found, so we add it
			err := v.AddUpstream(n)
			if err != nil {
				return err
			}
			log.Debug().Str("service", watcher).Msgf("Adding upstream %s %d %s", n.Host, n.Port, n.URI)
		}
	}

	// Remove upstreams that no longer exist by comparing available upstreams
	// in AWS and removing them from our list stored in []vmupstreams
	for _, x := range uslist {
		f := false
		for _, inst := range instances {

			var p VMUpstream

			p.Host = inst.AWSHost
			p.Port = inst.AWSPort
			p.URI = inst.AWSURI
			p.Status = true

			if x.CEqual(p) {
				f = true
			}
		}
		if f == false {
			err := v.DeleteUpstreamByHost(x.Host)
			if err != nil {
				return err
			}
			log.Debug().Str("service", watcher).Msgf("Deleting upstream %s", x.Host)
		}
	}

	// Active host list
	activeHostList, err := v.GetActiveHostList()
	if err != nil {
		return err
	}
	for _, t := range activeHostList {
		log.Debug().Str("service", watcher).Msgf("Current upstream: %s", t)
	}

	// Set the current upstreams that are available
	currentUpstreams.Set(float64(len(activeHostList)))

	return nil
}

//CEqual is a custom equal function to test all elements except for status which could be false if a node is marked down
func (v *VMUpstream) CEqual(c VMUpstream) bool {

	if v.Host != c.Host {
		return false
	}

	if v.Port != c.Port {
		return false
	}

	if v.URI != v.URI {
		return false
	}
	return true
}

//AWSServiceWorker Continuously updates upstreams based on changes in AWS
func (v *VMUpstreams) AWSServiceWorker() error {

	// Updates every five seconds so we do not overload AWS
	for {
		time.Sleep(time.Second * 30)
		err := v.LoadUpstreams()
		if err != nil {
			// log the error and keep going
			log.Error().Err(err).Str("service", watcher).Msg("Error loading upstreams")
		}
	}
}
