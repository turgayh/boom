run:
	go run cmd/main.go -env ./configs/.env

dev: 
	docker-compose up -d postgres rabbitmq boom

stop:
	docker-compose down
