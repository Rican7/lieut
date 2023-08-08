# Define directories
ROOT_DIR ?= ${CURDIR}
TOOLS_DIR ?= ${ROOT_DIR}/tools

# Set a local GOBIN, since the default value can't be trusted
# (See https://github.com/golang/go/issues/23439)
export GOBIN ?= ${TOOLS_DIR}/bin

# Build flags
GO_CLEAN_FLAGS ?= -i -r -x ${GO_BUILD_FLAGS}

# Tool flags
GOFUMPT_FLAGS ?=
GOLINT_MIN_CONFIDENCE ?= 0.3

# Set the mode for code-coverage
GO_TEST_COVERAGE_MODE ?= count
GO_TEST_COVERAGE_FILE_NAME ?= coverage.out


all: install-deps build

clean:
	go clean ${GO_CLEAN_FLAGS} ./...

build: install-deps
	go build ${GO_BUILD_FLAGS}

install-deps:
	go mod download

${TOOLS_DIR} tools install-deps-dev:
	cd ${TOOLS_DIR} && go install \
		golang.org/x/lint/golint \
		honnef.co/go/tools/cmd/staticcheck \
		mvdan.cc/gofumpt

update-deps:
	go get ./...

test:
	go test -v ./...

test-with-coverage:
	go test -cover -covermode ${GO_TEST_COVERAGE_MODE} ./...

test-with-coverage-formatted:
	go test -cover -covermode ${GO_TEST_COVERAGE_MODE} ./... | column -t | sort -r

test-with-coverage-profile:
	go test -covermode ${GO_TEST_COVERAGE_MODE} -coverprofile ${GO_TEST_COVERAGE_FILE_NAME} ./...

format-lint:
	@errors=$$(${GOBIN}/gofumpt -l ${GOFUMPT_FLAGS} .); if [ "$${errors}" != "" ]; then echo "Format lint failed on:\n$${errors}\n"; exit 1; fi

style-lint: install-deps-dev
	${GOBIN}/golint -min_confidence=${GOLINT_MIN_CONFIDENCE} -set_exit_status ./...
	${GOBIN}/staticcheck ./...

lint: install-deps-dev format-lint style-lint

vet:
	go vet ./...

format-fix:
	${GOBIN}/gofumpt -w ${GOFUMPT_FLAGS} .

fix: install-deps-dev format-fix
	go fix ./...


.PHONY: all clean build install-deps tools install-deps-dev update-deps test test-with-coverage test-with-coverage-formatted test-with-coverage-profile format-lint style-lint lint vet format-fix fix
