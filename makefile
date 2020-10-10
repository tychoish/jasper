name := jasper
buildDir := build
srcFiles := $(shell find . -name "*.go" -not -path "./$(buildDir)/*" -not -name "*_test.go" -not -path "*\#*")
testFiles := $(shell find . -name "*.go" -not -path "./$(buildDir)/*" -not -path "*\#*")
packages := $(name) cli remote remote-internal options mock testutil internal-executor buildsystem-generator
lintPackages := $(packages) mock testutil
testPackages := $(packages) mock
projectPath := github.com/mongodb/jasper

_compilePackages := $(subst $(name),,$(subst -,/,$(foreach target,$(testPackages),./$(target))))
coverageOutput := $(foreach target,$(testPackages),$(buildDir)/output.$(target).coverage)
coverageHtmlOutput := $(foreach target,$(testPackages),$(buildDir)/output.$(target).coverage.html)

# start environment setup
gobin := $(GO_BIN_PATH)
ifeq ($(gobin),)
gobin := go
endif
gopath := $(GOPATH)
gocache := $(abspath $(buildDir)/.cache)
ifeq ($(OS),Windows_NT)
gocache := $(shell cygpath -m $(gocache))
gopath := $(shell cygpath -m $(gopath))
endif

goEnv := GOPATH=$(gopath) GOCACHE=$(gocache)$(if $(GO_BIN_PATH), PATH="$(shell dirname $(GO_BIN_PATH)):$(PATH)")
# end environment setup

compile $(buildDir):
	@mkdir -p $(buildDir)
	$(goEnv) $(gobin) build $(_compilePackages)
compile-base:
	$(goEnv) $(gobin) build  ./

# convenience targets for runing tests and coverage tasks on a
# specific package.
test-%:$(buildDir)/output.%.test
	
coverage-%:$(buildDir)/output.%.coverage
	
html-coverage-%:$(buildDir)/output.%.coverage.html
	
lint-%:$(buildDir)/output.%.lint
	
# end convienence targets

# start lint setup targets
lintDeps := $(buildDir)/.lintSetup $(buildDir)/run-linter $(buildDir)/golangci-lint
$(buildDir)/.lintSetup:$(buildDir)/golangci-lint
	@touch $@
$(buildDir)/golangci-lint:$(buildDir)
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/76a82c6ed19784036bbf2d4c84d0228ca12381a4/install.sh | sh -s -- -b $(buildDir) v1.23.8 >/dev/null 2>&1
$(buildDir)/run-linter:cmd/run-linter/run-linter.go $(buildDir)/.lintSetup $(buildDir)
	@$(goEnv) $(gobin) build -o $@ $<
# end lint setup targets

# benchmark setup targets
$(buildDir)/run-benchmarks:cmd/run-benchmarks/run_benchmarks.go $(buildDir)
	$(goEnv) $(gobin) build -o $@ $<
# end benchmark setup targets

# cli targets
cli: $(name)
$(name): $(buildDir)/$(name)
$(buildDir)/$(name): cmd/$(name)/$(name).go
	@$(goEnv) $(gobin) build -o $@ $<
# end cli targets

# start test and coverage artifacts
#    This varable includes everything that the tests actually need to
#    run. (The "build" target is intentional and makes these targetsb
#    rerun as expected.)
testArgs := -v
ifneq (,$(RUN_TEST))
testArgs += -run='$(RUN_TEST)'
endif
ifneq (,$(RUN_COUNT))
testArgs += -count=$(RUN_COUNT)
endif
ifeq (,$(DISABLE_COVERAGE))
testArgs += -cover
endif
ifneq (,$(RACE_DETECTOR))
testArgs += -race
endif
ifneq (,$(SKIP_LONG))
testArgs += -short
endif
# test execution and output handlers
$(buildDir)/output.%.test:$(buildDir) .FORCE
	$(goEnv) $(gobin) test $(testArgs) ./$(if $(subst $(name),,$*),$(subst -,/,$*),) | tee $@
	@!( grep -s -q "^FAIL" $@ && grep -s -q "^WARNING: DATA RACE" $@)
	@(grep -s -q "^PASS" $@ || grep -s -q "no test files" $@)
$(buildDir)/output.%.coverage:$(buildDir) .FORCE
	$(goEnv) $(gobin) test $(testArgs) ./$(if $(subst $(name),,$*),$(subst -,/,$*),) -covermode=count -coverprofile $@ | tee $(buildDir)/output.$*.test
	@-[ -f $@ ] && $(gobin) tool cover -func=$@ | sed 's%$(projectPath)/%%' | column -t
$(buildDir)/output.%.coverage.html:$(buildDir)/output.%.coverage
	$(goEnv) $(gobin) tool cover -html=$< -o $@
#  targets to generate gotest output from the linter.
$(buildDir)/output.%.lint:$(buildDir)/run-linter $(buildDir) .FORCE
	@$(goEnv) ./$< --output=$@ --lintBin=$(buildDir)/golangci-lint --packages='$*'
#  targets to process and generate coverage reports
# end test and coverage artifacts

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

# user-facing targets for basic build and development operations
$(buildDir)/:
	@mkdir -p $@
proto:
	@mkdir -p remote/internal
	protoc --go_out=plugins=grpc:remote/internal *.proto
lint:$(buildDir) $(foreach target,$(packages),$(buildDir)/output.$(target).lint)

test:$(buildDir) $(foreach target,$(testPackages),$(buildDir)/output.$(target).test)

benchmarks:$(buildDir)/run-benchmarks $(buildDir) .FORCE
	$(goEnv) ./$(buildDir)/run-benchmarks $(run-benchmark)
coverage:$(buildDir) $(coverageOutput)
coverage-html:$(buildDir) $(coverageHtmlOutput)
phony += build lint $(buildDir) test coverage coverage-html docker-setup docker-cleanup $(buildDir)/$(name)
.PHONY: $(phony) .FORCE
.PRECIOUS:$(coverageOutput) $(coverageHtmlOutput)
.PRECIOUS:$(foreach target,$(testPackages),$(buildDir)/output.$(target).test)
.PRECIOUS:$(foreach target,$(packages),$(buildDir)/output.$(target).lint)
# end front-ends

.FORCE:

clean:
	rm -rf $(lintDeps) *.pb.go 

clean-results:
	rm -rf $(buildDir)/output.*
	find vendor/ -name "*.gif" -o -name "*.gz" -o -name "*.png" -o -name "*.ico" -o -name "*testdata*" | xargs rm -rf
	find vendor/ -type d -empty | xargs rm -rf
	find vendor/ -type d -name '.git' | xargs rm -rf
