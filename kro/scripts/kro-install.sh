#!/bin/bash

kubectx kind-kro

helm install kro oci://registry.k8s.io/kro/charts/kro \
  --namespace kro-system \
  --create-namespace

helm list -n kro-system

kubectl get pods -n kro-system

