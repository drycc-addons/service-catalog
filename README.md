## `service-catalog`

[![Build Status](https://woodpecker.drycc.cc/api/badges/drycc-addons/service-catalog/status.svg)](https://woodpecker.drycc.cc/drycc-addons/service-catalog)
[![Go Report Card](https://goreportcard.com/badge/github.com/drycc-addons/service-catalog)](https://goreportcard.com/report/github.com/drycc-addons/service-catalog)
[![codecov](https://codecov.io/gh/drycc-addons/service-catalog/branch/main/graph/badge.svg)](https://codecov.io/gh/drycc-addons/service-catalog)

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

Now our main goal is to upgrade some outdated APIs, including charts and source code. Then consider adding some new features.

If you are also using service-catalog, you can help us together.

* Github: https://github.com/drycc-addons/service-catalog
* Stable Charts: oci://registry.drycc.cc/charts/catalog
* Testing Charts: oci://registry.drycc.cc/charts-testing/catalog
* Helmbroker: https://github.com/drycc-addons/helmbroker

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
