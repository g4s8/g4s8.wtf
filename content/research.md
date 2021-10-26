---
title: "Research"
date: 2019-10-11T19:43:21+03:00
draft: false
---

The list of public research documents:

## Adaptive backpressure algorithm for reactive data flows

Type: technical report

In a data--intensive network application, the reactive streams specification
could be applied for handling data flows, increasing resource consumption of
the server machine. It's achieved by integrating backpressure into the data flow model.
The benefit of this approach is an increased ability of resources control
and easy auto-scaling server configuration.
Reactive data processing requires two parts of the data flow: producer and consumer.
One end of the flow could be a socket in a common data-driven network application,
and another could be a storage layer.
There are many different implementations for reactive processing of network connections,
such as [Netty](https://netty.io/),
[VertX](https://vertx.io/),
[WebFlux](https://spring.getdocs.org/en-US/spring-framework-docs/docs/spring-web-reactive/webflux/webflux.html)
but not so many implementations
for working with a file system. All current reactive implementations of file-system consumers and producers
are not very good in terms of memory consumption and processing speed in an asynchronous application.
In order to fix this gap, the new algorithm for managing backpressure on file-system consumers was introduced and
implemented in an open-source library
[github.com/cqfn/rio](https://github.com/cqfn/rio)
providing reactive API for the file-system objects.

By: KIRILL CHERNYAVSKIY

[Download PDF](/rio-introducing.pdf).

## Atomic commit protocol for DeGitX project

Type: technical report

To achieve strong consistency in DeGitX we need to solve a problem
of atomic commit of Git referencesover N git repository replicas.
It isn’t always possible to undo changes in git, so we need to manage
git transactions and provide abortion mechanism.
Fortunately, some atomic commit algorithms exists,so we just need to
adapt them to git references update. It is two-phase commit protocol (2PC),
three-phase commit protocol (3PC) and Paxos-commit. Theoretically,
each of them could solve ourproblem.
We have investigated how they could be implemented with git.
On the git side, we can use gitreference-transaction hook to
handle prepare and commit states.

By: STEPAN VALIAVSKII and KIRILL CHERNYAVSKIY

[Download PDF](/degitx-atomic-commit.pdf).


## DeGit - distributed git repositorymanager

Type: white paper

DeGitX is a distributed git repository manager
It provides a front-end interface for git operations byexposing one of the supported endpoints for git clients,
and hides the distributed nature of git storage located on back-end nodes.
The back-end keeps git repository replicas simultaneously on multiple different nodes to scale up the read capacity,
increase durability and provides better availability, especially for different geographically distributed regions.

By: KIRILL CHERNYAVSKIY

[Download PDF](/degitx-wp.pdf).

## Artipie - binary repository manager

Type: white paper

A  software  project  of  almost  any  size  needs  to  keep  its  binaryartifacts in a repository,
to enable access to them by programmers, tools, and other teams.
The quality of the software that managesthe repository matters.
There are a few categories of such a software, which have their pros and cons, currently on the market.
However, none of them fully satisfy the requirements of a large group of software companies.
That’s why a new product is being created.

By: YEGOR BUGAYENKO and KIRILL CHERNYAVSKIY

[Download PDF](/artipie-wp.pdf).
