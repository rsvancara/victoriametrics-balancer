//Package utility contains utility functions and data structures
package utility

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/rs/zerolog/log"
)

//VConfig configuration struct used for various parts of the application
type VConfig struct {
	AWSRegion                 string //AWSRegion region for aws
	AWSSearchTag              string //AWSSearchTag tag to filter on
	AWSSearchTagValue         string //AWSSearchTagValue the search tag value to filter on
	AWSPortTag                string //AWSPortTag Tag that specifies the destination port
	AWSURITag                 string //AWSURITag Tag that specifies the destination URI
	AWSPollingIntervalSeconds int    //AWSPollingTick How often to poll AWS for new nodes
	ServicePollingSeconds     int    //ServicePollingSeconds How oftent to poll services for availability
	HTTPTimeOut               int    //Client timeout for http requests
}

//VInstances EC2 instance list
type VInstances struct {
	Instances []VInstance
}

//VInstance An actual instance
type VInstance struct {
	AWSHost       string
	AWSURI        string
	AWSPort       int
	AWSInstanceID string
	AWSName       string
}

const configservice = "configservice"

//GetAWSInstancesByTag get all AWS hosts by tag including their port and URI
func GetAWSInstancesByTag(config *VConfig) ([]VInstance, error) {

	log.Debug().Str("service", configservice).Msgf("AWS Parameters - region: [%s] search: [%s] tag: [%s] uri: [%s] port: [%s]",
		config.AWSRegion, config.AWSSearchTag, config.AWSSearchTagValue, config.AWSURITag, config.AWSPortTag)

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
		Region: aws.String(config.AWSRegion),
		},
		SharedConfigState: session.SharedConfigEnable,
	}))

	tagName := fmt.Sprintf("tag:%s", config.AWSSearchTag)
	tagValues := config.AWSSearchTagValue

	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name: aws.String(tagName),
				Values: []*string{
					aws.String(tagValues),
				},
			},
		},
	}

	svc := ec2.New(sess)
	res, err := svc.DescribeInstances(params)
	if err != nil {
		log.Error().Err(err).Str("service", configservice).Msgf("Error getting list of aws instances by tag %s : %s", config.AWSSearchTag, config.AWSSearchTagValue)
		return nil, err
	}

	var instances []VInstance

	if len(res.Reservations) == 0 {
		log.Debug().Str("service", configservice).Msgf("No EC2 instances found, no upstreams will be configured.  Please check your settings")
		return nil, err
	}

	for _, r := range res.Reservations {
		for _, j := range r.Instances {
			port := 8428
			uri := "/api/v1/write"
			name := "unknown"
			for _, t := range j.Tags {
				if *t.Key == config.AWSPortTag {
					port, err = strconv.Atoi(*t.Value)
					if err != nil {
						log.Error().Err(err).Str("service", configservice).Msg("Error converting port to string")
					}
				}

				if *t.Key == config.AWSURITag {
					uri = *t.Value
				}

				if strings.ToLower(*t.Key) == "name" {
					name = *t.Value
				}

			}
			if *j.State.Name == "running" {
				var instance VInstance
				instance.AWSHost = *j.PrivateIpAddress
				instance.AWSInstanceID = *j.InstanceId
				instance.AWSPort = port
				instance.AWSURI = uri
				instance.AWSName = name

				instances = append(instances, instance)

				log.Debug().Str("service", configservice).Msgf("Found instance %s with IP %s using port %d with uri %s",
					*j.InstanceId, *j.PrivateIpAddress, port, uri)
			}
			//fmt.Println(*i.InstanceId, *i.State.Name, *i.PrivateIpAddress, port, uri)
		}
	}
	return instances, nil
}
