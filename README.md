# nats-configurator
Sidecar container for Kubernetes Nats pods for automatic provisioning in a mesh topology.

Usage:
```
kubectl apply -f ./deployment/nats-daemonset.yaml
```

This sidecar may be used also for any Kubernetes abstraction like: Deployment and StatefulSet.
It uses an API server to look up module IP addresses to connect nats server instances in a mesh topology.

Feel free to make a pull requests to improve this repository.
