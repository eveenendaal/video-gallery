.PHONY: build test frontend-build clean

build:
	go build ./...

test:
	go test ./...

frontend-build:
	npm install
	npm run build

clean:
	rm -rf public/styles.css node_modules
