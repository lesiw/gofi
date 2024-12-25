# igo: A Fake Go Interpreter

![igo demo](/../images/demo.gif)

This is a hack. It works by appending each line you type to a Go program and
rerunning it, then hiding the repeated output.

It is easily defeated by printing a non-deterministic number of lines.

If you got here by searching for a genuine interpreted implementation of the Go
spec, you might be looking for [yaegi][yaegi].

For best results, use with [rlwrap][rlwrap].

## Install

### curl

```sh
curl lesiw.io/igo | sh
```

### go install

```sh
go install lesiw.io/igo@latest
```


## Usage

```text
usage: igo [FILE]
```

Append to an existing Go file by passing it in as an argument, e.g. `igo
main.go`.

Run it without any arguments to start from an empty `package main`.

Type `.quit` to quit.

Prepend a line with `:` to send a command to the shell, e.g. `:go get
github.com/google/go-cmp`. 

[yaegi]: https://github.com/traefik/yaegi
[rlwrap]: https://github.com/hanslub42/rlwrap
