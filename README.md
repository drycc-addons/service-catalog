## `service-catalog`

[![Build Status](https://drone.drycc.cc/api/badges/drycc/service-catalog/status.svg)](https://drone.drycc.cc/drycc/service-catalog)
[![Go Report Card](https://goreportcard.com/badge/github.com/drycc/service-catalog)](https://goreportcard.com/report/github.com/drycc/service-catalog)
[![codecov](https://codecov.io/gh/drycc/service-catalog/branch/main/graph/badge.svg)](https://codecov.io/gh/drycc/service-catalog)

<p align="center">
    <a href="https://service-catalog.drycc.cc">
        <img src="/docsite/images/homepage-logo.png">
    </a>
</p>

Service Catalog lets you provision cloud services directly from the comfort of native Kubernetes tooling.
This project is in incubation to bring integration with service
brokers to the Kubernetes ecosystem via the [Open Service Broker API](https://github.com/openservicebrokerapi/servicebroker).

### About

It seems that the original [kubernetes-sigs/service-catalog](https://github.com/kubernetes-sigs/service-catalog) is no longer active.

However, we use a lot of service-catalog related components in drycc, so we decided to establish a branch.

If the service-catalog is reactivated at some point in the future, we expect that the changes of this branch can be finally merged into [kubernetes-sigs/service-catalog](https://github.com/kubernetes-sigs/service-catalog).

Now our main goal is to upgrade some outdated APIs, including charts and source code. Then consider adding some new features.

If you are also using service-catalog, you can help us together.

* Github: https://github.com/drycc/service-catalog
* Stable Charts: oci://registry.drycc.cc/charts/catalog
* Testing Charts: https://registry.drycc.cc/charts-testing/catalog
* Helmbroker: https://github.com/drycc/helmbroker

### Documentation

Our goal is to have extensive use-case and functional documentation.

See the [Service Catalog documentation](https://kubernetes.io/docs/concepts/service-catalog/)
on the main Kubernetes site, and [service-catalog.drycc.cc](https://service-catalog.drycc.cc/docs).

For details on broker servers that are compatible with this software, see the
Open Service Broker API project's [Getting Started guide](https://github.com/openservicebrokerapi/servicebroker/blob/master/gettingStarted.md).

#### Video links

- [Service Catalog Intro](https://www.youtube.com/watch?v=bm59dpmMhAk)
- [Service Catalog Deep-Dive](https://www.youtube.com/watch?v=0zp0y8Mo_BE)
- [Service Catalog Basic Demo](https://goo.gl/IJ6CV3)
- [SIG Service Catalog Meeting Playlist](https://goo.gl/ZmLNX9)

---

### Project Status

Service Catalog recently switched to a new [CRDs-based](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-resources) architecture. The old API Server-based implementation is available on the [v0.2 branch](https://github.com/kubernetes-sigs/service-catalog/tree/v0.2). We support this implementation by providing bug fixes until July 2020.

We are currently working towards a beta-quality release. See the [milestones list](https://github.com/kubernetes-sigs/service-catalog/milestones?direction=desc&sort=due_date&state=open)
for information about the issues and PRs in current and future milestones.

The project [roadmap](https://github.com/kubernetes-sigs/service-catalog/wiki/Roadmap)
contains information about our high-level goals for future milestones.

The release process of Service Catalog is described [here](https://github.com/kubernetes-sigs/service-catalog/wiki/Release-Process).

### Terminology

This project's problem domain contains a few inconvenient but unavoidable
overloads with other Kubernetes terms. Check out our [terminology page](./terminology.md)
for definitions of terms as they are used in this project.

### Contributing

Interested in contributing? Check out the [contribution guidelines](./CONTRIBUTING.md).

Also see the [developer's guide](./docs/devguide.md) for information on how to
build and test the code.

We have a mailing list available
[here](https://groups.google.com/forum/#!forum/kubernetes-sig-service-catalog).

### Code of Conduct

Participation in the Kubernetes community is governed by the
[Kubernetes Code of Conduct](./code-of-conduct.md).
