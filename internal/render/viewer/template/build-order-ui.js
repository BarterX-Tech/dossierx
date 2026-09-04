(function () {
  'use strict';

  // build-order-ui.js — the Build order tab's renderer glue. It is injected
  // AFTER the vendored mermaid build, and only into a viewer with at least one
  // locked build order (shell.html guards both script tags). It owns no
  // markup: the diagrams' text is produced server-side by internal/buildorder
  // (one <pre class="mermaid"> per non-empty phase, inside .bo-diagram), and
  // this file only turns visible, not-yet-rendered blocks into SVG and wires
  // node clicks to claim cards.
  //
  // Rendering is a STATE observation, not an event. One MutationObserver on
  // .layout — a node that survives a serve fragment swap, which replaces
  // <main class="content-area"> and <nav id="nav"> by outerHTML and with them
  // every rendered SVG — schedules one requestAnimationFrame pass per mutation
  // batch; the pass renders every pre.mermaid that is laid out (offsetParent
  // !== null) and not yet data-processed. So it does not matter whether a
  // block became visible through the sidebar, a .subtab click in the module
  // strip (which only flips `hidden` on .claim-groups and fires no event), a
  // hashchange, or a swap that delivered fresh source. One case the observer
  // cannot see is a deep link at load: the shell's showFromHash has already
  // revealed the section before this file is parsed, and on file:// nothing
  // mutates .layout afterwards — so one pass is also scheduled at parse time,
  // unconditionally.
  //
  // Never render inside display:none: mermaid measures label widths in the
  // target element, and inside a hidden ancestor every width is 0 and every
  // node comes out sized to nothing with no error. The pass re-checks
  // visibility inside the frame because a MutationObserver callback runs
  // before layout.

  window.__boErrors = window.__boErrors || [];

  var initialised = false;
  var scheduled = false;
  var rendering = false;
  var pendingAgain = false;

  function token(name) {
    var v = getComputedStyle(document.body).getPropertyValue(name);
    return (v || '').trim();
  }

  // initialise reads the palette from the page's own computed tokens so the
  // OS colour scheme and a project's viewer.theme are honoured. lineColor and
  // background are the two things a classDef cannot set (the edge stroke's
  // arrowhead markers live in <defs>, out of reach of page rules), and every
  // other colour comes from style.css's .bo-* rules over the node classes.
  // fontSize is 13px on purpose: mermaid's 16px default makes every label,
  // box and dagre layout ~23% wider. `look` is left at its default (classic)
  // so a default rectangle stays a <rect> and a stadium a <path>, which the
  // page's colour rules name by element.
  function initialise() {
    if (typeof window.mermaid === 'undefined') { return false; }
    window.mermaid.initialize({
      startOnLoad: false,
      securityLevel: 'strict',
      theme: 'base',
      flowchart: { useMaxWidth: false, htmlLabels: true, nodeSpacing: 26, rankSpacing: 40, curve: 'basis' },
      themeVariables: {
        fontFamily: token('--font-sans') || 'sans-serif',
        fontSize: '13px',
        lineColor: token('--muted') || '#536179',
        background: token('--paper') || '#f6f8fc'
      }
    });
    initialised = true;
    return true;
  }

  function writeBoError(el, err) {
    var message = (err && err.message) ? err.message : String(err);
    var p = document.createElement('p');
    p.className = 'bo-error';
    p.textContent = 'diagram failed to render: ' + message;
    var host = el.closest('.bo-diagram') || el.parentNode;
    if (host) { host.appendChild(p); }
    window.__boErrors.push(message);
    console.error('build order: diagram failed to render:', err);
  }

  function visibleUnrendered() {
    return Array.prototype.slice.call(document.querySelectorAll('.bo-diagram pre.mermaid')).filter(function (el) {
      return el.offsetParent !== null && !el.hasAttribute('data-processed');
    });
  }

  // renderPass renders ONE BLOCK AT A TIME. mermaid.run over a list collects
  // every element's error and throws only the first after the loop, so a
  // single catch around a batch knows a message but not which <pre> produced
  // it, and a second failing block produces no signal at all. A rejection is
  // written into its block, recorded on window.__boErrors, logged, and then
  // re-thrown from a fresh task so the browser raises an exception event (a
  // caught rejection alone never does) without aborting the other blocks.
  async function renderPass() {
    if (rendering) { pendingAgain = true; return; }
    rendering = true;
    try {
      if (!initialised && !initialise()) { return; }
      var nodes = visibleUnrendered();
      for (var i = 0; i < nodes.length; i += 1) {
        var el = nodes[i];
        if (el.dataset.src === undefined) { el.dataset.src = el.textContent; }
        try {
          await window.mermaid.run({ nodes: [el] });
        } catch (e) {
          writeBoError(el, e);
          (function (err) { window.setTimeout(function () { throw err; }, 0); })(e);
        }
      }
    } finally {
      rendering = false;
      if (pendingAgain) { pendingAgain = false; schedule(); }
    }
  }

  function schedule() {
    if (scheduled) { return; }
    scheduled = true;
    window.requestAnimationFrame(function () {
      scheduled = false;
      renderPass();
    });
  }

  // A colour-scheme flip re-initialises with freshly computed tokens and
  // re-renders every processed block FROM THE STASHED SOURCE. Not "clear
  // data-processed and re-run": mermaid reads the diagram text from the
  // element's markup, and after the first run that markup IS the rendered
  // SVG, so a naive re-run would feed mermaid its own SVG as flowchart source
  // and stamp a .bo-error into every block the moment the OS flips to dark.
  function rerenderAll() {
    initialised = false;
    document.querySelectorAll('.bo-diagram pre.mermaid[data-processed]').forEach(function (el) {
      if (el.dataset.src === undefined) { return; }
      el.textContent = el.dataset.src;
      el.removeAttribute('data-processed');
      var host = el.closest('.bo-diagram');
      if (host) { host.querySelectorAll('.bo-error').forEach(function (n) { n.remove(); }); }
    });
    schedule();
  }

  try {
    var mq = window.matchMedia('(prefers-color-scheme: dark)');
    if (mq && typeof mq.addEventListener === 'function') {
      mq.addEventListener('change', rerenderAll);
    } else if (mq && typeof mq.addListener === 'function') {
      mq.addListener(rerenderAll);
    }
  } catch (e) { /* no matchMedia: the page keeps its first palette */ }

  // readPayload reads the JSON block by id on EVERY call and caches nothing:
  // the block sits inside .content-area, so a serve fragment swap re-delivers
  // it beside the diagrams it describes, and a copy cached at load would go
  // stale the first time a re-locked module arrives.
  function readPayload() {
    var el = document.getElementById('dossierx-build-orders');
    if (!el) { return null; }
    try { return JSON.parse(el.textContent); } catch (e) { return null; }
  }

  // nodeClaimID strips a rendered node's DOM id — mermaid-<instance>-
  // flowchart-<sanitised>-<n> — to the sanitised id and looks it up in the
  // payload's node_ids index for its module; "_" substitution is not
  // invertible, which is why the index exists. Returns null on a miss.
  function nodeClaimID(node, moduleID) {
    var payload = readPayload();
    if (!payload || !node.id) { return null; }
    var sanitised = node.id.replace(/^.*-flowchart-/, '').replace(/-\d+$/, '');
    var mod = null;
    (payload.modules || []).forEach(function (m) { if (m.id === moduleID) { mod = m; } });
    if (!mod || !mod.node_ids) { return null; }
    var claimID = Object.prototype.hasOwnProperty.call(mod.node_ids, sanitised) ? mod.node_ids[sanitised] : null;
    if (claimID === null) { return null; }
    if (!mod.claims || !Object.prototype.hasOwnProperty.call(mod.claims, claimID)) { return null; }
    return claimID;
  }

  // A hit navigates to the claim's own card; the shell's resolve() turns the
  // claim id into its module and facet. A MISS — a claim the catalog no
  // longer holds, which a locked artifact permits — does NOT navigate:
  // resolve() would fall through to the first module's first facet and drop
  // the reader somewhere unrelated with nothing said. The node is marked
  // instead.
  document.addEventListener('click', function (event) {
    var node = event.target.closest && event.target.closest('.bo-diagram g.node');
    if (!node) { return; }
    var pre = node.closest('pre.mermaid');
    var moduleID = pre ? pre.dataset.module : '';
    var claimID = nodeClaimID(node, moduleID);
    if (claimID === null) {
      node.classList.add('bo-missing');
      node.setAttribute('title', 'this claim is no longer in the catalog');
      return;
    }
    window.location.hash = '#' + claimID;
  });

  var root = document.querySelector('.layout') || document.body;
  new MutationObserver(function () { schedule(); })
    .observe(root, { childList: true, subtree: true, attributes: true, attributeFilter: ['hidden'] });

  // Initialise SYNCHRONOUSLY at parse time, not in the first frame: mermaid
  // registers its own window "load" handler at parse and, with the default
  // startOnLoad, that handler runs mermaid.run() over EVERY .mermaid element
  // on the page — hidden ones included, under the default 16px theme and
  // useMaxWidth — before a requestAnimationFrame callback gets its turn. The
  // result was every block stamped data-processed with a 16x16 viewBox and
  // zero-width labels. startOnLoad: false has to be in the config before the
  // load event fires, so it is set here, and the render pass below re-uses
  // the same initialised config.
  if (typeof window.mermaid !== 'undefined') {
    window.mermaid.startOnLoad = false;
    initialise();
  }

  // The parse-time pass: see the file comment for the deep-link case.
  schedule();
})();
