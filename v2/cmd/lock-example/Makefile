
.PHONY: build run-dynamodb run-example stop-dynamodb

# Binary name
BINARY=lock-example

build:
	go build -o $(BINARY) main.go

run-dynamodb:
	@echo "Starting DynamoDB Local via Docker..."
	docker run -d --name dynamodb-local -p 8000:8000 amazon/dynamodb-local:latest
	@echo "DynamoDB Local started on port 8000"

stop-dynamodb:
	@echo "Stopping DynamoDB Local..."
	docker stop dynamodb-local && docker rm dynamodb-local

run-example: build
	@echo "Running example..."
	AWS_ACCESS_KEY_ID=fakeMyKeyId \
	AWS_SECRET_ACCESS_KEY=fakeSecretAccessKey \
	AWS_REGION=us-west-2 \
	DYNAMODB_ENDPOINT=http://localhost:8000 \
	./$(BINARY) --table locks my-lock sleep 2
