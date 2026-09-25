.PHONY: dev sqlc templ run build

# generate code and start the server
dev: sqlc templ tidy run

sqlc:
	sqlc generate

templ:
	templ generate

tidy:
	go mod tidy

run:
	go run .

build:
	go build .