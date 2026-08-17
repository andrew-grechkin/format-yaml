#!/usr/bin/env -S just --one --justfile

export tool := 'format-yaml'

# Build the binary to cache directory
@build: fix
    go build -o "$XDG_CACHE_HOME/go/bin/"

# Build cmd/tokenize-yaml (reads YAML from stdin, dumps goccy's lexer token stream as JSON). Companion probe for
# understanding what the tokenizer produces before goccy's parser turns it into an AST.
@build-tokenize-yaml: fix
    go build -o "$XDG_CACHE_HOME/go/bin/tokenize-yaml" ./cmd/tokenize-yaml

# Build cmd/probe-yaml (reads YAML from stdin, dumps our own tree as JSON). Visualizer for the pkg/tree design and a
# harness for input/expected-tree fixture tests. Will replace update-yaml/cmd/probe-yaml once the tree stabilizes.
@build-probe-yaml: fix
    go build -o "$XDG_CACHE_HOME/go/bin/probe-yaml" ./cmd/probe-yaml

# Build cmd/emit-yaml (reads YAML, builds tree, emits bytes). Target: byte-identical round-trip on every non-fail
# fixture. Divergences from source surface as `just test-tree-roundtrip` failures.
@build-emit-yaml: fix
    go build -o "$XDG_CACHE_HOME/go/bin/emit-yaml" ./cmd/emit-yaml

# Round-trip every non-fail test/tree fixture through emit-yaml: input.yaml → tree → emit → bytes compared byte-
# for-byte against the original input. Any diff = the emitter or tree lost information from the source.
test-tree-roundtrip: build-emit-yaml
    #!/usr/bin/env -S bash -Eeuo pipefail
    bin="$XDG_CACHE_HOME/go/bin/emit-yaml"
    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT
    fail=0
    shopt -s nullglob
    for source in test/tree/*.yaml test/fixtures/*.yaml; do
        name=${source#test/}
        base=$(basename "$source" .yaml)
        if [[ "$base" == fail-* ]]; then continue; fi
        echo -n "Round-trip $name... " >&2
        got="$tmp/$(basename "$source")"
        "$bin" < "$source" > "$got"
        if ! diff -q "$source" "$got" > /dev/null 2>&1; then
            echo "✗ FAIL" >&2
            diff -u --label "source" --label "emitted" "$source" "$got" >&2 || true
            fail=1
            continue
        fi
        echo "✓ PASS" >&2
    done
    exit "$fail"

# Temporary playground for developing pkg/tree: rebuilds probe-yaml, pipes a YAML sample into it, and shows the JSON
# tree. Sample source is edited inline here as new node types get added - not a permanent test, just a fast REPL.
try-probe-yaml: build-probe-yaml
    #!/usr/bin/env -S bash -Eeuo pipefail
    bin="$XDG_CACHE_HOME/go/bin/probe-yaml"
    src=$'# island before header\n\n# adjacent to header\n---\n\n# island before null\n\nnull\n\n# island before footer\n\n# adjacent to footer\n...\n'
    printf '%s' "$src" | "$bin"

# Run pkg/tree fixtures: each test/tree/<name>.yaml is fed to probe-yaml and its JSON output is compared to
# test/tree/<name>.json. Fails on any mismatch. Regenerate expected files with `just gen-tree-fixtures`.
test-tree: build-probe-yaml
    #!/usr/bin/env -S bash -Eeuo pipefail
    bin="$XDG_CACHE_HOME/go/bin/probe-yaml"
    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT
    fail=0
    shopt -s nullglob
    for source in test/tree/*.yaml; do
        name=$(basename "$source" .yaml)
        echo -n "Testing tree/$name... " >&2
        if [[ "$name" == fail-* ]]; then
            # Expected-fail case: probe-yaml must exit non-zero, stderr must match .err fixture.
            expected="test/tree/${name}.err"
            if [[ ! -f "$expected" ]]; then
                echo "✗ MISSING $expected (run just gen-tree-fixtures)" >&2
                fail=1
                continue
            fi
            got="$tmp/$name.err"
            if "$bin" < "$source" > /dev/null 2> "$got"; then
                echo "✗ FAIL (expected non-zero exit, got success)" >&2
                fail=1
                continue
            fi
            if ! diff -q "$expected" "$got" > /dev/null 2>&1; then
                echo "✗ FAIL (stderr mismatch)" >&2
                diff -u --label "expected" --label "actual" "$expected" "$got" >&2 || true
                fail=1
                continue
            fi
            echo "✓ PASS" >&2
            continue
        fi
        expected="test/tree/${name}.json"
        if [[ ! -f "$expected" ]]; then
            echo "✗ MISSING $expected (run just gen-tree-fixtures)" >&2
            fail=1
            continue
        fi
        got="$tmp/$name.json"
        "$bin" < "$source" > "$got"
        if ! diff -q "$expected" "$got" > /dev/null 2>&1; then
            echo "✗ FAIL" >&2
            diff -u --label "expected" --label "actual" "$expected" "$got" >&2 || true
            fail=1
            continue
        fi
        echo "✓ PASS" >&2
    done
    exit "$fail"

# Regenerate expected JSON for every test/tree/<name>.yaml by running probe-yaml. Overwrites existing files -
# use with care and eyeball the diff before committing.
gen-tree-fixtures: build-probe-yaml
    #!/usr/bin/env -S bash -Eeuo pipefail
    bin="$XDG_CACHE_HOME/go/bin/probe-yaml"
    shopt -s nullglob
    for source in test/tree/*.yaml; do
        name=$(basename "$source" .yaml)
        if [[ "$name" == fail-* ]]; then
            expected="test/tree/${name}.err"
            "$bin" < "$source" > /dev/null 2> "$expected" || true
            echo "wrote $expected" >&2
            continue
        fi
        expected="test/tree/${name}.json"
        "$bin" < "$source" > "$expected"
        echo "wrote $expected" >&2
    done

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

# Whole-program dead-code analysis over the cmd/ entry points. Flags functions that no reachable path calls -
# useful for pruning the vendored internal/token and internal/scanner packages down to what our tree actually
# needs. Installs golang.org/x/tools/cmd/deadcode if missing.
@deadcode:
    test -x "$XDG_CACHE_HOME/go/bin/deadcode" || GOBIN="$XDG_CACHE_HOME/go/bin" go install golang.org/x/tools/cmd/deadcode@latest
    "$XDG_CACHE_HOME/go/bin/deadcode" ./cmd/probe-yaml/... ./cmd/tokenize-yaml/... ./cmd/emit-yaml/...

# Report functions over cyclomatic complexity 15 (installs gocyclo if missing). Vendored goccy code under
# internal/scanner and internal/token is excluded - we keep those files verbatim so we can diff against upstream
# if we ever need to re-sync, and their complexity is not ours to fix.
@cc:
    test -x "$XDG_CACHE_HOME/go/bin/gocyclo" || GOBIN="$XDG_CACHE_HOME/go/bin" go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
    "$XDG_CACHE_HOME/go/bin/gocyclo" -over 15 -ignore 'internal/(scanner|token)' .

# Update Go dependencies
@update:
    go get -u
    go mod tidy

# Run Go unit tests
@test-unit:
    go test -v ./...


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
