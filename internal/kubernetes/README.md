Source for `kubectl get` command
================================

This directory contains code that was copied from the main
[kubernetes](https://github.com/kubernetes/kubernetes) repository to make use
of the `kubectl get` funtionality in the `kubectl chart get` command. The code
remains unaltered except for import path fixes. Also, tests and build scripts
were removed.

The main reason behind this is to avoid pulling in `k8s.io/kubernetes` as a
dependency which is more or less impossible since the kubernetes project moved
to go modules (and is also considered as wrong).

The package sources in this directory can be removed once the code for `kubectl
get` is moved from `k8s.io/kubernetes` to the `k8s.io/kubectl` package (which
hopefully will happen in the not so distant future).
