CLANG ?= clang
CFLAGS := -O2 -g -Wall -Werror $(CFLAGS)

# $BPF_CLANG is used in go:generate invocations.
generate: export BPF_CLANG := $(CLANG)
generate: export BPF_CFLAGS := $(CFLAGS)
generate:
	go generate ./...

