VERSION?=main

# Docker-related
docker_image := $(DOCKER_IMAGE)
ifeq ($(docker_image),)
	docker_image := "ubuntu"
endif

docker-setup:
	docker pull $(docker_image)

docker-cleanup:
	docker rm -f $(docker ps -a -q)
	docker rmi -f $(docker_image)
# end Docker

proto:
	@mkdir -p x/remote/internal
	protoc --go_out=plugins=grpc:x/remote/internal *.proto

go-mod-tidy:
	go mod tidy
	for i in $(shell find . -name "go.mod"); do pushd $$(dirname $$i); echo $(dirname $i); go mod tidy; popd; done

upgrade-fun:
	go get github.com/tychoish/fun@$(VERSION)
	for i in $(shell find . -name "go.mod"); do pushd $$(dirname $$i); echo $(dirname $i); go get github.com/tychoish/fun@$(VERSION); go mod tidy; go build ./... ; popd; done

upgrade-grip:
	go get github.com/tychoish/grip@$(VERSION)
	for i in $(shell find . -name "go.mod"); do pushd $$(dirname $$i); echo $(dirname $i); go get github.com/tychoish/grip@$(VERSION); go mod tidy; go build ./... ; popd; done

upgrade-jasper:
	git push --tags
	for i in $(shell find . -name "go.mod"); do pushd $$(dirname $$i); echo $(dirname $i); go get github.com/tychoish/jasper@$(VERSION); go mod tidy; go build ./...; popd; done

clean:
	rm -rf $(lintDeps) *.pb.go

