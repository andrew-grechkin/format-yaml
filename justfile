#!/usr/bin/env -S just --one --justfile

export tool := 'format-yaml'

# Build the binary to cache directory
@build: fix
    go build -o "$XDG_CACHE_HOME/go/bin/"

# Install the binary globally
@install:
    go install "$tool"

# Install the pre-commit hook
@install-hooks:
    ln -sf ../../hooks/pre-commit .git/hooks/pre-commit
    echo "installed .git/hooks/pre-commit -> hooks/pre-commit"

# Format Go source code
@fix:
    go fmt
    go fix

# Run Go linter
@lint: cc
    go vet

# Report functions over cyclomatic complexity 15 (installs gocyclo if missing)
@cc:
    test -x "$XDG_CACHE_HOME/go/bin/gocyclo" || GOBIN="$XDG_CACHE_HOME/go/bin" go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
    "$XDG_CACHE_HOME/go/bin/gocyclo" -over 15 .

# Update Go dependencies
@update:
    go get -u
    go mod tidy

# Run Go unit tests
@test-unit:
    go test -v ./...

# Run the fuzz test for FUZZTIME seconds (default 60)
@fuzz duration='60s':
    # Corpus lives under testdata/fuzz and is gitignored; delete testdata/ to reset.
    go test -fuzz=FuzzFormatBytes -fuzztime={{duration}} -run='^$' .

# Run integration tests by driving the binary against fixtures
test-int: build
    #!/usr/bin/env -S bash -Eeuo pipefail

    bin="$XDG_CACHE_HOME/go/bin/$tool"
    modes=(minimal standard full pedantic)

    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT

    fail=0
    for source in test/fixtures/*-source.yaml; do
        name=$(basename "$source" -source.yaml)
        for mode in "${modes[@]}"; do
            expected="test/fixtures/${name}-expected-${mode}.yaml"
            if [[ ! -f "$expected" ]]; then
                echo "✗ MISSING $name/$mode: $expected" >&2
                fail=1
                continue
            fi

            echo -n "Testing $name/$mode... " >&2
            first="$tmp/first.yaml"
            FORMAT_YAML_MODE="$mode" "$bin" < "$source" > "$first"
            if ! diff -q "$expected" "$first" > /dev/null 2>&1; then
                echo "✗ FAIL (first pass)" >&2
                diff -u --label "expected" --label "actual" "$expected" "$first" >&2 || true
                fail=1
                continue
            fi
            second="$tmp/second.yaml"
            FORMAT_YAML_MODE="$mode" "$bin" < "$first" > "$second"
            if ! diff -q "$first" "$second" > /dev/null 2>&1; then
                echo "✗ FAIL (not idempotent)" >&2
                diff -u --label "first-pass" --label "second-pass" "$first" "$second" >&2 || true
                fail=1
                continue
            fi
            echo "✓ PASS" >&2
        done
    done
    exit "$fail"

# Generate missing expected-* fixture files by running the current binary. Never overwrites.
gen-fixtures: build
    #!/usr/bin/env -S bash -Eeuo pipefail

    bin="$XDG_CACHE_HOME/go/bin/$tool"
    modes=(minimal standard full pedantic)

    created=0
    for source in test/fixtures/*-source.yaml; do
        name=$(basename "$source" -source.yaml)
        for mode in "${modes[@]}"; do
            expected="test/fixtures/${name}-expected-${mode}.yaml"
            if [[ -f "$expected" ]]; then continue; fi
            echo "Generating $name/$mode..." >&2
            FORMAT_YAML_MODE="$mode" "$bin" < "$source" > "$expected"
            created=$((created + 1))
        done
    done
    echo "Created $created fixture(s)." >&2

# Run all tests
test: lint test-unit test-int
