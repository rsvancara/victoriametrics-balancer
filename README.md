# VMWriter - Load Balancer

This application provides a mechanism to write to upstream "Prometheus API Compatible" remote_write endpoints.  This 
was built to work with Victoria Metrics, but really it can send to any upstream that accepts prometheus data.  For example,
InfluxDB using the prometheus adapter.   

Currenty this load balancer will simply replicate the incomming requests and forward each request to a determined upstream to handle the request.

The loadbalancer was designed to work with AWS to determine upstreams to send data too.  This is based on tags assigned to the instances.  In this way, vmwriter is AWS aware and will work with AWS Autoscaling Groups.  

The loadbalancer provides metrics which you can monitor and alert on.


## Compiling
There is a handy Gnu Make file which allows you to build the binary

```bash

make build

```


## Prometheus configuration

Set your environment and add any additional external labels you need to separate out your prometheus environments in 
your upstream collector, in this case Victoria Metrics.  Add the "remote_write" Section as show below pointing to your upstream
VMWriter instance

```yaml
global:
  scrape_interval: 15s
  external_labels:
    environment: dev

remote_write:
- url: http://8.8.8.8:5000/api/v1/write

```
## Running On MacOS

Use your local AWS Profile configuration

```bash

AWS_PROFILE=utility AWS_REGION=us-west-2 ./vmwriter --clustertag Cluster

```

## Running on EC2

Set an EC2 IAM Profile that includes permissions to an EC2 Read Only Policy.  

```bash
./vmwriter --clustertag Cluster
```

