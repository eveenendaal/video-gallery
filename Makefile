.PHONY: build frontend-build clean

build:
	go build ./...

frontend-build:
	npm install
	npm run build

clean:
	rm -rf public/styles.css node_modules
