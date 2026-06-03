package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/urfave/cli/v2"

	"github.com/arielsrv/dynamolock/v2"
)

const (
	leaseDurationSec   = 3
	tableCapacityUnits = 5
)

func main() {
	log.SetPrefix("lock-example: ")
	log.SetFlags(0)
	app := &cli.App{
		Name:  "lock-example",
		Usage: "lock and execute given command using dynamolock v2",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "release-on-error"},
			&cli.BoolFlag{Name: "wait-for-lock"},
			&cli.StringFlag{
				Name:  "table",
				Value: "locks",
			},
		},
		Action: func(c *cli.Context) error {
			lockName := c.Args().First()
			if lockName == "" {
				return errors.New("missing lock name")
			}
			cmd := c.Args().Tail()
			if len(cmd) == 0 {
				return errors.New("missing command")
			}
			tableName := c.String("table")

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()

			client, err := dialDynamoDB(ctx, tableName)
			if err != nil {
				return err
			}
			defer client.Close()

			if err = createTable(ctx, client, tableName); err != nil {
				return err
			}

			lock, err := grabLock(ctx, client, lockName, c.Bool("wait-for-lock"))
			if err != nil {
				return err
			}

			return runCommand(ctx, lock, c.Bool("release-on-error"), cmd)
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func dialDynamoDB(ctx context.Context, tableName string) (*dynamolock.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot load AWS config: %w", err)
	}

	client, err := dynamolock.New(
		dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
			if os.Getenv("DYNAMODB_ENDPOINT") != "" {
				o.BaseEndpoint = aws.String(os.Getenv("DYNAMODB_ENDPOINT"))
			}
		}),
		tableName,
		dynamolock.WithLeaseDuration(leaseDurationSec*time.Second),
		dynamolock.WithHeartbeatPeriod(1*time.Second),
		dynamolock.WithPartitionKeyName("key"),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot start dynamolock client: %w", err)
	}
	return client, nil
}

func createTable(ctx context.Context, client *dynamolock.Client, tableName string) error {
	_, err := client.CreateTableWithContext(
		ctx, tableName,
		dynamolock.WithProvisionedThroughput(&types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(tableCapacityUnits),
			WriteCapacityUnits: aws.Int64(tableCapacityUnits),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	)
	if err != nil {
		if _, ok := errors.AsType[*types.ResourceInUseException](err); ok {
			return nil
		}
		return fmt.Errorf("cannot create dynamolock client table: %w", err)
	}
	return nil
}

func grabLock(ctx context.Context, client *dynamolock.Client, lockName string, wait bool) (*dynamolock.Lock, error) {
	for {
		lock, err := client.AcquireLockWithContext(ctx, lockName, dynamolock.WithDeleteLockOnRelease())
		if err != nil {
			if wait && ctx.Err() == nil {
				time.Sleep(1 * time.Second)
				continue
			}
			return nil, fmt.Errorf("cannot lock %s: %w", lockName, err)
		}
		return lock, nil
	}
}

func runCommand(ctx context.Context, lock *dynamolock.Lock, releaseOnError bool, cmd []string) error {
	command := cmd[0]
	var parameters []string
	if len(cmd) > 1 {
		parameters = cmd[1:]
	}
	wrappedCommand := exec.CommandContext(ctx, command, parameters...)
	wrappedCommand.Stdin = os.Stdin
	wrappedCommand.Stdout = os.Stdout
	wrappedCommand.Stderr = os.Stderr
	if err := wrappedCommand.Run(); err != nil {
		if releaseOnError {
			log.Println("errored, releasing lock")
			if lockErr := lock.Close(); lockErr != nil {
				log.Println("cannot release lock after failure:", lockErr)
			}
		}
		return fmt.Errorf("command error: %w", err)
	}
	if lockErr := lock.Close(); lockErr != nil {
		log.Println("cannot release lock after completion:", lockErr)
	}
	return nil
}
