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

  function syncNavigation() {
    var active = document.querySelector('.system-nav-group .sec-tab.on');
    var group = active && active.closest('.system-nav-group');
    if (group) { group.open = true; }
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
      ['.claim-links > .claim-edges > .claim-sources', 'Sources', /^\s*sources:\s*/i],
      ['.build-order-module .claim-file', 'File', /^\s*file:\s*/i],
      ['.build-order-module .claim-rests-on', 'Rests On', /^\s*rests_on:\s*/i]
    ];
    rows.forEach(function (entry) {
      document.querySelectorAll(entry[0]).forEach(function (row) {
        if (row.querySelector(':scope > .claim-relation-label, :scope > .build-order-field__label')) { return; }
        var textNode = Array.prototype.slice.call(row.childNodes).find(function (node) { return node.nodeType === 3 && node.textContent.trim(); });
        if (!textNode) { return; }
        textNode.textContent = textNode.textContent.replace(entry[2], ' ');
        var label = document.createElement('span');
        label.className = entry[0].indexOf('.build-order-module') === 0 ? 'build-order-field__label' : 'claim-relation-label';
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
    if (!title) { return (claim.id || '').replace(/^build-order-/, '').replace(/[.-]/g, ' '); }
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

  function activeFacet() {
    var modules = Array.prototype.slice.call(document.querySelectorAll('.module-section:not(.track-section)'));
    var module = modules.find(function (section) { return !section.hidden; });
    if (!module) { return null; }
    var build = module.querySelector(':scope > .build-order-module:not([hidden])');
    if (build) {
      return { module: module, view: build, label: build.classList.contains('system-orientation-only') ? 'Orientation' : 'Build Order' };
    }
    var groups = Array.prototype.slice.call(module.querySelectorAll(':scope > .claim-group'));
    var group = groups.find(function (section) { return !section.hidden; });
    if (!group) { return null; }
    var tab = module.querySelector(':scope > .sub-nav .subtab[data-target="#' + group.id + '"]');
    return { module: module, view: group, label: tab ? tab.textContent.replace(/🔒/g, '').trim() : 'Claims' };
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
      toc.innerHTML = '<div class="facet-toc__head"><span><small>On this facet</small><strong class="facet-toc__name">Claims</strong></span><span class="facet-toc__total"></span></div><nav class="facet-toc__list"></nav><select class="facet-toc__select" aria-label="Jump to a claim in this facet"></select>';
      toc.querySelector('.facet-toc__select').addEventListener('change', function (event) {
        var claim = document.getElementById(event.target.value);
        if (claim) { claim.scrollIntoView({ behavior: 'smooth', block: 'start' }); }
      });
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

  function prepareBuildOrder(module) {
    var build = module.querySelector(':scope > .build-order-module');
    if (!build || build.dataset.systemPrepared === 'true') { return build; }
    build.dataset.systemPrepared = 'true';
    var title = build.querySelector(':scope > h3');
    if (title) { title.classList.add('system-build-title'); title.dataset.originalText = title.textContent; }
    var children = Array.prototype.slice.call(build.children);
    var orientation = children.find(function (node) {
      return node.classList.contains('build-order-phase') && node.textContent.trim().toLowerCase() === 'orientation';
    });
    if (orientation) {
      var start = children.indexOf(orientation);
      for (var index = start; index < children.length; index += 1) {
        var node = children[index];
        if (index > start && node.classList.contains('build-order-phase')) { break; }
        node.classList.add('system-orientation-node');
      }
      build.dataset.hasOrientation = 'true';
    }
    return build;
  }

  function activateFacet(module, mode, targetID, updateHash) {
    var build = prepareBuildOrder(module);
    var isBuild = mode === 'build-order' || mode === 'orientation';
    module.querySelectorAll(':scope > .claim-group').forEach(function (group) { group.hidden = isBuild || group.id !== targetID; });
    if (build) {
      build.hidden = !isBuild;
      build.classList.toggle('system-orientation-only', mode === 'orientation');
      var title = build.querySelector(':scope > .system-build-title');
      if (title) { title.textContent = mode === 'orientation' ? 'Orientation' : title.dataset.originalText; }
    }
    module.querySelectorAll(':scope > .sub-nav .subtab').forEach(function (tab) {
      var tabMode = tab.dataset.systemMode || 'claims';
      tab.classList.toggle('on', tabMode === mode && (isBuild || tab.dataset.target === '#' + targetID));
    });
    if (updateHash) { history.replaceState(null, '', '#' + targetID); }
    scrollTo(0, 0);
    renderToc();
  }

  function enhanceFacetNavigation() {
    document.querySelectorAll('.module-section:not(.track-section)').forEach(function (module) {
      var build = prepareBuildOrder(module);
      if (!build) { return; }
      var subnav = module.querySelector(':scope > .sub-nav');
      if (!subnav) {
        subnav = document.createElement('div');
        subnav.className = 'sub-nav system-created-sub-nav';
        var firstGroup = module.querySelector(':scope > .claim-group');
        var firstTab = document.createElement('button');
        firstTab.className = 'subtab';
        firstTab.dataset.target = '#' + firstGroup.id;
        firstTab.textContent = 'Claims';
        subnav.appendChild(firstTab);
        module.insertBefore(subnav, firstGroup);
      }
      if (!subnav.querySelector('[data-system-mode="build-order"]')) {
        [['build-order', 'Build Order', '#' + build.id], ['orientation', 'Orientation', '#orientation-' + module.id]].forEach(function (entry) {
          if (entry[0] === 'orientation' && build.dataset.hasOrientation !== 'true') { return; }
          var tab = document.createElement('button');
          tab.type = 'button';
          tab.className = 'subtab system-added-subtab';
          tab.dataset.target = entry[2];
          tab.dataset.systemMode = entry[0];
          tab.textContent = entry[1];
          tab.addEventListener('click', function (event) {
            event.preventDefault();
            event.stopImmediatePropagation();
            activateFacet(module, entry[0], entry[2].slice(1), true);
          }, true);
          subnav.appendChild(tab);
        });
      }
      subnav.querySelectorAll('.subtab:not(.system-added-subtab)').forEach(function (tab) {
        if (tab.dataset.systemBound === 'true') { return; }
        tab.dataset.systemBound = 'true';
        tab.addEventListener('click', function () { setTimeout(function () { activateFacet(module, 'claims', tab.dataset.target.slice(1), false); }, 0); });
      });
      var active = Array.prototype.slice.call(module.querySelectorAll(':scope > .claim-group')).find(function (group) { return !group.hidden; });
      if (active && !module.dataset.systemActivated) {
        module.dataset.systemActivated = 'true';
        activateFacet(module, 'claims', active.id, false);
      }
    });
  }

  function addModuleHeaders() {
    var labels = {};
    document.querySelectorAll('.sec-tab[data-target]').forEach(function (button) {
      labels[button.dataset.target.slice(1)] = button.textContent.replace(/🔒/g, '').trim();
    });
    document.querySelectorAll('.module-section:not(.track-section)').forEach(function (section) {
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
    enhanceFooters();
    enhanceFieldLabels();
    enhanceClaimDisclosures();
    revealHashTarget();
    addModuleHeaders();
    enhanceFacetNavigation();
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
  document.addEventListener('click', function () { setTimeout(function () { syncNavigation(); renderToc(); }, 0); });
  window.addEventListener('hashchange', function () { setTimeout(function () { revealHashTarget(); syncNavigation(); renderToc(); }, 0); });
  window.addEventListener('scroll', updateTocActive, { passive: true });
  window.dossierxEnhanceSystemRecord = enhance;
  enhance();
})();
