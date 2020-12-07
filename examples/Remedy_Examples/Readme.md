## Remedy Example
In this example we assume that:
 - Prometheus is up and running in the cluster in the system namespace.
 - It can be accessed over http at: prometheus.system.svc.cluster.local on port 9090 i.e., http://prometheus.system.svc.cluster.local:9090
 - You already have memory-demo pod running. (It can be created with `kubectl apply -f stress.yaml`)

## Healthcheck Container
In the example for healthcheck workflow there is a container `ravihari/ctrmemory:v2`
The container takes the following arguments in this order:
 - promanalysis.py (script already present in the container)
 - "http://prometheus.system.svc.cluster.local:9090" (Prometheus address)
 - namespace name
 - pod name
 - container name
 - threshold (if the value is greater than threshold the healthcheck fails)
 
## Kubectl Container
In the example for remedy workflow there is a container `ravihari/kubectl:v1`
The container takes any kubectl commands as arguments to run.
