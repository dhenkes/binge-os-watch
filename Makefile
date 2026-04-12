BINARY    := binge-os-watch
MAIN      := cmd/server/main.go
BUILD_DIR := build
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILDDATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILDDATE)
GOFLAGS   := -trimpath

PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64

.PHONY: all clean tidy check run test cover lint build $(PLATFORMS)

run:
	go run $(MAIN)

test:
	go test ./... -count=1

cover:
	go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1
	@echo "Open coverage.out with: go tool cover -html=coverage.out"

lint: check
	@command -v staticcheck >/dev/null 2>&1 || { echo "Install staticcheck: go install honnef.co/go/tools/cmd/staticcheck@latest"; exit 1; }
	staticcheck ./...

check:
	go vet ./...
	go test ./...

all: clean build

build: $(PLATFORMS)

$(PLATFORMS):
	$(eval GOOS   := $(word 1,$(subst /, ,$@)))
	$(eval GOARCH := $(word 2,$(subst /, ,$@)))
	$(eval EXT    := $(if $(filter windows,$(GOOS)),.exe,))
	$(eval OUT    := $(BUILD_DIR)/$(BINARY)-$(GOOS)-$(GOARCH)$(EXT))
	@echo "Building $(OUT)"
	@mkdir -p $(BUILD_DIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(OUT) $(MAIN)

tidy:
	go mod tidy

clean:
	rm -rf $(BUILD_DIR) coverage.out
