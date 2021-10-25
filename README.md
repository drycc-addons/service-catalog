## `service-catalog`

[![Build Status](https://drone.drycc.cc/api/badges/drycc/service-catalog/status.svg)](https://drone.drycc.cc/drycc/service-catalog)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-sigs/service-catalog)](https://goreportcard.com/report/github.com/kubernetes-sigs/service-catalog)

<p align="center">
    <a href="https://svc-cat.io">
        <img src="/docsite/images/homepage-logo.png">
    </a>
</p>

Service Catalog lets you provision cloud services directly from the comfort of native Kubernetes tooling.
This project is in incubation to bring integration with service
brokers to the Kubernetes ecosystem via the [Open Service Broker API](https://github.com/openservicebrokerapi/servicebroker).

### Documentation

Our goal is to have extensive use-case and functional documentation.

See the [Service Catalog documentation](https://kubernetes.io/docs/concepts/service-catalog/)
on the main Kubernetes site, and [svc-cat.io](https://svc-cat.io/docs).

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

We have biweekly meetings - see
[our SIG Readme](https://github.com/kubernetes/community/blob/master/sig-service-catalog/README.md#meetings)
for details. For meeting agendas
and notes, see [Kubernetes SIG Service Catalog Agenda](https://docs.google.com/document/d/17xlpkoEbPR5M6P5VDzNx17q6-IPFxKyebEekCGYiIKM/edit).

Previous meeting notes are also available:
[2016-08-29 through 2017-09-17](https://docs.google.com/document/d/10VsJjstYfnqeQKCgXGgI43kQWnWFSx8JTH7wFh8CmPA/edit).

### Code of Conduct

Participation in the Kubernetes community is governed by the
[Kubernetes Code of Conduct](./code-of-conduct.md).
