# Concourse: the continuous thing-doer.

Concourse is an automation system written in Go. It is most commonly used for
CI/CD pipelines, and is built to scale to the needs of any project.

![booklit pipeline](screenshots/booklit-pipeline.png)

Concourse is very opinionated about a few things: idempotency, immutability,
declarative config, stateless workers, and reproducible builds.

## Installation

Concourse is distributed as a single `concourse` binary, available on the
[Downloads page](https://concourse-ci.org/download.html).

If you want to just kick the tires, jump ahead to the [Quick
Start](#quick-start).

In addition to the `concourse` binary, there are a few other supported formats.
Consult their GitHub repos for more information:

* [Docker image](https://github.com/concourse/concourse-docker)
* [BOSH release](https://github.com/concourse/concourse-bosh-release)
* [Kubernetes Helm chart](https://github.com/helm/charts/tree/master/stable/concourse)


## Quick Start

```sh
$ wget https://concourse-ci.org/docker-compose.yml
$ docker-compose up
Creating docs_concourse-db_1 ... done
Creating docs_concourse_1    ... done
```

Concourse will be running at [localhost:8080](http://localhost:8080). You can
log in with the username/password as `test`/`test`.

Next, install `fly` by downloading it from the web UI and target your local
Concourse as the `test` user:

```sh
$ fly -t ci login -c http://localhost:8080 -u test -p test
logging in to team 'main'

target saved
```

### Configuring your first Pipeline

There is no GUI for configuring Concourse. Instead, individual
[Pipelines](https://concourse-ci.org/pipelines.html) are configured as
declarative YAML files like so:

```yaml
resources:
- name: booklit
  type: git
  source: {uri: "https://github.com/vito/booklit"}

jobs:
- name: unit
  plan:
  - get: booklit
    trigger: true
  - task: test
    file: booklit/ci/test.yml
```

Most operations are done via the accompanying `fly` CLI. If you've got Concourse
[installed](https://concourse-ci.org/install.html), try saving the above example
as `booklit.yml`, [target your Concourse
instance](https://concourse-ci.org/fly.html#fly-login), and then run:

```sh
fly -t $target set-pipeline -p booklit -c booklit.yml
```

Next, check it out in the [web UI](http://localhost:8080). Log in, click the
"play" button to un-pause the pipeline, and watch for the `unit` job to run (or
trigger it manually if you're impatient).


### What is a pipeline?

Pipelines are all the rage in CI these days, so defining this term more
specifically is kind of important - Concourse's are significantly different
from the rest.

A Concourse pipeline boils down to two things:
[Resources](https://concourse-ci.org/resources.html), which represent all
external state, and the [Jobs](https://concourse-ci.org/jobs.html) that
interact with them. A Concourse pipeline is not a simple sequence of actions to
run - it's a dependency flow, kind of like a distributed `Makefile`.

Pipelines are designed to be self-contained so as to minimize server-wide
configuration. Maximizing portability also mitigates risk, making it easier for
projects to recover from CI disasters.

Within pipelines, [jobs](https://concourse-ci.org/jobs.html) are designed to be
loosely coupled, allowing the pipeline to grow with the project's needs without
requiring engineers to keep too much in their head at a time.

Instead of configuring triggers and writing `git` and `s3` scripts,
[resources](https://concourse-ci.org/resources.html) are used to express source
code, dependencies, deployments, and any other external state.

New [types of resources](https://concourse-ci.org/resource-types.html) are
defined as part of the pipeline itself, keeping Concourse itself small and
generic without resorting to a plugin system.

Everything in Concourse runs in a container. Instead of modifying workers to
install build tools, [tasks](https://concourse-ci.org/tasks.html) describe
their own container image (typically using Docker images via the
[`registry-image`
resource](https://github.com/concourse/registry-image-resource)).


### ...What?

This is a lot to digest at once, so it may just sound like gobbeldigook. From
here it might be best to just kick the tires a bit more and use the above as a
gut-check until things start to make sense.

Concourse admittedly has a steeper learning curve at first, depending on your
background. A core goal of this project is for the curve to flatten out shortly
after and yield returns in other places: productivity, and making it easier to
sleep at night.

And feel free to PR any improvements you think would have made it clearer.


### Learn More

* The [Official Site](https://concourse-ci.org) for documentation and
  reference material.
* The [Concourse Tutorial](https://concoursetutorial.com) by Stark & Wayne is
  great for a guided introduction to all the core concepts.
* See Concourse in action with our [production
  pipelines](https://ci.concourse-ci.org)
* Hang around in the [forums](https://discuss.concourse-ci.org) or in
  [Discord](https://discord.gg/MeRxXKW).
* See what we're working on on the [project
  board](https://github.com/orgs/concourse/projects).


## Contributing

Our user base is basically everyone that develops software (and wants it to
work).

It's a lot of work, and we need your help! If you're interested, check out our
[contributing docs](CONTRIBUTING.md).
