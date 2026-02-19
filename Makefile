# Define directories
ROOT_DIR ?= ${CURDIR}
TOOLS_DIR ?= ${ROOT_DIR}/tools
INTEGRATIONS_DIR ?= ${ROOT_DIR}/integrations

# Set a local GOBIN, since the default value can't be trusted
# (See https://github.com/golang/go/issues/23439)
export GOBIN ?= ${TOOLS_DIR}/bin

# Build flags
GO_CLEAN_FLAGS ?= -i -r -x ${GO_BUILD_FLAGS}

# Tool flags, versions, and "stamps"
#
# NOTE: Stamp files mark installed versions of tools, to detect the known
# installed version of a tool compared to the configuration.
GOFUMPT_FLAGS ?=
GOLINT_MIN_CONFIDENCE ?= 0.3
GOLANGCI_LINT_VERSION ?= 2.9.0
GOLANGCI_LINT_STAMP = ${TOOLS_DIR}/.golangci-lint-${GOLANGCI_LINT_VERSION}.stamp

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

${TOOLS_DIR} tools install-deps-dev: ${GOLANGCI_LINT_STAMP}

${GOLANGCI_LINT_STAMP}:
# Install golangci-lint with a "stamped" version.
#
# This should automatically update the tool whenever the stamp doesn't exist.
	@rm -f -- ${TOOLS_DIR}/.golangci-lint-*.stamp
	@echo
	@echo "Installing golangci-lint v${GOLANGCI_LINT_VERSION}..."
	@echo
	curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b "${GOBIN}" v${GOLANGCI_LINT_VERSION}
	@touch "$@"

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

${INTEGRATIONS_DIR} test-integrations:
	cd ${INTEGRATIONS_DIR} && go test -v ./...

test-all: test test-integrations

format-check: install-deps install-deps-dev
	@echo "Checking code formatting..."
	"${GOBIN}"/golangci-lint fmt --diff ./...

format-fix format: install-deps install-deps-dev
	@echo "Formatting code..."
	"${GOBIN}"/golangci-lint fmt ./...

lint-check: install-deps install-deps-dev
	@echo "Checking code (linting)..."
# Fail on any diffs from `go fix`.
#
# TODO: Simplify this once https://github.com/golang/go/issues/77583 is fixed.
	errors=$$(go fix -diff ./...); if [ "$${errors}" != "" ]; then echo "$${errors}"; exit 1; fi
	"${GOBIN}"/golangci-lint run ./...

lint-fix: install-deps install-deps-dev
	@echo "Auto-fixing code..."
	go fix ./...
	"${GOBIN}"/golangci-lint run --fix ./...

check: install-deps-dev format-check lint-check

fix: install-deps-dev format-fix
	go fix ./...


.PHONY: all clean build install-deps tools install-deps-dev update-deps test test-with-coverage test-with-coverage-formatted test-with-coverage-profile test-integrations test-all format-check format-fix lint-check lint-fix check fix
