(function () {
  'use strict';

  function tabLabel(tab) {
    if (!tab) { return 'Claims'; }
    var named = tab.querySelector('.sec-tab__label');
    if (named) { return named.textContent.trim() || 'Claims'; }
    var copy = tab.cloneNode(true);
    copy.querySelectorAll('.dx-icon').forEach(function (node) { node.remove(); });
    return copy.textContent.replace(/🔒/g, '').trim() || 'Claims';
  }

  function attachIcon(host, name) {
    if (!host || host.querySelector('.dx-icon')) { return host; }
    var make = window.dxIcon;
    if (typeof make === 'function') {
      host.appendChild(make(name));
    }
    return host;
  }

  function ordinal(day) {
    var mod100 = day % 100;
    if (mod100 >= 11 && mod100 <= 13) { return day + 'th'; }
    return day + ({ 1: 'st', 2: 'nd', 3: 'rd' }[day % 10] || 'th');
  }

  // formatGeneratedTime renders the ABSOLUTE instant for the hover title
  // (reference-rules.md R10.3: "the exact build time stays on hover as a
  // title attribute, for the rare reader who needs it"). The visible text
  // is enhanceTimestamp's elapsed phrase, never this.
  function formatGeneratedTime(date) {
    var month = new Intl.DateTimeFormat(undefined, { month: 'long' }).format(date);
    var time = new Intl.DateTimeFormat(undefined, { hour: 'numeric', minute: '2-digit', hour12: true }).format(date);
    return 'Generated ' + ordinal(date.getDate()) + ' ' + month + ', ' + date.getFullYear() + ' ' + time.toUpperCase();
  }

  // freshnessPhrase implements R10.1-R10.3: elapsed time, one unit, never a
  // timestamp. Fresh (< 24h) and Ageing (1-7d) share one neutral
  // presentation (R10.2); Stale (> 7d) is flagged for enhanceTimestamp to
  // recolour, never re-worded here.
  function freshnessPhrase(generatedAt) {
    var hours = Math.max(0, Date.now() - generatedAt.getTime()) / 3600000;
    if (hours < 24) {
      var wholeHours = Math.max(1, Math.round(hours));
      return { phrase: 'Updated ' + wholeHours + ' hour' + (wholeHours === 1 ? '' : 's') + ' ago', stale: false };
    }
    var days = Math.round(hours / 24);
    return { phrase: 'Updated ' + days + ' day' + (days === 1 ? '' : 's') + ' ago', stale: hours >= 24 * 7 };
  }

  // isLive reads (never writes) viewer-runtime.js's own reachability-probe
  // signal (probeAndMount adds body.comments-live once /api/ping confirms
  // `dossierx serve`) — R10.5's hook, without this lane touching that
  // lane's function.
  function isLive() {
    return document.body.classList.contains('comments-live');
  }

  // enhanceTimestamp replaces the freshness footer's placeholder text with
  // either "Live" (R10.5, under a confirmed live serve) or the elapsed
  // phrase computed from the machine-readable instant render.go stamps
  // into data-generated-at (R10.4: "computed in the browser, not baked
  // into the HTML"). Re-run on an interval and whenever body's class list
  // changes (the probe resolves asynchronously, after first paint), so it
  // is idempotent and safe to call from enhance() on every pass too.
  function enhanceTimestamp() {
    var footer = document.querySelector('.freshness-footer');
    var phraseEl = footer && footer.querySelector('.freshness-footer__phrase');
    if (!footer || !phraseEl) { return; }
    var iso = phraseEl.dataset.generatedAt;
    var generatedAt = iso ? new Date(iso) : null;
    var validDate = generatedAt && !isNaN(generatedAt.getTime());
    if (validDate && !phraseEl.title) {
      phraseEl.title = formatGeneratedTime(generatedAt) + ' · shown in your local time';
    }
    var caption = footer.querySelector('.freshness-footer__caption');
    // Reassigning .textContent unconditionally replaces the text node
    // even when the string is unchanged, which is a childList mutation
    // inside .layout's own MutationObserver scope — and this function
    // runs on every enhance() pass. Comparing first avoids feeding that
    // observer's enhance()-on-mutation loop (see the matching guard and
    // its measurement note in updateFacetClaimControl above).
    if (isLive()) {
      if (phraseEl.textContent !== 'Live') { phraseEl.textContent = 'Live'; }
      footer.classList.remove('freshness-footer--stale');
      if (caption) { caption.hidden = true; }
      return;
    }
    if (caption) { caption.hidden = false; }
    if (!validDate) { return; }
    var freshness = freshnessPhrase(generatedAt);
    if (phraseEl.textContent !== freshness.phrase) { phraseEl.textContent = freshness.phrase; }
    footer.classList.toggle('freshness-footer--stale', freshness.stale);
  }

  function bindResizer() {
    var sidebar = document.getElementById('sidebar');
    var handle = document.getElementById('sidebarResizer');
    if (!sidebar || !handle || handle.dataset.bound === 'true') { return; }
    handle.dataset.bound = 'true';
    var startX = 0;
    var startWidth = 0;
    function setWidth(width) {
      var next = Math.max(220, Math.min(420, width));
      document.documentElement.style.setProperty('--system-record-sidebar-width', next + 'px');
      handle.setAttribute('aria-valuenow', String(Math.round(next)));
    }
    handle.addEventListener('pointerdown', function (event) {
      startX = event.clientX;
      startWidth = sidebar.getBoundingClientRect().width;
      handle.setPointerCapture(event.pointerId);
      document.body.classList.add('system-resizing');
    });
    handle.addEventListener('pointermove', function (event) {
      if (handle.hasPointerCapture(event.pointerId)) { setWidth(startWidth + event.clientX - startX); }
    });
    handle.addEventListener('pointerup', function (event) {
      if (handle.hasPointerCapture(event.pointerId)) { handle.releasePointerCapture(event.pointerId); }
      document.body.classList.remove('system-resizing');
    });
    handle.addEventListener('dblclick', function () { setWidth(270); });
    handle.addEventListener('keydown', function (event) {
      if (['ArrowLeft', 'ArrowRight', 'Home', 'End'].indexOf(event.key) < 0) { return; }
      event.preventDefault();
      var current = sidebar.getBoundingClientRect().width;
      if (event.key === 'Home') { setWidth(220); }
      else if (event.key === 'End') { setWidth(420); }
      else { setWidth(current + (event.key === 'ArrowRight' ? 12 : -12)); }
    });
  }

  // ------------------------------------------------------------------
  // Focus mode (reference-rules.md §11: R11.1-R11.6). ONE reversible
  // state — html[data-focus="on"] — replaces the engine's former two
  // independent per-rail toggles (the sidebar's #sidebarCollapseToggle and
  // the facet TOC's renderToc-injected .system-panel-toggle--toc; see
  // learnings/deadcode/L2.md). State lives on <html>, never inside
  // <main class="content-area"> or <nav id="nav">, so an SSE fragment swap
  // (shell.html's own comment: those two subtrees are replaced wholesale)
  // can never silently drop a reader out of the mode (R11.5's failure
  // mode, restated at 03 §8.6). shell.html's inline boot script (beside the
  // dossierx-theme read, before first paint) sets html[data-focus="on"]
  // from localStorage's dossierx-focus key ahead of this file loading,
  // guarded on window.innerWidth >= 861 so a phone that inherits an "on"
  // flag from an earlier desktop session never boots into a mode whose
  // rails it never had. That script carries no comments of its own on
  // purpose: html/template's contextual autoescaper re-serializes a
  // literal <script> block's JS and drops line comments, leaving
  // whitespace-only lines TestCommittedFixtureViewersAreNotStale rejects —
  // the same landmine LEARNINGS.md G10 documents for an HTML comment in
  // that file's SVG sprite, one script-tag scope over.
  // ------------------------------------------------------------------

  function isFocusOn() {
    return document.documentElement.getAttribute('data-focus') === 'on';
  }

  function focusEligible() {
    // R11.1's rails do not exist below 861px (M1 in both 02 §5 and 03
    // §5's mobile-rules tables) — a phone never had them, so the mode
    // itself is meaningless there (03 §2, "WHY MOBILE HAS NO FOCUS
    // CONTROL").
    return window.matchMedia('(min-width: 861px)').matches;
  }

  function setFocus(on) {
    on = !!on && focusEligible();
    if (on) { document.documentElement.setAttribute('data-focus', 'on'); }
    else { document.documentElement.removeAttribute('data-focus'); }
    try { localStorage.setItem('dossierx-focus', on ? 'on' : 'off'); } catch (e) {}
    document.querySelectorAll('.focus-toggle').forEach(function (button) {
      button.setAttribute('aria-pressed', String(on));
    });
  }

  function bindFocusControl() {
    if (document.documentElement.dataset.focusControlBound === 'true') { return; }
    document.documentElement.dataset.focusControlBound = 'true';
    document.addEventListener('click', function (event) {
      var target = event.target;
      if (!target || typeof target.closest !== 'function') { return; }
      if (!target.closest('[data-focus-toggle]')) { return; }
      setFocus(!isFocusOn());
    });
    // R11.4's primary exit is the `F` key. Inert while a text field has
    // focus (10 in 03 §9's open decisions) — the comment composer is one
    // keystroke away from the reading view, and a reader typing "Focus
    // mode is wrong here" must not be thrown into it — and inert below
    // 861px, where the control is not even rendered.
    document.addEventListener('keydown', function (event) {
      if (event.key !== 'f' && event.key !== 'F') { return; }
      if (event.metaKey || event.ctrlKey || event.altKey) { return; }
      var active = document.activeElement;
      var tag = active && active.tagName;
      if (active && (active.isContentEditable || tag === 'INPUT' || tag === 'TEXTAREA')) { return; }
      if (!focusEligible()) { return; }
      event.preventDefault();
      setFocus(!isFocusOn());
    });
    // A viewport that crosses the 861px boundary while focus is on must
    // not strand the reader in a mode with no rendered control to leave
    // it from — mobile never shows the rails focus hides (03 §2).
    var narrow = window.matchMedia('(max-width: 860px)');
    var onNarrowChange = function (mq) {
      if (mq.matches && isFocusOn()) { setFocus(false); }
    };
    if (typeof narrow.addEventListener === 'function') { narrow.addEventListener('change', onNarrowChange); }
    else if (typeof narrow.addListener === 'function') { narrow.addListener(onNarrowChange); }
  }

  function bindNavigationGroupPreferences() {
    document.querySelectorAll('.system-nav-group').forEach(function (group) {
      if (group.dataset.readerPreferenceBound === 'true') { return; }
      group.dataset.readerPreferenceBound = 'true';
      var summary = group.querySelector(':scope > summary');
      if (summary) {
        summary.addEventListener('click', function (event) {
          event.preventDefault();
          var willOpen = !group.open;
          group.dataset.readerClosed = String(!willOpen);
          group.open = willOpen;
        });
      }
      group.addEventListener('toggle', function () {
        group.dataset.readerClosed = group.open ? 'false' : 'true';
      });
    });
  }

  function syncNavigation(force) {
    var active = document.querySelector('.system-nav-group .sec-tab.on');
    var group = active && active.closest('.system-nav-group');
    if (!group || (!force && group.dataset.readerClosed === 'true')) { return; }
    group.dataset.readerClosed = 'false';
    group.open = true;
  }

  // enhanceFooters and enhanceFieldLabels were retired here: both targeted
  // the pre-redesign single-<details> mono digest and flat <li> labels
  // ("N links - N files - N drifted", "governed_by:" text prefixes) that
  // components.EdgesHTMLWithLinks no longer emits — see that Go function's
  // doc comment and docs/design/screens/05-claim-one-expansion-at-a-time.md
  // R09.1/R-F.1. The new footer strip (.claim-footer, .claim-footer-chip*,
  // .claim-relationship-direction*) ships its final vocabulary directly
  // from the server, so no client-side rewrite step is needed any more.

  var claimDisclosureSequence = 0;

  function setClaimExpanded(claim, expanded) {
    var toggle = claim && claim.querySelector(':scope > .k > .claim-collapse-toggle');
    var content = claim && claim.querySelector(':scope > .claim-collapse-content');
    if (!toggle || !content) { return; }
    claim.classList.toggle('claim--collapsed', !expanded);
    content.hidden = !expanded;
    toggle.setAttribute('aria-expanded', String(expanded));
    var title = toggle.dataset.claimTitle || 'claim';
    toggle.setAttribute('aria-label', (expanded ? 'Collapse ' : 'Expand ') + title);
    updateFacetClaimControl();
  }

  function enhanceClaimDisclosures() {
    document.querySelectorAll('.claim').forEach(function (claim) {
      if (claim.dataset.claimDisclosure === 'true') { return; }
      var head = claim.querySelector(':scope > .k');
      if (!head) { return; }

      var title = cleanTitle(claim) || 'claim';
      var content = document.createElement('div');
      content.className = 'claim-collapse-content';
      content.id = 'claim-content-' + (++claimDisclosureSequence);
      while (head.nextSibling) { content.appendChild(head.nextSibling); }

      var toggle = document.createElement('button');
      toggle.type = 'button';
      toggle.className = 'claim-collapse-toggle';
      toggle.dataset.claimTitle = title;
      toggle.setAttribute('aria-controls', content.id);
      toggle.setAttribute('aria-expanded', 'true');
      toggle.setAttribute('aria-label', 'Collapse ' + title);

      var label = head.querySelector(':scope > .label');
      if (label) {
        toggle.appendChild(label);
      } else {
        Array.prototype.slice.call(head.childNodes).forEach(function (node) {
          if (!(node.nodeType === 1 && node.classList.contains('claim-comments-slot'))) {
            toggle.appendChild(node);
          }
        });
      }
      var chevron = document.createElement('span');
      chevron.className = 'claim-collapse-chevron';
      chevron.setAttribute('aria-hidden', 'true');
      attachIcon(chevron, 'chevron-right');
      toggle.appendChild(chevron);
      head.insertBefore(toggle, head.firstChild);
      claim.appendChild(content);
      claim.dataset.claimDisclosure = 'true';
      toggle.addEventListener('click', function () {
        setClaimExpanded(claim, toggle.getAttribute('aria-expanded') !== 'true');
      });
    });
  }

  function revealHashTarget() {
    var raw = (window.location.hash || '').replace(/^#/, '').split('!')[0];
    if (!raw) { return; }
    var target;
    try { target = document.getElementById(decodeURIComponent(raw)); }
    catch (_) { return; }
    var claim = target && target.closest('.claim');
    if (claim) { setClaimExpanded(claim, true); }
  }

  function cleanTitle(claim) {
    var title = claim.querySelector(':scope > .k');
    if (!title) { return (claim.id || '').replace(/[.-]/g, ' '); }
    var copy = title.cloneNode(true);
    // .k-id is the v0.4.x mono id line card.html's head now renders beneath
    // the title (docs/design/screens/07-claim-boundary-no-embodiment.md
    // §4.4) — stripped here for the same reason .pill and .claim-comments-slot
    // are: every caller of cleanTitle (the collapse toggle's aria-label, the
    // facet TOC's item label) wants the human title alone, not the title with
    // the machine id run on right after it.
    copy.querySelectorAll('.pill, .claim-comments-slot, .k-id').forEach(function (node) { node.remove(); });
    return copy.textContent.replace(/\s+/g, ' ').trim();
  }

  function visibleClaims(view) {
    return Array.prototype.slice.call(view.querySelectorAll('.claim')).filter(function (claim) {
      var node = claim;
      while (node && node !== view) {
        if (node.hidden || getComputedStyle(node).display === 'none') { return false; }
        node = node.parentElement;
      }
      return true;
    });
  }

  // activeFacet returns null while the Build order tab is active (it is not a
  // module section: no header, TOC, status strip or claim controls are built
  // over the diagrams), so renderToc hides the TOC there.
  function activeFacet() {
    var modules = Array.prototype.slice.call(document.querySelectorAll('.module-section:not(.track-section):not(.build-order-section)'));
    var module = modules.find(function (section) { return !section.hidden; });
    if (!module) { return null; }
    var groups = Array.prototype.slice.call(module.querySelectorAll(':scope > .claim-group'));
    var group = groups.find(function (section) { return !section.hidden; });
    if (!group) { return null; }
    var tab = module.querySelector(':scope > .sub-nav .subtab[data-target="#' + group.id + '"]');
    return { module: module, view: group, label: tabLabel(tab) };
  }

  function updateFacetClaimControl(active, claims) {
    active = active || activeFacet();
    if (!active) { return; }
    claims = claims || visibleClaims(active.view);
    var control = active.view.querySelector(':scope > .facet-claim-controls');
    if (!control) { return; }
    var toggle = control.querySelector('.facet-claims-toggle');
    if (!toggle) { return; }
    var allCollapsed = claims.length > 0 && claims.every(function (claim) {
      var disclosure = claim.querySelector(':scope > .k > .claim-collapse-toggle');
      return disclosure && disclosure.getAttribute('aria-expanded') === 'false';
    });
    toggle.disabled = claims.length === 0;
    toggle.setAttribute('aria-pressed', String(allCollapsed));
    // Idempotency guard (cross-cutting fix, not new behaviour): this ran
    // unconditionally on every call, and updateFacetClaimControl is
    // itself called from renderToc on every enhance() pass — reassigning
    // .textContent replaces the text node even when the string is
    // unchanged, which is a childList mutation of an ancestor inside
    // .layout, which re-triggers the .layout MutationObserver that calls
    // enhance() in the first place. Measured before this fix: an idle
    // page accrues hundreds of .layout mutation records per second,
    // forever, entirely from this one line (plus this lane's own
    // .freshness-footer__phrase, guarded the same way in
    // enhanceTimestamp). Comparing first breaks the loop with no visible
    // behaviour change.
    var label = toggle.querySelector('.facet-claims-toggle__label');
    var nextLabel = allCollapsed ? 'Expand all claims' : 'Collapse all claims';
    if (label && label.textContent !== nextLabel) { label.textContent = nextLabel; }
  }

  function renderFacetClaimControl(active, claims) {
    var control = active.view.querySelector(':scope > .facet-claim-controls');
    if (!control) {
      control = document.createElement('div');
      control.className = 'facet-claim-controls';
      control.innerHTML = '<button class="facet-claims-toggle" type="button" aria-pressed="false"><svg class="dx-icon facet-claims-toggle__icon" aria-hidden="true"><use href="#dx-icon-chevron-down"></use></svg><span class="facet-claims-toggle__label">Collapse all claims</span></button>';
      active.view.insertBefore(control, active.view.firstChild);
      control.querySelector('.facet-claims-toggle').addEventListener('click', function () {
        var current = activeFacet();
        if (!current) { return; }
        var currentClaims = visibleClaims(current.view);
        var shouldExpand = currentClaims.length > 0 && currentClaims.every(function (claim) {
          var disclosure = claim.querySelector(':scope > .k > .claim-collapse-toggle');
          return disclosure && disclosure.getAttribute('aria-expanded') === 'false';
        });
        currentClaims.forEach(function (claim) { setClaimExpanded(claim, shouldExpand); });
        updateFacetClaimControl(current, currentClaims);
      });
    }
    updateFacetClaimControl(active, claims);
  }

  function updateTocActive() {
    // Last claim whose top has crossed the sticky-header band. Soft-mounted
    // surfaces call dossierxEnhanceSystemRecord after clone, which re-runs
    // renderToc → updateTocActive so .on is present without an IntersectionObserver
    // that would disagree with this scroll-position rule (theme-parity hover).
    var toc = document.getElementById('systemFacetToc');
    if (!toc || toc.hidden) { return; }
    var links = Array.prototype.slice.call(toc.querySelectorAll('.facet-toc__item'));
    if (!links.length) { return; }
    var current = links[0];
    links.forEach(function (link) {
      var claim = document.getElementById(link.dataset.claimTarget);
      if (claim && claim.getBoundingClientRect().top <= 190) { current = link; }
    });
    links.forEach(function (link) { link.classList.toggle('on', link === current); });
    var select = toc.querySelector('.facet-toc__select');
    if (select) { select.value = current.dataset.claimTarget; }
  }

  function renderToc() {
    // Theme-parity's hover probe forces :hover through CDP node ids. This
    // function replaceChildren()s the TOC list on every enhance pass (the
    // layout MutationObserver retriggers enhance when addModuleHeaders
    // rewrites the module head), so a node id dies between QuerySelector
    // and ForcePseudoState. The harness sets this flag for the duration of
    // that probe; it is otherwise unset.
    if (window.__dxParityFreezeToc) { return; }
    var toc = document.getElementById('systemFacetToc');
    if (!toc) {
      toc = document.createElement('aside');
      toc.id = 'systemFacetToc';
      toc.className = 'facet-toc';
      toc.setAttribute('aria-label', 'Claims in this facet');
      // R11.1: the facet TOC no longer carries its own collapse toggle —
      // focus mode (bindFocusControl, html[data-focus="on"]) is the one
      // control that hides it now, from the module head instead of from
      // here.
      // The freshness block (R10.1 "the right-rail footer", 02 §3/§4.17
      // node 1EZ-0 inside the facet panel 8P-0) is built here rather than
      // server-rendered inline: it used to be a static child of the left
      // sidebar-footer in shell.html, and this is the third and final
      // retry attempt landing its move into the right facet panel (see
      // learnings/inbox/L2.md). #sidebar carries data-generated-at (added
      // by this lane at shell.html's <aside id="sidebar"> tag) so the
      // instant survives the move without shell.html emitting the whole
      // block twice. A project with no GeneratedAt (offline/dev render)
      // renders no freshness block at all, matching the old {{if
      // .GeneratedAt}} guard.
      var generatedAt = document.getElementById('sidebar');
      generatedAt = generatedAt && generatedAt.dataset.generatedAt;
      var freshnessHTML = generatedAt
        ? '<div class="freshness-footer"><p class="freshness-footer__line"><svg class="dx-icon freshness-footer__icon" aria-hidden="true"><use href="#dx-icon-clock"/></svg><span class="freshness-footer__phrase" data-generated-at="' + generatedAt + '">Updated recently</span></p><p class="freshness-footer__caption">Claims changed since then are not in this view</p></div>'
        : '';
      toc.innerHTML = '<div class="facet-toc__grabber" aria-hidden="true"></div><div class="facet-toc__head"><span class="facet-toc__identity"><small>On this facet</small><strong class="facet-toc__name">Claims</strong></span><span class="facet-toc__total"></span><button class="facet-toc__close" type="button" aria-label="Close facet panel"><svg class="dx-icon" aria-hidden="true"><use href="#dx-icon-x"></use></svg></button></div><nav class="facet-toc__list"></nav><select class="facet-toc__select" aria-label="Jump to a claim in this facet"></select>' + freshnessHTML;
      toc.querySelector('.facet-toc__select').addEventListener('change', function (event) {
        var claim = document.getElementById(event.target.value);
        if (claim) { claim.scrollIntoView({ behavior: 'smooth', block: 'start' }); }
      });
      toc.querySelector('.facet-toc__close').addEventListener('click', closeFacetToc);
      document.body.appendChild(toc);
    }
    var active = activeFacet();
    if (!active) { toc.hidden = true; return; }
    toc.hidden = false;
    var claims = visibleClaims(active.view);
    toc.querySelector('.facet-toc__name').textContent = active.label;
    toc.querySelector('.facet-toc__total').textContent = claims.length + (claims.length === 1 ? ' claim' : ' claims');
    renderFacetClaimControl(active, claims);
    var list = toc.querySelector('.facet-toc__list');
    var select = toc.querySelector('.facet-toc__select');
    list.replaceChildren();
    select.replaceChildren();
    claims.forEach(function (claim, index) {
      var number = String(index + 1).padStart(2, '0');
      var label = cleanTitle(claim);
      var button = document.createElement('button');
      button.type = 'button';
      button.className = 'facet-toc__item';
      button.dataset.claimTarget = claim.id;
      var count = document.createElement('span');
      var strong = document.createElement('strong');
      count.textContent = number;
      strong.textContent = label;
      button.append(count, strong);
      button.addEventListener('click', function () {
        claim.scrollIntoView({ behavior: 'smooth', block: 'start' });
        closeFacetToc();
      });
      list.appendChild(button);
      var option = document.createElement('option');
      option.value = claim.id;
      option.textContent = number + ' · ' + label;
      select.appendChild(option);
    });
    updateTocActive();
    ensureFacetTocTrigger(active, claims.length);
  }

  // ---------------------------------------------------------------------
  // 02 §5 M1/M8 + §4.18 (node 4XR-0): at <=860px the facet panel is a
  // bottom sheet, closed by default, opened from an "On this facet"
  // trigger at the right end of the active module's facet tab strip
  // (.sub-nav). The trigger is hidden above 860px by CSS
  // (.facet-toc-trigger has no rule outside that lane-owned @media block).
  // A module with exactly one facet renders no .sub-nav at all (shell.html
  // guards it on .HasSubNav), so it gets no trigger either — matching
  // "the one 1-facet module (no sub-nav head)" state, which has nothing to
  // open.
  // ---------------------------------------------------------------------
  function facetTocScrim() {
    var scrim = document.getElementById('facetTocScrim');
    if (!scrim) {
      scrim = document.createElement('button');
      scrim.id = 'facetTocScrim';
      scrim.type = 'button';
      scrim.className = 'facet-toc-scrim';
      scrim.setAttribute('aria-label', 'Close facet panel');
      scrim.tabIndex = -1;
      scrim.addEventListener('click', closeFacetToc);
      document.body.appendChild(scrim);
    }
    return scrim;
  }

  function closeFacetToc() {
    document.body.classList.remove('facet-toc-open');
    document.querySelectorAll('.facet-toc-trigger[aria-expanded="true"]').forEach(function (trigger) {
      trigger.setAttribute('aria-expanded', 'false');
    });
  }

  function openFacetToc(trigger) {
    facetTocScrim();
    document.body.classList.add('facet-toc-open');
    document.querySelectorAll('.facet-toc-trigger').forEach(function (other) {
      other.setAttribute('aria-expanded', String(other === trigger));
    });
  }

  function ensureFacetTocTrigger(active, count) {
    var subNav = active.module.querySelector(':scope > .sub-nav');
    if (!subNav) { return; }
    var trigger = subNav.querySelector(':scope > .facet-toc-trigger');
    if (!trigger) {
      trigger = document.createElement('button');
      trigger.type = 'button';
      trigger.className = 'facet-toc-trigger';
      trigger.setAttribute('aria-expanded', 'false');
      trigger.setAttribute('aria-controls', 'systemFacetToc');
      trigger.innerHTML = '<span>On this facet</span><span class="facet-toc-trigger__count"></span>';
      trigger.addEventListener('click', function () {
        if (document.body.classList.contains('facet-toc-open')) { closeFacetToc(); }
        else { openFacetToc(trigger); }
      });
      subNav.appendChild(trigger);
    }
    var countEl = trigger.querySelector('.facet-toc-trigger__count');
    if (countEl && countEl.textContent !== String(count)) { countEl.textContent = String(count); }
  }

  document.addEventListener('keydown', function (event) {
    if (event.key === 'Escape' && document.body.classList.contains('facet-toc-open')) { closeFacetToc(); }
  });

  function addModuleHeaders() {
    var labels = {};
    document.querySelectorAll('.sec-tab[data-target]').forEach(function (button) {
      labels[button.dataset.target.slice(1)] = tabLabel(button);
    });
    // focusState joins the per-module signature below so a Focus toggle
    // (which does not itself call addModuleHeaders) still gets picked up
    // the next time something else does — .focus-toggle's own
    // aria-pressed is kept live regardless, by setFocus writing it
    // directly (see bindFocusControl above).
    var focusState = isFocusOn() ? 'on' : 'off';
    var moduleSections = Array.prototype.slice.call(
      document.querySelectorAll('.module-section:not(.track-section):not(.build-order-section)')
    );
    var moduleCount = moduleSections.length;
    moduleSections.forEach(function (section, moduleIndex) {
      var total = parseInt(section.getAttribute('data-claim-count') || '0', 10);
      var locked = parseInt(section.getAttribute('data-locked-count') || '0', 10);
      var facets = parseInt(section.getAttribute('data-facet-count') || '0', 10);
      if (!facets) { facets = section.querySelectorAll(':scope > .claim-group').length; }
      var label = labels[section.id] || section.id.replace(/-/g, ' ');
      // 02 §4.7 row 1 / 02 §6 "Module eyebrow (64-0)": MODULE NN / NN, the
      // module's 1-based index over the module count.
      var eyebrowText = 'MODULE ' + String(moduleIndex + 1).padStart(2, '0') + ' / ' + String(moduleCount).padStart(2, '0');
      var existing = section.querySelector(':scope > .system-record-head');
      // Idempotency guard. enhance() re-runs on every mutation of .layout
      // (a MutationObserver drives it, for the soft-mount / SSE-swap
      // cases that need it), and this function's own remove()+rebuild is
      // ITSELF a childList mutation of .layout — so an unconditional
      // rebuild here means the observer re-schedules enhance() via
      // requestAnimationFrame forever, once per frame, never settling.
      // Measured: an unmodified page left alone accrues hundreds of
      // .layout mutation records per second. Skip the rebuild when
      // nothing the header actually shows has changed; a real change
      // (claim counts after a live reload, a facet renamed, focus
      // toggled) still invalidates the signature and rebuilds normally.
      var signature = [label, total, locked, facets, focusState, eyebrowText].join('|');
      if (existing && existing.dataset.signature === signature) { return; }
      if (existing) { existing.remove(); }
      var header = document.createElement('header');
      header.dataset.signature = signature;
      header.className = 'system-record-head';
      var copy = document.createElement('div');
      var eyebrow = document.createElement('p');
      var title = document.createElement('h2');
      var metric = document.createElement('div');
      eyebrow.className = 'system-record-head__eyebrow';
      eyebrow.textContent = eyebrowText;
      title.textContent = label;
      metric.className = 'system-record-head__metric';
      // 02 §4.7/§9.13: "31 of 31 locked" — the numeral alone is bold and
      // "claims" is dropped, matching the board over the pre-revamp
      // "<strong>N of M</strong> claims locked" wording. The bar beside it
      // is a confirmation of that same phrase, not a second fact — 6px
      // tall, no label of its own. The Focus control (R11.1) is the last
      // element in the cluster, past a hairline divider (02 §4.8).
      var pct = total > 0 ? Math.round((locked / total) * 100) : 0;
      metric.innerHTML =
        '<span class="system-record-head__count"><strong>' + locked + '</strong> of ' + total + ' locked</span>' +
        '<span class="system-record-head__bar" aria-hidden="true"><span class="system-record-head__bar-fill" style="width:' + pct + '%"></span></span>' +
        '<span class="focus-group"><span class="focus-divider" aria-hidden="true"></span>' +
        '<button type="button" class="focus-toggle" data-focus-toggle aria-pressed="' + (isFocusOn() ? 'true' : 'false') + '" aria-label="Focus — hide navigation and read this facet at full width" title="Focus (F)">' +
        '<svg class="dx-icon focus-toggle__icon" aria-hidden="true"><use href="#dx-icon-maximize"></use></svg>' +
        '<span class="focus-toggle__label">Focus</span>' +
        '<span class="focus-toggle__hint">F</span>' +
        '</button></span>';
      copy.append(eyebrow, title);
      header.append(copy, metric);
      section.insertBefore(header, section.firstChild);
    });
  }

  function applyThemeChoice(choice) {
    if (choice !== 'light' && choice !== 'dark') { choice = 'system'; }
    document.documentElement.setAttribute('data-theme', choice);
    try { localStorage.setItem('dossierx-theme', choice); } catch (e) {}
    document.querySelectorAll('.theme-control [data-theme-choice]').forEach(function (button) {
      button.setAttribute('aria-pressed', String(button.getAttribute('data-theme-choice') === choice));
    });
  }

  function bindThemeControl() {
    var current = document.documentElement.getAttribute('data-theme') || 'system';
    applyThemeChoice(current);
    if (document.documentElement.dataset.themeControlBound === 'true') { return; }
    document.documentElement.dataset.themeControlBound = 'true';
    document.addEventListener('click', function (event) {
      var target = event.target;
      if (!target || typeof target.closest !== 'function') { return; }
      var button = target.closest('[data-theme-choice]');
      if (!button || !button.closest('.theme-control')) { return; }
      applyThemeChoice(button.getAttribute('data-theme-choice'));
    });
  }

  // enhanceGraphLabels() used to live here and, on every #dxgPane mutation,
  // forcibly overwrote graph-ui.js's own [data-dxg-type] / [data-dxg-edge]
  // labels back to "Rests On" / "Mirrors" / "Governed By" and renamed the
  // "edge types" / "edges" headings to "Relationships" — a fixup for a
  // pre-13 build of the pane that had no display-label map of its own.
  // graph-ui.js now builds every one of those labels correctly at the
  // source (RELATIONSHIP_LABELS, 13 §4.2/§4.5/§6: "Depends on" / "Says the
  // same thing" / "Governed by", the "Relations" eyebrow, the "marks"
  // legend group), and this shim was firing on every rebuild and silently
  // reverting all of it back to the pre-13 wording — removed here rather
  // than left to fight graph-ui.js on every interaction. Found via
  // graph_redrive_test.go/graph_pane_test.go passing while the rendered
  // page itself still showed the old strings; see learnings/inbox/L9.md.
  // The #dxgPane MutationObserver that called this on every pane mutation
  // is removed below it, for the same reason.

  var running = false;
  function enhance() {
    if (running) { return; }
    running = true;
    bindThemeControl();
    bindResizer();
    bindFocusControl();
    bindNavigationGroupPreferences();
    enhanceClaimDisclosures();
    revealHashTarget();
    addModuleHeaders();
    if (typeof window.dossierxPositionStatusStrip === 'function') {
      window.dossierxPositionStatusStrip();
    }
    syncNavigation();
    renderToc();
    // enhanceTimestamp() runs after renderToc(): the freshness footer now
    // lives INSIDE the facet-toc panel renderToc creates (02 §3/§4.17,
    // "the right facet panel" — see the .freshness-footer move below), so
    // on the very first pass the footer does not exist until the TOC is
    // built. renderToc() must still run after syncNavigation(), which is
    // what makes activeFacet() see the right module/facet's hidden state.
    enhanceTimestamp();
    running = false;
  }

  var scheduled = false;
  var layout = document.querySelector('.layout');
  if (layout) { new MutationObserver(function () {
    if (scheduled) { return; }
    scheduled = true;
    requestAnimationFrame(function () { scheduled = false; enhance(); });
  }).observe(layout, { childList: true, subtree: true }); }
  document.addEventListener('click', function (event) {
    // Keep the selected tab's group visible when navigation itself changes,
    // but respect a reader explicitly closing that group. Running
    // syncNavigation after every click used to close a <details> and reopen it
    // in the same event cycle whenever it contained the active tab.
    var navigationChanged = !!event.target.closest('.sec-tab');
    setTimeout(function () {
      if (navigationChanged) { syncNavigation(true); }
      renderToc();
    }, 0);
  });
  window.addEventListener('hashchange', function () { setTimeout(function () { revealHashTarget(); syncNavigation(true); renderToc(); }, 0); });
  window.addEventListener('scroll', updateTocActive, { passive: true });
  // R10.4: the elapsed phrase is computed in the browser, so it has to be
  // recomputed while the tab stays open, not just once at load. The class
  // observer catches viewer-runtime.js's asynchronous probeAndMount adding
  // body.comments-live (R10.5's "Live" signal) without this lane polling
  // for it or that lane calling back into this file.
  window.setInterval(enhanceTimestamp, 60000);
  new MutationObserver(enhanceTimestamp).observe(document.body, { attributes: true, attributeFilter: ['class'] });
  window.dossierxEnhanceSystemRecord = enhance;
  enhance();
})();
