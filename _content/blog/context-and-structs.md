---
title: Contexts and structs
date: 2021-02-24
by:
- Jean Barkhuysen, Matt T. Proud
tags:
- context
- cancellation
---

## Introduction

In many Go APIs, especially modern ones, the first argument to functions and methods is often [`context.Context`](/pkg/context/). Context provides a means of transmitting deadlines, caller cancellations, and other request-scoped values across API boundaries and between processes. It is often used when a library interacts — directly or transitively — with remote servers, such as databases, APIs, and the like.

The [documentation for context](/pkg/context/) states:

> Contexts should not be stored inside a struct type, but instead passed to each function that needs it.

This article expands on that advice with reasons and examples describing why it's important to pass Context rather than store it in another type. It also highlights a rare case where storing Context in a struct type may make sense, and how to do so safely.

## Prefer contexts passed as arguments

To understand the advice to not store context in structs, let's consider the preferred context-as-argument approach:

```
// Worker fetches and adds works to a remote work orchestration server.
type Worker struct { /* … */ }

type Work struct { /* … */ }

func New() *Worker {
  return &Worker{}
}

func (w *Worker) Fetch(ctx context.Context) (*Work, error) {
  _ = ctx // A per-call ctx is used for cancellation, deadlines, and metadata.
}

func (w *Worker) Process(ctx context.Context, work *Work) error {
  _ = ctx // A per-call ctx is used for cancellation, deadlines, and metadata.
}
```

Here, the `(*Worker).Fetch` and `(*Worker).Process` methods both accept a context directly. With this pass-as-argument design, users can set per-call deadlines, cancellation, and metadata. And, it's clear how the `context.Context` passed to each method will be used: there's no expectation that a `context.Context` passed to one method will be used by any other method. This is because the context is scoped to as small an operation as it needs to be, which greatly increases the utility and clarity of `context` in this package.

## Storing context in structs leads to confusion

Let's inspect again the `Worker` example above with the disfavored context-in-struct approach. The problem with it is that when you store the context in a struct, you obscure lifetime to the callers, or worse intermingle two scopes together in unpredictable ways:

```
type Worker struct {
  ctx context.Context
}

func New(ctx context.Context) *Worker {
  return &Worker{ctx: ctx}
}

func (w *Worker) Fetch() (*Work, error) {
  _ = w.ctx // A shared w.ctx is used for cancellation, deadlines, and metadata.
}

func (w *Worker) Process(work *Work) error {
  _ = w.ctx // A shared w.ctx is used for cancellation, deadlines, and metadata.
}
```

The `(*Worker).Fetch` and `(*Worker).Process` method both use a context stored in Worker. This prevents the callers of Fetch and Process (which may themselves have different contexts) from specifying a deadline, requesting cancellation, and attaching metadata on a per-call basis. For example: the user is unable to provide a deadline just for `(*Worker).Fetch`, or cancel just the `(*Worker).Process` call. The caller's lifetime is intermingled with a shared context, and the context is scoped to the lifetime where the `Worker` is created.

The API is also much more confusing to users compared to the pass-as-argument approach. Users might ask themselves:

- Since `New` takes a `context.Context`, is the constructor doing work that needs cancellation or deadlines?
- Does the `context.Context` passed in to `New` apply to work in `(*Worker).Fetch` and `(*Worker).Process`? Neither? One but not the other?

The API would need a good deal of documentation to explicitly tell the user exactly what the `context.Context` is used for. The user might also have to read code rather than being able to rely on the structure of the API conveys.

And, finally, it can be quite dangerous to design a production-grade server whose requests don't each have a context and thus can't adequately honor cancellation. Without the ability to set per-call deadlines, [your process could backlog](https://sre.google/sre-book/handling-overload/) and exhaust its resources (like memory)!

## Exception to the rule: preserving backwards compatibility

When Go 1.7 — which [introduced context.Context](/doc/go1.7) — was released, a large number of APIs had to add context support in backwards compatible ways. For example, [`net/http`'s `Client` methods](/pkg/net/http/), like `Get` and `Do`, were excellent candidates for context. Each external request sent with these methods would benefit from having the deadline, cancellation, and metadata support that came with `context.Context`.

There are two approaches for adding support for `context.Context` in backwards compatible ways: including a context in a struct, as we'll see in a moment, and duplicating functions, with duplicates accepting `context.Context` and having `Context` as their function name suffix. The duplicate approach should be preferred over the context-in-struct, and is further discussed in [Keeping your modules compatible](/blog/module-compatibility). However, in some cases it's impractical: for example, if your API exposes a large number of functions, then duplicating them all might be infeasible.

The `net/http` package chose the context-in-struct approach, which provides a useful case study. Let's look at `net/http`'s `Do`. Prior to the introduction of `context.Context`, `Do` was defined as follows:

```
// Do sends an HTTP request and returns an HTTP response [...]
func (c *Client) Do(req *Request) (*Response, error)
```

After Go 1.7, `Do` might have looked like the following, if not for the fact that it would break backwards compatibility:

```
// Do sends an HTTP request and returns an HTTP response [...]
func (c *Client) Do(ctx context.Context, req *Request) (*Response, error)
```

But, preserving the backwards compatibility and adhering to the [Go 1 promise of compatibility](/doc/go1compat) is crucial for the standard library. So, instead, the maintainers chose to add a `context.Context` on the `http.Request` struct in order to allow support `context.Context` without breaking backwards compatibility:

```
// A Request represents an HTTP request received by a server or to be sent by a client.
// ...
type Request struct {
  ctx context.Context

  // ...
}

// NewRequestWithContext returns a new Request given a method, URL, and optional
// body.
// [...]
// The given ctx is used for the lifetime of the Request.
func NewRequestWithContext(ctx context.Context, method, url string, body io.Reader) (*Request, error) {
  // Simplified for brevity of this article.
  return &Request{
    ctx: ctx,
    // ...
  }
}

// Do sends an HTTP request and returns an HTTP response [...]
func (c *Client) Do(req *Request) (*Response, error)
```

When retrofitting your API to support context, it may make sense to add a `context.Context` to a struct, as above. However, remember to first consider duplicating your functions, which allows retrofitting `context.Context` in a backwards compatibility without sacrificing utility and comprehension. For example:

```
// Call uses context.Background internally; to specify the context, use
// CallContext.
func (c *Client) Call() error {
  return c.CallContext(context.Background())
}

func (c *Client) CallContext(ctx context.Context) error {
  // ...
}
```

## Conclusion

Context makes it easy to propagate important cross-library and cross-API information down a calling stack. But, it must be used consistently and clearly in order to remain comprehensible, easy to debug, and effective.

When passed as the first argument in a method rather than stored in a struct type, users can take full advantage of its extensibility in order to build a powerful tree of cancellation, deadline, and metadata information through the call stack. And, best of all, its scope is clearly understood when it's passed in as an argument, leading to clear comprehension and debuggability up and down the stack.

When designing an API with context, remember the advice: pass `context.Context` in as an argument; don't store it in structs.

## Further reading

- [Go Concurrency Patterns: Context (2014 blog post)](context.md)