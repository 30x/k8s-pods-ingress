all: build

check: test lint

clean:
	rm -f coverage.out k8s-router router/router.test kubernetes/kubernetes.test nginx/nginx.test utils/utils.test

lint:
	golint router
	golint kubernetes
	golint nginx

test:
	go test -cover $$(glide novendor)

build: main.go
	go build

build-for-container: main.go
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w' -o k8s-router .
