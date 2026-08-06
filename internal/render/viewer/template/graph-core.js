(function () {
  'use strict';

  // ==================================================================
  // graph-core.js — pure computation for the DossierX claims graph pane
  // ==================================================================
  //
  // This file contains NO DOM, NO canvas, NO document access and no global
  // beyond the single namespace assignment at the bottom. Everything here is
  // callable from a bare `node -e` harness and from a single
  // chromedp.Evaluate() call against one loaded page. That is not a style
  // preference: viewer-tests proves this file table-driven over ONE page load,
  // and a function that needed a DOM could not be reached that way.
  //
  // ------------------------------------------------------------------
  // THE JSON-ABLE BOUNDARY RULE
  // ------------------------------------------------------------------
  //
  // Every exported function takes and returns PLAIN JSON-able values only:
  // objects, arrays, strings, finite numbers, booleans, null. Never a Map,
  // never a Set, never a cyclic object, and never `undefined`, `NaN` or
  // `Infinity`. Internal use of Map/Set is fine and encouraged — they just may
  // not cross the boundary, because the test boundary is json.Marshal going in
  // and CDP returnByValue coming out, and a Map arrives as `{}`.
  //
  // ------------------------------------------------------------------
  // EXPORTED SURFACE — window.dossierxGraphCore
  // ------------------------------------------------------------------
  //
  // This list is a stated API, not an accident. viewer-tests hangs its whole
  // suite off these names.
  //
  // Constants (all frozen, all JSON-able):
  //   EDGE_TYPES           ["rests_on", "mirrors", "governed_by"]
  //   DIRECTED_EDGE_TYPES  ["rests_on", "governed_by"] — the SCC edge set
  //   GHOST_PREFIX         "ghost:" — id prefix of an out-of-scope endpoint
  //   FACET_SLOT_COUNT     20 — the categorical palette's slot count
  //   FACT_RULE_IDS        the eight fact rule ids, in emission order
  //   HINT_RULE_IDS        the two heuristic rule ids, in emission order
  //   OVERLAYS             the closed overlay set — six, plus "none"
  //   BUILD_PHASES         the build roles missing_build_phase looks for
  //
  // Helpers:
  //   groupId(groupType, name)            -> "module:engine" / "facet:contract"
  //   edgeKey(edge)                       -> "from|type|to"
  //
  // Scope, representatives and edges (design section 3):
  //   scopeFilter(nodes, moduleScope, facetScope)
  //                                       -> [node]
  //   representatives(nodes, granularity, expandedGroups)
  //                                       -> {repByClaim, repNodes}
  //   aggregateEdges(edges, repByClaim, enabledTypes)
  //                                       -> [{from, to, type, weight}]
  //   degrees(nodeIds, edges)             -> {id: {in, out, total}}
  //   degreeFor(id, degrees, repByClaim)  -> {id, scale, in, out, total}
  //
  // Structure (design section 3.1):
  //   scc(nodeIds, edges)                 -> [[id, ...]]
  //   selfEdges(nodeIds, edges)           -> [id, ...]
  //
  // Encoding channels (design sections 4.2 and 4.3):
  //   facetSlot(facets, facet)            -> 0..19, or -1
  //   governors(edges)                    -> [id, ...]
  //   governanceScope(edges)              -> {nodeIds, edgeKeys}
  //
  // Verdicts (design section 5):
  //   gapRules(nodes, edges, options)     -> {facts: [...], hints: [...]}
  //
  // Hash state (design section 9):
  //   defaultState()                      -> state object
  //   encodeState(state)                  -> compact string
  //   decodeState(string)                 -> state object
  //
  // ------------------------------------------------------------------
  // THE EDGE ARGUMENT — two accepted shapes, one internal shape
  // ------------------------------------------------------------------
  //
  // Every function taking `edges` accepts either the payload's object form
  // {from, to, type} or a positional array form [from, to] / [from, to, type].
  // The array form exists so a one-liner harness can call scc() without
  // building objects. An edge with no type is treated as a DIRECTED edge of
  // unnamed type: it participates in scc(), and it is excluded from the
  // by-type filters (governed_by in particular) that name a type explicitly.
  //
  // Returned edges are always the object form.

  // EDGE_TYPES is the closed set of relations model.Claim declares. It is the
  // canonical ordering used by encodeState and by every by-type sort.
  var EDGE_TYPES = Object.freeze(['rests_on', 'mirrors', 'governed_by']);

  // DIRECTED_EDGE_TYPES is the subset scc() walks. `mirrors` is excluded
  // because it is reciprocal by design — a mirrored pair is not a dependency
  // loop, and counting it as one would ring every mirrored claim red.
  var DIRECTED_EDGE_TYPES = Object.freeze(['rests_on', 'governed_by']);

  // GHOST_PREFIX marks an edge endpoint that resolved to no in-scope
  // representative. aggregateEdges() emits "ghost:<claim id>" for it rather
  // than dropping the edge, because scoping must never hide that a claim
  // reaches outward. The pane draws these hollow and unlabeled, and no gap
  // rule counts them.
  var GHOST_PREFIX = 'ghost:';

  // FACET_SLOT_COUNT is the categorical palette's width. graph.css defines
  // --dxg-facet-1 .. --dxg-facet-20 plus --dxg-facet-other; facetSlot() maps a
  // facet to an index into that ramp by POSITION, never by name, because a
  // project names its own facets and the engine hardcodes none of them.
  var FACET_SLOT_COUNT = 20;

  // FACT_RULE_IDS and HINT_RULE_IDS are the stable ids the pane and the
  // browser tests key off. Display text is never a key.
  var FACT_RULE_IDS = Object.freeze([
    'cycle',
    'self_edge',
    'isolated',
    'weakly_linked',
    'review_pending',
    'open_threads',
    'sink_group',
    'orphan_group'
  ]);
  var HINT_RULE_IDS = Object.freeze(['missing_build_phase', 'density_outlier']);

  // ------------------------------------------------------------------
  // Internal helpers — none of these cross the exported boundary.
  // ------------------------------------------------------------------

  // asArray returns v when it is an array and an empty array otherwise, so
  // every entry point tolerates null, undefined and a scalar without throwing.
  // Build() is total on the Go side; this file is total on the client side.
  function asArray(v) {
    return Array.isArray(v) ? v : [];
  }

  // asString coerces to a string, mapping null/undefined to "". No exported
  // function may hand back undefined, and "" is the payload's own convention
  // for an absent module, facet or build role.
  //
  // A non-finite number also becomes "", not the text "NaN". Every id, module
  // and facet here arrives as parsed JSON that a caller may have built badly,
  // and the boundary rule says NaN and Infinity do not cross it — silently
  // laundering one into a plausible-looking string identifier would be the
  // exact violation the rule exists to prevent, one step further along.
  function asString(v) {
    if (typeof v === 'string') {
      return v;
    }
    if (v === null || v === undefined) {
      return '';
    }
    if (typeof v === 'number' && !isFinite(v)) {
      return '';
    }
    return String(v);
  }

  // cmpStr is a total, locale-independent string comparator. Sort order is
  // asserted byte-for-byte by Go tests, so it must not depend on a locale.
  function cmpStr(a, b) {
    return a < b ? -1 : a > b ? 1 : 0;
  }

  // sortedUnique returns a new sorted array with duplicates removed.
  function sortedUnique(ids) {
    var seen = Object.create(null);
    var out = [];
    for (var i = 0; i < ids.length; i++) {
      var id = ids[i];
      if (!seen[id]) {
        seen[id] = true;
        out.push(id);
      }
    }
    out.sort(cmpStr);
    return out;
  }

  // normalizeEdge maps either accepted edge shape onto {from, to, type}, or
  // returns null when the value carries no usable endpoints. A missing type
  // becomes "" — see the edge-argument note above for what "" means.
  function normalizeEdge(e) {
    if (!e) {
      return null;
    }
    var from, to, type;
    if (Array.isArray(e)) {
      from = asString(e[0]);
      to = asString(e[1]);
      type = asString(e[2]);
    } else {
      from = asString(e.from);
      to = asString(e.to);
      type = asString(e.type);
    }
    if (from === '' || to === '') {
      return null;
    }
    return { from: from, to: to, type: type };
  }

  // normalizeEdges maps a whole edge list, dropping unusable entries.
  function normalizeEdges(edges) {
    var list = asArray(edges);
    var out = [];
    for (var i = 0; i < list.length; i++) {
      var e = normalizeEdge(list[i]);
      if (e) {
        out.push(e);
      }
    }
    return out;
  }

  // idSet builds a lookup Set from an id array. Internal only.
  function idSet(ids) {
    var s = new Set();
    var list = asArray(ids);
    for (var i = 0; i < list.length; i++) {
      s.add(asString(list[i]));
    }
    return s;
  }

  // ------------------------------------------------------------------
  // Exported helpers
  // ------------------------------------------------------------------

  // groupId is the id of a collapsed group node: "module:<m>" / "facet:<f>".
  // It is the vocabulary the gaps rail, the expanded-group set and the two
  // group-level rules all speak, so a group has ONE name everywhere it can be
  // named. The two scope selects do NOT use it — each of them already knows
  // which axis it is, so its values are bare module and facet names — and the
  // prefix here is what still distinguishes a module called `contract` from a
  // facet called `contract` in the one place both can appear at once.
  function groupId(groupType, name) {
    return asString(groupType) + ':' + asString(name);
  }

  // edgeKey is the stable identity of one edge, used by governanceScope's
  // edgeKeys and by any caller that needs a set of edges as plain strings.
  // The separator is "|", which no claim id can contain (ids are dotted
  // slugs), so the key is unambiguous.
  function edgeKey(edge) {
    var e = normalizeEdge(edge);
    if (!e) {
      return '';
    }
    return e.from + '|' + e.type + '|' + e.to;
  }

  // ------------------------------------------------------------------
  // Scope and the representative-node rule (design section 3)
  // ------------------------------------------------------------------

  // scopeFilter narrows the claim set the whole pane operates on, along TWO
  // INDEPENDENT AXES that compose as an INTERSECTION.
  //
  //   moduleScope  ""  every module   |  "<m>"  only claims whose module is <m>
  //   facetScope   ""  every facet    |  "<f>"  only claims whose facet is <f>
  //
  // A claim is in scope when
  //
  //     (moduleScope === "" || claim.module === moduleScope)
  //       && (facetScope === "" || claim.facet === facetScope)
  //
  // so "" + "" is the whole project, "<m>" + "" is one module, "" + "<f>" is
  // one facet ACROSS EVERY MODULE — a view the design lists as a scope axis in
  // its own right — and "<m>" + "<f>" is the intersection of the two. Neither
  // axis constrains the other and neither has to be chosen first.
  //
  // THE EMPTY STRING IS THE ONLY "ALL" SENTINEL, and that is what makes this
  // total rather than merely defensive: asString(null), asString(undefined), a
  // missing field and a non-finite number all become "" and all mean "do not
  // filter on this axis". There is no reserved word, so a project is free to
  // name a module `all` without that name quietly meaning something else.
  //
  // THE INTERSECTION MAY BE EMPTY, AND THAT IS AN ANSWER RATHER THAN A FAULT.
  // Module `telemetry` may simply carry no `verification` claims. This
  // function returns the empty array for that pair; the pane STATES the empty
  // combination in words (graph-ui.js's renderEmptyScopeNotice) so a reader can
  // tell "this combination has no claims" from "the graph broke". That
  // replaces the single-axis control's old rule, which returned every node for
  // anything it did not recognise on the grounds that an unannounced empty
  // canvas is indistinguishable from a project with no claims. The objection
  // was right and the notice is the answer to it: with two axes an empty
  // result is common enough to deserve being said out loud rather than hidden
  // behind a filter that silently stops filtering.
  //
  // The returned array holds the SAME node objects, not copies: nothing here
  // mutates a payload node, and the pane's redraw path runs on every control
  // change.
  function scopeFilter(nodes, moduleScope, facetScope) {
    var list = asArray(nodes);
    var wantModule = asString(moduleScope);
    var wantFacet = asString(facetScope);
    if (wantModule === '' && wantFacet === '') {
      return list.slice();
    }
    var out = [];
    for (var i = 0; i < list.length; i++) {
      var n = list[i] || {};
      if (wantModule !== '' && asString(n.module) !== wantModule) {
        continue;
      }
      if (wantFacet !== '' && asString(n.facet) !== wantFacet) {
        continue;
      }
      out.push(list[i]);
    }
    return out;
  }

  // normalizeGranularity maps anything unrecognised onto "claims", the
  // granularity at which nothing is collapsed and therefore nothing is hidden.
  function normalizeGranularity(g) {
    var s = asString(g);
    return s === 'module' || s === 'facet' ? s : 'claims';
  }

  // groupNameOf returns the node's group name under the given group type.
  // Either may be "" — a claim is allowed no module and no facet, and the
  // empty group is a real group the pane buckets under a catch-all label.
  function groupNameOf(node, groupType) {
    var n = node || {};
    return groupType === 'facet' ? asString(n.facet) : asString(n.module);
  }

  // representatives implements the one rule scope, granularity and drill-down
  // all collapse to:
  //
  //   Every claim resolves to a representative node — ITSELF if its group is
  //   expanded, its GROUP node otherwise.
  //
  // granularity sets the default: "claims" expands every group, "module" and
  // "facet" collapse every group of that type. expandedGroups is the
  // per-group override a double-click writes; an entry may be either a bare
  // group name ("engine") or a full group id ("module:engine"), because the
  // pane holds names and the rail holds ids.
  //
  // Returns:
  //   repByClaim  {claimId: representativeId} for every node passed in
  //   repNodes    the representative nodes, sorted by id — claim nodes pass
  //               through unchanged, group nodes are synthesised with
  //               kind "group"
  //
  // The synthesised group node carries `members`, the sorted ids of the claims
  // it stands for. That is what lets the pane ring a group red because one
  // member sits in a cycle (design section 3.1) without recomputing anything.
  function representatives(nodes, granularity, expandedGroups) {
    var list = asArray(nodes);
    var gran = normalizeGranularity(granularity);
    var expanded = idSet(expandedGroups);

    var repByClaim = {};
    var repNodes = [];

    if (gran === 'claims') {
      for (var i = 0; i < list.length; i++) {
        var claimNode = list[i] || {};
        var cid = asString(claimNode.id);
        if (cid === '') {
          continue;
        }
        repByClaim[cid] = cid;
        repNodes.push(list[i]);
      }
      repNodes.sort(function (a, b) {
        return cmpStr(asString((a || {}).id), asString((b || {}).id));
      });
      return { repByClaim: repByClaim, repNodes: repNodes };
    }

    var groups = new Map(); // group id -> group node under construction
    for (var j = 0; j < list.length; j++) {
      var node = list[j] || {};
      var id = asString(node.id);
      if (id === '') {
        continue;
      }
      var name = groupNameOf(node, gran);
      var gid = groupId(gran, name);
      if (expanded.has(name) || expanded.has(gid)) {
        repByClaim[id] = id;
        repNodes.push(list[j]);
        continue;
      }
      repByClaim[id] = gid;
      var g = groups.get(gid);
      if (!g) {
        g = {
          id: gid,
          title: name,
          kind: 'group',
          group_type: gran,
          group_name: name,
          module: gran === 'module' ? name : '',
          facet: gran === 'facet' ? name : '',
          size: 0,
          members: []
        };
        groups.set(gid, g);
        repNodes.push(g);
      }
      g.members.push(id);
      g.size++;
    }

    groups.forEach(function (g) {
      g.members.sort(cmpStr);
    });
    repNodes.sort(function (a, b) {
      return cmpStr(asString((a || {}).id), asString((b || {}).id));
    });
    return { repByClaim: repByClaim, repNodes: repNodes };
  }

  // typeEnabled reports whether an edge type survives the edge-type toggles.
  // enabledTypes omitted (not an array) means "no toggles applied, everything
  // passes". Given an array, the match is exact — which means an untyped edge
  // (type "") passes only if "" is listed. Untyped edges are a harness
  // convenience, not a payload shape, so that is the honest strict reading.
  function typeEnabled(type, enabledTypes) {
    if (!Array.isArray(enabledTypes)) {
      return true;
    }
    for (var i = 0; i < enabledTypes.length; i++) {
      if (asString(enabledTypes[i]) === type) {
        return true;
      }
    }
    return false;
  }

  // aggregateEdges maps claim-level edges through the representative mapping
  // and collapses the result.
  //
  //   1. drop edges whose type is toggled off
  //   2. resolve each endpoint through repByClaim; an endpoint with no
  //      representative becomes GHOST_PREFIX + its claim id, so a claim
  //      reaching outside the current scope stays visible as a reach
  //   3. drop an edge whose BOTH endpoints are ghosts — an edge between two
  //      out-of-scope claims is not this view's business
  //   4. drop self-loops. This is the step that makes a collapsed module
  //      readable: every intra-module edge becomes a loop on the group node,
  //      and drawing those would say nothing except "this module has edges"
  //   5. aggregate by (from, to, type), summing weight
  //
  // weight is the number of claim-level edges an aggregated edge stands for.
  // An input edge that already carries a weight contributes that weight, so
  // re-aggregating an aggregated set is stable rather than lossy.
  //
  // Sorted by (from, type, to) — the same ordering internal/graph emits the
  // payload's own edge list in, so a reader comparing the two is comparing
  // like with like.
  function aggregateEdges(edges, repByClaim, enabledTypes) {
    var raw = asArray(edges);
    var rep = repByClaim && typeof repByClaim === 'object' ? repByClaim : {};
    var buckets = new Map();
    var order = [];

    for (var i = 0; i < raw.length; i++) {
      var e = normalizeEdge(raw[i]);
      if (!e) {
        continue;
      }
      if (!typeEnabled(e.type, enabledTypes)) {
        continue;
      }
      var fromGhost = !Object.prototype.hasOwnProperty.call(rep, e.from);
      var toGhost = !Object.prototype.hasOwnProperty.call(rep, e.to);
      if (fromGhost && toGhost) {
        continue;
      }
      var from = fromGhost ? GHOST_PREFIX + e.from : asString(rep[e.from]);
      var to = toGhost ? GHOST_PREFIX + e.to : asString(rep[e.to]);
      if (from === to) {
        continue;
      }
      var w = 1;
      if (!Array.isArray(raw[i]) && raw[i] && typeof raw[i].weight === 'number' && isFinite(raw[i].weight)) {
        w = raw[i].weight;
      }
      var key = from + '|' + e.type + '|' + to;
      var bucket = buckets.get(key);
      if (!bucket) {
        bucket = { from: from, to: to, type: e.type, weight: 0 };
        buckets.set(key, bucket);
        order.push(bucket);
      }
      bucket.weight += w;
    }

    order.sort(function (a, b) {
      return cmpStr(a.from, b.from) || cmpStr(a.type, b.type) || cmpStr(a.to, b.to);
    });
    return order;
  }

  // degrees counts edges per node, SCOPE-RELATIVE — which is the whole point.
  // The payload carries project-wide in_degree/out_degree as a fact; this is
  // the verdict the browser computes once a reader narrows the view, and the
  // two genuinely disagree. A claim well-connected project-wide can be the
  // most isolated thing inside one module, and radius has to say so.
  //
  //   nodeIds  the ids in play; every one gets an entry, zeros included
  //   edges    claim-level or aggregated; an aggregated edge contributes its
  //            weight, an unweighted edge contributes 1
  //
  // Returns {id: {in, out, total}} with total === in + out. An endpoint absent
  // from nodeIds (a ghost, or an out-of-scope claim) contributes to the
  // endpoint that IS present and gets no entry of its own. A self-loop counts
  // once on each side, so its total is 2.
  //
  // WHICH GRAPH THIS DESCRIBES: exactly the one whose ids and edges you pass.
  // Pass the DRAWN node ids and the AGGREGATED edges and every entry is a
  // degree in the view; pass claim ids and claim edges and every entry is a
  // claim-level degree. The two disagree, and the result carries no marker
  // saying which one it is — so a caller holding the drawn-graph map must not
  // look a claim up in it by first swapping in that claim's representative.
  // That swap is how the detail panel came to report "degree (view) 6" for a
  // claim with no edges at all: the 6 was its module's. degreeFor() below
  // exists so that lookup states whose number it just returned.
  function degrees(nodeIds, edges) {
    var ids = asArray(nodeIds);
    var out = {};
    for (var i = 0; i < ids.length; i++) {
      var id = asString(ids[i]);
      if (id !== '') {
        out[id] = { in: 0, out: 0, total: 0 };
      }
    }
    var list = asArray(edges);
    for (var j = 0; j < list.length; j++) {
      var e = normalizeEdge(list[j]);
      if (!e) {
        continue;
      }
      var w = 1;
      if (!Array.isArray(list[j]) && list[j] && typeof list[j].weight === 'number' && isFinite(list[j].weight)) {
        w = list[j].weight;
      }
      if (Object.prototype.hasOwnProperty.call(out, e.from)) {
        out[e.from].out += w;
        out[e.from].total += w;
      }
      if (Object.prototype.hasOwnProperty.call(out, e.to)) {
        out[e.to].in += w;
        out[e.to].total += w;
      }
    }
    return out;
  }

  // degreeFor looks one id up in a degrees() map and returns a record that
  // NAMES THE NODE THE NUMBERS BELONG TO. That field is the whole point of
  // the function: at module granularity a claim is not a node, and the only
  // honest readings are "this claim is not drawn" or "here is its module's
  // degree, and it is the module's". Handing back the module's numbers under
  // the claim's name is the third reading, and it is a lie the pane shipped.
  //
  //   id            the id a reader selected — a claim id or a drawn node id
  //   degreesByNode a degrees() result over the DRAWN graph
  //   repByClaim    a representatives() result's repByClaim, or omitted
  //
  // Returns {id, scale, in, out, total}, always, never null:
  //
  //   scale "node"            id is itself drawn; the numbers are its own and
  //                           the returned id equals the id passed in
  //   scale "representative"  id is not drawn but the node standing for it is;
  //                           the numbers are THAT node's and the returned id
  //                           names it, so a caller cannot render them without
  //                           saying whose they are
  //   scale "absent"          neither is drawn; id is "" and the counts are 0
  //
  // Callers are expected to render `id` whenever scale is "representative".
  // The counts are still returned rather than withheld because "the module
  // this claim collapsed into has 6 edges" is a useful answer — it is only
  // wrong when it is presented as the claim's own.
  function degreeFor(id, degreesByNode, repByClaim) {
    var want = asString(id);
    var deg = degreesByNode && typeof degreesByNode === 'object' ? degreesByNode : {};
    var rep = repByClaim && typeof repByClaim === 'object' ? repByClaim : {};

    if (want !== '' && Object.prototype.hasOwnProperty.call(deg, want)) {
      return degreeRecord(want, 'node', deg[want]);
    }
    var repId = want !== '' && Object.prototype.hasOwnProperty.call(rep, want) ? asString(rep[want]) : '';
    if (repId !== '' && repId !== want && Object.prototype.hasOwnProperty.call(deg, repId)) {
      return degreeRecord(repId, 'representative', deg[repId]);
    }
    return { id: '', scale: 'absent', in: 0, out: 0, total: 0 };
  }

  // degreeRecord coerces one degrees() entry into the exported shape. Counts
  // are forced finite because the record crosses the JSON-able boundary.
  function degreeRecord(id, scale, entry) {
    var d = entry && typeof entry === 'object' ? entry : {};
    return {
      id: id,
      scale: scale,
      in: finiteNumber(d.in),
      out: finiteNumber(d.out),
      total: finiteNumber(d.total)
    };
  }

  function finiteNumber(v) {
    return typeof v === 'number' && isFinite(v) ? v : 0;
  }

  // ------------------------------------------------------------------
  // Structure: strongly connected components (design section 3.1)
  // ------------------------------------------------------------------

  // isDirectedType reports whether an edge type participates in cycle
  // detection. `mirrors` does not: reciprocity is the lint's job and a
  // mirrored pair is a two-cycle by construction, so including it would ring
  // every correctly mirrored claim red. An untyped edge ("") is treated as
  // directed, which is what makes the [from, to] pair form usable in a
  // one-liner harness.
  function isDirectedType(type) {
    if (type === '') {
      return true;
    }
    for (var i = 0; i < DIRECTED_EDGE_TYPES.length; i++) {
      if (DIRECTED_EDGE_TYPES[i] === type) {
        return true;
      }
    }
    return false;
  }

  // scc returns the strongly connected components of the CLAIM-LEVEL,
  // scope-filtered graph over the directed edge types.
  //
  // Running this before the representative rule rather than after it is not a
  // detail — it is the difference between a correct graph and one that lies.
  // When a module collapses to one node, every intra-module edge becomes a
  // self-loop on that node; an SCC pass over the aggregated set that treated
  // those as cycles would ring every module with any internal edge red.
  // Cycle membership is a property of CLAIMS. A group node is ringed red iff
  // it contains at least one claim in a component, which the caller answers
  // from the group node's `members`.
  //
  // Returned components:
  //   - every component of size >= 2
  //   - a component of size 1 ONLY when the node carries a literal directed
  //     self-edge. Tarjan emits a singleton for every node; a singleton with
  //     no self-edge is just a node, not a cycle.
  //
  // Ordering is deterministic and asserted on exactly by Go tests: members
  // sorted within a component, components sorted by their smallest member.
  //
  // THE WALK IS ITERATIVE, over an explicit frame stack, for the reason the
  // engine's own findEdgeCycles is: recursion depth here is the length of the
  // longest authored dependency chain, and nothing in the schema bounds that.
  // A recursive Tarjan blows the JS stack somewhere in the low thousands; a
  // 10,000-link chain is a test case, not a hypothetical.
  //
  // Algorithm: Tarjan, "Depth-first search and linear graph algorithms",
  // SIAM Journal on Computing 1(2), 1972 — restructured so the recursive call
  // becomes a push onto `work` and the post-call low-link update becomes the
  // pop path.
  function scc(nodeIds, edges) {
    var ids = asArray(nodeIds).map(asString).filter(function (id) {
      return id !== '';
    });
    var known = new Set(ids);
    var roots = sortedUnique(ids);

    // adj: id -> sorted successor ids, both endpoints known, directed types
    // only. Sorting the successors is what makes the component ordering
    // reproducible across engines whose object key order differs.
    var adj = new Map();
    var selfLoop = new Set();
    var list = asArray(edges);
    for (var i = 0; i < list.length; i++) {
      var e = normalizeEdge(list[i]);
      if (!e || !isDirectedType(e.type)) {
        continue;
      }
      if (!known.has(e.from) || !known.has(e.to)) {
        continue;
      }
      if (e.from === e.to) {
        selfLoop.add(e.from);
      }
      var succ = adj.get(e.from);
      if (!succ) {
        succ = [];
        adj.set(e.from, succ);
      }
      succ.push(e.to);
    }
    adj.forEach(function (succ) {
      succ.sort(cmpStr);
    });

    var index = 0;
    var idx = new Map(); // id -> discovery index
    var low = new Map(); // id -> low-link
    var stack = []; // Tarjan's component stack
    var onStack = new Set();
    var components = [];

    for (var r = 0; r < roots.length; r++) {
      if (idx.has(roots[r])) {
        continue;
      }
      // Each frame is [node, next successor offset]. Pushing a frame is the
      // recursive call; popping one is the return.
      var work = [[roots[r], 0]];
      while (work.length > 0) {
        var frame = work[work.length - 1];
        var node = frame[0];
        if (frame[1] === 0) {
          idx.set(node, index);
          low.set(node, index);
          index++;
          stack.push(node);
          onStack.add(node);
        }
        var succs = adj.get(node) || [];
        var descended = false;
        for (var k = frame[1]; k < succs.length; k++) {
          var w = succs[k];
          if (!idx.has(w)) {
            frame[1] = k + 1;
            work.push([w, 0]);
            descended = true;
            break;
          }
          if (onStack.has(w)) {
            var cand = idx.get(w);
            if (cand < low.get(node)) {
              low.set(node, cand);
            }
          }
        }
        if (descended) {
          continue;
        }
        frame[1] = succs.length;

        if (low.get(node) === idx.get(node)) {
          var comp = [];
          for (;;) {
            var popped = stack.pop();
            onStack.delete(popped);
            comp.push(popped);
            if (popped === node) {
              break;
            }
          }
          if (comp.length >= 2 || selfLoop.has(comp[0])) {
            comp.sort(cmpStr);
            components.push(comp);
          }
        }

        work.pop();
        if (work.length > 0) {
          var parent = work[work.length - 1][0];
          if (low.get(node) < low.get(parent)) {
            low.set(parent, low.get(node));
          }
        }
      }
    }

    components.sort(function (a, b) {
      return cmpStr(a[0], b[0]);
    });
    return components;
  }

  // selfEdges returns the ids in nodeIds that are their own target under ANY
  // edge type — rests_on, mirrors or governed_by. It is reported separately
  // from scc() and never merged into the cycle list, because the engine
  // already has a dedicated error-severity `self-edge` lint distinct from
  // `cycle`, and a pane that folded the two together would be telling a
  // reader a different story than `dossierx check` tells them.
  function selfEdges(nodeIds, edges) {
    var known = idSet(nodeIds);
    var hits = [];
    var list = asArray(edges);
    for (var i = 0; i < list.length; i++) {
      var e = normalizeEdge(list[i]);
      if (e && e.from === e.to && known.has(e.from)) {
        hits.push(e.from);
      }
    }
    return sortedUnique(hits);
  }

  // ------------------------------------------------------------------
  // Encoding channels (design sections 4.2 and 4.3)
  // ------------------------------------------------------------------

  // facetSlot maps a facet onto a categorical palette slot, 0..19, by its
  // POSITION in the project's own facet list — never by its name.
  //
  // The engine hardcodes no facet name anywhere, and testdata's portability
  // fixture exists specifically to prove it: cfg.Facets is an arbitrary
  // project-authored list. A palette keyed on names would work for exactly one
  // project's vocabulary and silently grey out every other project's.
  //
  // Returns -1 for a facet that is empty or absent from the list. -1 is the
  // reserved --dxg-facet-other slot, not an error.
  //
  // Nothing here knows about colour. This returns an index; which hex it maps
  // to is graph.css's business, read at draw time off the document element so
  // an OS light/dark switch repaints with no palette duplicated in JS.
  //
  // Wrapping at 20 stops the ramp REPEATING inside the range most projects
  // live in. It does not make 20 colours distinguishable — roughly twelve is
  // the practical ceiling, which is why the legend names every facet in text
  // and the detail panel names the selected node's facet. A reader never has
  // to resolve a colour to answer "which facet is this?".
  function facetSlot(facets, facet) {
    var want = asString(facet);
    if (want === '') {
      return -1;
    }
    var list = asArray(facets);
    for (var i = 0; i < list.length; i++) {
      if (asString(list[i]) === want) {
        return i % FACET_SLOT_COUNT;
      }
    }
    return -1;
  }

  // GOVERNED_TYPE is the one edge type the governance channels key off.
  var GOVERNED_TYPE = 'governed_by';

  // governors returns the sorted ids that are the TARGET of at least one
  // governance edge — the claims that govern something.
  //
  // This is the wedge-marker set. The wedge sits on the governing node rather
  // than on the edge because it is the one governance signal a reader can use
  // without following a line at all: a doctrine claim is findable at a glance
  // on a 400-node canvas, which is exactly the tracing work this pane exists
  // to remove.
  //
  // Direction matters and is easy to get backwards: a claim declares
  // `governed_by: {type: X}`, so the edge runs claim -> governor and the
  // governor is `to`.
  function governors(edges) {
    var list = asArray(edges);
    var hits = [];
    for (var i = 0; i < list.length; i++) {
      var e = normalizeEdge(list[i]);
      if (e && e.type === GOVERNED_TYPE) {
        hits.push(e.to);
      }
    }
    return sortedUnique(hits);
  }

  // governanceScope returns everything the governance overlay keeps lit:
  //
  //   nodeIds   sorted ids of every governor and every claim they govern
  //   edgeKeys  sorted edgeKey() of every governance edge
  //
  // Only governance edges are in scope. A rests_on edge that happens to join
  // two governance participants is dimmed with everything else, because the
  // question this overlay answers is "what does this doctrine actually
  // reach?", and reach is carried by the governance edges alone — lighting up
  // an unrelated dependency between two governed claims would answer a
  // different question badly.
  function governanceScope(edges) {
    var list = asArray(edges);
    var nodes = [];
    var keys = [];
    for (var i = 0; i < list.length; i++) {
      var e = normalizeEdge(list[i]);
      if (!e || e.type !== GOVERNED_TYPE) {
        continue;
      }
      nodes.push(e.from);
      nodes.push(e.to);
      keys.push(e.from + '|' + e.type + '|' + e.to);
    }
    return { nodeIds: sortedUnique(nodes), edgeKeys: sortedUnique(keys) };
  }

  // ------------------------------------------------------------------
  // Gap rules (design section 5)
  // ------------------------------------------------------------------
  //
  // EVERY rule is computed against the CURRENT VIEW, here in the browser.
  // That is the whole reason this file exists rather than a precomputed gap
  // list in the payload: a claim that is isolated within module:viewer may be
  // well-connected project-wide, and a panel that ignored the reader's scope
  // would be confidently wrong the moment they narrowed it.
  //
  // "THE CURRENT VIEW" INCLUDES GRANULARITY, NOT ONLY SCOPE.
  //
  // This is the correction the integration drive forced. Scope was honoured
  // from the first version; granularity was not. At granularity "module" the
  // canvas drew five group nodes while the rail still named 29 claim ids
  // under "exactly one edge in this view" — ids that were not nodes, could
  // not be seen, and whose edges were not the edges on screen. Four of eight
  // fact rules were byte-identical across all three granularities, which is
  // the signature of a rule computed over the raw claim list and captioned
  // "in this view". A rule reporting on nodes nobody can see is worse than no
  // rule, so the connectivity and attention rules now run over the
  // REPRESENTATIVE graph — the nodes and edges actually drawn.
  //
  // WHAT EACH RULE MEANS AT COLLAPSED GRANULARITY, stated rather than left to
  // be inferred, because the rail's wording has to match it:
  //
  //   cycle          CLAIM level, always, at every granularity. Design 3.1 is
  //   self_edge      explicit: collapse a module and every intra-module edge
  //                  becomes a self-loop, so an SCC pass over the aggregated
  //                  set would ring every module with any internal edge red.
  //                  Cycle membership is a property of CLAIMS. These two
  //                  therefore name claim ids even when the claim is not
  //                  drawn, and the pane resolves each to the group standing
  //                  for it (ringing that group red, centring it on a jump).
  //                  That is a deliberate exception, not the same defect: the
  //                  finding is about a claim, and it says so.
  //
  //   isolated       "isolated in this view" / "exactly one edge in this
  //   weakly_linked  view", read over the nodes and edges ON SCREEN. At
  //                  "claims" every node is a claim and nothing changes. At
  //                  "module"/"facet" a listed id is a GROUP with zero or one
  //                  drawn edge — the group nothing links to. Edges inside a
  //                  collapsed group are not drawn and so are not counted;
  //                  parallel claim edges between the same two groups draw as
  //                  one line and count once, because the reader counts lines.
  //
  //   review_pending an honest group-level ROLL-UP. At "claims" a listed id
  //   open_threads   is the claim carrying the state. At "module"/"facet" it
  //                  is a group CONTAINING at least one claim that carries
  //                  it — the same reading the halo already draws on a
  //                  collapsed group node, so rail and canvas now agree. The
  //                  alternative was to declare them not applicable and emit
  //                  nothing, which would have read as "no claim needs
  //                  review" — a different and worse lie.
  //
  //   sink_group     unchanged: group-level by definition, over cross-group
  //   orphan_group   claim edges, keyed by options.groupBy.
  //
  //   the two hints  unchanged: module-level by definition (a facet has no
  //                  build), and labelled as guesses.
  //
  // Drill-down composes for free, because it composes inside
  // representatives(): a group the reader expanded contributes its claims as
  // nodes and the rules judge those claims, while its collapsed siblings are
  // still judged as groups.
  //
  // Emission shape, fixed: {rule, node_ids, kind}. Exactly three keys. The
  // `rule` id is stable and is what the rail and the browser tests key off —
  // display text is never a key. There is deliberately no fourth key naming
  // the scale of the ids: the ids are the ids of DRAWN NODES, which is the
  // property that makes the rail's existing wording true, and a key saying so
  // would be a caption on a fact rather than the fact.
  //
  // Every rule id in FACT_RULE_IDS appears at least once, with an empty
  // node_ids when it found nothing, so the rail can render a stable block
  // list and honestly show an empty cycle block. `cycle` is the one rule that
  // may appear MORE than once: one finding per component, because two
  // separate loops are two separate answers, not one merged id list.
  //
  // Which rules honour the edge-type toggles:
  //
  //   honour them   isolated, weakly_linked, sink_group, orphan_group — every
  //                 connectivity verdict, because "connected" means connected
  //                 by the relations the reader is currently looking at
  //   ignore them   cycle, self_edge — structural defects are drawn in every
  //                 overlay and must not be hideable by a toggle
  //   n/a           review_pending, open_threads — node facts, no edges
  //
  // Ghost endpoints are counted by NOTHING here. An edge to a claim outside
  // the current scope resolves to a ghost, and design section 3 says a ghost
  // is not counted in any gap rule — so every rule runs over edges with BOTH
  // endpoints in scope, which is also why aggregation inside this function
  // can never produce a ghost.

  // BUILD_PHASES is the ordered set of real build phases a module is expected
  // to cover. model.BuildRole also defines "out-of-scope", which is
  // deliberately NOT here: it marks a deferred claim, not a phase whose
  // absence is a gap.
  var BUILD_PHASES = Object.freeze(['orientation', 'schema', 'behavior', 'api', 'verification']);

  function finding(rule, nodeIds, kind) {
    return { rule: rule, node_ids: nodeIds, kind: kind };
  }

  // median of an already-sorted numeric array. Returns 0 for an empty one,
  // which the callers treat as "no basis for a comparison" and skip.
  function median(sorted) {
    var n = sorted.length;
    if (n === 0) {
      return 0;
    }
    if (n % 2 === 1) {
      return sorted[(n - 1) / 2];
    }
    return (sorted[n / 2 - 1] + sorted[n / 2]) / 2;
  }

  // ------------------------------------------------------------------
  // The two heuristics (design section 5)
  // ------------------------------------------------------------------
  //
  // These are GUESSES and they are kept in their own array for that reason.
  // False positives here are guaranteed, not merely possible: a module that
  // legitimately has no verification phase will be listed every single time.
  // A separate array is what stops the panel rendering a guess in the same
  // block as a fact by accident — the wording and the visual separation are
  // the only things that keep a heuristic honest rather than annoying, and a
  // shared array would eventually lose both.
  //
  // Both emit the same {rule, node_ids, kind} shape with kind "hint", and
  // both always appear, with an empty node_ids when they found nothing.

  // missingBuildPhase: a module with at least one LOCKED claim and zero
  // claims in some build phase. The locked precondition is what keeps this
  // from firing on every module a project has merely started — a module with
  // nothing locked is not missing a phase, it is unfinished, and the reader
  // already knows that. Verification is the usual absentee.
  //
  // Grouping is always by module regardless of the rail's groupBy: the phase
  // vocabulary is a property of a module's build, and a facet has no build.
  function missingBuildPhase(nodes) {
    var lockedIn = new Set();
    var phasesIn = new Map(); // module -> Set of build roles present
    var moduleNames = [];
    for (var i = 0; i < nodes.length; i++) {
      var n = nodes[i] || {};
      var mod = asString(n.module);
      moduleNames.push(mod);
      if (asString(n.status) === 'locked') {
        lockedIn.add(mod);
      }
      var seen = phasesIn.get(mod);
      if (!seen) {
        seen = new Set();
        phasesIn.set(mod, seen);
      }
      var role = asString(n.build_role);
      if (role !== '') {
        seen.add(role);
      }
    }
    var hits = [];
    var names = sortedUnique(moduleNames);
    for (var j = 0; j < names.length; j++) {
      if (!lockedIn.has(names[j])) {
        continue;
      }
      var present = phasesIn.get(names[j]) || new Set();
      for (var k = 0; k < BUILD_PHASES.length; k++) {
        if (!present.has(BUILD_PHASES[k])) {
          hits.push(groupId('module', names[j]));
          break;
        }
      }
    }
    return hits;
  }

  // densityOutlier: a facet whose claim count in one module sits far below its
  // median across the other modules — the shape of "this module forgot to
  // write its contracts down".
  //
  // "Far below" is made concrete rather than left to taste: strictly less than
  // half the median. Two guards keep the guess from degenerating:
  //
  //   - at least three modules must be in scope, since a median over one or
  //     two numbers describes nothing
  //   - the median must be at least 2, since half of 1 is 0 and every module
  //     without the facet would then be an outlier
  //
  // A module with zero claims of the facet counts as 0, not as absent — that
  // is precisely the case worth flagging.
  function densityOutlier(nodes) {
    var moduleNames = [];
    var facetNames = [];
    var counts = new Map(); // module + " " + facet -> count
    for (var i = 0; i < nodes.length; i++) {
      var n = nodes[i] || {};
      var mod = asString(n.module);
      var facet = asString(n.facet);
      moduleNames.push(mod);
      if (facet !== '') {
        facetNames.push(facet);
      }
      var key = mod + ' ' + facet;
      counts.set(key, (counts.get(key) || 0) + 1);
    }
    var modules = sortedUnique(moduleNames);
    var facets = sortedUnique(facetNames);
    if (modules.length < 3) {
      return [];
    }
    var hits = [];
    for (var f = 0; f < facets.length; f++) {
      var per = [];
      for (var m = 0; m < modules.length; m++) {
        per.push(counts.get(modules[m] + ' ' + facets[f]) || 0);
      }
      var sorted = per.slice().sort(function (a, b) {
        return a - b;
      });
      var med = median(sorted);
      if (med < 2) {
        continue;
      }
      for (var q = 0; q < modules.length; q++) {
        if (per[q] * 2 < med) {
          hits.push(groupId('module', modules[q]));
        }
      }
    }
    return sortedUnique(hits);
  }

  // normalizeGroupBy picks the grouping the two group rules run over.
  function normalizeGroupBy(g) {
    return asString(g) === 'facet' ? 'facet' : 'module';
  }

  // gapRules computes every verdict the gaps rail renders.
  //
  //   nodes    the SCOPE-FILTERED claim-level nodes
  //   edges    claim-level edges; anything touching an out-of-scope claim is
  //            dropped here rather than by the caller
  //   options  {enabledTypes: [...], groupBy: "module" | "facet",
  //             granularity: "claims" | "module" | "facet", expanded: [...]}
  //            enabledTypes omitted means all three types are on
  //            groupBy omitted means "module"
  //            granularity omitted means "claims" — nothing collapsed, which
  //              is the only granularity at which claim ids and drawn node
  //              ids are the same thing. A CALLER THAT DRAWS A COLLAPSED
  //              GRAPH AND DOES NOT PASS ITS GRANULARITY HERE GETS A RAIL
  //              THAT DESCRIBES A DIFFERENT GRAPH THAN THE CANVAS.
  //            expanded omitted means no per-group drill-down override
  //
  // granularity and expanded are the SAME two values handed to
  // representatives(), and they must be, because this function calls it: the
  // rail and the canvas are then reading one representative mapping rather
  // than two that agree by coincidence.
  //
  // Returns {facts: [...], hints: [...]}. The two arrays are separate so the
  // panel cannot render a heuristic alongside a fact by accident — see the
  // hints section below for why that separation is load-bearing.
  function gapRules(nodes, edges, options) {
    var opts = options && typeof options === 'object' ? options : {};
    var enabled = Array.isArray(opts.enabledTypes) ? opts.enabledTypes : EDGE_TYPES;
    var groupBy = normalizeGroupBy(opts.groupBy);
    var granularity = normalizeGranularity(opts.granularity);

    var list = asArray(nodes);
    var ids = [];
    var byId = new Map();
    for (var i = 0; i < list.length; i++) {
      var n = list[i] || {};
      var id = asString(n.id);
      if (id === '' || byId.has(id)) {
        continue;
      }
      ids.push(id);
      byId.set(id, n);
    }
    var inScope = new Set(ids);
    var sortedIds = sortedUnique(ids);

    // Edges with both endpoints in scope, in two flavours: every directed
    // relation (structural rules) and only the enabled types (connectivity).
    var structural = [];
    var connective = [];
    var rawEdges = asArray(edges);
    for (var j = 0; j < rawEdges.length; j++) {
      var e = normalizeEdge(rawEdges[j]);
      if (!e || !inScope.has(e.from) || !inScope.has(e.to)) {
        continue;
      }
      structural.push(e);
      if (typeEnabled(e.type, enabled)) {
        connective.push(e);
      }
    }

    // The scoped claim nodes, in id order — the input to both the
    // representative rule and the two heuristics.
    var scopedNodes = [];
    for (var s0 = 0; s0 < sortedIds.length; s0++) {
      scopedNodes.push(byId.get(sortedIds[s0]));
    }

    // THE DRAWN GRAPH. Everything the rail says "in this view" about is read
    // off these two, never off the claim list:
    //
    //   drawnIds    the ids of the nodes on screen — claim nodes at "claims"
    //               granularity, group nodes where a group is collapsed, and
    //               claim nodes again inside a group the reader expanded
    //   drawnEdges  those nodes' edges, mapped through the representatives,
    //               with intra-group self-loops dropped and parallel claim
    //               edges between the same pair collapsed to the one line
    //               that is actually drawn
    //
    // aggregateEdges is passed `connective`, whose endpoints are all in scope
    // and therefore all in repByClaim, so no ghost can appear here — which is
    // design section 3's rule that a ghost is counted by no gap rule, held by
    // construction rather than by a filter.
    //
    // Weight is deliberately stripped before counting degree. An aggregated
    // edge's weight is how many CLAIM edges it stands for; the connectivity
    // rules ask how many EDGES A READER CAN SEE, and three claim edges
    // between two collapsed modules are one line on the canvas.
    var reps = representatives(scopedNodes, granularity, opts.expanded);
    var drawnIds = [];
    var drawnNodes = new Map();
    for (var d0 = 0; d0 < reps.repNodes.length; d0++) {
      var rn = reps.repNodes[d0] || {};
      var rid = asString(rn.id);
      if (rid === '' || drawnNodes.has(rid)) {
        continue;
      }
      drawnIds.push(rid);
      drawnNodes.set(rid, rn);
    }
    var aggregated = aggregateEdges(connective, reps.repByClaim, null);
    var drawnEdges = [];
    for (var d1 = 0; d1 < aggregated.length; d1++) {
      drawnEdges.push({ from: aggregated[d1].from, to: aggregated[d1].to, type: aggregated[d1].type });
    }

    var facts = [];

    // cycle — one finding per component of size >= 2, always at least one
    // finding. scc() also returns the size-1 component of a claim that is its
    // own target; that one is filtered out HERE and reported by self_edge
    // instead. The engine has a dedicated error-severity `self-edge` lint
    // distinct from `cycle`, and a rail that listed the same claim under both
    // would be telling a reader a different story than `dossierx check` does.
    var components = scc(sortedIds, structural);
    var cycles = [];
    for (var c = 0; c < components.length; c++) {
      if (components[c].length >= 2) {
        cycles.push(components[c]);
      }
    }
    if (cycles.length === 0) {
      facts.push(finding('cycle', [], 'fact'));
    } else {
      for (var cc = 0; cc < cycles.length; cc++) {
        facts.push(finding('cycle', cycles[cc], 'fact'));
      }
    }

    // self_edge — reported under its own name, never merged into cycle.
    facts.push(finding('self_edge', selfEdges(sortedIds, structural), 'fact'));

    // isolated / weakly_linked — degree over the DRAWN graph, so "isolated in
    // this view" and "exactly one edge in this view" are statements about
    // nodes and lines a reader can point at. At "claims" granularity every
    // drawn node is a claim and this is the same set it always was.
    var deg = degrees(drawnIds, drawnEdges);
    var isolated = [];
    var weak = [];
    var sortedDrawnIds = sortedUnique(drawnIds);
    for (var k = 0; k < sortedDrawnIds.length; k++) {
      var total = deg[sortedDrawnIds[k]].total;
      if (total === 0) {
        isolated.push(sortedDrawnIds[k]);
      } else if (total === 1) {
        weak.push(sortedDrawnIds[k]);
      }
    }
    facts.push(finding('isolated', isolated, 'fact'));
    facts.push(finding('weakly_linked', weak, 'fact'));

    // review_pending / open_threads — engine-managed states that demand a
    // human. Both are node facts and neither depends on any edge, but both
    // still answer for the DRAWN node: a claim when the claim is drawn, and
    // otherwise the group that stands for it, listed once because at least
    // one of its members carries the state. That is the same reading the
    // canvas already draws — graph-ui.js haloes a collapsed group when any
    // member is review_pending — so the rail and the canvas now agree instead
    // of the rail naming claims the canvas has no node for.
    var pending = [];
    var threads = [];
    for (var m = 0; m < sortedDrawnIds.length; m++) {
      var drawn = drawnNodes.get(sortedDrawnIds[m]) || {};
      // A synthesised group node is the only thing that carries `members`.
      // Both halves of the test are load-bearing: a payload claim is free to
      // declare any `kind` string, and one that happened to say "group" must
      // still answer for itself rather than for an empty member list.
      var isGroup = drawn.kind === 'group' && Array.isArray(drawn.members);
      var members = isGroup ? drawn.members : [sortedDrawnIds[m]];
      var anyPending = false;
      var anyThreads = false;
      for (var mm = 0; mm < members.length; mm++) {
        var node = byId.get(asString(members[mm])) || {};
        if (node.review_pending === true) {
          anyPending = true;
        }
        var open = typeof node.open_comments === 'number' && isFinite(node.open_comments) ? node.open_comments : 0;
        if (open > 0) {
          anyThreads = true;
        }
      }
      if (anyPending) {
        pending.push(sortedDrawnIds[m]);
      }
      if (anyThreads) {
        threads.push(sortedDrawnIds[m]);
      }
    }
    facts.push(finding('review_pending', pending, 'fact'));
    facts.push(finding('open_threads', threads, 'fact'));

    // sink_group / orphan_group — group connectivity over CROSS-group edges
    // only. An edge inside a group says nothing about how the group connects
    // to the rest of the project, which is the only question these two ask.
    var groupOf = new Map(); // claim id -> group name
    var groupNames = [];
    for (var g = 0; g < sortedIds.length; g++) {
      var gname = groupNameOf(byId.get(sortedIds[g]), groupBy);
      groupOf.set(sortedIds[g], gname);
      groupNames.push(gname);
    }
    groupNames = sortedUnique(groupNames);

    var hasOut = new Set();
    var hasIn = new Set();
    for (var p = 0; p < connective.length; p++) {
      var gf = groupOf.get(connective[p].from);
      var gt = groupOf.get(connective[p].to);
      if (gf === gt) {
        continue;
      }
      hasOut.add(gf);
      hasIn.add(gt);
    }

    var sinks = [];
    var orphans = [];
    for (var q = 0; q < groupNames.length; q++) {
      var name = groupNames[q];
      var out = hasOut.has(name);
      var incoming = hasIn.has(name);
      if (out && !incoming) {
        sinks.push(groupId(groupBy, name));
      }
      if (!out && !incoming) {
        orphans.push(groupId(groupBy, name));
      }
    }
    facts.push(finding('sink_group', sinks.sort(cmpStr), 'fact'));
    facts.push(finding('orphan_group', orphans.sort(cmpStr), 'fact'));

    // Heuristics, in their own array. Both take the scoped CLAIM nodes only —
    // no edges, no representatives — and both always appear. They are
    // deliberately granularity-independent: a build phase is a property of a
    // module's build whatever the canvas is currently collapsed to, and the
    // ids they emit are module group ids at every granularity. Their labels
    // say "module" out loud for exactly that reason.
    var hints = [
      finding('missing_build_phase', missingBuildPhase(scopedNodes), 'hint'),
      finding('density_outlier', densityOutlier(scopedNodes), 'hint')
    ];

    return { facts: facts, hints: hints };
  }

  // ------------------------------------------------------------------
  // Hash state (design section 9)
  // ------------------------------------------------------------------
  //
  // The viewer's hash is one reading-view target id. The graph appends its
  // own state after a "!" separator:
  //
  //     #<existing-target-id>!g=<compact-graph-state>
  //
  // This pair encodes and decodes only the part after "g=". Owning nothing
  // before the "!" is the point: shell.html truncates its lookup at the first
  // "!" and re-appends whatever followed, so a filter change can never reset
  // the reading view and the reading view can never erase the filters.
  //
  // Values are percent-encoded, so a module or facet named with a separator
  // character survives. "!" is escaped too, even though only the FIRST one in
  // the whole hash is structural — a state string that cannot contain the
  // separator at all is one less thing to reason about.
  //
  // THE KEYS, and what each carries:
  //
  //   md  scope: module   "" for every module, else the module name
  //   fc  scope: facet    "" for every facet, else the facet name
  //   gr  granularity     "claims" | "module" | "facet"
  //   ov  overlay         one of OVERLAYS
  //   ty  edge types      a subset of TYPE_LETTERS, positional against EDGE_TYPES
  //   lb  labels          "1" | "0"
  //   ex  expanded groups comma-separated group ids
  //   se  selected node   a node id, or ""
  //
  // md and fc are two keys rather than one because scope is two independent
  // axes (see scopeFilter). They replaced a single `sc` key that packed one
  // axis into "module:<m>" / "facet:<f>" and could therefore express only one
  // of the two at a time. THERE IS NO COMPATIBILITY SHIM FOR `sc`: this format
  // has never been released, and decodeState ignores unknown keys, so a hash
  // carrying the old key decodes to the default whole-project scope rather
  // than to anything half-understood.

  // OVERLAYS is the closed set: six overlays plus "none". Anything else
  // decodes to "none" rather than leaving the pane in a state it cannot draw.
  var OVERLAYS = Object.freeze([
    'none',
    'isolated',
    'cycles',
    'governance',
    'review',
    'comments',
    'status'
  ]);

  // TYPE_LETTERS keeps the enabled-type set to three characters in the URL.
  // The mapping is positional against EDGE_TYPES, so the two cannot drift.
  var TYPE_LETTERS = 'rmg';

  // defaultState returns a fresh state object — everything on, nothing
  // filtered, nothing selected. Fresh rather than shared: a caller that
  // mutated a shared constant would poison every later default.
  function defaultState() {
    return {
      scopeModule: '',
      scopeFacet: '',
      granularity: 'claims',
      overlay: 'none',
      types: EDGE_TYPES.slice(),
      labels: true,
      expanded: [],
      selected: ''
    };
  }

  // enc / dec are the percent-encoding pair. dec never throws: a hand-edited
  // hash with a malformed escape decodes to the raw text rather than taking
  // the pane down.
  function enc(v) {
    return encodeURIComponent(asString(v)).replace(/!/g, '%21');
  }

  function dec(v) {
    try {
      return decodeURIComponent(v);
    } catch (err) {
      return v;
    }
  }

  function normalizeOverlay(v) {
    var s = asString(v);
    for (var i = 0; i < OVERLAYS.length; i++) {
      if (OVERLAYS[i] === s) {
        return s;
      }
    }
    return 'none';
  }

  // normalizeTypes returns the enabled edge types in EDGE_TYPES order, with
  // unknown entries dropped and duplicates removed. Canonical ordering is
  // what makes encodeState stable: the same state must always produce the
  // same string, whatever order the caller happened to build the array in.
  function normalizeTypes(v) {
    if (!Array.isArray(v)) {
      return EDGE_TYPES.slice();
    }
    var out = [];
    for (var i = 0; i < EDGE_TYPES.length; i++) {
      for (var j = 0; j < v.length; j++) {
        if (asString(v[j]) === EDGE_TYPES[i]) {
          out.push(EDGE_TYPES[i]);
          break;
        }
      }
    }
    return out;
  }

  // normalizeState fills every missing field from the defaults and canonicalises
  // the two order-sensitive ones, so encodeState is a function of MEANING and
  // not of the caller's array order.
  function normalizeState(state) {
    var s = state && typeof state === 'object' ? state : {};
    var d = defaultState();
    return {
      scopeModule:
        s.scopeModule === undefined || s.scopeModule === null ? d.scopeModule : asString(s.scopeModule),
      scopeFacet:
        s.scopeFacet === undefined || s.scopeFacet === null ? d.scopeFacet : asString(s.scopeFacet),
      granularity: normalizeGranularity(s.granularity === undefined ? d.granularity : s.granularity),
      overlay: normalizeOverlay(s.overlay === undefined ? d.overlay : s.overlay),
      types: normalizeTypes(s.types === undefined ? d.types : s.types),
      labels: s.labels === undefined ? d.labels : !!s.labels,
      expanded: sortedUnique(asArray(s.expanded).map(asString)),
      selected: s.selected === undefined || s.selected === null ? '' : asString(s.selected)
    };
  }

  // encodeState renders a state object as the compact string that follows
  // "g=" in the hash. Same state in, same string out — always.
  function encodeState(state) {
    var s = normalizeState(state);
    var letters = '';
    for (var i = 0; i < s.types.length; i++) {
      var at = EDGE_TYPES.indexOf(s.types[i]);
      if (at >= 0) {
        letters += TYPE_LETTERS.charAt(at);
      }
    }
    var expanded = [];
    for (var j = 0; j < s.expanded.length; j++) {
      expanded.push(enc(s.expanded[j]));
    }
    return (
      'md=' + enc(s.scopeModule) +
      '&fc=' + enc(s.scopeFacet) +
      '&gr=' + enc(s.granularity) +
      '&ov=' + enc(s.overlay) +
      '&ty=' + letters +
      '&lb=' + (s.labels ? '1' : '0') +
      '&ex=' + expanded.join(',') +
      '&se=' + enc(s.selected)
    );
  }

  // decodeState parses that string back. It is total: an empty, truncated,
  // reordered or hand-mangled string yields a usable state rather than an
  // exception, because the input is a URL a human may have typed.
  //
  // An ABSENT key falls back to its default; a key present with an empty
  // value means the empty value. That distinction is what makes the round
  // trip lossless for "no edge types enabled" and "no groups expanded".
  function decodeState(str) {
    var out = defaultState();
    var text = asString(str);
    if (text === '') {
      return out;
    }
    var parts = text.split('&');
    for (var i = 0; i < parts.length; i++) {
      var part = parts[i];
      if (part === '') {
        continue;
      }
      var at = part.indexOf('=');
      var key = at < 0 ? part : part.slice(0, at);
      var val = at < 0 ? '' : part.slice(at + 1);
      if (key === 'md') {
        out.scopeModule = dec(val);
      } else if (key === 'fc') {
        out.scopeFacet = dec(val);
      } else if (key === 'gr') {
        out.granularity = normalizeGranularity(dec(val));
      } else if (key === 'ov') {
        out.overlay = normalizeOverlay(dec(val));
      } else if (key === 'ty') {
        var types = [];
        for (var t = 0; t < EDGE_TYPES.length; t++) {
          if (val.indexOf(TYPE_LETTERS.charAt(t)) >= 0) {
            types.push(EDGE_TYPES[t]);
          }
        }
        out.types = types;
      } else if (key === 'lb') {
        out.labels = val === '1';
      } else if (key === 'ex') {
        var groups = [];
        if (val !== '') {
          var raw = val.split(',');
          for (var g = 0; g < raw.length; g++) {
            groups.push(dec(raw[g]));
          }
        }
        out.expanded = sortedUnique(groups);
      } else if (key === 'se') {
        out.selected = dec(val);
      }
      // Unknown keys are ignored rather than preserved. A future version's
      // extra field must not survive a round trip through this one and be
      // handed back as if this version had understood it. The retired `sc`
      // key is exactly such a key now: a hash carrying it lands on the
      // default whole-project scope, which is the honest reading of a
      // selection this version cannot express.
    }
    return out;
  }

  var api = {
    EDGE_TYPES: EDGE_TYPES,
    DIRECTED_EDGE_TYPES: DIRECTED_EDGE_TYPES,
    GHOST_PREFIX: GHOST_PREFIX,
    FACET_SLOT_COUNT: FACET_SLOT_COUNT,
    FACT_RULE_IDS: FACT_RULE_IDS,
    HINT_RULE_IDS: HINT_RULE_IDS,
    OVERLAYS: OVERLAYS,
    BUILD_PHASES: BUILD_PHASES,

    groupId: groupId,
    edgeKey: edgeKey,

    scopeFilter: scopeFilter,
    representatives: representatives,
    aggregateEdges: aggregateEdges,
    degrees: degrees,
    degreeFor: degreeFor,

    scc: scc,
    selfEdges: selfEdges,

    facetSlot: facetSlot,
    governors: governors,
    governanceScope: governanceScope,

    gapRules: gapRules,

    defaultState: defaultState,
    encodeState: encodeState,
    decodeState: decodeState
  };

  // The one global. Bound through a root expression rather than `window`
  // directly so the same file loads unchanged in a bare node harness, which is
  // how every pure function here is proven during development.
  var root = typeof window !== 'undefined' ? window : globalThis;
  root.dossierxGraphCore = api;
})();
