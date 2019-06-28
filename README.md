kubectl-chart
=============

A `kubectl` plugin to ease management of cluster components using helm charts.
Minimum required Kubernetes version currently is 1.13 for `kubectl-chart` to
work.

**This is still heavily WIP and will likely change a lot, so do not use in
production.**

More documentation for code and usage will be added soon. Code is a little
messy in this early stage and subject for cleanup.

Why?
----

- Render and apply multiple charts in one step
- Need for proper pruneing of resources removed from a chart (leveraging prune
  functionality of the `kubectl apply` command)
- Prettier resource diffs for charts (leveraging functionality of `kubectl
  diff`, enhanced by custom diff printer)
- Dry-run for chart deletions
- Simple chart lifecycle hooks (like helm, WIP)
- Diffs for deleted chart resources (not supported by `kubectl diff` yet, TODO)
- Resource diffs for all charts while dry-run and apply (TODO)
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
