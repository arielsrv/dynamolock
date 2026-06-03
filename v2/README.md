# DynamoDB Lock Client for Go — v2

[![Build status](https://github.com/arielsrv/dynamolock/actions/workflows/v2.yml/badge.svg)](https://github.com/arielsrv/dynamolock/actions/workflows/v2.yml)
[![GoDoc](https://pkg.go.dev/badge/github.com/arielsrv/dynamolock/v2)](https://pkg.go.dev/github.com/arielsrv/dynamolock/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/arielsrv/dynamolock/v2)](https://goreportcard.com/report/github.com/arielsrv/dynamolock/v2)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

`dynamolock/v2` is a general-purpose **distributed locking** library for Go,
backed by Amazon DynamoDB. It supports both fine-grained and coarse-grained
locks: the lock key can be any arbitrary string (up to DynamoDB's key size
limit).

It is a Go port of Amazon's original
[`dynamodb-lock-client`](https://github.com/awslabs/dynamodb-lock-client),
written against the **AWS SDK for Go v2**.

> ℹ️ **Status:** this library is feature-complete and in maintenance mode.
> Bug fixes and dependency updates are made on a best-effort basis; no new
> features are planned.

---

## Table of contents

- [Why dynamolock?](#why-dynamolock)
- [Installation](#installation)
- [Quick start](#quick-start)
  - [1. Create the table](#1-create-the-table)
  - [2. Acquire, use and release a lock](#2-acquire-use-and-release-a-lock)
- [Features](#features)
  - [Automatic heartbeats](#automatic-heartbeats)
  - [Reading a lock without acquiring it](#reading-a-lock-without-acquiring-it)
  - [Session monitor (lease safety callback)](#session-monitor-lease-safety-callback)
  - [Storing arbitrary data with a lock](#storing-arbitrary-data-with-a-lock)
  - [Sort-key partitioned tables](#sort-key-partitioned-tables)
  - [Context-aware API](#context-aware-api)
- [Avoiding clock-skew issues](#avoiding-clock-skew-issues)
- [Required DynamoDB IAM actions](#required-dynamodb-iam-actions)
- [Differences from v1](#differences-from-v1)
- [Example CLI](#example-cli)
- [Contributing](#contributing)
- [License](#license)

---

## Why dynamolock?

Typical use cases:

- **Mutual exclusion across workers.** A distributed system that processes work
  per customer / per campaign / per entity, and needs to guarantee that only
  one host operates on each entity at a time.
- **Leader election.** Pick a single leader across a fleet of hosts. When the
  leader dies, another host takes over within a configurable `LeaseDuration`.
- **Cross-process locks** in environments where DynamoDB is already available,
  avoiding the need to operate a separate locking service (ZooKeeper, etcd,
  Redis, …).

---

## Installation

```sh
go get github.com/arielsrv/dynamolock/v2
```

Requires Go modules. The minimum Go version is the one declared in
[`go.mod`](./go.mod).

---

## Quick start

### 1. Create the table

The lock table needs a hash (partition) key. You can create it from the AWS
console, with Terraform/CloudFormation, or with the convenience helper
`CreateTableWithContext` shown below. Table creation is asynchronous — wait
for the table to become `ACTIVE` before issuing lock calls.

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/arielsrv/dynamolock/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-west-2"))
	if err != nil {
		log.Fatal(err)
	}

	c, err := dynamolock.New(
		dynamodb.NewFromConfig(cfg),
		"locks",
		dynamolock.WithLeaseDuration(3*time.Second),
		dynamolock.WithHeartbeatPeriod(1*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	log.Println("ensuring table exists")
	if _, err := c.CreateTableWithContext(ctx, "locks",
		dynamolock.WithProvisionedThroughput(&types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	); err != nil {
		log.Fatal(err)
	}
}
```

### 2. Acquire, use and release a lock

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/arielsrv/dynamolock/v2"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-west-2"))
	if err != nil {
		log.Fatal(err)
	}

	c, err := dynamolock.New(
		dynamodb.NewFromConfig(cfg),
		"locks",
		dynamolock.WithLeaseDuration(3*time.Second),
		dynamolock.WithHeartbeatPeriod(1*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	data := []byte("some content a")
	lock, err := c.AcquireLockWithContext(ctx, "spock",
		dynamolock.WithData(data),
		dynamolock.ReplaceData(),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("lock content:", string(lock.Data()))

	// ... do work while holding the lock ...

	log.Println("releasing lock")
	ok, err := c.ReleaseLockWithContext(ctx, lock)
	if err != nil {
		log.Fatal("error releasing lock:", err)
	}
	if !ok {
		log.Fatal("lost lock before release")
	}
	log.Println("done")
}
```

---

## Features

### Automatic heartbeats

When the client is created with `WithHeartbeatPeriod(d)`, a background
goroutine periodically refreshes the `RecordVersionNumber` of every held lock
so they don't expire while the process is alive. The lock is released only
when you call `ReleaseLock` / `lock.Close()` (or when the process exits and
the lease eventually expires).

If you prefer to call `SendHeartbeat` manually, disable the goroutine with
`DisableHeartbeat()` at client construction time.

### Reading a lock without acquiring it

You can inspect the current owner and payload of a lock without trying to
grab it:

```go
lock, err := c.GetWithContext(ctx, "kirk")
if err != nil {
	// ...
}
fmt.Println(lock.OwnerName(), string(lock.Data()))
```

### Session monitor (lease safety callback)

Long-running critical sections sometimes need to know when the lease is about
to expire (for example, because heartbeats have been failing). Pass
`WithSessionMonitor` to `AcquireLock` and a callback will fire as soon as the
lock has less than `safeTime` remaining:

```go
lock, err := c.AcquireLockWithContext(ctx, "leader",
	dynamolock.WithSessionMonitor(500*time.Millisecond, func() {
		log.Println("lease almost expired, stopping critical work")
	}),
)
```

### Storing arbitrary data with a lock

Use `WithData([]byte)` (optionally combined with `ReplaceData()`) when
acquiring the lock to attach a payload. Use `WithDataAfterRelease([]byte)`
on release to leave a "tombstone" payload behind. Data is retrieved with
`lock.Data()` or via `GetWithContext`.

### Sort-key partitioned tables

The client supports tables that use a composite primary key
(partition + sort). Configure both the client and the table creation:

```go
c, err := dynamolock.New(db, "locks",
	dynamolock.WithPartitionKeyName("key"),
	dynamolock.WithSortKey("scope", "tenant-42"),
)

_, err = c.CreateTableWithContext(ctx, "locks",
	dynamolock.WithCustomPartitionKeyName("key"),
	dynamolock.WithSortKeyName("scope"),
)
```

All locks created through that client will be scoped to the provided sort-key
value.

### Context-aware API

Every operation has a `*WithContext` variant: `AcquireLockWithContext`,
`ReleaseLockWithContext`, `SendHeartbeatWithContext`, `GetWithContext`,
`CreateTableWithContext`, `CloseWithContext`. Prefer them — they support
cancellation and deadlines and are the recommended API in v2. The shorter
forms (`AcquireLock`, `ReleaseLock`, …) are kept for convenience and use
`context.Background()` internally.

---

## Avoiding clock-skew issues

The client never stores absolute timestamps in DynamoDB — only the relative
**lease duration** is recorded. To expire a lock, `AcquireLock` reads the
current item, remembers its `RecordVersionNumber` (a GUID), waits up to one
lease duration, then re-reads. If the GUID hasn't changed, the previous owner
is considered dead and the lock is taken over.

This means two hosts may disagree about wall-clock time and still cooperate
safely — only their local monotonic clocks need to be roughly accurate over
the lease duration.

---

## Required DynamoDB IAM actions

For full functionality (including `CreateTable`, tagging and TTL helpers),
the IAM role used by the client should be allowed to perform the following
actions on the lock table:

- `GetItem`
- `PutItem`
- `UpdateItem`
- `DeleteItem`
- `BatchGetItem`
- `CreateTable`
- `DescribeTable`
- `ListTables`
- `UpdateTable`
- `DeleteTable`
- `DescribeTimeToLive`
- `UpdateTimeToLive`
- `TagResource`
- `UntagResource`
- `ListTagsOfResource`

If you create and manage the table externally, you can restrict the policy
to just the item-level actions (`GetItem`, `PutItem`, `UpdateItem`,
`DeleteItem`, `BatchGetItem`).

---

## Differences from v1

- Uses **AWS SDK for Go v2** (`github.com/aws/aws-sdk-go-v2/...`) instead of
  the legacy v1 SDK.
- All public methods have explicit `context.Context` variants, and the
  context-aware forms are the preferred API.
- The v1 module is retired; new code should depend on
  `github.com/arielsrv/dynamolock/v2`.

---

## Example CLI

A small CLI that wraps an arbitrary command in a DynamoDB lock is available
under [`cmd/lock-example`](./cmd/lock-example). It is also a good reference
for setting up the client against **DynamoDB Local** via the
`DYNAMODB_ENDPOINT` environment variable.

```sh
cd cmd/lock-example
make run-dynamodb    # start DynamoDB Local
make run-example     # build & run a sample locked command
make stop-dynamodb   # stop DynamoDB Local
```

---

## Contributing

Issues and pull requests are welcome at
<https://github.com/arielsrv/dynamolock>. Please run the linters and tests
before opening a PR:

```sh
make test    # unit + integration tests
make lint    # golangci-lint
```

This package is covered by the SLA published at
<https://github.com/arielsrv/public/blob/master/SLA.md>.

---

## License

Licensed under the [Apache License, Version 2.0](LICENSE).

