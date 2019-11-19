# Kube Problem Reporter

Simple kube cluster watcher that checks periodically if all nodes and pods in a certain namespace are running and sends a message to a slack channel if there is a problem with a node or pod.

# How to install

Fill in your slack token and channel id in `kube/deployment.yaml`. Then deploy the reporter:

```
kubectl apply -f kube
```

You also need to create a cluster role binding for the service account kube-problem, otherwise the reporter cannot access any resources:

```
# replace NAMESPACE with the namespace you have deployed kube-problem into
kubectl create clusterrolebinding kube-problem-binding --clusterrole=view --serviceaccount=NAMESPACE:kube-problem
```
