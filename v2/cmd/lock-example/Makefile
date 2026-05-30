
.PHONY: build run-dynamodb run-example stop-dynamodb

# Binary name
BINARY=lock-example
DYNAMO_DIR=../../local-dynamodb

build:
	go build -o $(BINARY) main.go

run-dynamodb:
	@echo "Starting DynamoDB Local..."
	cd $(DYNAMO_DIR) && java -Djava.library.path=./DynamoDBLocal_lib -jar DynamoDBLocal.jar -sharedDb -inMemory & echo $$! > dynamodb.pid

stop-dynamodb:
	@if [ -f $(DYNAMO_DIR)/dynamodb.pid ]; then \
		echo "Stopping DynamoDB Local..."; \
		kill $$(cat $(DYNAMO_DIR)/dynamodb.pid) && rm $(DYNAMO_DIR)/dynamodb.pid; \
	else \
		echo "DynamoDB Local pid file not found."; \
	fi

run-example: build
	@echo "Running example..."
	AWS_ACCESS_KEY_ID=fakeMyKeyId \
	AWS_SECRET_ACCESS_KEY=fakeSecretAccessKey \
	AWS_REGION=us-west-2 \
	DYNAMODB_ENDPOINT=http://localhost:8000 \
	./$(BINARY) --table locks my-lock sleep 2
