(function () {
  'use strict';

  // ==================================================================
  // graph-ui.js — the DossierX claims graph pane
  // ==================================================================
  //
  // DOM construction, canvas rendering, force layout, controls, panels,
  // refresh and hash state. Every verdict this file DRAWS is computed by
  // graph-core.js, which is DOM-free and unit-tested; nothing here decides
  // what is a cycle, what is isolated or which palette slot a facet takes.
  // That boundary is what keeps the untestable half — pixels — free of logic
  // a wrong pixel could hide.
  //
  // ------------------------------------------------------------------
  // FIVE CONTRACTS THIS FILE KEEPS, EACH FOR A STATED REASON
  // ------------------------------------------------------------------
  //
  // 1. INERT UNTIL FIRST OPENED. At parse time this file registers ONE
  //    delegated listener and reads the hash. It builds no DOM, starts no
  //    simulation and — this part is a contract, not an optimisation —
  //    PARSES NO PAYLOAD. A browser test replaces the payload block's
  //    textContent before opening the pane in order to render a cycle the
  //    rendered document could never legally contain, and that only works
  //    because the parse happens at first open.
  //
  // 2. ONE DELEGATED LISTENER ON document, never a listener on the trigger
  //    button. The button lives inside <nav id="nav">, which a live-reload
  //    fragment swap replaces wholesale by outerHTML assignment; a listener
  //    bound to the button would be destroyed with it and the pane would
  //    stop opening after the first edit of a serve session. The pane root
  //    itself is mounted OUTSIDE div.layout, so it and all of its state
  //    survive that same swap.
  //
  // 3. THE HASH IS WRITTEN WITH history.replaceState ONLY. The viewer's hash
  //    is a reading-view target id; the graph appends its own state after a
  //    "!" separator and owns nothing before it. replaceState does not fire
  //    hashchange, so changing a filter never re-enters the reading view's
  //    routing and can never reset it to the first module. The one place
  //    location.hash is assigned is the detail panel's "open claim" action,
  //    which WANTS the reading view to move.
  //
  // 4. NO NETWORK EXCEPT ONE SAME-ORIGIN GET. The refresh button fetches the
  //    relative path /api/graph and nothing else. There is no external
  //    reference of any kind in this file — no remote script, no font, no
  //    remote image — and the repository's offline scan walks it to prove it.
  //
  // 5. COLOUR COMES FROM THE STYLESHEET, NOT FROM HERE. The canvas reads
  //    every colour it draws with getComputedStyle on the document element,
  //    so a palette exists in exactly one place and an OS light/dark switch
  //    repaints correctly with no copy to keep in sync.

  var root = typeof window !== 'undefined' ? window : globalThis;

  // ------------------------------------------------------------------
  // Constants
  // ------------------------------------------------------------------

  // The pane root and the payload block are both authored in shell.html.
  // This file never creates either: a pane with no mount point does nothing
  // at all rather than inventing a place to live.
  var PANE_ID = 'dxgPane';
  var PAYLOAD_ID = 'dossierx-graph';

  // The trigger's contract, in one place. It is an ATTRIBUTE, not a class,
  // and deliberately not `.sec-tab` — the viewer's existing delegated handler
  // matches .sec-tab and would also switch modules.
  var OPEN_SELECTOR = '[data-dxg-open]';

  // shell.html adds `comments-live` to <body> once its own ~1s probe has
  // confirmed a live serve. This file reuses that verdict rather than probing
  // again; see maybeBuildRefresh.
  var LIVE_CLASS = 'comments-live';

  // Additive with the existing body.nav-open / body.comments-open scroll
  // locks. Three classes each setting overflow: hidden compose without a
  // release-order hazard.
  var BODY_OPEN_CLASS = 'dxg-open';

  // The one network path this file knows. Relative, same-origin, read-only.
  var GRAPH_ENDPOINT = '/api/graph';

  // The hash contract: #<reading-view-target>!g=<compact-graph-state>.
  var HASH_MARK = '!';
  var HASH_PREFIX = 'g=';

  // AUTO_COLLAPSE_ABOVE is the claim count above which the pane opens at
  // module granularity instead of drawing every claim. 600 is a guess and
  // will be wrong for somebody, which is exactly why it lives here as ONE
  // named constant, is stated out loud in the notice the reader sees, and is
  // overridable from that notice. It gets revisited when a real corpus this
  // large exists.
  var AUTO_COLLAPSE_ABOVE = 600;

  // Above this many drawn nodes, labels are suppressed unless the reader has
  // zoomed in: past it the labels overlap into a grey smear that hides the
  // shape the pane exists to show.
  var LABEL_NODE_CEILING = 220;

  // Above this many drawn nodes the layout drops its O(n^2) repulsion pass
  // and runs on springs plus gravity alone. Auto-collapse means this is
  // normally unreachable; it is the floor under a reader who overrode it.
  var PAIRWISE_CEILING = 900;

  // Last-resort colour, used only if graph.css did not load at all — every
  // real value comes from getComputedStyle. A visible grey graph is a better
  // failure than a canvas throwing on an empty fillStyle.
  var FALLBACK_COLOR = '#808080';

  // The overlay select's contents, in the frozen order: isolated, cycles,
  // GOVERNANCE, review, comments, status. The ids are graph-core.js's closed
  // OVERLAYS set; the text is display only and nothing keys off it.
  var OVERLAY_OPTIONS = [
    ['none', 'none'],
    ['isolated', 'isolated & weakly linked'],
    ['cycles', 'dependency cycles'],
    ['governance', 'governance'],
    ['review', 'review pending'],
    ['comments', 'open comment threads'],
    ['status', 'draft vs locked']
  ];

  var GRANULARITY_OPTIONS = [
    ['claims', 'every claim'],
    ['module', 'collapse to modules'],
    ['facet', 'collapse to facets']
  ];

  // Display text for a gap rule id. The rail renders BOTH this and the raw
  // rule id, because the id is the stable key the browser tests and any
  // future export use, and hiding it would make a reader guess which line of
  // the design a block corresponds to.
  var RULE_LABELS = {
    cycle: 'dependency cycle',
    self_edge: 'claim depends on itself',
    isolated: 'isolated in this view',
    weakly_linked: 'exactly one edge in this view',
    review_pending: 'review pending',
    open_threads: 'open comment threads',
    sink_group: 'group nothing links into',
    orphan_group: 'group with no outside edges',
    missing_build_phase: 'module missing a build phase',
    density_outlier: 'facet thin in one module'
  };

  // ------------------------------------------------------------------
  // Pane state. All of it client-only, all of it destroyed by nothing
  // short of a full page load — which is the point of mounting outside
  // the swapped subtree.
  // ------------------------------------------------------------------

  var payload = null; // parsed at FIRST OPEN, never before
  var mounted = false;
  var isOpen = false;
  var el = {}; // built DOM, by role
  var state = null; // graph-core.js state shape
  var camera = { zoom: 1, x: 0, y: 0 };
  var positions = Object.create(null); // claim/group id -> {x, y, vx, vy}
  var scene = null; // last recompute(): nodes, edges, verdicts
  var hoverFacet = null; // legend hover: null = none, else the hovered facet name
  var frame = 0; // requestAnimationFrame handle
  var alpha = 0; // force-layout temperature
  var parseTimeHash = ''; // the graph state segment seen at parse time
  var collapseOverride = false; // the reader insisted on every claim
  var notices = []; // {text, kind, action} rendered above the canvas
  var dragging = null; // {id} while a node is being dragged
  var panning = null; // {x, y} while the background is being dragged
  var movedWhileDown = false;

  // ------------------------------------------------------------------
  // Small helpers
  // ------------------------------------------------------------------

  // graph-core.js is loaded by the script block immediately before this one,
  // so it is present at parse time — but it is read through a function rather
  // than captured in a var so that a page which somehow loaded them out of
  // order degrades to an inert pane instead of a parse-time exception.
  function core() {
    return root.dossierxGraphCore || null;
  }

  function h(tag, cls, text) {
    var node = document.createElement(tag);
    if (cls) {
      node.className = cls;
    }
    if (text !== undefined && text !== null) {
      node.textContent = String(text);
    }
    return node;
  }

  function clear(node) {
    while (node && node.firstChild) {
      node.removeChild(node.firstChild);
    }
  }

  function str(v) {
    return v === null || v === undefined ? '' : String(v);
  }

  function num(v) {
    return typeof v === 'number' && isFinite(v) ? v : 0;
  }

  // ------------------------------------------------------------------
  // The hash segment (design section 9)
  // ------------------------------------------------------------------
  //
  // Deliberately no regular expression anywhere in this file: the offline
  // scan's comment stripper treats a "/" as division, so a regex literal is
  // one more thing to reason about for no gain over indexOf.

  function stripLeadingHash(raw) {
    var s = str(raw);
    return s.charAt(0) === '#' ? s.slice(1) : s;
  }

  // hashHead is everything BEFORE the "!" — the reading view's target id,
  // which this file preserves verbatim and never interprets.
  function hashHead(raw) {
    var s = stripLeadingHash(raw);
    var at = s.indexOf(HASH_MARK);
    return at < 0 ? s : s.slice(0, at);
  }

  // hashGraphState is everything after "!g=", or "" when the hash carries no
  // graph segment at all.
  function hashGraphState(raw) {
    var s = stripLeadingHash(raw);
    var at = s.indexOf(HASH_MARK);
    if (at < 0) {
      return '';
    }
    var rest = s.slice(at + 1);
    return rest.indexOf(HASH_PREFIX) === 0 ? rest.slice(HASH_PREFIX.length) : '';
  }

  // ------------------------------------------------------------------
  // The payload — read ONCE, at first open
  // ------------------------------------------------------------------

  // normalizePayload makes every field the rest of this file reads present
  // and of the right type, so no drawing path has to defend itself against a
  // truncated or hand-edited payload block.
  function normalizePayload(raw) {
    if (!raw || typeof raw !== 'object') {
      return null;
    }
    var groups = raw.groups && typeof raw.groups === 'object' ? raw.groups : {};
    var dropped = raw.dropped && typeof raw.dropped === 'object' ? raw.dropped : {};
    return {
      schema: num(raw.schema),
      generated_at: str(raw.generated_at),
      nodes: Array.isArray(raw.nodes) ? raw.nodes : [],
      edges: Array.isArray(raw.edges) ? raw.edges : [],
      groups: {
        modules: Array.isArray(groups.modules) ? groups.modules : [],
        facets: Array.isArray(groups.facets) ? groups.facets : []
      },
      dropped: { unresolved_edges: num(dropped.unresolved_edges) }
    };
  }

  // readPayload is the ONLY place this file parses JSON, and it is reached
  // only from the first-open path. Nothing above it runs at parse time.
  function readPayload() {
    var block = document.getElementById(PAYLOAD_ID);
    if (!block) {
      return null;
    }
    try {
      return normalizePayload(JSON.parse(block.textContent || ''));
    } catch (err) {
      return null;
    }
  }

  // ------------------------------------------------------------------
  // Mount — builds the pane's containers once, on first open
  // ------------------------------------------------------------------

  function mountPane(pane) {
    var surface = h('div', 'dxg-surface');
    surface.setAttribute('role', 'dialog');
    surface.setAttribute('aria-modal', 'true');
    surface.setAttribute('aria-label', 'Claims graph');
    surface.tabIndex = -1;

    el.head = h('header', 'dxg-head');
    el.controls = h('div', 'dxg-controls');
    el.notices = h('div', 'dxg-notices');

    var body = h('div', 'dxg-body');
    el.holder = h('div', 'dxg-canvas-holder');
    el.canvas = h('canvas', 'dxg-canvas');
    el.hint = h(
      'p',
      'dxg-canvas-hint',
      'drag node · drag background to pan · scroll to zoom · double-click a group to expand it'
    );
    el.holder.appendChild(el.canvas);
    el.holder.appendChild(el.hint);

    el.rail = h('aside', 'dxg-rail');
    el.gaps = h('div', 'dxg-gaps');
    el.detail = h('div', 'dxg-detail');
    el.rail.appendChild(el.gaps);
    el.rail.appendChild(el.detail);

    body.appendChild(el.holder);
    body.appendChild(el.rail);

    el.legend = h('div', 'dxg-legend');

    surface.appendChild(el.head);
    surface.appendChild(el.controls);
    surface.appendChild(el.notices);
    surface.appendChild(body);
    surface.appendChild(el.legend);

    clear(pane);
    pane.appendChild(surface);
    el.pane = pane;
    el.surface = surface;
    el.ctx = el.canvas.getContext ? el.canvas.getContext('2d') : null;

    buildControls();
    bindCanvas();
    bindWindow();
    mounted = true;
  }

  // ------------------------------------------------------------------
  // Open and close
  // ------------------------------------------------------------------

  function openPane() {
    var pane = document.getElementById(PANE_ID);
    if (!pane || !core()) {
      return; // no mount point, or the core never loaded: stay inert
    }
    if (!mounted) {
      mountPane(pane);
    }
    if (payload === null) {
      payload = readPayload();
    }

    // The hash is re-read on every open rather than trusted from parse time,
    // because the reading view rewrites the hash as a reader moves around and
    // the graph segment rides along with it.
    var encoded = hashGraphState(root.location ? root.location.hash : '');
    if (encoded === '' && parseTimeHash !== '') {
      encoded = parseTimeHash;
    }
    if (state === null) {
      state = encoded === '' ? core().defaultState() : core().decodeState(encoded);
      applyAutoCollapse(encoded !== '');
    } else if (encoded !== '') {
      state = core().decodeState(encoded);
    }

    pane.hidden = false;
    isOpen = true;
    document.body.classList.add(BODY_OPEN_CLASS);

    // Rebuilt on EVERY open, not only the first: a pane opened inside the
    // live probe's ~1s window would otherwise be missing its refresh button
    // forever.
    buildHeader();
    refreshControls();
    buildLegend();
    resizeCanvas();
    recompute();
    startLayout(true);
    writeHash();
    if (el.surface) {
      el.surface.focus();
    }
  }

  function closePane() {
    if (!mounted || !isOpen) {
      return;
    }
    isOpen = false;
    el.pane.hidden = true;
    document.body.classList.remove(BODY_OPEN_CLASS);
    stopLayout();
  }

  function togglePane() {
    if (isOpen) {
      closePane();
    } else {
      openPane();
    }
  }

  // ------------------------------------------------------------------
  // Parse-time work. One listener, one hash read. Nothing else.
  // ------------------------------------------------------------------

  if (typeof document !== 'undefined') {
    parseTimeHash = hashGraphState(root.location ? root.location.hash : '');

    document.addEventListener('click', function (e) {
      var target = e.target && e.target.closest ? e.target.closest(OPEN_SELECTOR) : null;
      if (!target) {
        return;
      }
      e.preventDefault();
      togglePane();
    });
  }

  // ------------------------------------------------------------------
  // The control bar — five groups, in the frozen prototype order
  // ------------------------------------------------------------------
  //
  //   Scope · Granularity · Highlight overlay · Edge types · View
  //
  // Built ONCE at mount; its option lists are refilled from the payload on
  // every open and on every refresh, because a refreshed payload can carry a
  // module or facet that did not exist a minute ago.

  function controlGroup(labelText) {
    var group = h('div', 'dxg-ctl');
    group.appendChild(h('span', 'dxg-ctl-label', labelText));
    return group;
  }

  function selectControl(id, onChange) {
    var sel = h('select', 'dxg-select');
    sel.id = id;
    sel.addEventListener('change', onChange);
    return sel;
  }

  function option(value, text) {
    var opt = h('option', '', text);
    opt.value = value;
    return opt;
  }

  function toggleButton(label, pressed, onClick) {
    var btn = h('button', 'dxg-toggle', label);
    btn.type = 'button';
    btn.setAttribute('aria-pressed', pressed ? 'true' : 'false');
    btn.addEventListener('click', onClick);
    return btn;
  }

  function buildControls() {
    var c = core();

    var scopeGroup = controlGroup('Scope');
    el.scope = selectControl('dxgScope', function () {
      state.scope = el.scope.value;
      onControlChange(true);
    });
    scopeGroup.appendChild(el.scope);

    var granGroup = controlGroup('Granularity');
    el.granularity = selectControl('dxgGranularity', function () {
      state.granularity = el.granularity.value;
      noteManualGranularity();
      onControlChange(true);
    });
    for (var g = 0; g < GRANULARITY_OPTIONS.length; g++) {
      el.granularity.appendChild(option(GRANULARITY_OPTIONS[g][0], GRANULARITY_OPTIONS[g][1]));
    }
    granGroup.appendChild(el.granularity);

    var overlayGroup = controlGroup('Highlight overlay');
    el.overlay = selectControl('dxgOverlay', function () {
      state.overlay = el.overlay.value;
      onControlChange(false);
    });
    for (var o = 0; o < OVERLAY_OPTIONS.length; o++) {
      el.overlay.appendChild(option(OVERLAY_OPTIONS[o][0], OVERLAY_OPTIONS[o][1]));
    }
    overlayGroup.appendChild(el.overlay);

    // Edge types: one independently toggleable button per relation, so a
    // cluttered graph can be read one relation at a time. The list comes from
    // graph-core.js's EDGE_TYPES, not from a literal here, so the two cannot
    // drift.
    var typeGroup = controlGroup('Edge types');
    el.typeButtons = {};
    var types = c ? c.EDGE_TYPES : [];
    for (var t = 0; t < types.length; t++) {
      (function (type) {
        var btn = toggleButton(type, true, function () {
          toggleType(type);
        });
        btn.setAttribute('data-dxg-type', type);
        el.typeButtons[type] = btn;
        typeGroup.appendChild(btn);
      })(types[t]);
    }

    var viewGroup = controlGroup('View');
    el.labels = toggleButton('labels', true, function () {
      state.labels = !state.labels;
      onControlChange(false);
    });
    el.labels.setAttribute('data-dxg-labels', '');
    el.relayout = h('button', 'dxg-btn', 're-run layout');
    el.relayout.type = 'button';
    el.relayout.setAttribute('data-dxg-relayout', '');
    el.relayout.addEventListener('click', function () {
      positions = Object.create(null);
      recompute();
      startLayout(true);
    });
    viewGroup.appendChild(el.labels);
    viewGroup.appendChild(el.relayout);

    el.controls.appendChild(scopeGroup);
    el.controls.appendChild(granGroup);
    el.controls.appendChild(overlayGroup);
    el.controls.appendChild(typeGroup);
    el.controls.appendChild(viewGroup);
  }

  function toggleType(type) {
    var next = [];
    var had = false;
    for (var i = 0; i < state.types.length; i++) {
      if (state.types[i] === type) {
        had = true;
      } else {
        next.push(state.types[i]);
      }
    }
    if (!had) {
      next.push(type);
    }
    state.types = next;
    onControlChange(true);
  }

  // refreshControls repopulates the scope options from the CURRENT payload
  // and pushes the state object back onto every control. It is the one place
  // control DOM and state are reconciled, so a state arriving from a pasted
  // hash and one arriving from a click take the same path.
  function refreshControls() {
    var c = core();
    var groups = payload ? payload.groups : { modules: [], facets: [] };

    clear(el.scope);
    el.scope.appendChild(option('all', 'all claims'));
    var i;
    for (i = 0; i < groups.modules.length; i++) {
      el.scope.appendChild(option('module:' + groups.modules[i], 'module — ' + groups.modules[i]));
    }
    for (i = 0; i < groups.facets.length; i++) {
      el.scope.appendChild(option('facet:' + groups.facets[i], 'facet — ' + groups.facets[i]));
    }
    // A scope naming a module or facet the payload no longer has would leave
    // the select showing something the graph is not doing. Fall back rather
    // than draw an empty canvas.
    el.scope.value = state.scope;
    if (el.scope.selectedIndex < 0) {
      state.scope = 'all';
      el.scope.value = 'all';
    }

    el.granularity.value = state.granularity;
    el.overlay.value = state.overlay;

    var types = c ? c.EDGE_TYPES : [];
    for (i = 0; i < types.length; i++) {
      var btn = el.typeButtons[types[i]];
      if (btn) {
        btn.setAttribute('aria-pressed', state.types.indexOf(types[i]) >= 0 ? 'true' : 'false');
      }
    }
    el.labels.setAttribute('aria-pressed', state.labels ? 'true' : 'false');
  }

  // onControlChange is the single funnel every control passes through:
  // recompute the scene, write the hash, redraw. reheat is true for the
  // changes that move nodes (scope, granularity, edge types) and false for
  // the ones that only repaint (overlay, labels).
  function onControlChange(reheat) {
    refreshControls();
    recompute();
    writeHash();
    if (reheat) {
      startLayout(false);
    } else {
      draw();
    }
  }

  // ------------------------------------------------------------------
  // The legend strip — facet identity's second channel
  // ------------------------------------------------------------------
  //
  // Every facet the PROJECT declared, by its own name, against its assigned
  // swatch. Hovering an entry dims every node on the canvas that is not a
  // member, which is the disambiguator that still works at twenty facets
  // where colour alone has stopped working at about twelve.

  // GOVERNED_SAMPLE is a constant string with no interpolation of any kind —
  // no payload value reaches it — so assigning it as markup cannot carry
  // project data into the document. It draws the same three visual channels
  // the canvas draws: a curve where the other relations are straight, and a
  // double chevron where rests_on has one.
  var GOVERNED_SAMPLE =
    '<svg viewBox="0 0 34 12" aria-hidden="true" focusable="false" width="34" height="12">' +
    '<path d="M1 9 Q 13 1 24 6" fill="none" stroke="currentColor" stroke-width="1.6"/>' +
    '<path d="M20 2.5 L25 6 L20 9.5" fill="none" stroke="currentColor" stroke-width="1.6"/>' +
    '<path d="M24.5 2.5 L29.5 6 L24.5 9.5" fill="none" stroke="currentColor" stroke-width="1.6"/>' +
    '</svg>';

  function buildLegend() {
    clear(el.legend);
    var list = h('ul', 'dxg-legend-list');
    var facets = payload ? payload.groups.facets : [];
    var c = core();

    for (var i = 0; i < facets.length; i++) {
      var name = str(facets[i]);
      var slot = c ? c.facetSlot(facets, name) : -1;
      var item = h('li', 'dxg-legend-item');
      item.setAttribute('data-dxg-facet', name);
      var swatch = h('span', 'dxg-legend-swatch dxg-swatch-' + (slot < 0 ? 'other' : slot + 1));
      item.appendChild(swatch);
      item.appendChild(h('span', 'dxg-legend-name', name));
      bindLegendHover(item, name);
      list.appendChild(item);
    }

    // The catch-all slot only earns a row when some claim actually wears it.
    if (payload && hasUnslottedFacet(facets)) {
      var other = h('li', 'dxg-legend-item');
      other.setAttribute('data-dxg-facet', '');
      other.appendChild(h('span', 'dxg-legend-swatch dxg-swatch-other'));
      other.appendChild(h('span', 'dxg-legend-name', 'no facet'));
      bindLegendHover(other, '');
      list.appendChild(other);
    }

    var edgeItem = h('li', 'dxg-legend-item dxg-legend-item--edge');
    edgeItem.setAttribute('data-dxg-edge', 'governed_by');
    var sample = h('span', 'dxg-legend-edge');
    sample.innerHTML = GOVERNED_SAMPLE;
    edgeItem.appendChild(sample);
    edgeItem.appendChild(h('span', 'dxg-legend-name', 'governed_by'));
    list.appendChild(edgeItem);

    el.legend.appendChild(list);
  }

  function hasUnslottedFacet(facets) {
    var c = core();
    for (var i = 0; i < payload.nodes.length; i++) {
      if (!c || c.facetSlot(facets, str(payload.nodes[i].facet)) < 0) {
        return true;
      }
    }
    return false;
  }

  function bindLegendHover(item, name) {
    item.addEventListener('mouseenter', function () {
      hoverFacet = name;
      draw();
    });
    item.addEventListener('mouseleave', function () {
      hoverFacet = null;
      draw();
    });
  }

  // ------------------------------------------------------------------
  // The pane header — how fresh is what you are looking at?
  // ------------------------------------------------------------------
  //
  // A live-reload fragment swap replaces the nav and the content area. It
  // does NOT replace the payload block, which sits outside both, so a claim
  // edited during a serve session updates the reading view and not the graph.
  // That is accepted, and this line is what makes it honest rather than a
  // trap: the pane states, in words, how old the shape on screen is. A reader
  // an hour into a session can see at a glance that they are looking at an
  // hour-old picture.
  //
  // It works identically in a static file:// viewer, where the answer is
  // simply "when check ran" — and there the pane offers no refresh button,
  // because there is no server to ask.

  // relativeStamp renders generated_at as a phrase. Deliberately coarse: the
  // question a reader is asking is "is this stale?", not "how many seconds".
  function relativeStamp(iso) {
    var when = Date.parse(iso);
    if (!isFinite(when)) {
      return 'payload generation time unknown';
    }
    var secs = Math.round((Date.now() - when) / 1000);
    if (secs < 0) {
      // A clock skew between the machine that rendered and the one reading.
      // Saying "in 3 seconds" would be absurd; saying "just now" is true
      // enough at the resolution this line is asking about.
      secs = 0;
    }
    if (secs < 45) {
      return 'payload generated just now';
    }
    var mins = Math.round(secs / 60);
    if (mins < 60) {
      return 'payload generated ' + mins + (mins === 1 ? ' minute ago' : ' minutes ago');
    }
    var hours = Math.round(mins / 60);
    if (hours < 24) {
      return 'payload generated ' + hours + (hours === 1 ? ' hour ago' : ' hours ago');
    }
    var days = Math.round(hours / 24);
    return 'payload generated ' + days + (days === 1 ? ' day ago' : ' days ago');
  }

  // buildHeader is rebuilt on EVERY open, never only on the first. Two things
  // depend on that: the stamp goes stale as a session runs, and shell.html's
  // live probe may not have added `comments-live` yet when a fast reader
  // opens the pane — the next open gains the refresh button rather than the
  // pane being wrong for the life of the page.
  function buildHeader() {
    clear(el.head);
    el.head.appendChild(h('h2', 'dxg-title', 'Claims graph'));

    var stamp = h('span', 'dxg-stamp');
    stamp.setAttribute('data-dxg-stamp', '');
    if (payload) {
      stamp.textContent = relativeStamp(payload.generated_at);
      // The absolute value on the title attribute, so "4 minutes ago" is
      // never the only answer available.
      stamp.title = payload.generated_at || 'no generation time in payload';
    } else {
      stamp.textContent = 'no graph payload in this document';
      stamp.title = 'the payload block is missing or could not be parsed';
    }
    el.stamp = stamp;
    el.head.appendChild(stamp);

    var actions = h('div', 'dxg-head-actions');
    maybeBuildRefresh(actions);

    var close = h('button', 'dxg-btn', 'close');
    close.type = 'button';
    close.setAttribute('data-dxg-close', '');
    close.addEventListener('click', closePane);
    actions.appendChild(close);

    el.head.appendChild(actions);
  }

  // ------------------------------------------------------------------
  // Refresh — ABSENT without a server, not disabled
  // ------------------------------------------------------------------
  //
  // shell.html already probes for a live serve: a relative fetch of the ping
  // endpoint with a ~1s timeout, requiring res.ok, a JSON content type and
  // the expected body, and only then adding `comments-live` to <body>. On
  // file:// that fetch rejects and is swallowed, which is the correct outcome
  // for a read-only document. This file REUSES that verdict rather than
  // probing again — one probe, one answer, one place to change it.
  //
  // The button is CREATED only when the class is present, and not created at
  // all otherwise. A disabled-looking control in a static file would be a
  // promise the artifact cannot keep; a control that is simply not there says
  // the truth, which is that this document is a snapshot.

  function maybeBuildRefresh(actions) {
    if (!document.body.classList.contains(LIVE_CLASS)) {
      return;
    }
    var btn = h('button', 'dxg-btn', 'refresh');
    btn.type = 'button';
    btn.setAttribute('data-dxg-refresh', '');
    btn.addEventListener('click', function () {
      doRefresh(btn);
    });
    el.refresh = btn;
    actions.appendChild(btn);
  }

  // doRefresh asks the server for a payload stamped now. A FAILED fetch
  // leaves the graph exactly as it was and says so inline: a broken pane is
  // never an acceptable outcome of asking for fresher data.
  function doRefresh(btn) {
    if (typeof fetch !== 'function') {
      showNotice('this browser cannot fetch a fresh payload', 'warn');
      return;
    }
    btn.disabled = true;
    btn.textContent = 'refreshing…';
    var done = function () {
      btn.disabled = false;
      btn.textContent = 'refresh';
    };
    fetch(GRAPH_ENDPOINT, { headers: { Accept: 'application/json' } })
      .then(function (res) {
        if (!res.ok) {
          throw new Error('status ' + res.status);
        }
        return res.json();
      })
      .then(function (next) {
        done();
        applyPayload(next);
      })
      .catch(function (err) {
        done();
        // The graph is untouched. Only the notice strip changes.
        showNotice('could not fetch a fresh payload (' + str(err && err.message) + ')', 'warn');
        renderNotices();
      });
  }

  // ------------------------------------------------------------------
  // Notices — the strip between the controls and the canvas
  // ------------------------------------------------------------------
  //
  // Two kinds, kept apart. STANDING notices are recomputed from the payload
  // and the current state every time the strip renders: unresolved edges,
  // auto-collapse, and the warning a reader gets after overriding it.
  // TRANSIENT ones are the report of something that just happened — a failed
  // refresh — and survive until the next thing happens.

  var transientNotice = null;

  function showNotice(text, kind) {
    transientNotice = { text: text, kind: kind };
  }

  function clearNotice() {
    transientNotice = null;
  }

  function noticeRow(text, kind, actionText, onAction) {
    var row = h('div', 'dxg-notice' + (kind === 'warn' ? ' dxg-notice--warn' : ''), text);
    if (actionText) {
      var btn = h('button', 'dxg-notice-action', actionText);
      btn.type = 'button';
      btn.addEventListener('click', onAction);
      row.appendChild(btn);
    }
    return row;
  }

  function renderNotices() {
    if (!el.notices) {
      return;
    }
    clear(el.notices);
    if (!payload) {
      el.notices.appendChild(
        noticeRow('this document carries no readable graph payload', 'warn')
      );
      return;
    }

    // serve never lints, so a live session genuinely can carry an edge whose
    // target is not a claim. Saying so beats silently drawing a smaller graph
    // than the data describes.
    var dropped = payload.dropped.unresolved_edges;
    if (dropped > 0) {
      el.notices.appendChild(
        noticeRow(
          dropped +
            (dropped === 1 ? ' edge points' : ' edges point') +
            ' at an id this project does not define, and is not drawn',
          'warn'
        )
      );
    }

    renderCollapseNotice();

    if (transientNotice) {
      el.notices.appendChild(noticeRow(transientNotice.text, transientNotice.kind));
    }
  }

  // ------------------------------------------------------------------
  // applyPayload — a refresh must not throw away what you were looking at
  // ------------------------------------------------------------------
  //
  // Preserved verbatim: the camera (zoom and pan), every control (scope,
  // granularity, overlay, enabled edge types, labels), the expanded-group
  // set, and node positions by id. A node whose id is NEW is seeded near its
  // group's centroid rather than at the origin, so asking for fresher data
  // does not fling the layout apart. Selection survives if its id survives.
  //
  // The header timestamp updating is the visible proof the button did
  // something — it is the one thing that is deliberately NOT preserved.

  function applyPayload(next) {
    var p = normalizePayload(next);
    if (!p) {
      showNotice('the fresh payload could not be read; the graph is unchanged', 'warn');
      renderNotices();
      return;
    }

    payload = p;
    clearNotice();

    // Selection is a claim id, not an index: it survives unless the claim
    // itself is gone.
    if (state.selected !== '' && !hasNode(state.selected)) {
      state.selected = '';
    }

    // Positions are keyed by id and simply kept. Ids that vanished leave
    // stale entries behind, which cost nothing and would be needed again the
    // moment an undo brings the claim back; ids that are new get seeded by
    // seedPosition() the first time the layout asks where they are.
    buildHeader();
    refreshControls();
    buildLegend();
    recompute();
    startLayout(false);
  }

  function hasNode(id) {
    if (!payload) {
      return false;
    }
    for (var i = 0; i < payload.nodes.length; i++) {
      if (str(payload.nodes[i].id) === id) {
        return true;
      }
    }
    return false;
  }

  // ------------------------------------------------------------------
  // Positions
  // ------------------------------------------------------------------

  // seedPosition places a node the layout has never seen. A node with a group
  // whose other members are already placed lands near their centroid; one
  // with no placed sibling lands on a small ring around the canvas centre.
  // Neither is the origin, and that is the point: every new node arriving at
  // 0,0 makes a refresh look like an explosion.
  function seedPosition(node, index, total, groupCentroids) {
    var gname = groupKeyOf(node);
    var seed = groupCentroids[gname];
    var cx = el.holder ? el.holder.clientWidth / 2 : 300;
    var cy = el.holder ? el.holder.clientHeight / 2 : 200;
    var angle = (index / Math.max(1, total)) * Math.PI * 2;
    var radius = 60 + (index % 7) * 14;
    if (seed) {
      return {
        x: seed.x + Math.cos(angle) * 34,
        y: seed.y + Math.sin(angle) * 34,
        vx: 0,
        vy: 0
      };
    }
    return {
      x: cx + Math.cos(angle) * radius,
      y: cy + Math.sin(angle) * radius,
      vx: 0,
      vy: 0
    };
  }

  // groupKeyOf is the bucket seeding uses: a claim's module, a group node's
  // own name, and a ghost's nothing.
  function groupKeyOf(node) {
    if (node.kind === 'group') {
      return str(node.group_name);
    }
    if (node.kind === 'ghost') {
      return '';
    }
    return str(node.module);
  }

  // ensurePositions gives every node in the scene a place, seeding the ones
  // that have none from the centroid of their already-placed group siblings.
  function ensurePositions(nodes) {
    var centroids = Object.create(null);
    var sums = Object.create(null);
    var i;
    for (i = 0; i < nodes.length; i++) {
      var known = positions[nodes[i].id];
      if (!known) {
        continue;
      }
      var key = groupKeyOf(nodes[i]);
      if (!sums[key]) {
        sums[key] = { x: 0, y: 0, n: 0 };
      }
      sums[key].x += known.x;
      sums[key].y += known.y;
      sums[key].n++;
    }
    for (var k in sums) {
      if (Object.prototype.hasOwnProperty.call(sums, k) && sums[k].n > 0) {
        centroids[k] = { x: sums[k].x / sums[k].n, y: sums[k].y / sums[k].n };
      }
    }
    var missing = 0;
    for (i = 0; i < nodes.length; i++) {
      if (!positions[nodes[i].id]) {
        positions[nodes[i].id] = seedPosition(nodes[i], missing, nodes.length, centroids);
        missing++;
      }
    }
  }

  // ------------------------------------------------------------------
  // recompute — the whole scene, from the payload and the current state
  // ------------------------------------------------------------------
  //
  // Every verdict below is computed by graph-core.js against the CURRENT
  // SCOPE. Nothing is precomputed in Go, because a gap list computed
  // project-wide would be confidently wrong the moment a reader narrows the
  // view to one module — a claim isolated inside module:viewer may be the
  // best-connected thing in the project.
  //
  // The one ordering that must not be got wrong: SCC runs at CLAIM level,
  // before the representative rule, never over the aggregated edges. When a
  // module collapses to one node every intra-module edge becomes a self-loop
  // on it, and an SCC pass that treated those as cycles would ring every
  // module with any internal edge red. Cycle membership is a property of
  // claims; a group node is ringed red because one of its MEMBERS is in a
  // component.

  function recompute() {
    var c = core();
    if (!c || !payload) {
      scene = null;
      renderNotices();
      renderGaps();
      renderDetail();
      return;
    }

    var i;
    var scoped = c.scopeFilter(payload.nodes, state.scope);
    var inScopeIds = Object.create(null);
    for (i = 0; i < scoped.length; i++) {
      inScopeIds[str(scoped[i].id)] = true;
    }

    var reps = c.representatives(scoped, state.granularity, state.expanded);
    var drawEdgesList = c.aggregateEdges(payload.edges, reps.repByClaim, state.types);

    // Ghost endpoints: an edge reaching a claim outside the current scope is
    // drawn to a hollow, unlabeled stub rather than dropped. Scoping must
    // never hide that a claim reaches outward.
    var nodes = reps.repNodes.slice();
    var byId = Object.create(null);
    for (i = 0; i < nodes.length; i++) {
      byId[str(nodes[i].id)] = nodes[i];
    }
    for (i = 0; i < drawEdgesList.length; i++) {
      nodes = addGhost(nodes, byId, drawEdgesList[i].from, c.GHOST_PREFIX);
      nodes = addGhost(nodes, byId, drawEdgesList[i].to, c.GHOST_PREFIX);
    }

    var nodeIds = [];
    for (i = 0; i < nodes.length; i++) {
      nodeIds.push(str(nodes[i].id));
    }
    var deg = c.degrees(nodeIds, drawEdgesList);

    // Claim-level edges with BOTH endpoints in scope. The structural rules
    // and the governance channels run over these, never over the aggregated
    // set.
    var inScopeEdges = [];
    for (i = 0; i < payload.edges.length; i++) {
      var e = payload.edges[i] || {};
      if (inScopeIds[str(e.from)] && inScopeIds[str(e.to)]) {
        inScopeEdges.push(e);
      }
    }

    var gaps = c.gapRules(scoped, payload.edges, {
      enabledTypes: state.types,
      groupBy: state.granularity === 'facet' ? 'facet' : 'module'
    });

    // Component membership, then the claim-level edges INSIDE a component,
    // mapped through their representatives so the red edges land on whatever
    // the reader is currently looking at.
    var componentOf = Object.create(null);
    var cycleIds = Object.create(null);
    var comps = 0;
    for (i = 0; i < gaps.facts.length; i++) {
      var f = gaps.facts[i];
      if (f.rule !== 'cycle' || f.node_ids.length === 0) {
        continue;
      }
      for (var m = 0; m < f.node_ids.length; m++) {
        componentOf[f.node_ids[m]] = comps;
        cycleIds[f.node_ids[m]] = true;
      }
      comps++;
    }
    var redEdgeKeys = Object.create(null);
    for (i = 0; i < inScopeEdges.length; i++) {
      var ce = inScopeEdges[i];
      var a = componentOf[str(ce.from)];
      var b = componentOf[str(ce.to)];
      if (a === undefined || a !== b) {
        continue;
      }
      redEdgeKeys[
        str(reps.repByClaim[str(ce.from)]) + '|' + str(ce.type) + '|' + str(reps.repByClaim[str(ce.to)])
      ] = true;
    }

    var selfIds = Object.create(null);
    for (i = 0; i < gaps.facts.length; i++) {
      if (gaps.facts[i].rule === 'self_edge') {
        for (var s = 0; s < gaps.facts[i].node_ids.length; s++) {
          selfIds[gaps.facts[i].node_ids[s]] = true;
        }
      }
    }

    // Governance: the wedge set and the overlay's scope, both scope-relative
    // and both mapped through the representatives.
    var governorIds = Object.create(null);
    var govList = c.governors(inScopeEdges);
    for (i = 0; i < govList.length; i++) {
      governorIds[govList[i]] = true;
    }
    var govScope = c.governanceScope(inScopeEdges);
    var govNodeIds = Object.create(null);
    for (i = 0; i < govScope.nodeIds.length; i++) {
      govNodeIds[str(reps.repByClaim[govScope.nodeIds[i]])] = true;
    }
    var govEdgeKeys = Object.create(null);
    for (i = 0; i < inScopeEdges.length; i++) {
      if (str(inScopeEdges[i].type) !== 'governed_by') {
        continue;
      }
      govEdgeKeys[
        str(reps.repByClaim[str(inScopeEdges[i].from)]) +
          '|governed_by|' +
          str(reps.repByClaim[str(inScopeEdges[i].to)])
      ] = true;
    }

    // Which facets each drawn node stands for. A claim stands for its own; a
    // collapsed group stands for every facet among its members. This is what
    // lets a legend hover dim non-members even at module granularity.
    var facetsOfNode = Object.create(null);
    for (i = 0; i < scoped.length; i++) {
      var rep = str(reps.repByClaim[str(scoped[i].id)]);
      if (!facetsOfNode[rep]) {
        facetsOfNode[rep] = Object.create(null);
      }
      facetsOfNode[rep][str(scoped[i].facet)] = true;
    }

    scene = {
      scoped: scoped,
      nodes: nodes,
      byId: byId,
      edges: drawEdgesList,
      deg: deg,
      gaps: gaps,
      cycleIds: cycleIds,
      selfIds: selfIds,
      redEdgeKeys: redEdgeKeys,
      governorIds: governorIds,
      govNodeIds: govNodeIds,
      govEdgeKeys: govEdgeKeys,
      facetsOfNode: facetsOfNode,
      repByClaim: reps.repByClaim
    };

    ensurePositions(nodes);
    renderNotices();
    renderGaps();
    renderDetail();
  }

  function addGhost(nodes, byId, id, prefix) {
    if (id.indexOf(prefix) !== 0 || byId[id]) {
      return nodes;
    }
    var ghost = { id: id, title: '', module: '', facet: '', kind: 'ghost', status: '' };
    byId[id] = ghost;
    nodes.push(ghost);
    return nodes;
  }

  // ------------------------------------------------------------------
  // The palette, read from the stylesheet at draw time
  // ------------------------------------------------------------------
  //
  // Never duplicated in JavaScript. Reading it off the document element on
  // every draw is what makes an OS light/dark switch repaint correctly with
  // no second copy of the ramp to keep in sync — the browser has already
  // re-resolved the custom properties by the time the next frame runs.

  function cssVar(cs, name) {
    var v = cs.getPropertyValue(name);
    v = v ? v.trim() : '';
    return v === '' ? FALLBACK_COLOR : v;
  }

  function readPalette() {
    var cs = getComputedStyle(document.documentElement);
    var facets = [];
    for (var i = 1; i <= 20; i++) {
      facets.push(cssVar(cs, '--dxg-facet-' + i));
    }
    return {
      facets: facets,
      other: cssVar(cs, '--dxg-facet-other'),
      cycle: cssVar(cs, '--dxg-cycle'),
      halo: cssVar(cs, '--dxg-halo'),
      governed: cssVar(cs, '--dxg-governed'),
      ink: cssVar(cs, '--ink'),
      muted: cssVar(cs, '--muted'),
      faint: cssVar(cs, '--faint'),
      paper: cssVar(cs, '--paper'),
      accent: cssVar(cs, '--accent'),
      link: cssVar(cs, '--link'),
      warn: cssVar(cs, '--warn')
    };
  }

  // ------------------------------------------------------------------
  // Node encoding (design section 4)
  // ------------------------------------------------------------------
  //
  //   fill    facet, by slot — swapped WHOLESALE while an overlay is active
  //   radius  total edge degree WITHIN THE CURRENT SCOPE, which is what makes
  //           an isolated claim pre-attentive: literally the smallest thing
  //           on the canvas
  //   ring    locked solid / draft dashed — independent of fill and size, so
  //           it survives every overlay
  //   halo    review_pending or open comments; never more than one at a time,
  //           because a halo is reserved for engine-managed states that
  //           demand a human and two of them stacked says nothing extra
  //   wedge   this node governs at least one other claim
  //   ghost   an endpoint outside the current scope: hollow and unlabeled

  function facetSlotOf(node) {
    var c = core();
    if (!c || !payload) {
      return -1;
    }
    if (node.kind === 'ghost') {
      return -1;
    }
    if (node.kind === 'group' && node.group_type !== 'facet') {
      // A module has no single facet. Borrowing one of its members' colours
      // would assert something false, so it wears the catch-all slot.
      return -1;
    }
    return c.facetSlot(payload.groups.facets, str(node.facet || node.group_name));
  }

  function radiusOf(node) {
    if (node.kind === 'ghost') {
      return 3.5;
    }
    var d = scene.deg[node.id] ? scene.deg[node.id].total : 0;
    var r = 4.5 + 2.1 * Math.sqrt(d);
    if (node.kind === 'group') {
      r += Math.sqrt(num(node.size)) * 0.9;
    }
    return Math.max(4, Math.min(26, r));
  }

  function isFacetMember(node) {
    if (hoverFacet === null) {
      return true;
    }
    var set = scene.facetsOfNode[node.id];
    return set ? set[hoverFacet] === true : false;
  }

  function baseFill(node, pal) {
    var slot = facetSlotOf(node);
    return slot < 0 ? pal.other : pal.facets[slot];
  }

  // nodeAlpha folds the two dimming channels — an active overlay and a legend
  // hover — into one number. Dimming rather than hiding is deliberate: a
  // reader must still be able to see the shape they are filtering against.
  function nodeAlpha(node) {
    var a = 1;
    if (state.overlay !== 'none' && !overlayMatches(node)) {
      a = 0.14;
    }
    if (!isFacetMember(node)) {
      a = Math.min(a, 0.14);
    }
    return a;
  }

  function drawNodes(ctx, pal) {
    var i;
    for (i = 0; i < scene.nodes.length; i++) {
      var node = scene.nodes[i];
      var pos = positions[node.id];
      if (!pos) {
        continue;
      }
      var r = radiusOf(node);
      ctx.globalAlpha = nodeAlpha(node);

      // fill (or hollow, for a ghost)
      ctx.beginPath();
      ctx.arc(pos.x, pos.y, r, 0, Math.PI * 2);
      if (node.kind === 'ghost') {
        ctx.fillStyle = pal.paper;
        ctx.fill();
      } else {
        ctx.fillStyle = state.overlay === 'none' ? baseFill(node, pal) : overlayFill(node, pal);
        ctx.fill();
      }

      // ring: status, or red for a claim inside a cycle
      ctx.beginPath();
      ctx.arc(pos.x, pos.y, r, 0, Math.PI * 2);
      var inCycle = coversAny(node, scene.cycleIds) || coversAny(node, scene.selfIds);
      if (inCycle) {
        ctx.strokeStyle = pal.cycle;
        ctx.lineWidth = 2.4;
        ctx.setLineDash([]);
      } else if (node.kind === 'ghost') {
        ctx.strokeStyle = pal.faint;
        ctx.lineWidth = 1;
        ctx.setLineDash([2, 2]);
      } else if (str(node.status) === 'locked') {
        ctx.strokeStyle = pal.ink;
        ctx.lineWidth = 1.4;
        ctx.setLineDash([]);
      } else if (node.kind === 'group') {
        ctx.strokeStyle = pal.muted;
        ctx.lineWidth = 1.4;
        ctx.setLineDash([]);
      } else {
        ctx.strokeStyle = pal.muted;
        ctx.lineWidth = 1.2;
        ctx.setLineDash([3, 2]);
      }
      ctx.stroke();
      ctx.setLineDash([]);

      // halo: at most one, review_pending winning over open threads
      var halo = haloKind(node);
      if (halo !== '') {
        ctx.beginPath();
        ctx.arc(pos.x, pos.y, r + 4, 0, Math.PI * 2);
        ctx.strokeStyle = pal.halo;
        ctx.lineWidth = halo === 'review' ? 2 : 1.2;
        if (halo === 'threads') {
          ctx.setLineDash([2, 3]);
        }
        ctx.stroke();
        ctx.setLineDash([]);
      }

      // selection
      if (state.selected !== '' && node.id === selectedRepId()) {
        ctx.beginPath();
        ctx.arc(pos.x, pos.y, r + 7, 0, Math.PI * 2);
        ctx.strokeStyle = pal.accent;
        ctx.lineWidth = 1.6;
        ctx.stroke();
      }

      drawWedge(ctx, pal, node, pos, r);
    }
    ctx.globalAlpha = 1;
  }

  function haloKind(node) {
    if (node.kind !== 'group' && node.review_pending === true) {
      return 'review';
    }
    if (node.kind !== 'group' && num(node.open_comments) > 0) {
      return 'threads';
    }
    if (node.kind === 'group' && groupHas(node, 'review_pending')) {
      return 'review';
    }
    if (node.kind === 'group' && groupHas(node, 'open_comments')) {
      return 'threads';
    }
    return '';
  }

  // groupHas answers an engine-managed-state question about a COLLAPSED
  // group by asking its members, so collapsing a module never hides that
  // something inside it is waiting on a human.
  function groupHas(node, field) {
    if (!node.members) {
      return false;
    }
    for (var i = 0; i < node.members.length; i++) {
      var member = claimById(node.members[i]);
      if (!member) {
        continue;
      }
      if (field === 'review_pending' && member.review_pending === true) {
        return true;
      }
      if (field === 'open_comments' && num(member.open_comments) > 0) {
        return true;
      }
    }
    return false;
  }

  function claimById(id) {
    for (var i = 0; i < payload.nodes.length; i++) {
      if (str(payload.nodes[i].id) === id) {
        return payload.nodes[i];
      }
    }
    return null;
  }

  // coversAny asks "does this drawn node stand for any of these claims?" —
  // itself, for a claim node; any member, for a collapsed group.
  function coversAny(node, idSet) {
    if (node.members) {
      for (var i = 0; i < node.members.length; i++) {
        if (idSet[node.members[i]]) {
          return true;
        }
      }
      return false;
    }
    return idSet[node.id] === true;
  }

  function selectedRepId() {
    if (!scene || state.selected === '') {
      return '';
    }
    if (scene.byId[state.selected]) {
      return state.selected;
    }
    return str(scene.repByClaim[state.selected]);
  }

  // ------------------------------------------------------------------
  // Labels
  // ------------------------------------------------------------------

  function labelsVisible() {
    if (!state.labels) {
      return false;
    }
    // Past the ceiling the labels overlap into a smear that hides the shape
    // the pane exists to show — unless the reader has zoomed in, at which
    // point they are asking to read a neighbourhood rather than see a shape.
    return scene.nodes.length <= LABEL_NODE_CEILING || camera.zoom > 1.6;
  }

  function labelOf(node) {
    if (node.kind === 'ghost') {
      return '';
    }
    if (node.kind === 'group') {
      return (str(node.group_name) || 'no ' + str(node.group_type)) + ' (' + num(node.size) + ')';
    }
    return str(node.title) || str(node.id);
  }

  function drawLabels(ctx, pal) {
    ctx.font = '11px ' + (getComputedStyle(document.body).fontFamily || 'sans-serif');
    ctx.textAlign = 'center';
    ctx.textBaseline = 'top';
    for (var i = 0; i < scene.nodes.length; i++) {
      var node = scene.nodes[i];
      var pos = positions[node.id];
      var text = labelOf(node);
      if (!pos || text === '') {
        continue;
      }
      ctx.globalAlpha = Math.min(nodeAlpha(node), 0.92);
      ctx.fillStyle = pal.muted;
      ctx.fillText(text, pos.x, pos.y + radiusOf(node) + 3);
    }
    ctx.globalAlpha = 1;
  }

  // ------------------------------------------------------------------
  // The canvas itself
  // ------------------------------------------------------------------

  function resizeCanvas() {
    if (!el.canvas || !el.holder) {
      return;
    }
    var dpr = root.devicePixelRatio || 1;
    var width = el.holder.clientWidth || 1;
    var height = el.holder.clientHeight || 1;
    el.canvas.width = Math.max(1, Math.round(width * dpr));
    el.canvas.height = Math.max(1, Math.round(height * dpr));
    el.dpr = dpr;
    el.width = width;
    el.height = height;
  }

  function draw() {
    if (!el.ctx) {
      return;
    }
    var ctx = el.ctx;
    ctx.setTransform(el.dpr || 1, 0, 0, el.dpr || 1, 0, 0);
    ctx.clearRect(0, 0, el.width || 0, el.height || 0);
    if (!scene) {
      return;
    }
    var pal = readPalette();
    ctx.save();
    ctx.translate(camera.x, camera.y);
    ctx.scale(camera.zoom, camera.zoom);
    drawEdges(ctx, pal);
    drawNodes(ctx, pal);
    if (labelsVisible()) {
      drawLabels(ctx, pal);
    }
    ctx.restore();
  }

  // ------------------------------------------------------------------
  // Edges, and governed_by's FOUR channels (design section 4.3)
  // ------------------------------------------------------------------
  //
  // governed_by is the relation a reader most often needs to isolate, and the
  // one an edge style alone cannot make findable. Telling a dashed line from
  // a dotted line at a glance across a 400-node canvas means tracing lines,
  // which is precisely the work this pane exists to remove. So it gets four
  // INDEPENDENT channels, any one of which answers the question on its own:
  //
  //   1. STROKE   --dxg-governed, a hue reserved outside the 20-slot facet
  //               ramp, so a governance edge can never be mistaken for a
  //               facet's colour at any facet count in either theme
  //   2. ROUTING  a quadratic curve, where rests_on and mirrors are straight
  //   3. HEAD     a double chevron, where rests_on has one and mirrors none
  //   4. MARKER   a wedge on the node that GOVERNS — the target of the edge —
  //               so a doctrine claim is findable without following any line
  //
  // Plus the governance overlay (below), because four channels still leave a
  // reader answering "what does this doctrine actually reach?" by tracing.

  var CURVE_BOW = 0.16; // how far a governance curve bows off the chord

  function edgeStroke(edge, pal) {
    if (scene.redEdgeKeys[edgeKeyOf(edge)]) {
      return pal.cycle;
    }
    if (edge.type === 'governed_by') {
      return pal.governed;
    }
    return pal.muted;
  }

  function edgeKeyOf(edge) {
    return edge.from + '|' + edge.type + '|' + edge.to;
  }

  function edgeAlpha(edge) {
    var a = 0.55;
    if (scene.redEdgeKeys[edgeKeyOf(edge)]) {
      a = 0.95;
    }
    if (state.overlay !== 'none' && !overlayMatchesEdge(edge)) {
      a = 0.07;
    }
    return a;
  }

  function drawEdges(ctx, pal) {
    for (var i = 0; i < scene.edges.length; i++) {
      var edge = scene.edges[i];
      var a = positions[edge.from];
      var b = positions[edge.to];
      if (!a || !b) {
        continue;
      }
      var curved = edge.type === 'governed_by';
      ctx.globalAlpha = edgeAlpha(edge);
      ctx.strokeStyle = edgeStroke(edge, pal);
      ctx.lineWidth = Math.min(3, 0.9 + Math.log(1 + num(edge.weight)) * 0.5);

      var target = scene.byId[edge.to];
      var stop = target ? radiusOf(target) + 2 : 4;
      var ctrl = controlPoint(a, b, curved);
      var end = shorten(ctrl, b, stop);

      ctx.beginPath();
      ctx.moveTo(a.x, a.y);
      if (curved) {
        ctx.quadraticCurveTo(ctrl.x, ctrl.y, end.x, end.y);
      } else {
        ctx.lineTo(end.x, end.y);
      }
      ctx.stroke();

      // Arrowheads. mirrors gets none: it is reciprocal by design, and a head
      // on both ends of a symmetric relation is noise.
      var angle = Math.atan2(end.y - ctrl.y, end.x - ctrl.x);
      if (edge.type === 'governed_by') {
        chevron(ctx, end.x, end.y, angle, 6.5);
        chevron(
          ctx,
          end.x - Math.cos(angle) * 4.5,
          end.y - Math.sin(angle) * 4.5,
          angle,
          6.5
        );
      } else if (edge.type === 'rests_on') {
        chevron(ctx, end.x, end.y, angle, 6);
      }
    }
    ctx.globalAlpha = 1;
  }

  // controlPoint is the midpoint for a straight edge and a point bowed off
  // the chord's perpendicular for a governance edge. Bowing by a FRACTION of
  // the chord keeps short and long governance edges equally recognisable.
  function controlPoint(a, b, curved) {
    var mx = (a.x + b.x) / 2;
    var my = (a.y + b.y) / 2;
    if (!curved) {
      return { x: mx, y: my };
    }
    var dx = b.x - a.x;
    var dy = b.y - a.y;
    return { x: mx - dy * CURVE_BOW, y: my + dx * CURVE_BOW };
  }

  // shorten pulls the endpoint back off the target node's rim so an
  // arrowhead lands on the circle rather than inside it.
  function shorten(from, to, by) {
    var dx = to.x - from.x;
    var dy = to.y - from.y;
    var len = Math.sqrt(dx * dx + dy * dy);
    if (len <= by || len === 0) {
      return { x: to.x, y: to.y };
    }
    return { x: to.x - (dx / len) * by, y: to.y - (dy / len) * by };
  }

  function chevron(ctx, x, y, angle, size) {
    var spread = 0.42;
    ctx.beginPath();
    ctx.moveTo(x - Math.cos(angle - spread) * size, y - Math.sin(angle - spread) * size);
    ctx.lineTo(x, y);
    ctx.lineTo(x - Math.cos(angle + spread) * size, y - Math.sin(angle + spread) * size);
    ctx.stroke();
  }

  // drawWedge is channel four: a filled wedge on every node that is the
  // TARGET of at least one governed_by edge in scope. Direction is easy to
  // get backwards — a claim declares `governed_by: {type: X}`, so the edge
  // runs claim -> governor and the GOVERNOR is the target. graph-core.js's
  // governors() is the single place that direction is decided.
  function drawWedge(ctx, pal, node, pos, r) {
    if (!coversAny(node, scene.governorIds)) {
      return;
    }
    var tip = r + 6.5;
    var base = r + 1;
    var angle = -Math.PI / 4; // up and to the right, clear of the label below
    var spread = 0.34;
    ctx.beginPath();
    ctx.moveTo(pos.x + Math.cos(angle) * tip, pos.y + Math.sin(angle) * tip);
    ctx.lineTo(
      pos.x + Math.cos(angle - spread) * base,
      pos.y + Math.sin(angle - spread) * base
    );
    ctx.lineTo(
      pos.x + Math.cos(angle + spread) * base,
      pos.y + Math.sin(angle + spread) * base
    );
    ctx.closePath();
    ctx.fillStyle = pal.governed;
    ctx.fill();
  }

  // ------------------------------------------------------------------
  // Overlays — the node fill is swapped WHOLESALE, never overloaded
  // ------------------------------------------------------------------
  //
  // Six overlays plus "none", and while one is active every non-matching node
  // and edge is dimmed rather than recoloured with a second signal. Cramming
  // four meanings into one dot is what makes a graph unreadable; asking one
  // question at a time is what makes it answerable.
  //
  // THE GOVERNANCE OVERLAY is the one that could not be replaced by an edge
  // style. It dims everything except the governors, the claims they govern,
  // and the governance edges between them — so "what does this doctrine
  // actually reach?" is answered in one click instead of one trace. Note what
  // it deliberately does NOT light: a rests_on edge that happens to join two
  // governance participants stays dimmed, because reach is carried by the
  // governance edges alone and lighting an unrelated dependency would answer
  // a different question badly. That decision lives in graph-core.js's
  // governanceScope(), not here.

  function overlayMatches(node) {
    switch (state.overlay) {
      case 'isolated':
        return coversAnyRule(node, 'isolated') || coversAnyRule(node, 'weakly_linked');
      case 'cycles':
        return coversAny(node, scene.cycleIds) || coversAny(node, scene.selfIds);
      case 'governance':
        return scene.govNodeIds[node.id] === true;
      case 'review':
        return haloKind(node) === 'review';
      case 'comments':
        return haloKind(node) === 'threads';
      case 'status':
        // Both halves match: this overlay is a two-colour split, not a
        // filter. Nothing is dimmed and the fill answers the question.
        return node.kind !== 'ghost';
      default:
        return true;
    }
  }

  function overlayMatchesEdge(edge) {
    if (state.overlay === 'governance') {
      return scene.govEdgeKeys[edgeKeyOf(edge)] === true;
    }
    if (state.overlay === 'cycles') {
      return scene.redEdgeKeys[edgeKeyOf(edge)] === true;
    }
    // Every other overlay is about nodes. An edge stays lit when both of its
    // endpoints are lit, which keeps the neighbourhood readable rather than
    // leaving matched nodes floating unconnected.
    var a = scene.byId[edge.from];
    var b = scene.byId[edge.to];
    return !!a && !!b && overlayMatches(a) && overlayMatches(b);
  }

  // overlayFill is the semantic colour a MATCHING node takes. Non-matching
  // nodes keep a neutral fill and are dimmed by nodeAlpha, so the two states
  // differ in both hue and contrast rather than hue alone.
  function overlayFill(node, pal) {
    if (!overlayMatches(node)) {
      return pal.faint;
    }
    switch (state.overlay) {
      case 'isolated':
        return pal.warn;
      case 'cycles':
        return pal.cycle;
      case 'governance':
        return pal.governed;
      case 'review':
        return pal.halo;
      case 'comments':
        return pal.link;
      case 'status':
        return statusOf(node) === 'locked' ? pal.accent : pal.halo;
      default:
        return baseFill(node, pal);
    }
  }

  // statusOf answers for a collapsed group too: a group reads as locked only
  // when every member is, because "this module is locked" is a claim about
  // all of it.
  function statusOf(node) {
    if (node.kind !== 'group') {
      return str(node.status);
    }
    if (!node.members || node.members.length === 0) {
      return '';
    }
    for (var i = 0; i < node.members.length; i++) {
      var member = claimById(node.members[i]);
      if (!member || str(member.status) !== 'locked') {
        return 'draft';
      }
    }
    return 'locked';
  }

  // coversAnyRule asks whether a drawn node stands for any claim named by a
  // gap rule. Keyed off the STABLE rule id from graph-core.js, never off the
  // rail's display text.
  function coversAnyRule(node, ruleId) {
    if (!scene) {
      return false;
    }
    for (var i = 0; i < scene.gaps.facts.length; i++) {
      var f = scene.gaps.facts[i];
      if (f.rule !== ruleId) {
        continue;
      }
      var set = Object.create(null);
      for (var j = 0; j < f.node_ids.length; j++) {
        set[f.node_ids[j]] = true;
      }
      if (coversAny(node, set) || set[node.id] === true) {
        return true;
      }
    }
    return false;
  }

  // ------------------------------------------------------------------
  // Force layout
  // ------------------------------------------------------------------
  //
  // A small, explicit, cooling simulation rather than a library: three forces
  // and a temperature. It has no stable pixels and therefore no test asserts
  // on its output — which is exactly why every VERDICT the pane draws is
  // computed before this runs, in a file that has no canvas at all.
  //
  //   repulsion   every pair pushes apart, so unrelated claims separate
  //   springs     every drawn edge pulls its endpoints together
  //   gravity     a weak pull to the centre, so a disconnected component
  //               drifts to the edge instead of off the canvas entirely
  //
  // The pairwise pass is O(n^2) and is skipped above PAIRWISE_CEILING drawn
  // nodes. Auto-collapse normally keeps the count far below that; this is the
  // floor under a reader who overrode it.

  var REPULSION = 2600;
  var SPRING = 0.012;
  var SPRING_LENGTH = 62;
  var GRAVITY = 0.012;
  var DAMPING = 0.82;
  var COOLING = 0.975;

  function startLayout(reheat) {
    alpha = reheat ? 1 : Math.max(alpha, 0.45);
    if (!frame && isOpen) {
      frame = root.requestAnimationFrame(tickLayout);
    }
  }

  function stopLayout() {
    if (frame) {
      root.cancelAnimationFrame(frame);
      frame = 0;
    }
  }

  function tickLayout() {
    frame = 0;
    if (!isOpen || !scene) {
      return;
    }
    stepLayout();
    draw();
    if (alpha > 0.02 || dragging) {
      frame = root.requestAnimationFrame(tickLayout);
    }
  }

  function stepLayout() {
    alpha *= COOLING;
    var nodes = scene.nodes;
    var n = nodes.length;
    var i;
    var centre = screenToWorld((el.width || 600) / 2, (el.height || 400) / 2);

    if (n <= PAIRWISE_CEILING) {
      for (i = 0; i < n; i++) {
        var pa = positions[nodes[i].id];
        for (var j = i + 1; j < n; j++) {
          var pb = positions[nodes[j].id];
          var dx = pb.x - pa.x;
          var dy = pb.y - pa.y;
          var d2 = dx * dx + dy * dy;
          if (d2 < 0.01) {
            // Two nodes exactly on top of each other have no direction to
            // separate along. Nudge deterministically by index rather than
            // randomly, so a redraw does not jitter.
            dx = (j - i) * 0.1;
            dy = 0.1;
            d2 = dx * dx + dy * dy;
          }
          if (d2 > 90000) {
            continue; // far enough that the force rounds to nothing
          }
          var force = (REPULSION * alpha) / d2;
          var dist = Math.sqrt(d2);
          var fx = (dx / dist) * force;
          var fy = (dy / dist) * force;
          pa.vx -= fx;
          pa.vy -= fy;
          pb.vx += fx;
          pb.vy += fy;
        }
      }
    }

    for (i = 0; i < scene.edges.length; i++) {
      var from = positions[scene.edges[i].from];
      var to = positions[scene.edges[i].to];
      if (!from || !to) {
        continue;
      }
      var ex = to.x - from.x;
      var ey = to.y - from.y;
      var elen = Math.sqrt(ex * ex + ey * ey) || 1;
      var pull = (elen - SPRING_LENGTH) * SPRING * alpha;
      from.vx += (ex / elen) * pull;
      from.vy += (ey / elen) * pull;
      to.vx -= (ex / elen) * pull;
      to.vy -= (ey / elen) * pull;
    }

    for (i = 0; i < n; i++) {
      var p = positions[nodes[i].id];
      if (dragging && dragging.id === nodes[i].id) {
        p.vx = 0;
        p.vy = 0;
        continue;
      }
      p.vx += (centre.x - p.x) * GRAVITY * alpha;
      p.vy += (centre.y - p.y) * GRAVITY * alpha;
      p.vx *= DAMPING;
      p.vy *= DAMPING;
      p.x += p.vx;
      p.y += p.vy;
    }
  }

  // ------------------------------------------------------------------
  // Interaction: drag a node, drag the background, scroll to zoom,
  // double-click a group to expand it
  // ------------------------------------------------------------------
  //
  // Pointer events throughout, so a finger and a mouse take the same path and
  // the canvas needs no separate touch handling. touch-action: none in
  // graph.css is what stops a drag scrolling the page out from under the
  // reader.

  function screenToWorld(sx, sy) {
    return { x: (sx - camera.x) / camera.zoom, y: (sy - camera.y) / camera.zoom };
  }

  function eventPoint(e) {
    var rect = el.canvas.getBoundingClientRect();
    return screenToWorld(e.clientX - rect.left, e.clientY - rect.top);
  }

  // hitTest walks backwards so the node drawn last — the one visually on top
  // — is the one a click selects.
  function hitTest(world) {
    for (var i = scene.nodes.length - 1; i >= 0; i--) {
      var node = scene.nodes[i];
      var p = positions[node.id];
      if (!p) {
        continue;
      }
      var dx = world.x - p.x;
      var dy = world.y - p.y;
      var r = radiusOf(node) + 3;
      if (dx * dx + dy * dy <= r * r) {
        return node;
      }
    }
    return null;
  }

  function bindCanvas() {
    el.canvas.addEventListener('pointerdown', function (e) {
      if (!scene) {
        return;
      }
      movedWhileDown = false;
      var world = eventPoint(e);
      var node = hitTest(world);
      if (node) {
        var p = positions[node.id];
        dragging = { id: node.id, dx: world.x - p.x, dy: world.y - p.y };
      } else {
        panning = { x: e.clientX, y: e.clientY };
        el.canvas.classList.add('dxg-canvas--drag');
      }
      if (el.canvas.setPointerCapture) {
        el.canvas.setPointerCapture(e.pointerId);
      }
    });

    el.canvas.addEventListener('pointermove', function (e) {
      if (!scene) {
        return;
      }
      if (dragging) {
        movedWhileDown = true;
        var world = eventPoint(e);
        var p = positions[dragging.id];
        p.x = world.x - dragging.dx;
        p.y = world.y - dragging.dy;
        p.vx = 0;
        p.vy = 0;
        startLayout(false);
        return;
      }
      if (panning) {
        movedWhileDown = true;
        camera.x += e.clientX - panning.x;
        camera.y += e.clientY - panning.y;
        panning.x = e.clientX;
        panning.y = e.clientY;
        draw();
      }
    });

    el.canvas.addEventListener('pointerup', function (e) {
      var wasDragging = dragging;
      dragging = null;
      panning = null;
      el.canvas.classList.remove('dxg-canvas--drag');
      if (el.canvas.releasePointerCapture) {
        el.canvas.releasePointerCapture(e.pointerId);
      }
      if (movedWhileDown || !scene) {
        return;
      }
      // A press that moved nothing is a click: select, or clear.
      var node = wasDragging ? scene.byId[wasDragging.id] : hitTest(eventPoint(e));
      select(node ? node.id : '');
    });

    el.canvas.addEventListener('pointercancel', function () {
      dragging = null;
      panning = null;
      el.canvas.classList.remove('dxg-canvas--drag');
    });

    el.canvas.addEventListener(
      'wheel',
      function (e) {
        if (!scene) {
          return;
        }
        e.preventDefault();
        var rect = el.canvas.getBoundingClientRect();
        var sx = e.clientX - rect.left;
        var sy = e.clientY - rect.top;
        var factor = Math.exp(-e.deltaY * 0.0015);
        var next = Math.max(0.15, Math.min(6, camera.zoom * factor));
        // Keep the world point under the cursor exactly where it is, so the
        // reader zooms into what they are pointing at rather than the middle.
        camera.x = sx - ((sx - camera.x) * next) / camera.zoom;
        camera.y = sy - ((sy - camera.y) * next) / camera.zoom;
        camera.zoom = next;
        draw();
      },
      { passive: false }
    );

    el.canvas.addEventListener('dblclick', function (e) {
      if (!scene) {
        return;
      }
      var node = hitTest(eventPoint(e));
      if (!node) {
        return;
      }
      if (node.kind === 'group') {
        expandGroup(node.id);
      } else if (node.kind !== 'ghost' && state.granularity !== 'claims') {
        // Double-clicking a claim inside an expanded group collapses it
        // again — the same gesture, reversed, on the same target.
        collapseGroup(node);
      }
    });
  }

  // expandGroup and collapseGroup are the per-group override the granularity
  // select sets a default for. The set is stored as full group ids
  // ("module:engine"), which is the same vocabulary the scope control and the
  // rail use, so a reader never meets two names for one thing.
  function expandGroup(groupId) {
    if (state.expanded.indexOf(groupId) < 0) {
      state.expanded = state.expanded.concat([groupId]);
    }
    onControlChange(true);
  }

  function collapseGroup(node) {
    var c = core();
    var gid = c.groupId(state.granularity, str(state.granularity === 'facet' ? node.facet : node.module));
    var next = [];
    for (var i = 0; i < state.expanded.length; i++) {
      if (state.expanded[i] !== gid) {
        next.push(state.expanded[i]);
      }
    }
    state.expanded = next;
    onControlChange(true);
  }

  function select(id) {
    state.selected = id;
    renderDetail();
    writeHash();
    draw();
  }

  function bindWindow() {
    root.addEventListener('resize', function () {
      if (!isOpen) {
        return;
      }
      resizeCanvas();
      draw();
    });

    document.addEventListener('keydown', function (e) {
      if (!isOpen) {
        return;
      }
      if (e.key === 'Escape') {
        closePane();
      }
    });

    bindHashListener();
  }

  // ------------------------------------------------------------------
  // The gaps rail — facts above heuristics, and the two never mix
  // ------------------------------------------------------------------
  //
  // graph-core.js returns {facts, hints} as two separate arrays, and this
  // renders them into two visually separate blocks with a caption over the
  // second. That separation is load-bearing, not decorative: false positives
  // among the hints are guaranteed rather than merely possible — a module
  // that legitimately has no verification phase is listed every single time.
  // A shared block would eventually lose the wording and the border that keep
  // a guess honest instead of annoying.
  //
  // Every block keys off the STABLE rule id, never off display text. The rail
  // shows the id as well as the phrase, so a reader can match what they are
  // looking at to the design's rule table without guessing.
  //
  // A rule that found nothing still renders, dimmed and with a zero count.
  // An empty cycle block is the correct and honest reading for a corpus that
  // passes check — hiding it would leave a reader unable to tell "no cycles"
  // from "not computed".

  var MAX_LISTED_IDS = 40;

  function renderGaps() {
    if (!el.gaps) {
      return;
    }
    clear(el.gaps);
    if (!scene) {
      el.gaps.appendChild(h('p', 'dxg-detail-empty', 'nothing to analyse'));
      return;
    }

    el.gaps.appendChild(h('h3', 'dxg-rail-title', 'Gaps in this view'));
    var i;
    for (i = 0; i < scene.gaps.facts.length; i++) {
      el.gaps.appendChild(ruleBlock(scene.gaps.facts[i], false));
    }

    var hints = h('div', 'dxg-hints');
    hints.setAttribute('data-dxg-hints', '');
    hints.appendChild(
      h(
        'p',
        'dxg-hints-caption',
        'Heuristics — guesses about this project, not findings. They block nothing, and some of them are wrong by construction.'
      )
    );
    for (i = 0; i < scene.gaps.hints.length; i++) {
      hints.appendChild(ruleBlock(scene.gaps.hints[i], true));
    }
    el.gaps.appendChild(hints);

    el.gaps.appendChild(
      h(
        'p',
        'dxg-scope-note',
        'Every verdict here is computed for the current scope. For the project-wide verdict, run dossierx check.'
      )
    );
  }

  function ruleBlock(finding, isHint) {
    var ids = Array.isArray(finding.node_ids) ? finding.node_ids : [];
    var block = h('div', 'dxg-rule' + (ids.length === 0 ? ' dxg-rule--empty' : ''));
    block.setAttribute('data-dxg-rule', finding.rule);
    block.setAttribute('data-dxg-kind', finding.kind);

    var head = h('div', 'dxg-rule-head');
    head.appendChild(h('span', 'dxg-rule-name', finding.rule));
    head.appendChild(h('span', 'dxg-rule-phrase', RULE_LABELS[finding.rule] || ''));
    if (isHint) {
      head.appendChild(h('span', 'dxg-hint-label', 'guess'));
    }
    head.appendChild(h('span', 'dxg-rule-count', ids.length));
    block.appendChild(head);

    if (ids.length > 0) {
      var list = h('ul', 'dxg-rule-ids');
      var shown = Math.min(ids.length, MAX_LISTED_IDS);
      for (var i = 0; i < shown; i++) {
        list.appendChild(jumpItem(ids[i]));
      }
      if (ids.length > shown) {
        var rest = h('li', '', '+' + (ids.length - shown) + ' more');
        rest.className = 'dxg-rule-more';
        list.appendChild(rest);
      }
      block.appendChild(list);
    }
    return block;
  }

  function jumpItem(id) {
    var item = h('li');
    var btn = h('button', 'dxg-jump', id);
    btn.type = 'button';
    btn.title = id;
    btn.setAttribute('data-dxg-jump', id);
    btn.addEventListener('click', function () {
      jumpTo(id);
    });
    item.appendChild(btn);
    return item;
  }

  // jumpTo selects the claim (or the group standing for it) and brings it to
  // the centre of the canvas. A rail entry naming a node a reader cannot find
  // is only half an answer.
  function jumpTo(id) {
    var repId = scene && scene.byId[id] ? id : str(scene ? scene.repByClaim[id] : '');
    if (repId === '' || !positions[repId]) {
      // The id names a group the current granularity does not draw — select
      // it anyway so the detail panel can say what it is.
      select(id);
      return;
    }
    var p = positions[repId];
    camera.x = (el.width || 0) / 2 - p.x * camera.zoom;
    camera.y = (el.height || 0) / 2 - p.y * camera.zoom;
    select(id);
  }

  // ------------------------------------------------------------------
  // The detail panel — facet identity's THIRD channel
  // ------------------------------------------------------------------
  //
  // Twenty categorical colours are not twenty distinguishable colours. This
  // panel is why that is acceptable: it names the selected node's facet IN
  // TEXT, so a reader never has to resolve a colour to answer "which facet is
  // this?". The legend answers it for a whole class; this answers it for the
  // thing under the pointer.
  //
  // It also states BOTH degrees. The scope-relative one is what the radius
  // encodes and what the connectivity rules judged; the project-wide one is
  // the payload's own fact. They genuinely disagree — a claim well-connected
  // project-wide can be the most isolated thing inside one module — and a
  // panel showing only one of them would make the other look like a bug.

  function detailRow(rows, label, value) {
    rows.appendChild(h('dt', '', label));
    var dd = h('dd');
    if (value && value.nodeType) {
      dd.appendChild(value);
    } else {
      dd.textContent = str(value);
    }
    rows.appendChild(dd);
  }

  function renderDetail() {
    if (!el.detail) {
      return;
    }
    clear(el.detail);
    if (!scene || state.selected === '') {
      el.detail.appendChild(
        h('p', 'dxg-detail-empty', 'select a node for its facet, status, degree and governors')
      );
      return;
    }

    var id = state.selected;
    var node = scene.byId[id] || null;
    var claim = claimById(id);
    var c = core();

    el.detail.appendChild(h('p', 'dxg-detail-id', id));
    var rows = h('dl', 'dxg-detail-rows');

    if (node && node.kind === 'group') {
      detailRow(rows, 'kind', 'collapsed ' + str(node.group_type) + ' group');
      detailRow(rows, 'members', num(node.size) + ' claims');
      detailRow(rows, 'facets here', facetNamesOf(node).join(', ') || 'none');
      detailRow(rows, 'degree (view)', degreeText(id));
      detailRow(rows, 'in a cycle', coversAny(node, scene.cycleIds) ? 'yes — a member is' : 'no');
      el.detail.appendChild(rows);
      var expand = h('button', 'dxg-detail-open', 'expand this group');
      expand.type = 'button';
      expand.addEventListener('click', function () {
        expandGroup(id);
      });
      el.detail.appendChild(expand);
      return;
    }

    if (node && node.kind === 'ghost') {
      detailRow(rows, 'kind', 'outside the current scope');
      detailRow(rows, 'claim', id.slice(c ? c.GHOST_PREFIX.length : 0));
      el.detail.appendChild(rows);
      return;
    }

    if (!claim) {
      detailRow(rows, 'kind', 'not a claim in this payload');
      el.detail.appendChild(rows);
      return;
    }

    // Facet BY NAME, never by slot number and never by colour.
    var facet = str(claim.facet);
    detailRow(rows, 'facet', facet === '' ? 'no facet' : facet);
    detailRow(rows, 'module', str(claim.module) === '' ? 'no module' : str(claim.module));
    detailRow(rows, 'status', str(claim.status) || 'unknown');
    detailRow(rows, 'kind', str(claim.kind) || 'unknown');
    detailRow(rows, 'build role', str(claim.build_role) === '' ? 'none set' : str(claim.build_role));
    detailRow(rows, 'degree (view)', degreeText(selectedRepId()));
    detailRow(
      rows,
      'degree (project)',
      num(claim.in_degree) + num(claim.out_degree) + ' — ' + num(claim.in_degree) + ' in, ' + num(claim.out_degree) + ' out'
    );
    detailRow(rows, 'review pending', claim.review_pending === true ? 'yes' : 'no');
    detailRow(rows, 'open threads', num(claim.open_comments));
    detailRow(rows, 'governed by', governorsOf(id).join(', ') || 'nothing');
    detailRow(rows, 'governs', governedOf(id).join(', ') || 'nothing');
    detailRow(rows, 'in a cycle', scene.cycleIds[id] ? 'yes' : scene.selfIds[id] ? 'self-edge' : 'no');
    el.detail.appendChild(rows);

    // The one place this file assigns location.hash. It fires hashchange,
    // which is exactly what is wanted here: the reading view's existing
    // deep-link scroll-and-highlight path does the rest with no new code.
    var open = h('a', 'dxg-detail-open', 'open this claim in the reading view');
    open.href = '#' + id + HASH_MARK + HASH_PREFIX + c.encodeState(state);
    open.setAttribute('data-dxg-open-claim', id);
    open.addEventListener('click', function (e) {
      e.preventDefault();
      closePane();
      root.location.hash = '#' + id + HASH_MARK + HASH_PREFIX + c.encodeState(state);
    });
    el.detail.appendChild(open);
  }

  function degreeText(id) {
    var d = scene && scene.deg[id];
    if (!d) {
      return 'not drawn in this view';
    }
    return d.total + ' — ' + d.in + ' in, ' + d.out + ' out';
  }

  function facetNamesOf(node) {
    var set = scene.facetsOfNode[node.id];
    var out = [];
    if (!set) {
      return out;
    }
    for (var k in set) {
      if (Object.prototype.hasOwnProperty.call(set, k)) {
        out.push(k === '' ? 'no facet' : k);
      }
    }
    out.sort();
    return out;
  }

  // Direction, stated once so it cannot be got backwards: a claim declares
  // `governed_by: {type: X}`, so the edge runs CLAIM -> GOVERNOR.
  function governorsOf(id) {
    var out = [];
    for (var i = 0; i < payload.edges.length; i++) {
      var e = payload.edges[i];
      if (str(e.type) === 'governed_by' && str(e.from) === id) {
        out.push(str(e.to));
      }
    }
    return out;
  }

  function governedOf(id) {
    var out = [];
    for (var i = 0; i < payload.edges.length; i++) {
      var e = payload.edges[i];
      if (str(e.type) === 'governed_by' && str(e.to) === id) {
        out.push(str(e.from));
      }
    }
    return out;
  }

  // ------------------------------------------------------------------
  // Auto-collapse above AUTO_COLLAPSE_ABOVE claims
  // ------------------------------------------------------------------
  //
  // Above the threshold the pane opens at module granularity rather than
  // drawing every claim, shows a notice naming the REAL numbers, and offers a
  // manual override that WARNS RATHER THAN BLOCKS. A reader who wants every
  // node gets every node; they are simply told first what that costs.
  //
  // 600 is a guess and will be wrong for somebody. That is why it is one
  // named constant, why the notice states it, and why the override exists —
  // a threshold a reader cannot see and cannot cross is a threshold nobody
  // can report as wrong.
  //
  // A pasted deep link is never overridden: if the hash already carries a
  // granularity, that is the reader's explicit choice and this stays out of
  // the way.

  var autoCollapsed = false;

  function applyAutoCollapse(fromHash) {
    autoCollapsed = false;
    if (!payload || fromHash) {
      return;
    }
    if (payload.nodes.length > AUTO_COLLAPSE_ABOVE && state.granularity === 'claims') {
      state.granularity = 'module';
      autoCollapsed = true;
    }
  }

  // noteManualGranularity runs when the reader changes the granularity
  // control themselves. Choosing "every claim" above the threshold is the
  // override; it is honoured, and it earns the warning.
  function noteManualGranularity() {
    autoCollapsed = false;
    collapseOverride =
      state.granularity === 'claims' && !!payload && payload.nodes.length > AUTO_COLLAPSE_ABOVE;
  }

  function renderCollapseNotice() {
    if (!payload) {
      return;
    }
    var total = payload.nodes.length;

    if (autoCollapsed) {
      var groups = countGroups();
      el.notices.appendChild(
        noticeRow(
          'showing ' +
            total +
            ' claims collapsed into ' +
            groups +
            ' modules, because this project is over the ' +
            AUTO_COLLAPSE_ABOVE +
            '-claim threshold',
          'info',
          'show every claim anyway',
          function () {
            autoCollapsed = false;
            collapseOverride = true;
            state.granularity = 'claims';
            onControlChange(true);
          }
        )
      );
      return;
    }

    if (collapseOverride && state.granularity === 'claims' && total > AUTO_COLLAPSE_ABOVE) {
      el.notices.appendChild(
        noticeRow(
          'drawing all ' +
            total +
            ' claims at once — over the ' +
            AUTO_COLLAPSE_ABOVE +
            '-claim threshold, so the layout will be slow and labels will overlap',
          'warn',
          'collapse to modules',
          function () {
            collapseOverride = false;
            state.granularity = 'module';
            onControlChange(true);
          }
        )
      );
    }
  }

  // countGroups reports how many distinct modules the payload actually has,
  // so the notice names a real number rather than a plausible one.
  function countGroups() {
    var seen = Object.create(null);
    var n = 0;
    for (var i = 0; i < payload.nodes.length; i++) {
      var m = str(payload.nodes[i].module);
      if (!seen[m]) {
        seen[m] = true;
        n++;
      }
    }
    return n;
  }

  // ------------------------------------------------------------------
  // Hash state (design section 9)
  // ------------------------------------------------------------------
  //
  //     #<existing-target-id>!g=<compact-graph-state>
  //
  // This file owns everything after the "!" and nothing before it. The half
  // before is the reading view's target id, preserved byte for byte.
  //
  // WRITES GO THROUGH history.replaceState, ALWAYS. replaceState does not
  // fire hashchange, so changing a filter never re-enters the reading view's
  // routing — which matters because that routing falls back to the FIRST
  // MODULE for anything it does not recognise, and a bare graph-state hash
  // would therefore reset the reader's place in the document and then, on its
  // own replaceState, erase the graph state it had just written.
  //
  // The hashchange listener exists for exactly one case: a URL someone else
  // pasted into the bar. Our own writes never reach it.

  function writeHash() {
    if (!isOpen || !state || !root.history || !root.history.replaceState) {
      return;
    }
    var c = core();
    if (!c) {
      return;
    }
    var next = '#' + hashHead(root.location.hash) + HASH_MARK + HASH_PREFIX + c.encodeState(state);
    if (root.location.hash !== next) {
      root.history.replaceState(null, '', next);
    }
  }

  function bindHashListener() {
    root.addEventListener('hashchange', function () {
      if (!mounted || !isOpen) {
        // The pane reads the hash fresh on every open, so an unopened pane
        // needs no listener work at all.
        return;
      }
      var c = core();
      var encoded = hashGraphState(root.location.hash);
      if (encoded === '' || !c) {
        return;
      }
      var incoming = c.decodeState(encoded);
      if (c.encodeState(incoming) === c.encodeState(state)) {
        return; // the reading view moved; the graph state is unchanged
      }
      state = incoming;
      refreshControls();
      recompute();
      startLayout(false);
    });
  }
})();
