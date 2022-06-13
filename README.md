# kube-checker

A small CLI tool to check kubernetes for version deprecations.

## Basic usage

### Check specific namespace

```shell
go run ./main.go --kubeconfig /<path-to-kubeconfig>/.kube/config --namespace default --graph-file output
```

### Check entire cluster

```shell
go run ./main.go --kubeconfig /<path-to-kubeconfig>/.kube/config --graph-file output
```
