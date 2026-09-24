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
    var elapsedMs = Math.max(0, Date.now() - generatedAt.getTime());
    var hours = elapsedMs / 3600000;
    // R10.2's sub-hour band. Without it `Math.max(1, ...)` below clamped every
    // age under 90 minutes up to "Updated 1 hour ago", so a viewer opened
    // seconds after a build claimed to be an hour stale. One unit still, per
    // R10.1 — minutes never pair with seconds.
    if (hours < 1) {
      var minutes = Math.round(elapsedMs / 60000);
      if (minutes < 1) { return { phrase: 'Updated just now', stale: false }; }
      return { phrase: 'Updated ' + minutes + ' minute' + (minutes === 1 ? '' : 's') + ' ago', stale: false };
    }
    if (hours < 24) {
      var wholeHours = Math.max(1, Math.round(hours));
      return { phrase: 'Updated ' + wholeHours + ' hour' + (wholeHours === 1 ? '' : 's') + ' ago', stale: false };
    }
    var days = Math.round(hours / 24);
    return { phrase: 'Updated ' + days + ' day' + (days === 1 ? '' : 's') + ' ago', stale: hours >= 24 * 7 };
  }

  // enhanceTimestamp replaces the freshness footer's placeholder text with
  // the elapsed phrase computed from the machine-readable instant render.go
  // stamps into data-generated-at. Paper keeps its age and explanatory caption.
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
    // Reassigning .textContent unconditionally replaces the text node
    // even when the string is unchanged, which is a childList mutation
    // inside .layout's own MutationObserver scope — and this function
    // runs on every enhance() pass. Comparing first avoids feeding that
    // observer's enhance()-on-mutation loop (see the matching guard and
    // its measurement note in updateFacetClaimControl above).
    if (!validDate) { return; }
    var freshness = freshnessPhrase(generatedAt);
    if (phraseEl.textContent !== freshness.phrase) { phraseEl.textContent = freshness.phrase; }
    footer.classList.toggle('freshness-footer--stale', freshness.stale);
    var live = footer.querySelector('.freshness-footer__live');
    if (document.body.classList.contains('comments-live')) {
      if (!live) {
        live = document.createElement('span');
        live.className = 'freshness-footer__live';
        live.textContent = 'Live';
        phraseEl.parentNode.appendChild(live);
      }
      live.hidden = false;
    } else if (live) {
      live.hidden = true;
    }
  }

  // ------------------------------------------------------------------
  // Focus mode (reference-rules.md §11: R11.1-R11.6). ONE reversible
  // state — html[data-focus="on"] — replaces the engine's former two
  // independent per-rail toggles (the sidebar's #sidebarCollapseToggle and
  // the facet TOC's renderToc-injected .system-panel-toggle--toc, neither of
  // which survives — viewer-tests/claim_collapse_test.go asserts both are
  // absent and that exactly one .focus-toggle replaces them). State lives on
  // <html>, never inside
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
  // the same landmine an HTML comment in that file's SVG sprite hit, one
  // script-tag scope over.
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
  // ("N links - N files - N drifted", "rests_on:" text prefixes) that
  // components.EdgesHTMLWithLinks no longer emits — see that Go function's
  // doc comment and docs/design/screens/05-claim-one-expansion-at-a-time.md
  // R09.1/R-F.1. The new footer strip (.claim-footer, .claim-footer-chip*,
  // .claim-relationship-direction*) ships its final vocabulary directly
  // from the server, so no client-side rewrite step is needed any more.

  var claimBodyDisclosureSequence = 0;
  var claimBodyTabIndexes = new WeakMap();

  function claimBodyFocusable(body) {
    return Array.prototype.slice.call(body.querySelectorAll('a[href], button, input, select, textarea, [tabindex]'));
  }

  function restoreClaimBodyFocus(body) {
    claimBodyFocusable(body).forEach(function (node) {
      if (!claimBodyTabIndexes.has(node)) { return; }
      var previous = claimBodyTabIndexes.get(node);
      if (previous === null) { node.removeAttribute('tabindex'); }
      else { node.setAttribute('tabindex', previous); }
      claimBodyTabIndexes.delete(node);
    });
  }

  function suppressClippedClaimBodyFocus(body) {
    restoreClaimBodyFocus(body);
    var bodyBottom = body.getBoundingClientRect().bottom;
    claimBodyFocusable(body).forEach(function (node) {
      var rect = node.getBoundingClientRect();
      if (rect.top < bodyBottom - 1) { return; }
      claimBodyTabIndexes.set(node, node.hasAttribute('tabindex') ? node.getAttribute('tabindex') : null);
      node.setAttribute('tabindex', '-1');
    });
  }

  // Collapsing a long body removes up to ~1.5 viewports of prose from between
  // the reader's eye and the rest of the page: measured 845px at 1440 and
  // 1373px at 390. Nothing compensated, so the card the reader was reading
  // ended up hundreds of pixels ABOVE the viewport and they were left on a
  // claim two cards down.
  //
  // Paper does not specify this: the file carries fourteen "...more" nodes and
  // no "less" node, and neither screen 02 nor 05 says what a collapse does to
  // scroll position. This is a decided convention, not a transcription — the
  // card the reader was reading stays the thing they are looking at.
  //
  // Only when the card's top edge has been left above the deep-link line. If
  // it is already on screen we leave the page alone; a scroll nobody asked for
  // is its own surprise.
  function keepCollapsedClaimInView(claim) {
    if (!claim) { return; }
    // `.claim { scroll-margin-top }` already encodes the sticky sub-nav's
    // clearance and is where a deep link lands. Reading it keeps one number
    // for both landings.
    var offset = parseFloat(getComputedStyle(claim).scrollMarginTop) || 0;
    var top = claim.getBoundingClientRect().top;
    if (top >= offset - 1) { return; }
    // `html { scroll-behavior: smooth }` would animate this correction over
    // ~900px. The collapse is instant, so the catch-up is too.
    window.scrollTo({ top: Math.max(0, Math.round(window.scrollY + top - offset)), behavior: 'instant' });
  }

  // BODY_LIKE is what the four-line disclosure clamps: a claim's own body, and
  // the passage-by-passage diff that stands in for it while the reader is
  // looking at what changed since approval.
  //
  // The diff is in this set because it IS the body, shown a passage at a time
  // — so a claim that clamps to four lines with "... more" in one state and
  // runs to full height in the other is the same claim behaving as two
  // different components. Sharing the mechanism rather than copying it also
  // means the toggle's label, its aria wiring and its scroll compensation
  // cannot drift between the two.
  // .claim-edit-current is the other state of the same surface: the current
  // wording rendered from the same passages, with the changed one ruled. It
  // stands in for the body exactly as the diff does, so it clamps like it.
  var BODY_LIKE = '.claim-body, .claim-edit-diff, .claim-edit-current';

  function setClaimBodyExpanded(wrapper, expanded) {
    var body = wrapper && wrapper.querySelector(':scope > .claim-body, :scope > .claim-edit-diff, :scope > .claim-edit-current');
    var toggle = wrapper && wrapper.querySelector(':scope > .claim-body-disclosure__toggle');
    if (!body || !toggle || toggle.hidden) { return; }
    wrapper.classList.toggle('claim-body-disclosure--expanded', expanded);
    wrapper.classList.toggle('claim-body-disclosure--collapsed', !expanded);
    toggle.setAttribute('aria-expanded', String(expanded));
    toggle.textContent = expanded ? 'less' : '… more';
    wrapper.dataset.readerExpanded = String(expanded);
    if (expanded) { restoreClaimBodyFocus(body); }
    else { requestAnimationFrame(function () { suppressClippedClaimBodyFocus(body); }); }
  }

  function syncClaimBodyDisclosures() {
    document.querySelectorAll(BODY_LIKE).forEach(function (body) {
      var wrapper = body.parentElement && body.parentElement.classList.contains('claim-body-disclosure')
        ? body.parentElement
        : null;
      if (!wrapper) {
        wrapper = document.createElement('div');
        wrapper.className = 'claim-body-disclosure';
        // The diff's wrapper is marked, because the card hides the claim's
        // own body while the changes are showing and must not hide the diff
        // along with it — both are a .claim-body-disclosure by then.
        if (body.classList.contains('claim-edit-diff') || body.classList.contains('claim-edit-current')) {
          wrapper.classList.add('claim-body-disclosure--edit');
        }
        body.parentNode.insertBefore(wrapper, body);
        wrapper.appendChild(body);
        if (!body.id) { body.id = 'claim-body-' + (++claimBodyDisclosureSequence); }
        var toggle = document.createElement('button');
        toggle.type = 'button';
        toggle.className = 'claim-body-disclosure__toggle';
        toggle.setAttribute('aria-controls', body.id);
        toggle.hidden = true;
        toggle.addEventListener('click', function () {
          var expand = toggle.getAttribute('aria-expanded') !== 'true';
          setClaimBodyExpanded(wrapper, expand);
          // Only on a reader-driven collapse. setClaimBodyExpanded is also
          // called by syncClaimBodyDisclosures on every navigation and by
          // revealHashTarget; compensating there would move the page under the
          // reader for reasons they never triggered.
          if (!expand) { keepCollapsedClaimInView(wrapper.closest('.claim')); }
        });
        wrapper.appendChild(toggle);
      }

      // A display:none module has no usable geometry. It is measured the
      // first time navigation reveals it, via the document click hook below.
      if (!body.getClientRects().length) { return; }
      var lineHeight = parseFloat(getComputedStyle(body).lineHeight) || 28;
      // A steps claim always needs the control, however short its prose: its
      // `.step` siblings are hidden while collapsed (style.css), so without a
      // toggle they would be unreachable. Paper's collapsed card for one of
      // these (6KC-0) carries the truncated body and the "...more" control and
      // nothing else.
      var stepsHost = body.closest('.claim');
      var hasSteps = !!(stepsHost && stepsHost.querySelector(':scope > .step'));
      var needsDisclosure = hasSteps || body.scrollHeight > (lineHeight * 4) + 1;
      var control = wrapper.querySelector(':scope > .claim-body-disclosure__toggle');
      control.hidden = !needsDisclosure;
      if (!needsDisclosure) {
        wrapper.classList.remove('claim-body-disclosure--collapsed', 'claim-body-disclosure--expanded');
        restoreClaimBodyFocus(body);
        return;
      }
      setClaimBodyExpanded(wrapper, wrapper.dataset.readerExpanded === 'true');
    });
  }

  function revealHashTarget() {
    var raw = (window.location.hash || '').replace(/^#/, '').split('!')[0];
    if (!raw) { return; }
    var target;
    try { target = document.getElementById(decodeURIComponent(raw)); }
    catch (_) { return; }
    var wrapper = target && target.closest('.claim-body-disclosure');
    if (wrapper) { setClaimBodyExpanded(wrapper, true); }
  }

  function enhanceConformanceFooters() {
    document.querySelectorAll('details.claim-conformance').forEach(function (original) {
      var claim = original.closest('.claim');
      var footer = claim && claim.querySelector('.claim-footer');
      var summary = original.querySelector(':scope > summary');
      if (!footer || !summary) { return; }
      var labelNode = summary.querySelector('strong');
      var label = labelNode ? labelNode.textContent.trim() : 'Checks';
      if (label === 'No checks declared') {
        var empty = document.createElement('span');
        empty.className = 'claim-footer-chip claim-footer-chip--checks claim-footer-chip--empty';
        var emptyLabel = document.createElement('span');
        emptyLabel.className = 'claim-footer-chip-label';
        emptyLabel.textContent = label;
        empty.appendChild(emptyLabel);
        original.replaceWith(empty);
        footer.insertBefore(empty, footer.querySelector('.claim-comments-slot'));
        return;
      }

      var door = document.createElement('details');
      door.className = 'claim-conformance-door';
      Array.prototype.slice.call(original.attributes).forEach(function (attribute) {
        if (attribute.name !== 'class' && attribute.name !== 'open') { door.setAttribute(attribute.name, attribute.value); }
      });
      if (original.open) { door.open = true; }
      summary.className = 'claim-conformance-head claim-footer-chip claim-footer-chip--checks';
      summary.replaceChildren();
      var chipLabel = document.createElement('span');
      chipLabel.className = 'claim-footer-chip-label';
      chipLabel.textContent = label;
      summary.append(chipLabel, window.dossierxFooterChevron());
      door.appendChild(summary);

      var panel = document.createElement('div');
      panel.className = 'claim-conformance claim-footer-panel';
      Array.prototype.slice.call(original.attributes).forEach(function (attribute) {
        if (attribute.name !== 'class' && attribute.name !== 'name' && attribute.name !== 'open') { panel.setAttribute(attribute.name, attribute.value); }
      });
      while (original.firstChild) { panel.appendChild(original.firstChild); }
      original.remove();
      var comments = footer.querySelector('.claim-comments-slot');
      footer.insertBefore(door, comments);
      footer.insertBefore(panel, comments);
    });
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
    var modules = Array.prototype.slice.call(document.querySelectorAll('.module-section:not(.track-section):not(.build-order-section):not(.constitution-section)'));
    var module = modules.find(function (section) { return !section.hidden; });
    if (!module) { return null; }
    var groups = Array.prototype.slice.call(module.querySelectorAll(':scope > .claim-group'));
    var group = groups.find(function (section) { return !section.hidden; });
    if (!group) { return null; }
    var tab = module.querySelector(':scope > .sub-nav .subtab[data-target="#' + group.id + '"]');
    return { module: module, view: group, label: tabLabel(tab) };
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
      // retry attempt landing its move into the right facet panel. #sidebar
      // carries data-generated-at (added
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
      toc.innerHTML = '<div class="facet-toc__grabber" aria-hidden="true"></div><div class="facet-toc__head"><span class="facet-toc__identity"><small>On this facet</small><span class="facet-toc__mobile-identity"><strong class="facet-toc__name">Claims</strong><span class="facet-toc__total"></span></span></span><button class="facet-toc__close" type="button" aria-label="Close facet panel"><svg class="dx-icon" aria-hidden="true"><use href="#dx-icon-x"></use></svg></button></div><nav class="facet-toc__list"></nav><select class="facet-toc__select" aria-label="Jump to a claim in this facet"></select>' + freshnessHTML;
      toc.querySelector('.facet-toc__select').addEventListener('change', function (event) {
        navigateToClaim(event.target.value);
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
    var list = toc.querySelector('.facet-toc__list');
    var select = toc.querySelector('.facet-toc__select');
    list.replaceChildren();
    select.replaceChildren();
    claims.forEach(function (claim) {
      var label = cleanTitle(claim);
      var button = document.createElement('button');
      button.type = 'button';
      button.className = 'facet-toc__item';
      button.dataset.claimTarget = claim.id;
      var count = document.createElement('span');
      var strong = document.createElement('strong');
      strong.textContent = label;
      // Read the total stamped from the authoritative readiness assessment.
      // Counting .claim-readiness-blocker nodes undercounts any claim whose
      // progressively disclosed list has not been expanded yet.
      var readinessDoor = claim.querySelector('[data-readiness-fact-count]');
      var blockerCount = readinessDoor ? parseInt(readinessDoor.dataset.readinessFactCount || '0', 10) : 0;
      if (!Number.isFinite(blockerCount)) { blockerCount = 0; }
      count.className = 'facet-toc__blocker-count';
      count.textContent = blockerCount ? String(blockerCount) : '';
      if (blockerCount) {
        count.setAttribute('aria-label', blockerCount + (blockerCount === 1 ? ' blocker' : ' blockers'));
      } else {
        count.setAttribute('aria-hidden', 'true');
      }
      button.append(strong, count);
      button.dataset.claimTarget = claim.id;
      button.addEventListener('click', function () { navigateToClaim(button.dataset.claimTarget); });
      list.appendChild(button);
      var option = document.createElement('option');
      option.value = claim.id;
      option.textContent = label;
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

  var facetTocFocusReturn = null;
  var facetTocFocusFrame = 0;

  function cancelFacetTocFocusRestore() {
    if (!facetTocFocusFrame) { return; }
    window.cancelAnimationFrame(facetTocFocusFrame);
    facetTocFocusFrame = 0;
  }

  function closeFacetToc(restoreTrigger) {
    var wasOpen = document.body.classList.contains('facet-toc-open');
    cancelFacetTocFocusRestore();
    document.body.classList.remove('facet-toc-open');
    document.querySelectorAll('.facet-toc-trigger[aria-expanded="true"]').forEach(function (trigger) {
      trigger.setAttribute('aria-expanded', 'false');
    });
    if (wasOpen && restoreTrigger !== false && facetTocFocusReturn) {
      var trigger = facetTocFocusReturn;
      if (trigger.isConnected && typeof trigger.focus === 'function') { trigger.focus({ preventScroll: true }); }
      facetTocFocusFrame = window.requestAnimationFrame(function () {
        facetTocFocusFrame = 0;
        if (trigger.isConnected && typeof trigger.focus === 'function') { trigger.focus({ preventScroll: true }); }
      });
    }
  }

  function openFacetToc(trigger) {
    cancelFacetTocFocusRestore();
    facetTocFocusReturn = trigger;
    facetTocScrim();
    document.body.classList.add('facet-toc-open');
    document.querySelectorAll('.facet-toc-trigger').forEach(function (other) {
      other.setAttribute('aria-expanded', String(other === trigger));
    });
  }

  // A tapped claim row NAVIGATES to the claim; it does not scroll a node this
  // closure happens to be holding. Three things made the old
  // `claim.scrollIntoView(); closeFacetToc();` unreliable:
  //
  //  1. closeFacetToc restores focus to the trigger synchronously AND again in
  //     a rAF. Both are scroll-affecting, and focus({preventScroll}) is not
  //     honoured everywhere, so the restore cancelled the smooth scroll that
  //     had just started and the tap looked inert. Navigation is therefore
  //     deferred one frame PAST closeFacetToc's own rAF re-focus, which also
  //     keeps the "close returns focus to the trigger" contract that
  //     viewer-tests/component_fit_test.go pins.
  //  2. `claim` was a captured node. renderToc() replaceChildren()s this list
  //     on every document click, every .layout mutation and every hashchange,
  //     and SoftMount's mountSurface clones fresh cards into the host — a
  //     detached node makes scrollIntoView a silent no-op. An id survives all
  //     of that; a node reference does not.
  //  3. SoftMount: a claim still living in a <template> is invisible to
  //     getElementById, so nothing here may resolve it. Assigning
  //     location.hash is the established cross-file mechanism (graph-ui.js's
  //     "back to this claim in the reading view" does the same, for the same
  //     reason): hashchange reaches viewer-runtime.js's showFromHash ->
  //     showModuleFacet, which mountSurface()es the facet BEFORE resolving the
  //     id, then scrolls and adds .claim-target-highlight. The hash also makes
  //     the jump shareable.
  //
  // The '!' suffix is the graph pane's half of the hash and is preserved
  // verbatim, exactly as viewer-runtime.js's hashGraphSuffix does.
  // The landing is INSTANT, deliberately. `html { scroll-behavior: smooth }`
  // turns every scrollIntoView in this viewer into an animation whose length
  // grows with the distance, and a real facet here is 26-33 claims and
  // 10,000-17,000px tall, so showModuleFacet's scrollIntoView takes ~1.6s to
  // cross one. Worse, the same hashchange also runs revealHashTarget,
  // syncNavigation and renderToc plus the .layout observer's enhance() pass,
  // which hold the main thread for ~1s BEFORE that animation gets its first
  // frame. Measured on the real corpus: a tapped row does not move the page
  // for a full second and finishes 2.4s later, which is what a reader reports
  // as "the navigation items are not clickable". A TOC row is a jump, not a
  // reading gesture, so land it in one frame, where a deep link lands.
  function landOnClaim(id) {
    var claim = document.getElementById(id);
    if (!claim) { return false; }
    claim.scrollIntoView({ block: 'start', behavior: 'instant' });
    return true;
  }

  function navigateToClaim(id) {
    if (!id) { return; }
    var raw = window.location.hash || '';
    var bang = raw.indexOf('!');
    var next = '#' + id + (bang < 0 ? '' : raw.slice(bang));
    var already = raw === next;
    // LAND FIRST, SYNCHRONOUSLY, before anything else in this handler.
    // Measured on the real corpus: deferring the landing to the hashchange
    // left the page motionless for over 600ms — the hash assignment costs a
    // frame, and the hashchange task then queues behind ~900ms of
    // renderToc/syncNavigation/enhance() work on the main thread. A reader
    // gets no feedback in that window and clicks again, which is exactly the
    // "the rows do nothing" report. renderToc builds these rows from live DOM
    // nodes (visibleClaims), so the target is ALWAYS already mounted and a
    // synchronous scroll is safe here; the hashchange landing below stays as
    // the backstop for the case where it is not.
    landOnClaim(id);
    closeFacetToc();
    window.requestAnimationFrame(function () {
      if (already) {
        // Re-assigning an identical hash fires no hashchange, so land it here.
        landOnClaim(id);
        return;
      }
      // Land as soon as the hashchange has been PROCESSED, not on a guessed
      // frame. viewer-runtime.js registered its own hashchange listener at
      // load, so this one — added later — runs after it in the same dispatch,
      // by which point showModuleFacet has mounted the surface and the id
      // resolves. A rAF-timed landing raced that task and frequently fired
      // while the claim was still unmounted, leaving the slow smooth scroll
      // in charge.
      window.addEventListener('hashchange', function once() {
        window.removeEventListener('hashchange', once);
        landOnClaim(id);
      });
      window.location.hash = next;
      // showModuleFacet re-renders the facet, which rebuilds the sub-nav and
      // with it the trigger, so closeFacetToc's focus restore lands on a node
      // that is no longer in the document and focus falls to <body>. Re-assert
      // it on whatever trigger now exists, once the swap has settled.
      // preventScroll keeps this from undoing the deep-link scroll that the
      // hashchange just performed.
      window.requestAnimationFrame(function () {
        // The hashchange task has run by now, so showModuleFacet has mounted
        // the surface and the id resolves. Re-issue the landing without the
        // animation; this supersedes the smooth scroll showModuleFacet just
        // started. A claim that still does not resolve leaves that scroll
        // alone, so this can only make the jump faster, never break it.
        landOnClaim(id);
        var active = document.activeElement;
        if (active && active !== document.body && active !== document.documentElement) { return; }
        var trigger = document.querySelector('.facet-toc-trigger');
        if (trigger) { trigger.focus({ preventScroll: true }); }
      });
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
      // Paper node 4XS-0 leads the chip with a 12px list glyph. #dx-icon-menu
      // is the closest sprite (its third stroke is full-width where Paper's is
      // short); an exact #dx-icon-list symbol would be the faithful
      // alternative if one is ever added to the sprite sheet.
      trigger.innerHTML = '<svg class="dx-icon" aria-hidden="true"><use href="#dx-icon-menu"></use></svg><span>On this facet</span><span class="facet-toc-trigger__count"></span>';
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
      document.querySelectorAll('.module-section:not(.track-section):not(.build-order-section):not(.constitution-section)')
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
      metric.dataset.allLocked = String(total > 0 && locked === total);
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
    syncThemeControls(choice);
  }

  function effectiveTheme(choice) {
    if (choice === 'light' || choice === 'dark') { return choice; }
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }

  function syncThemeControls(choice) {
    var effective = effectiveTheme(choice);
    document.querySelectorAll('.theme-control [data-theme-choice]').forEach(function (button) {
      button.setAttribute('aria-pressed', String(button.getAttribute('data-theme-choice') === effective));
    });
    document.querySelectorAll('[data-theme-toggle]').forEach(function (button) {
      var next = effective === 'dark' ? 'light' : 'dark';
      var use = button.querySelector('use');
      button.dataset.themeCurrent = effective;
      button.setAttribute('aria-label', 'Use ' + next + ' theme');
      button.setAttribute('title', 'Use ' + next + ' theme');
      if (use) { use.setAttribute('href', effective === 'dark' ? '#dx-icon-moon' : '#dx-icon-sun'); }
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
      var toggle = target.closest('[data-theme-toggle]');
      if (toggle) {
        applyThemeChoice(toggle.dataset.themeCurrent === 'dark' ? 'light' : 'dark');
        return;
      }
      var button = target.closest('[data-theme-choice]');
      if (!button || !button.closest('.theme-control')) { return; }
      applyThemeChoice(button.getAttribute('data-theme-choice'));
    });
    var scheme = window.matchMedia('(prefers-color-scheme: dark)');
    var syncSystemChoice = function () {
      var choice = document.documentElement.getAttribute('data-theme') || 'system';
      if (choice === 'system') { syncThemeControls(choice); }
    };
    if (typeof scheme.addEventListener === 'function') { scheme.addEventListener('change', syncSystemChoice); }
    else if (typeof scheme.addListener === 'function') { scheme.addListener(syncSystemChoice); }
  }

  // enhanceGraphLabels() used to live here and, on every #dxgPane mutation,
  // forcibly overwrote graph-ui.js's own [data-dxg-type] / [data-dxg-edge]
  // labels back to "Rests On" and renamed the
  // "edge types" / "edges" headings to "Relationships" — a fixup for a
  // pre-13 build of the pane that had no display-label map of its own.
  // graph-ui.js now builds every one of those labels correctly at the
  // source (RELATIONSHIP_LABELS, 13 §4.2/§4.5/§6: "Depends on" /
  // the "Relations" eyebrow, the "marks"
  // legend group), and this shim was firing on every rebuild and silently
  // reverting all of it back to the pre-13 wording — removed here rather
  // than left to fight graph-ui.js on every interaction. Found via
  // graph_redrive_test.go/graph_pane_test.go passing while the rendered
  // page itself still showed the old strings.
  // The #dxgPane MutationObserver that called this on every pane mutation
  // is removed below it, for the same reason.

  var running = false;
  function sentenceCaseReadingLabels() {
    // Only generated navigation and claim labels; authored body text and IDs
    // keep their exact spelling. Existing acronyms and mixed-case names stay.
    document.querySelectorAll('.system-nav-group__body .sec-tab__label, .claim .k-title').forEach(function (label) {
      if (label.dataset.readingCase === 'true') { return; }
      var words = label.textContent.trim().split(/\s+/);
      label.textContent = words.map(function (word, index) {
        return index > 0 && /^[A-Z][a-z]+$/.test(word) ? word.toLowerCase() : word;
      }).join(' ');
      label.dataset.readingCase = 'true';
    });
  }

  function enhance() {
    if (running) { return; }
    running = true;
    sentenceCaseReadingLabels();
    bindThemeControl();
    bindFocusControl();
    bindNavigationGroupPreferences();
    enhanceConformanceFooters();
    syncClaimBodyDisclosures();
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
      syncClaimBodyDisclosures();
      renderToc();
    }, 0);
  });
  window.addEventListener('hashchange', function () { setTimeout(function () { revealHashTarget(); syncNavigation(true); renderToc(); }, 0); });
  window.addEventListener('scroll', updateTocActive, { passive: true });
  var claimBodyResizeFrame = 0;
  window.addEventListener('resize', function () {
    if (claimBodyResizeFrame) { cancelAnimationFrame(claimBodyResizeFrame); }
    claimBodyResizeFrame = requestAnimationFrame(function () {
      claimBodyResizeFrame = 0;
      syncClaimBodyDisclosures();
    });
  });
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
