# flare

Tool to help debug kubernetes deployments


#### Example Usage
```
▶ ./flare --help
Usage of ./flare:
  -kubeconfig string
        (optional) absolute path to the kubeconfig file

```

#### Sample Output
```
▶ ./flare
✓ - API Responsive
✗ - Infrastructure Pods Health
cilium-operator-84bdd6f7b6-qxcgm - cilium-operator
cilium-operator-84bdd6f7b6-v5hrv - cilium-operator
metrics-server-76f8d9fc69-s4lh8 - metrics-server

✓ - Node Healthchecks
✓ - Node Overcommit
✓ - Webhooks
✗ - Endpoints
Service clientip has no active endpoints!
Service dashboard-metrics-scraper has no active endpoints!
Service grumble has no active endpoints!
Service synapse has no active endpoints!
Service cluster-autoscaler has no active endpoints!

✓ - Events
```
