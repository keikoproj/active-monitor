# Active-Monitor

[![Maintenance](https://img.shields.io/badge/Maintained%3F-yes-green.svg)][GithubMaintainedUrl]
[![contrib](https://img.shields.io/badge/contributions-welcome-orange.svg)][GithubUrl]
<!--[![PR](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)][GithubPrsUrl]-->
<!--[![slack](https://img.shields.io/badge/slack-join%20the%20conversation-ff69b4.svg)][SlackUrl]-->

[![Build Status][BuildStatusImg]][BuildMasterUrl]
[![Go Report Card][GoReportBadgeUrl]][GoReportMasterUrl]
![version](https://img.shields.io/badge/version-0.1.0-blue.svg?cacheSeconds=2592000)

## Motivation
Active-Monitor is a Kubernetes custom resource controller which enables deep cluster monitoring using [Argo workflows](https://github.com/argoproj/argo).

While it is not too difficult to know that all entities in a cluster are running indvidually, it is often quite challenging to know that they can all coordinate with each other as required for successful cluster operation (network connectivity, volume access, etc).

## Overview
Active-Monitor will create a new `health` namespace when installed in the cluster. Users can then create and submit HealthCheck objects to the Kubernetes server. A HealthCheck is essentially an instrumented wrapper around an Argo workflow.

The workflow is run periodically, as definied by its individual spec, and watched by the Active-Monitor controller.

Active-Monitor sets the status of the HealthCheck CR to indicate whether the monitoring check succeeded or failed. External systems can query these CRs and take appropriate action if they failed.

Typical examples of such workflows include tests for basic Kubernetes object creation/deletion, tests for cluster-wide services such as policy engines checks, authentication and authorization checks, etc.

The sort of HealthChecks one could run with Active-Monitor are:
- verify namespace and deployment creation
- verify AWS resources are using < 80% of their instance limits 
- verify kube-dns by running DNS lookups on the network
- verify kube-dns by running DNS lookups on localhost
- verify KIAM agent by running aws sts get-caller-identity on all available nodes

## Dependencies
* Kubernetes command line tool (kubectl)
* Kubernetes Cluster
* [Argo Workflows Controller](https://github.com/argoproj/argo)

## Generates Resources
* `activemonitor.orkaproj.io/v1alpha1/HealthCheck`
* `argoproj.io/v1alpha1/Workflow`

## Installation Guide
```
# install argo workflow-controller
kubectl apply -f https://raw.githubusercontent.com/orkaproj/active-monitor/master/deploy/deploy-argo.yaml

# install active-monitor controller
kubectl apply -f https://raw.githubusercontent.com/orkaproj/active-monitor/master/config/crd/bases/activemonitor.orkaproj.io_healthchecks.yaml
kubectl apply -f https://raw.githubusercontent.com/orkaproj/active-monitor/master/deploy/deploy-active-monitor.yaml

# finally, run the controller either with local source
make run
# or via docker container
docker run orkaproj/active-monitor:latest
```

## Usage and Examples
#### Run example active-monitor resources
```
kubectl apply -f examples/*.yaml
```

List all health checks:
`kubectl get healthchecks -n health`
```
kubectl get healthchecks -n health
NAME                           AGE
healthcheck-aws-instance-limits    28d
healthcheck-dns                    28d
healthcheck-dns-hostnetwork        28d
healthcheck-kiam-agent             28d
healthcheck-kiam-agent-all-nodes   28d
healthcheck-namespace              28d
```

View additional details/status of a health check:
`kubectl describe healthcheck-kiam-agent-all-nodes`
```
...
Status:
  Failed Count:              165
  Finished At:               2019-06-25T20:46:46Z
  Last Failed At:            2019-06-25T20:13:39Z
  Last Failed Workflow:      healthcheck-kiam-agent-all-nodes-test-l78f2
  Last Successful Workflow:  healthcheck-kiam-agent-all-nodes-test-82qtn
  Status:                    Succeeded
  Success Count:             2175
Events:
  Type    Reason  Age                   From     Message
  ----    ------  ----                  ----     -------
  Normal  Synced  16m (x2340 over 28d)  healthcheck  HealthCheck synced successfully
```

#### Sample HealthCheck CR:
```yaml
apiVersion: activemonitor.orkaproj.io/v1alpha1
kind: HealthCheck
metadata:
  generateName: dns-healthcheck-
  namespace: health
spec:
  repeatInterval: 60
  description: "Monitor pod dns connections"
  workflow:
    generateName: dns-workflow-
    resource:
      namespace: health
      serviceAccount: activemonitor-controller-sa
      source:
        inline: |
            apiVersion: argoproj.io/v1alpha1
            kind: Workflow
            spec:
              ttlSecondsAfterFinished: 60
              entrypoint: start
              templates:
              - name: start
                retryStrategy:
                  limit: 3
                container: 
                  image: tutum/dnsutils
                  command: [sh, -c]
                  args: ["nslookup www.google.com"]
```
![Active-Monitor Architecture](./images/monitoring-example.png)<!-- .element height="50%" width="50%" -->

#### Access Workflows on Argo UI
```
kubectl -n health port-forward deployment/argo-ui 8001:8001
```

Then visit: [http://127.0.0.1:8001](http://127.0.0.1:8001)

## Prometheus Metrics

Active-Monitor controller also exports metrics in Prometheus format which can be further used for notifications and alerting.

Prometheus metrics are availabe on `:2112/metrics`
```
kubectl -n health port-forward deployment/active-monitor-controller 2112:2112
```
Then visit: [http://localhost:2112/metrics](http://localhost:2112/metrics)

Active-Monitor, by default, exports following Promethus metrics:

- `healthcheck_success_count` - The total number of successful monitor resources
- `healthcheck_error_count` - The total number of errored monitor resources
- `healthcheck_runtime_seconds` - Time taken for the workflow to complete

Active-Monitor also supports custom metrics. For this to work, your workflow should export a global parameter. The parameter will be programatically available in the completed workflow object under: `workflow.status.outputs.parameters`.

The global output parameters should look like below:
```
"{\"metrics\":
  [
    {\"name\": \"custom_total\", \"value\": 123, \"metrictype\": \"gauge\", \"help\": \"custom total\"},
    {\"name\": \"custom_metric\", \"value\": 12.3, \"metrictype\": \"gauge\", \"help\": \"custom metric\"}
  ]
}"
```

Here is an example: [custom-metrics.yaml](./examples/custom-metrics.yaml) which shows how to produce custom metrics.

## ❤ Contributing ❤

Please see [CONTRIBUTING.md](.github/CONTRIBUTING.md).

## Developer Guide

Please see [DEVELOPER.md](.github/DEVELOPER.md).

## License
The Apache 2 license is used in this project. Details can be found in the [LICENSE](./LICENSE) file.

## Orka Projects
[Instance Manager][InstanceManagerUrl] -
[Forensics Controller][ForensicsControllerUrl] -
[Addon Manager][AddonManagerUrl] -
[Rollup Controller][RollupControllerUrl] -
[Active Monitor][ActiveMonitorControllerUrl] -
[Minion Manager][MinionManagerUrl] -
[Governor][GovernorUrl]

<!-- URLs -->
[InstanceManagerUrl]: https://github.com/orkaproj/instance-manager
[ForensicsControllerUrl]: https://github.com/orkaproj/kube-forensics
[AddonManagerUrl]: https://github.com/orkaproj/addon-manager
[RollupControllerUrl]: https://github.com/orkaproj/rollup
[ActiveMonitorControllerUrl]: https://github.com/orkaproj/active-monitor
[MinionManagerUrl]: https://github.com/orkaproj/minion-mgr
[GovernorUrl]: https://github.com/orkaproj/governor

[GithubMaintainedUrl]: https://github.com/orkaproj/active-monitor/graphs/commit-activity
[GithubPrsUrl]: https://github.com/orkaproj/active-monitor/pulls
[GithubUrl]: https://github.com/orkaproj/active-monitor/
[SlackUrl]: https://orkaproj.slack.com/messages/??

[BuildStatusImg]: https://travis-ci.org/orkaproj/active-monitor.svg?branch=master
[BuildMasterUrl]: https://travis-ci.org/orkaproj/active-monitor

[GoReportBadgeUrl]: https://goreportcard.com/badge/github.com/orkaproj/active-monitor
[GoReportMasterUrl]: https://goreportcard.com/report/github.com/orkaproj/active-monitor