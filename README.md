kubectl-chart
=============

[![Build Status](https://travis-ci.org/martinohmann/kubectl-chart.svg?branch=master)](https://travis-ci.org/martinohmann/kubectl-chart)
[![Go Report Card](https://goreportcard.com/badge/github.com/martinohmann/kubectl-chart?style=flat)](https://goreportcard.com/report/github.com/martinohmann/kubectl-chart)
[![GoDoc](https://godoc.org/github.com/martinohmann/kubectl-chart?status.svg)](https://godoc.org/github.com/martinohmann/kubectl-chart)

A `kubectl` plugin to ease management of cluster components using helm charts.
Minimum required Kubernetes version currently is 1.13 for `kubectl-chart` to
work.

**This is still heavily WIP and will likely change a lot, so do not use in
production.**

More documentation for code and usage will be added soon. Code is a little
messy in this early stage and subject for cleanup.

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

Planned features
----------------

- Simple chart lifecycle hooks (like helm, WIP)
- Configurable pruning of PVC of deleted StatefulSets (TODO)
- Listing all deployed resources of a chart (similar to `kubectl get all` with filter, TODO)

Installation
------------

```sh
$ git clone https://github.com/martinohmann/kubectl-chart
$ cd kubectl-chart
$ make install
```

This will install the `kubectl-chart` binary to `$GOPATH/bin/kubectl-chart`.
After that, it is available as a `kubectl` plugin under the command `kubectl chart`.

Usage examples
--------------

Diff chart:

```
$ kubectl chart diff --chart-dir path/to/chart
```

Diff all charts in a directory:

```
$ kubectl chart diff --chart-dir path/to/charts -R
```

Dry run apply with chart value overrides:

```
$ kubectl chart apply --chart-dir path/to/chart --server-dry-run --value-file path/to/values.yaml
```

Dry run delete charts by filter:

```
$ kubectl chart delete --chart-dir path/to/charts -R --chart-filter chart1,chart3 --dry-run
```

Render chart:

```
$ kubectl chart render --chart-dir path/to/chart
```

Render chart hooks:

```
$ kubectl chart render --chart-dir path/to/chart --hook all
```

License
-------

The source code of kubectl-chart is released under the MIT
License. See the bundled LICENSE file for details.
