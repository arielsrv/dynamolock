# dynamolock v2 example

This example demonstrates how to use `dynamolock/v2` with the AWS SDK v2 to acquire a distributed lock in DynamoDB and execute a command.

## Requirements

- Go 1.21+ (this project uses modern Go features).
- Configured AWS credentials (or a running local DynamoDB).

## How to run

From the `v2` directory:

```bash
# Build the example
go build -o lock-example ./cmd/lock-example/main.go

# Run a locked command
./lock-example --table my-locks my-unique-lock sleep 5
```

## Makefile

A `Makefile` is provided to simplify running the example with a local DynamoDB instance.

From the `v2/cmd/lock-example` directory:

```bash
# Start DynamoDB Local in background
make run-dynamodb

# Build and run the example
make run-example

# Stop DynamoDB Local
make stop-dynamodb
```

## Options

- `--table`: DynamoDB table name (default "locks").
- `--wait-for-lock`: Waits for the lock to become available if it is already taken.
- `--release-on-error`: Releases the lock even if the command fails (by default, the lock is held until the lease expires if the command fails catastrophically).

## Code

The example is located at `v2/cmd/lock-example/main.go` and uses the following v2 features:

- Integration with `github.com/aws/aws-sdk-go-v2`.
- Use of `context.Context` in all calls.
- Configuration of `LeaseDuration` and `HeartbeatPeriod`.
