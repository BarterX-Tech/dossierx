(function () {
  'use strict';

  function ordinal(day) {
    var mod100 = day % 100;
    if (mod100 >= 11 && mod100 <= 13) { return day + 'th'; }
    return day + ({ 1: 'st', 2: 'nd', 3: 'rd' }[day % 10] || 'th');
  }

  function formatGeneratedTime(text) {
    var match = text.match(/(\d{4})-(\d{2})-(\d{2})\s+(\d{2}):(\d{2})\s+UTC/);
    if (!match) { return text; }
    var date = new Date(Date.UTC(+match[1], +match[2] - 1, +match[3], +match[4], +match[5]));
    var month = new Intl.DateTimeFormat(undefined, { month: 'long' }).format(date);
    var time = new Intl.DateTimeFormat(undefined, { hour: 'numeric', minute: '2-digit', hour12: true }).format(date);
    return 'Generated ' + ordinal(date.getDate()) + ' ' + month + ', ' + date.getFullYear() + ' ' + time.toUpperCase();
  }

  function enhanceTimestamp() {
    var footer = document.querySelector('.sidebar-footer');
    if (!footer || footer.dataset.localTime === 'true') { return; }
    var original = footer.textContent.trim();
    footer.dataset.localTime = 'true';
    footer.title = original + ' · shown in your local time';
    footer.textContent = formatGeneratedTime(original);
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

  function bindSidebarCollapse() {
    var toggle = document.getElementById('sidebarCollapseToggle');
    if (!toggle || toggle.dataset.bound === 'true') { return; }
    toggle.dataset.bound = 'true';
    toggle.addEventListener('click', function () {
      var collapsed = !document.body.classList.contains('system-sidebar-collapsed');
      document.body.classList.toggle('system-sidebar-collapsed', collapsed);
      toggle.setAttribute('aria-expanded', String(!collapsed));
      toggle.setAttribute('aria-label', collapsed ? 'Show navigation' : 'Hide navigation');
      toggle.title = collapsed ? 'Show navigation' : 'Hide navigation';
    });
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

  function enhanceFooters() {
    document.querySelectorAll('.claim-links').forEach(function (details) {
      var summary = details.querySelector(':scope > .claim-links-summary');
      if (!summary || summary.querySelector('.claim-footer__title')) { return; }
      var match = summary.textContent.trim().match(/(\d+)\s+links?\s+-\s+(\d+)\s+files?(?:\s+-\s+(\d+)\s+sources?)?(?:\s+-\s+(\d+)\s+drifted)?/i);
      if (!match) { return; }
      var relationships = +match[1];
      var files = +match[2];
      var sources = match[3] == null ? null : +match[3];
      var drifted = match[4] == null ? null : +match[4];
      summary.textContent = '';
      var identity = document.createElement('span');
      identity.className = 'claim-footer__identity';
      identity.innerHTML = '<span><strong class="claim-footer__title">Evidence &amp; relationships</strong><small>Trace this claim through its supporting record</small></span>';
      var counts = document.createElement('span');
      counts.className = 'claim-footer__counts';
      function count(value, singular, className) {
        var item = document.createElement('span');
        if (className) { item.className = className; }
        var strong = document.createElement('strong');
        strong.textContent = value;
        item.append(strong, ' ' + (singular === 'drifted' || value === 1 ? singular : singular + 's'));
        counts.appendChild(item);
      }
      count(relationships, 'relationship');
      if (sources != null) { count(sources, 'source'); }
      count(files, 'file');
      if (drifted != null) { count(drifted, 'drifted', 'claim-footer__drifted'); }
      var chevron = document.createElement('span');
      chevron.className = 'claim-footer__chevron';
      chevron.setAttribute('aria-hidden', 'true');
      counts.appendChild(chevron);
      summary.append(identity, counts);
    });
  }

  function enhanceFieldLabels() {
    var rows = [
      ['.claim-links > .claim-edges > .claim-governed', 'Governed By', /^\s*governed_by:\s*/i],
      ['.claim-links > .claim-edges > .claim-mirrors', 'Mirrors', /^\s*mirrors:\s*/i],
      ['.claim-links > .claim-edges > .claim-rests-on', 'Rests On', /^\s*rests_on:\s*/i],
      ['.claim-links > .claim-edges > .claim-depended-by', 'Depended On By', /^\s*depended\s+on\s+by:\s*/i],
      ['.claim-links > .claim-edges > .claim-migrated', 'Migrated From', /^\s*migrated_from:\s*/i],
      ['.claim-links > .claim-edges > .claim-implemented-in', 'Implemented In', /^\s*implemented\s+in:\s*/i],
      ['.claim-links > .claim-edges > .claim-review-pending', 'Review Pending', /^\s*review_pending\s*/i],
      ['.claim-links > .claim-edges > .claim-sources', 'Sources', /^\s*sources:\s*/i]
    ];
    rows.forEach(function (entry) {
      document.querySelectorAll(entry[0]).forEach(function (row) {
        if (row.querySelector(':scope > .claim-relation-label')) { return; }
        var textNode = Array.prototype.slice.call(row.childNodes).find(function (node) { return node.nodeType === 3 && node.textContent.trim(); });
        if (!textNode) { return; }
        textNode.textContent = textNode.textContent.replace(entry[2], ' ');
        var label = document.createElement('span');
        label.className = 'claim-relation-label';
        label.textContent = entry[1];
        row.insertBefore(label, textNode);
      });
    });
  }

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
    copy.querySelectorAll('.pill, .claim-comments-slot').forEach(function (node) { node.remove(); });
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
    return { module: module, view: group, label: tab ? tab.textContent.replace(/🔒/g, '').trim() : 'Claims' };
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
    toggle.querySelector('.facet-claims-toggle__label').textContent = allCollapsed ? 'Expand all claims' : 'Collapse all claims';
  }

  function renderFacetClaimControl(active, claims) {
    var control = active.view.querySelector(':scope > .facet-claim-controls');
    if (!control) {
      control = document.createElement('div');
      control.className = 'facet-claim-controls';
      control.innerHTML = '<button class="facet-claims-toggle" type="button" aria-pressed="false"><span class="facet-claims-toggle__icon" aria-hidden="true"></span><span class="facet-claims-toggle__label">Collapse all claims</span></button>';
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
      toc.innerHTML = '<div class="facet-toc__head"><span class="facet-toc__identity"><small>On this facet</small><strong class="facet-toc__name">Claims</strong></span><span class="facet-toc__total"></span><button class="system-panel-toggle system-panel-toggle--toc" type="button" aria-controls="systemFacetToc" aria-expanded="true" aria-label="Hide table of contents" title="Hide table of contents"><span class="system-panel-toggle__chevron" aria-hidden="true"></span></button></div><nav class="facet-toc__list"></nav><select class="facet-toc__select" aria-label="Jump to a claim in this facet"></select>';
      toc.querySelector('.facet-toc__select').addEventListener('change', function (event) {
        var claim = document.getElementById(event.target.value);
        if (claim) { claim.scrollIntoView({ behavior: 'smooth', block: 'start' }); }
      });
      toc.querySelector('.system-panel-toggle--toc').addEventListener('click', function (event) {
        var collapsed = !document.body.classList.contains('system-toc-collapsed');
        document.body.classList.toggle('system-toc-collapsed', collapsed);
        event.currentTarget.setAttribute('aria-expanded', String(!collapsed));
        event.currentTarget.setAttribute('aria-label', collapsed ? 'Show table of contents' : 'Hide table of contents');
        event.currentTarget.title = collapsed ? 'Show table of contents' : 'Hide table of contents';
      });
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
      button.addEventListener('click', function () { claim.scrollIntoView({ behavior: 'smooth', block: 'start' }); });
      list.appendChild(button);
      var option = document.createElement('option');
      option.value = claim.id;
      option.textContent = number + ' · ' + label;
      select.appendChild(option);
    });
    updateTocActive();
  }

  function addModuleHeaders() {
    var labels = {};
    document.querySelectorAll('.sec-tab[data-target]').forEach(function (button) {
      labels[button.dataset.target.slice(1)] = button.textContent.replace(/🔒/g, '').trim();
    });
    document.querySelectorAll('.module-section:not(.track-section):not(.build-order-section)').forEach(function (section) {
      if (section.querySelector(':scope > .system-record-head')) { return; }
      var claims = Array.prototype.slice.call(section.querySelectorAll('.claim-group .claim[data-status]'));
      var unique = {};
      claims.forEach(function (claim) { unique[claim.id || claim.dataset.claimId] = claim; });
      var records = Object.keys(unique).map(function (key) { return unique[key]; });
      var locked = records.filter(function (claim) { return claim.dataset.status === 'locked'; }).length;
      var header = document.createElement('header');
      header.className = 'system-record-head';
      var copy = document.createElement('div');
      var title = document.createElement('h2');
      var summary = document.createElement('p');
      var metric = document.createElement('div');
      title.textContent = labels[section.id] || section.id.replace(/-/g, ' ');
      summary.className = 'system-record-head__summary';
      summary.textContent = records.length + ' claims across ' + section.querySelectorAll(':scope > .claim-group').length + ' record sections.';
      metric.className = 'system-record-head__metric';
      metric.innerHTML = '<strong>' + locked + ' of ' + records.length + '</strong> claims locked';
      copy.append(title, summary);
      header.append(copy, metric);
      section.insertBefore(header, section.firstChild);
    });
  }

  function enhanceGraphLabels() {
    var pane = document.getElementById('dxgPane');
    if (!pane || !pane.querySelector('.dxg-surface')) { return; }
    var names = { rests_on: 'Rests On', mirrors: 'Mirrors', governed_by: 'Governed By' };
    pane.querySelectorAll('[data-dxg-type]').forEach(function (button) {
      var next = names[button.dataset.dxgType];
      if (next && button.textContent !== next) { button.textContent = next; }
    });
    pane.querySelectorAll('[data-dxg-edge]').forEach(function (item) {
      var name = item.querySelector('.dxg-legend-name');
      var next = names[item.dataset.dxgEdge];
      if (name && next && name.textContent !== next) { name.textContent = next; }
    });
    pane.querySelectorAll('.dxg-ctl-label').forEach(function (label) {
      if (label.textContent.trim().toLowerCase() === 'edge types') { label.textContent = 'Relationships'; }
    });
    pane.querySelectorAll('.dxg-legend-group').forEach(function (label) {
      if (label.textContent.trim().toLowerCase() === 'edges') { label.textContent = 'Relationships'; }
    });
  }

  var running = false;
  function enhance() {
    if (running) { return; }
    running = true;
    enhanceTimestamp();
    bindResizer();
    bindSidebarCollapse();
    bindNavigationGroupPreferences();
    enhanceFooters();
    enhanceFieldLabels();
    enhanceClaimDisclosures();
    revealHashTarget();
    addModuleHeaders();
    if (typeof window.dossierxPositionStatusStrip === 'function') {
      window.dossierxPositionStatusStrip();
    }
    syncNavigation();
    renderToc();
    enhanceGraphLabels();
    running = false;
  }

  var scheduled = false;
  var layout = document.querySelector('.layout');
  if (layout) { new MutationObserver(function () {
    if (scheduled) { return; }
    scheduled = true;
    requestAnimationFrame(function () { scheduled = false; enhance(); });
  }).observe(layout, { childList: true, subtree: true }); }
  var graphPane = document.getElementById('dxgPane');
  if (graphPane) {
    new MutationObserver(function () { requestAnimationFrame(enhanceGraphLabels); })
      .observe(graphPane, { childList: true, subtree: true });
  }
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
  window.dossierxEnhanceSystemRecord = enhance;
  enhance();
})();
