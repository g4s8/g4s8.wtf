+++
draft = true
date = 2023-05-02T05:38:46+04:00
title = "Advancced Go dependency management"
description = "TODO"
slug = "advanced-go-dependency"
tags = ["go"]
categories = ["go"]
+++

In this post:
 - Create multiple modules with transitive dependencies
 - Update minor and major versions of dependencies
 - Create nested submodule for module
 - Use different major vertsion of one dependency
 - TODO: use replace
 - TODO: simulate conflict
 - TODO: ambiguous import - module and submodule

first: see setup.sh

## Case 1

add changes to common (e.g. logging), these changes are propagated to transitive dependencies.

## Case 2

major version update keeps transitive dependencies unchanged

## Case 3

submodule
