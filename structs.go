/*
Copyright 2015 github.com/ucirello

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

package dynamolock

import (
	"time"

	"github.com/aws/aws-sdk-go/service/dynamodb"
)

type acquireLockOptions struct {
	additionalAttributes        map[string]*dynamodb.AttributeValue
	sessionMonitor              *sessionMonitor
	partitionKey                string
	data                        []byte
	refreshPeriod               time.Duration
	additionalTimeToWaitForLock time.Duration
	replaceData                 bool
	deleteLockOnRelease         bool
	failIfLocked                bool
}

type getLockOptions struct {
	start                             time.Time
	lockTryingToBeAcquired            *Lock
	sessionMonitor                    *sessionMonitor
	additionalAttributes              map[string]*dynamodb.AttributeValue
	partitionKeyName                  string
	data                              []byte
	millisecondsToWait                time.Duration
	refreshPeriodDuration             time.Duration
	deleteLockOnRelease               bool
	alreadySleptOnceForOneLeasePeriod bool
	replaceData                       bool
	failIfLocked                      bool
}

type releaseLockOptions struct {
	lockItem   *Lock
	data       []byte
	deleteLock bool
}

type createDynamoDBTableOptions struct {
	billingMode           string
	provisionedThroughput *dynamodb.ProvisionedThroughput
	tableName             string
	partitionKeyName      string
	tags                  []*dynamodb.Tag
}
