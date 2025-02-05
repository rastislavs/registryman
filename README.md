[![Go Reference](https://pkg.go.dev/badge/github.com/kubermatic-labs/registryman.svg)](https://pkg.go.dev/github.com/kubermatic-labs/registryman) ![CI](https://github.com/kubermatic-labs/registryman/actions/workflows/ci.yaml/badge.svg)

-----
# Registryman

Registryman (Registry Manager) allows you to declare your Docker registry
projects, project members and project replication rules in a declarative way (by
virtue of YAML files), which will then be applied to your Docker registries.
This enables consistent, always up-to-date team members and access rules.

## Features

* Managing replication rules and project membership on a project level.
* Dry-runs of any action taken, for greater peace of mind.

## Installation

There is no official release yet.

## Build From Source

This project uses Go 1.16 and Go modules for its dependencies. You can get it via `go get`:

```bash
GO111MODULE=on go get github.com/kubermatic-labs/registryman
```

## Concept

Registryman parses the given directory for configuration (.yaml) files. The yaml
files are defined as custom Kubernetes resources.

Registryman supports three types of resources:
  * Registry
  * Project
  * Scanner

The Registry resources describe the Docker registries of the system. Each
registry configures the API endpoint and the credentials. From replication
perspective you can configure up to 1 global and arbitrary number of local
registries.

Currently, the following Registry providers are supported:
- Harbor (https://goharbor.io)
- Azure Container Registry (in progress)

The Project resources describe the members of the project. Each member has a type
(User, Group or Robot) and a Role. The role shows the capabilities for the given
member, e.g. Guest, ProjectAdmin, etc.

From replication point of view, a Project can be either local or global. While a
global project is automatically provisioned in each registry, a local project is
provisioned in the specified registries only.

Replication rules are automatically provisioned for each project so that the
repositories of a global project are synchronized from the global registry to
the local registries.

Scanner describes an external vulnerability scanner that can be assigned to a
project.

Registry and Project resources are declaratively configured as separate files.
For examples, see the `examples` directory.

## Usage

### Applying the configuration

Since Registryman works in a declarative way, first you describe the expected
state as configuration (.yaml) files and then you apply them.

```bash
$ registryman apply <path-to-configuration-dir>
```

An example output of this run could be:
```bash
1.6230650316837864e+09	info	reading config files	{"dir": "testdata/state1/"}
1.6230650316861527e+09	info	inspecting registry	{"registry_name": "harbor-1"}
1.6230650320118732e+09	info	ACTIONS:
1.623065032011928e+09	info	adding project os-images
1.6230650321776721e+09	info	adding member alpha to os-images
1.6230650322818873e+09	info	adding member beta to os-images
1.623065032417013e+09	info	adding replication rule for os-images: harbor-2 [Push] on EventBased
1.6230650325978034e+09	info	inspecting registry	{"registry_name": "harbor-2"}
1.6230650328970125e+09	info	ACTIONS:
1.6230650328970747e+09	info	removing project test
1.6230650329916346e+09	info	adding project os-images
1.6230650331280136e+09	info	adding member alpha to os-images
1.6230650332148027e+09	info	adding member beta to os-images
1.623065033302417e+09	info	adding project app-images
1.6230650334264066e+09	info	adding member alpha to app-images
1.6230650335348642e+09	info	adding member beta to app-images
```

You can see the registries which are configured by Registryman and for each
registry you can see the performed action.

With the `dry-run` flag you can simulate the operation without performing any
action on the Docker registries, e.g.

```bash
$ registryman apply <path-to-configuration-dir>

1.623065212853344e+09	info	reading config files	{"dir": "testdata/init"}
1.6230652128544164e+09	info	inspecting registry	{"registry_name": "harbor-1"}
1.6230652131570046e+09	info	ACTIONS:
1.623065213157117e+09	info	removing replication rule for os-images: harbor-2 [Push] on EventBased	{"dry-run": true}
1.6230652131571586e+09	info	removing project os-images	{"dry-run": true}
1.623065213157202e+09	info	inspecting registry	{"registry_name": "harbor-2"}
1.6230652135110424e+09	info	ACTIONS:
1.6230652135110905e+09	info	removing project app-images	{"dry-run": true}
1.6230652135111215e+09	info	removing project os-images	{"dry-run": true}
```

### Checking the actual registry state

Registryman can generate the state of the managed registries using the `state`
command. Similarly to apply, you have to specify the directory where the YAML
files describing the registries reside.

```bash
$ registryman status <path-to-configuration-dir>
```

### Generating the Swagger API

Registryman can generate the API definition in Swagger format using

```bash
$ registryman swagger
```

The Swagger schema is generated on the standard out in JSON format.

# Development

## Generating the code

Some of the source code is generated by tools. These tools are vendored, so
before generating the source code first you have to reset the vendor directory:

```bash
$ go mod vendor
```

Then you can call the script that regenerates the source code files:

```bash
$ hack/update-codegen.sh
```
