# 2. Use Go for the implementation

* Status: accepted
* Date: 2021-06-29
* Authors: @richardcase
* Deciders: @mnowster @pfzero

## Context

For the implemenetation of reignite we need to consider whether its implemented in Go or should we consider Rust. Go is the language used predominatly at Weaveworks and within the Kubernetes ecosystem. However, we need to consider Rust for reignite for the following reasons:

* [AWS](https://aws.amazon.com/blogs/opensource/innovating-with-rust/) / [Microsoft](https://msrc-blog.microsoft.com/2019/07/22/why-rust-for-safe-systems-programming/) are publicly advocating Rust for system programming
* Firecracker & Open Hypervisor are borth written in Rust. There are benefits in terms of contributing to these projects
* WASM support is mature
* Rust is very efficient and [fast](https://benchmarksgame-team.pages.debian.net/benchmarksgame/which-programs-are-fastest.html)

## Decision

The decision is to use Go for the implementation of reignite due to the available skillsets of engineers. We can reconsider this in the future.

## Consequences
N/A