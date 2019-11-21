# Kube Problem Reporter

Simple kubernetes cluster watcher that checks periodically if all nodes and pods in a certain namespace are running and sends a message to a slack channel if there is a problem with a node or pod.

Problems reporter reports:
- Node conditions such as memory pressure or disk pressure
- High node resource utilization (>95% of memory or cpu) (only if metrics server is available)
- Critical pod status such as ErrImagePull, Error, CrashLoopBackOff etc.
- Pods that are still not running for more than 30 minutes
- Pods that have restarted in the last hour with a non zero exit code

Watched namespaces and nodes can be configured with the WATCH_NODES and WATCH_NAMESPACES environment variables.

# How to install

Fill in your slack token and channel_id in `kube/deployment.yaml`. Then deploy the reporter:

```
kubectl create namespace kube-problem
kubectl apply -n kube-problem -f kube
kubectl create clusterrolebinding kube-problem-binding --clusterrole=kube-problem --serviceaccount=kube-problem:kube-problem
```

# Contribute

To start kube problem reporter in development mode, download devspace and simply run:

```
kubectl create namespace kube-problem
kubectl create clusterrolebinding kube-problem-binding --clusterrole=kube-problem --serviceaccount=kube-problem:kube-problem
devspace dev -n kube-problem
```

Then start the reporter in the terminal with:

```
go run main.go
```
