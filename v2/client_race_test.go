//go:build race

/*
Copyright 2021 U. Cirello (cirello.io and github.com/cirello-io)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dynamolock_test

import (
	"sync"
	"testing"
	"time"

	"cirello.io/dynamolock/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// TestRaceConcurrentAcquire verifies that multiple goroutines racing to acquire
// the same lock do not trigger the race detector.
func TestRaceConcurrentAcquire(t *testing.T) {
	t.Parallel()
	svc := dynamodb.NewFromConfig(defaultConfig(t))
	c, err := dynamolock.New(
		svc,
		"locks",
		dynamolock.WithLeaseDuration(3*time.Second),
		dynamolock.WithHeartbeatPeriod(1*time.Second),
		dynamolock.WithOwnerName("TestRaceConcurrentAcquire"),
		dynamolock.WithPartitionKeyName("key"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_, _ = c.CreateTable(
		"locks",
		dynamolock.WithProvisionedThroughput(&types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	)

	const workers = 10
	var wg sync.WaitGroup
	for i := range workers {
		wg.Go(func() {
			lock, acquireErr := c.AcquireLock(
				"race-concurrent-acquire",
				dynamolock.WithAdditionalTimeToWaitForLock(5*time.Second),
				dynamolock.WithRefreshPeriod(100*time.Millisecond),
				dynamolock.WithData([]byte("worker content")),
				dynamolock.ReplaceData(),
			)
			if acquireErr != nil {
				t.Log("worker", i, "did not acquire lock:", acquireErr)
				return
			}
			defer c.ReleaseLock(lock) //nolint:errcheck
		})
	}
	wg.Wait()
}

// TestRaceConcurrentHeartbeat verifies that sending heartbeats concurrently on
// the same lock does not trigger the race detector.
func TestRaceConcurrentHeartbeat(t *testing.T) {
	t.Parallel()
	svc := dynamodb.NewFromConfig(defaultConfig(t))
	c, err := dynamolock.New(
		svc,
		"locks",
		dynamolock.WithLeaseDuration(10*time.Second),
		dynamolock.DisableHeartbeat(),
		dynamolock.WithOwnerName("TestRaceConcurrentHeartbeat"),
		dynamolock.WithPartitionKeyName("key"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_, _ = c.CreateTable(
		"locks",
		dynamolock.WithProvisionedThroughput(&types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	)

	lock, err := c.AcquireLock("race-concurrent-heartbeat")
	if err != nil {
		t.Fatal(err)
	}
	defer c.ReleaseLock(lock) //nolint:errcheck

	const workers = 5
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			_ = c.SendHeartbeat(lock)
		})
	}
	wg.Wait()
}

// TestRaceConcurrentRelease verifies that releasing a lock from multiple
// goroutines simultaneously does not trigger the race detector.
func TestRaceConcurrentRelease(t *testing.T) {
	t.Parallel()
	svc := dynamodb.NewFromConfig(defaultConfig(t))
	c, err := dynamolock.New(
		svc,
		"locks",
		dynamolock.WithLeaseDuration(10*time.Second),
		dynamolock.DisableHeartbeat(),
		dynamolock.WithOwnerName("TestRaceConcurrentRelease"),
		dynamolock.WithPartitionKeyName("key"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_, _ = c.CreateTable(
		"locks",
		dynamolock.WithProvisionedThroughput(&types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	)

	lock, err := c.AcquireLock("race-concurrent-release")
	if err != nil {
		t.Fatal(err)
	}

	const workers = 5
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			_, _ = c.ReleaseLock(lock)
		})
	}
	wg.Wait()
}

// TestRaceAcquireAndClose verifies that acquiring locks while closing the
// client concurrently does not trigger the race detector.
func TestRaceAcquireAndClose(t *testing.T) {
	t.Parallel()
	svc := dynamodb.NewFromConfig(defaultConfig(t))
	c, err := dynamolock.New(
		svc,
		"locks",
		dynamolock.WithLeaseDuration(3*time.Second),
		dynamolock.WithHeartbeatPeriod(1*time.Second),
		dynamolock.WithOwnerName("TestRaceAcquireAndClose"),
		dynamolock.WithPartitionKeyName("key"),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = c.CreateTable(
		"locks",
		dynamolock.WithProvisionedThroughput(&types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	)

	var wg sync.WaitGroup
	for i := range 5 {
		wg.Go(func() {
			lock, acquireErr := c.AcquireLock(
				"race-acquire-and-close",
				dynamolock.WithAdditionalTimeToWaitForLock(3*time.Second),
				dynamolock.WithRefreshPeriod(50*time.Millisecond),
			)
			if acquireErr != nil {
				t.Log("worker", i, "acquire error (expected during close):", acquireErr)
				return
			}
			_, _ = c.ReleaseLock(lock)
		})
	}

	// Close the client while goroutines are still trying to acquire.
	time.Sleep(10 * time.Millisecond)
	c.Close()
	wg.Wait()
}

// TestRaceHeartbeatAndClose verifies that heartbeats running concurrently with
// client close do not trigger the race detector.
func TestRaceHeartbeatAndClose(t *testing.T) {
	t.Parallel()
	svc := dynamodb.NewFromConfig(defaultConfig(t))
	c, err := dynamolock.New(
		svc,
		"locks",
		dynamolock.WithLeaseDuration(10*time.Second),
		dynamolock.WithHeartbeatPeriod(100*time.Millisecond),
		dynamolock.WithOwnerName("TestRaceHeartbeatAndClose"),
		dynamolock.WithPartitionKeyName("key"),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = c.CreateTable(
		"locks",
		dynamolock.WithProvisionedThroughput(&types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	)

	lock, err := c.AcquireLock("race-heartbeat-and-close")
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			_ = c.SendHeartbeat(lock)
		})
	}

	c.Close()
	wg.Wait()
}

// TestRaceLockIsExpired verifies that calling IsExpired concurrently while a
// lock is being updated does not trigger the race detector.
func TestRaceLockIsExpired(t *testing.T) {
	t.Parallel()
	svc := dynamodb.NewFromConfig(defaultConfig(t))
	c, err := dynamolock.New(
		svc,
		"locks",
		dynamolock.WithLeaseDuration(10*time.Second),
		dynamolock.WithHeartbeatPeriod(100*time.Millisecond),
		dynamolock.WithOwnerName("TestRaceLockIsExpired"),
		dynamolock.WithPartitionKeyName("key"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	_, _ = c.CreateTable(
		"locks",
		dynamolock.WithProvisionedThroughput(&types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		}),
		dynamolock.WithCustomPartitionKeyName("key"),
	)

	lock, err := c.AcquireLock("race-lock-is-expired")
	if err != nil {
		t.Fatal(err)
	}
	defer c.ReleaseLock(lock) //nolint:errcheck

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			_ = lock.IsExpired()
		})
		wg.Go(func() {
			_ = c.SendHeartbeat(lock)
		})
	}
	wg.Wait()
}
