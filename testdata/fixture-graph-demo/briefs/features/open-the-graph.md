---
summary: A reader opens the claims graph from the sidebar and sees every rests_on edge the corpus declares.
status: locked
rests_on:
  - engine.contract.build-is-pure
  - viewer.contract.render-is-pure
  - engine.internals.edge-kinds
  - viewer.internals.payload-shape
  - cli.contract.check-noun
---
# Open the graph

## What it is

One button beside the modules opens a pane that draws the whole corpus: each claim a node, each `rests_on` a line.

## How it works

`dossierx check` writes the graph into the viewer as data; the pane reads it and draws nothing it was not given.

Reading order follows [Reading the graph](../graph/reading-the-graph.md) · narrowing follows [Scope the graph](scope-the-graph.md).
