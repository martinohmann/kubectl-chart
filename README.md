kubectl-chart
=============

[![Build Status](https://travis-ci.org/martinohmann/kubectl-chart.svg?branch=master)](https://travis-ci.org/martinohmann/kubectl-chart)
[![codecov](https://codecov.io/gh/martinohmann/kubectl-chart/branch/master/graph/badge.svg)](https://codecov.io/gh/martinohmann/kubectl-chart)
[![Go Report Card](https://goreportcard.com/badge/github.com/martinohmann/kubectl-chart?style=flat)](https://goreportcard.com/report/github.com/martinohmann/kubectl-chart)
[![GoDoc](https://godoc.org/github.com/martinohmann/kubectl-chart?status.svg)](https://godoc.org/github.com/martinohmann/kubectl-chart)
[![GitHub release](https://img.shields.io/github/release/martinohmann/kubectl-chart)](https://godoc.org/github.com/martinohmann/kubectl-chart/releases)

A `kubectl` plugin to ease management of cluster components using helm charts.
Minimum required Kubernetes version currently is 1.13 for `kubectl-chart` to
work.

**This is still alpha quality and will likely change a lot, so be careful to use it in production. You have been warned.**

More documentation for code and usage will be added soon.

Why?
----

The goal that `kubectl-chart` is trying to achieve is to ease the management of
core cluster components like monitoring, operators, CNI, CI/CD tooling and the
like. All common operations should only require a single command (e.g. render,
diff and dry-run apply in one step). Lastly it should integrate some features
not (yet) present in kubernetes (e.g. optional PVC deletion for stateful sets,
lifecycle hooks and dry-run for deletions).

It is built as a single binary and is completely stateless. It is also well
suited for integration into CI/CD because it provides dry-run and diff
functionality for chart changes.

Features
--------

- Render and apply multiple charts in one step
- Proper pruneing of resources removed from a chart (leveraging prune
  functionality of the `kubectl apply` command)
- Prettier resource diffs for charts (leveraging functionality of `kubectl
  diff`, enhanced by custom diff printer)
- Dry-run for chart deletions
- Diffs for deleted chart resources
- Resource diffs for all charts while dry-run and apply
- Simple chart lifecycle hooks (similar to helm hooks)
- Configurable pruning of PVC of deleted StatefulSets
- Dumping of merged chart values for debugging
- Color indicators for printed resource operations to increase visibility

Roadmap / Planned features
--------------------------

- Full integration test coverage
- Listing all deployed resources of a chart (similar to `kubectl get all` with filter)
- Delete chart resources by selector
- Automatic PersistentVolumeClaim resizing
- Optional rollback of partially applied changes on failure

Installation
------------

After `kubectl-chart` is installed, it is available as a `kubectl` plugin under
the command `kubectl chart`.

### From source

```sh
git clone https://github.com/martinohmann/kubectl-chart
cd kubectl-chart
make install
kubectl chart version
```

This will install the `kubectl-chart` binary to `$GOPATH/bin/kubectl-chart`.

### From binary release

Currently only Linux and MacOSX are packaged as binary releases.

```
curl -SsL -o kubectl-chart "https://github.com/martinohmann/kubectl-chart/releases/download/v0.0.2/kubectl-chart_0.0.2_$(uname -s | tr '[:upper:]' '[:lower:]')_x86_64"
chmod +x kubectl-chart
sudo mv kubectl-chart /usr/local/bin
```

You can verify the installation by printing the version:

```
kubectl chart version
```

Usage examples
--------------

Diff chart:

```
kubectl chart diff -f path/to/chart
```

Diff all charts in a directory:

```
kubectl chart diff -f path/to/charts -R
```

Dry run apply with chart value overrides:

```
kubectl chart apply -f path/to/chart --server-dry-run --values path/to/values.yaml
```

Dry run delete charts by filter:

```
kubectl chart delete -f path/to/charts -R --chart-filter chart1,chart3 --dry-run
```

Render chart:

```
kubectl chart render -f path/to/chart
```

Render chart hooks:

```
kubectl chart render -f path/to/chart --hook-type all
```

How does it work?
-----------------

It is pretty simple. `kubectl-chart` adds a label to each chart resource it
renders so it can identify resources belonging to a chart once they are
deployed into a cluster. The label makes it possible to easily diff charts
against the live objects and identify resources that were deleted from the
chart but are still present in the cluster.

Using the label `kubectl chart apply` internally runs logic equivalent to
`kubectl apply --prune -l <chart-label-selector>` to transparently prune
removed resources.

License
-------

The source code of kubectl-chart is released under the MIT
License. See the bundled LICENSE file for details.
