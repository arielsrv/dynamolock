/*
Copyright 2015 github.com/arielsrv

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
	"context"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var dynamoEndpoint string

func TestMain(m *testing.M) {
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "amazon/dynamodb-local:latest",
			ExposedPorts: []string{"8000/tcp"},
			WaitingFor:   wait.ForListeningPort("8000/tcp"),
		},
		Started: true,
	})
	if err != nil {
		panic("cannot start DynamoDB container: " + err.Error())
	}
	host, err := container.Host(ctx)
	if err != nil {
		panic("cannot get DynamoDB container host: " + err.Error())
	}
	port, err := container.MappedPort(ctx, "8000")
	if err != nil {
		panic("cannot get DynamoDB container port: " + err.Error())
	}
	dynamoEndpoint = "http://" + host + ":" + port.Port() + "/"
	exitCode := m.Run()
	_ = container.Terminate(ctx)
	os.Exit(exitCode)
}
