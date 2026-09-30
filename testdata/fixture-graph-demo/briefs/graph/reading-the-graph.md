---
summary: How a reader walks the claims graph pane, from a module to the claims it rests on.
rests_on:
  - engine.contract.build-is-pure
  - viewer.contract.render-is-pure
---
# Reading the graph

The pane draws every `rests_on` edge the corpus declares and nothing it infers.

## Why it exists

A module review reads one module; the graph is where a reader sees what the module leans on.

![A sketch of the pane](pane-sketch.svg)
