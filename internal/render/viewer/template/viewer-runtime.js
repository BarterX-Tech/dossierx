    (function () {
      'use strict';

      function dxIcon(name) {
        if (!/^[a-z0-9-]+$/.test(name)) {
          throw new Error('unknown icon');
        }
        var wrap = document.createElement('span');
        wrap.innerHTML = '<svg class="dx-icon" aria-hidden="true"><use href="#dx-icon-' + name + '"></use></svg>';
        return wrap.firstChild;
      }
      window.dxIcon = dxIcon;

      function footerChevron() {
        // innerHTML, not createElementNS: an explicit SVG namespace URI
        // string trips TestNoNetworkReferencesAnywhereInEngine.
        var wrap = document.createElement('span');
        wrap.innerHTML = '<svg class="claim-footer__chevron" viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="m6 9 6 6 6-6"></path></svg>';
        return wrap.firstChild;
      }
      window.dossierxFooterChevron = footerChevron;

      // ================================================================
      // Navigation lookup maps + view state (idempotent initViewer)
      // ================================================================
      //
      // These NodeLists and lookup maps are all RE-DERIVED by initViewer(),
      // which is safe to call any number of times. Phase 5 keeps everything the
      // deep-link/tab machinery reads in these IIFE-scope vars (not const at the
      // top) precisely so a later SSE fragment swap can replace <main> + <nav>
      // and then call initViewer() again to re-point them at the fresh DOM.
      var moduleSections, moduleTabs;
      var facetToModule, moduleDefaultFacet, firstModuleID, claimToFacet, sourceToFacet;

      // initViewer re-queries the section/tab NodeLists and rebuilds every
      // lookup map from the CURRENT DOM. Called once at load and (Phase 5c) after
      // each SSE re-render. It attaches NO listeners — those are delegated once
      // on surviving nodes below, so re-running this never double-binds.
      function initViewer() {
        // Fragment swaps replace <main>; drop the stale mount pointer.
        mountedSurfaceID = null;
        // Two-level show/hide: one .module-section per sidebar .sec-tab, and
        // (when a module has more than one facet) one .claim-group per .subtab
        // inside that module's .sub-nav. A single-facet module still has exactly
        // one .claim-group child; it just has no .sub-nav strip to click.
        moduleSections = document.querySelectorAll('.module-section');
        moduleTabs = document.querySelectorAll('.sec-tab');

        // facetToModule maps every facet (.claim-group) id to its owning
        // module-section id; moduleDefaultFacet maps a module-section id to its
        // first facet's id (shown when the module tab itself, not a specific
        // subtab, is clicked/hashed to).
        facetToModule = {};
        moduleDefaultFacet = {};
        moduleSections.forEach(function (sec) {
          var facets = sec.querySelectorAll('.claim-group');
          facets.forEach(function (g, i) {
            facetToModule[g.id] = sec.id;
            if (i === 0) { moduleDefaultFacet[sec.id] = g.id; }
          });
        });
        firstModuleID = moduleSections.length ? moduleSections[0].id : '';

        // claimToFacet maps every individual claim card's own id (the full claim
        // id, e.g. "widget.doctrine.foo") to its owning .claim-group (facet) id,
        // so an edges-footer link (`href="#<claim-id>"`) resolves to a specific
        // card instead of the unknown-hash fallback. Overview claims render N
        // times but only the canonical copy keeps its id (render.stripOverviewIDs
        // strips the rest), so only that copy is indexed here — consistent with
        // the comment JS keying off data-claim-id (never id) for state fan-out.
        claimToFacet = {};
        document.querySelectorAll('.claim-group').forEach(function (g) {
          // Index both live-mounted cards and inert <template> payloads so a
          // deep link to an unmounted surface still resolves to that facet.
          var roots = [g];
          var tmpl = g.querySelector(':scope > template.dossierx-surface-template');
          if (tmpl) { roots.push(tmpl.content); }
          roots.forEach(function (root) {
            root.querySelectorAll('.claim[id]').forEach(function (c) {
              if (!Object.prototype.hasOwnProperty.call(claimToFacet, c.id)) {
                claimToFacet[c.id] = g.id;
              }
            });
          });
        });

        // sourceToFacet maps every SOURCE ROW's anchor id
        // ("<claim-id>-source-<n>", components.ClaimSourceAnchorID) to its
        // owning .claim-group. A "[n]" citation marker in a claim body links
        // straight at one of these, and resolve() falls back to the FIRST
        // MODULE for any hash it does not recognize — so without this index a
        // reader clicking a citation would be moved to a different module
        // entirely, which is both wrong and hard to attribute to the click.
        // Only the canonical copy of a claim carries these ids
        // (render.stripDuplicateClaimIDs removes them from an overview note's
        // repeated copies and from a track's inline copy), so this map is
        // one-to-one for the same reason claimToFacet is.
        sourceToFacet = {};
        document.querySelectorAll('.claim-group').forEach(function (g) {
          var roots = [g];
          var tmpl = g.querySelector(':scope > template.dossierx-surface-template');
          if (tmpl) { roots.push(tmpl.content); }
          roots.forEach(function (root) {
            root.querySelectorAll('.claim-source[id]').forEach(function (s) {
              if (!Object.prototype.hasOwnProperty.call(sourceToFacet, s.id)) {
                sourceToFacet[s.id] = g.id;
              }
            });
          });
        });

        // Zero-thread chips arrive from the server hidden (a file:// export has
        // no comment API to reach). Re-apply the probe's verdict over whatever
        // DOM is current — on first load `mounted` is still false and this is a
        // no-op, after a fragment swap it re-reveals the fresh chips.
        syncEmptyChips();

        // Re-arm the source-note clamps over the current DOM. This is the one
        // thing initViewer does that owns a resource rather than a lookup map,
        // which is exactly why it belongs here: the call disconnects the single
        // observer before re-observing, so the nodes a fragment swap discarded
        // are released in the same pass that adopts their replacements. The
        // delegated click that works the control is still attached once, below.
        mountSourceNoteClamps();
      }

      // resolve maps an arbitrary hash fragment to a {module, facet, claim}
      // triple. Checked in order: a claim id -> its own card's facet + module; a
      // source row's anchor id -> the same, but scrolled to the row rather than
      // to the card; a facet id -> its own module + itself (a Build order
      // module's "#dossierx-build-order-<module>" group is a facet of the #dossierx-build-order
      // section here, nothing special); a bare module id (which a TRACK section
      // and the Build order section also are — see track_view.go) -> that
      // module + its default facet; anything else -> the first module + its
      // default facet.
      function resolve(id) {
        if (Object.prototype.hasOwnProperty.call(claimToFacet, id)) {
          var facetID = claimToFacet[id];
          return { module: facetToModule[facetID], facet: facetID, claim: id };
        }
        // A source row is returned as the `claim` so showModuleFacet scrolls to
        // and highlights the ROW the citation named, not merely the card it
        // sits in — the whole point of a citation marker is the specific line.
        // The collapsed footer around it is opened by the
        // `.claim-links:has(.claim-source:target)` rule in style.css, never
        // from here: a URL fragment is unknowable server-side and the disclosure
        // has to open on a static file:// export too.
        if (Object.prototype.hasOwnProperty.call(sourceToFacet, id)) {
          var srcFacet = sourceToFacet[id];
          return { module: facetToModule[srcFacet], facet: srcFacet, claim: id };
        }
        if (Object.prototype.hasOwnProperty.call(facetToModule, id)) {
          return { module: facetToModule[id], facet: id };
        }
        if (Object.prototype.hasOwnProperty.call(moduleDefaultFacet, id)) {
          return { module: id, facet: moduleDefaultFacet[id] };
        }
        return { module: firstModuleID, facet: moduleDefaultFacet[firstModuleID] };
      }


      // Deferred surface mounting: claim card HTML lives in <template
      // class="dossierx-surface-template"> nodes (inert — not in the live DOM).
      // Large corpora (Curtainly-scale, ~80+ claim cards) mount only the active
      // surface on first paint; smaller fixtures mount every surface so print,
      // theme probes, and live-reload witnesses that call getElementById keep
      // working without visiting every facet first.
      var mountedSurfaceID = null;
      var SOFT_MOUNT_MIN_CLAIMS = 80;

      function claimCorpusSize() {
        var n = 0;
        document.querySelectorAll('template.dossierx-surface-template').forEach(function (tmpl) {
          n += tmpl.content.querySelectorAll('.claim').length;
        });
        return n;
      }

      function softMountEnabled() {
        return claimCorpusSize() >= SOFT_MOUNT_MIN_CLAIMS;
      }

      // Surfaces mount on first visit and STAY mounted. Clearing inactive
      // hosts would drop in-memory UI state (source-note clamps, open
      // disclosures) and break tests that compare a module card with its
      // track copy. The load-time win is still intact for large corpora: only
      // the first surface is cloned during init; the rest remain inert
      // <template>s until navigated to (or until mountAllSurfaces runs).
      function mountSurface(surfaceID) {
        if (!surfaceID) { return; }
        var group = document.getElementById(surfaceID);
        if (!group) { return; }
        var host = group.querySelector(':scope > [data-dossierx-surface-host]');
        var tmpl = group.querySelector(':scope > template.dossierx-surface-template');
        if (!host || !tmpl) {
          mountedSurfaceID = surfaceID;
          return;
        }
        if (!host.querySelector('.claim')) {
          host.appendChild(tmpl.content.cloneNode(true));
          if (typeof window.dossierxEnhanceSystemRecord === 'function') {
            window.dossierxEnhanceSystemRecord();
          }
          // The edited-since-approval chips and panels are attached HERE, at
          // mount, and BEFORE the clamps below — not left to the next
          // renderStatusStrip pass.
          //
          // They insert a node above each edited claim's body, so attaching
          // them later moves everything under them. Two things read that
          // height and cannot be allowed to read it early: a deep link has
          // already scrolled to its claim, so the panels appearing afterwards
          // slid the page out from under the sticky header; and the
          // source-note clamps measure scrollHeight against clientHeight, so
          // a mid-measurement insertion decides "show more" from a layout
          // that is about to change. Decorating first means the surface is at
          // its final height before anything measures or scrolls it.
          renderApprovedEdits();
          // Source-note clamps observe live nodes only; re-arm after every
          // first-time mount so a freshly revealed surface is measured.
          if (typeof mountSourceNoteClamps === 'function') {
            mountSourceNoteClamps();
          }
        }
        // The no-template path (an eagerly rendered surface) never enters the
        // block above, and still has claims to decorate. renderApprovedEdits
        // is idempotent — every node it creates is either replaced wholesale
        // or guarded — so the second call costs nothing on the path that did.
        renderApprovedEdits();
        mountedSurfaceID = surfaceID;
      }

      function mountAllSurfaces() {
        // Track claim-groups always soft-mount (see shell.html) and must stay
        // inert until the reader opens them — mounting them here would
        // materialize off-screen track copies and break first-paint measurers
        // (source-note clamps). Only module/facet surfaces are remounted for
        // the small-corpus getElementById witnesses.
        document.querySelectorAll('.claim-group[data-dossierx-surface]').forEach(function (g) {
          if (g.closest('.track-section')) { return; }
          mountSurface(g.getAttribute('data-dossierx-surface') || g.id);
        });
      }

      function showModuleFacet(moduleID, facetID, opts) {
        if (!moduleID) { return; }

        moduleSections.forEach(function (sec) {
          sec.hidden = (sec.id !== moduleID);
        });
        moduleTabs.forEach(function (b) {
          b.classList.toggle('on', b.dataset.target === '#' + moduleID);
        });

        var activeSection = document.getElementById(moduleID);
        if (activeSection) {
          if (!facetID || facetToModule[facetID] !== moduleID) {
            facetID = moduleDefaultFacet[moduleID];
          }
          activeSection.querySelectorAll('.claim-group').forEach(function (g) {
            g.hidden = (g.id !== facetID);
          });
          activeSection.querySelectorAll('.subtab').forEach(function (b) {
            b.classList.toggle('on', b.dataset.target === '#' + facetID);
          });
        }

        // Materialize the active surface BEFORE status-strip filtering and
        // before resolving a deep-linked claim id: getElementById / query
        // cannot see cards that still live only in a <template>. Build-order
        // sections have no surface template and no-op.
        mountSurface(facetID);

        if (lastStatusData) {
          renderStatusStrip(lastStatusData);
        } else {
          positionStatusStrip();
        }

        // A resolved claim id keeps the full claim id as the hash target (not
        // just its facet) so the URL stays deep-linkable/shareable and a refresh
        // lands back on the same card, not just the tab.
        //
        // The graph half of the hash is preserved across every rewrite: this
        // function runs on every tab click and every deep link, and dropping
        // the suffix here would erase whatever filter state the graph pane had
        // just written (see hashGraphSuffix).
        var claimID = opts && opts.claim;
        var hashTarget = claimID || facetID || moduleID;
        var nextHash = '#' + hashTarget + hashGraphSuffix();
        if (!(opts && opts.skipHash) && window.location.hash !== nextHash) {
          history.replaceState(null, '', nextHash);
        }

        if (claimID) {
          var target = document.getElementById(claimID);
          if (target) {
            target.scrollIntoView({ block: 'start' });
            target.classList.add('claim-target-highlight');
            window.setTimeout(function () {
              target.classList.remove('claim-target-highlight');
            }, 2000);
          }
        }
      }

      // ----------------------------------------------------------------
      // The hash carries TWO independent states, separated by '!':
      //
      //     #<reading-view-target>!g=<compact-graph-state>
      //
      // Everything before the '!' is this file's and always was. Everything
      // from the '!' onward belongs to graph-ui.js, which writes it via
      // history.replaceState only. A hash with no '!' behaves exactly as it
      // did before the graph pane existed.
      //
      // The split matters in both directions. resolve() falls back to the
      // FIRST MODULE for anything it does not recognize, so a bare graph-state
      // hash reaching it would silently reset the reader's module — hence
      // hashId() truncating. And showModuleFacet rewrites the hash on every
      // nav, so a rewrite that dropped the suffix would erase the graph state
      // the pane had just written — hence hashGraphSuffix() being appended.
      // ----------------------------------------------------------------

      // hashGraphSuffix returns the graph half of the current hash, '!' and
      // all, or '' when there is none. It reads the raw hash and does not
      // decode: it is re-appended verbatim, so decoding it here would risk
      // re-encoding it differently on the way back out.
      function hashGraphSuffix() {
        var raw = window.location.hash || '';
        var at = raw.indexOf('!');
        return at < 0 ? '' : raw.slice(at);
      }

      function hashId() {
        var raw = (window.location.hash || '').replace(/^#/, '');
        var at = raw.indexOf('!');
        if (at >= 0) { raw = raw.slice(0, at); }
        return decodeURIComponent(raw);
      }

      function showFromHash(opts) {
        var target = resolve(hashId());
        var merged = Object.assign({}, opts, { claim: target.claim });
        showModuleFacet(target.module, target.facet, merged);
      }

      // ================================================================
      // Responsive off-canvas nav drawer (below 860px)
      // ================================================================
      var navToggle = document.getElementById('navToggle');
      var navOverlay = document.getElementById('navOverlay');
      var drawerFocusReturn = null;
      var graphFocusReturn = null;
      var focusRestoreFrame = 0;
      var searchFocusFrame = 0;

      function cancelFocusRestore() {
        if (!focusRestoreFrame) { return; }
        window.cancelAnimationFrame(focusRestoreFrame);
        focusRestoreFrame = 0;
      }

      function cancelSearchFocus() {
        if (!searchFocusFrame) { return; }
        window.cancelAnimationFrame(searchFocusFrame);
        searchFocusFrame = 0;
      }

      function restoreFocus(target, fallbackSelector) {
        cancelFocusRestore();
        function apply() {
          var next = target && target.isConnected ? target : document.querySelector(fallbackSelector || '');
          if (next && typeof next.focus === 'function') { next.focus(); }
        }
        apply();
        // Closing a drawer can hide the previously focused node; Chrome then
        // moves focus to <body> after this turn. Re-apply on the next two
        // frames so Escape lands on the opener instead of the document body.
        focusRestoreFrame = window.requestAnimationFrame(function () {
          apply();
          focusRestoreFrame = window.requestAnimationFrame(function () {
            focusRestoreFrame = 0;
            apply();
          });
        });
      }

      // setDrawer owns body.nav-open (sidebar transform + #navOverlay). Opening
      // the nav closes any open comment panel: THOSE TWO overlays are mutually
      // exclusive, each with its OWN overflow lock, and must never both hold the
      // body scroll-lock at once.
      //
      // The claims graph pane (body.dxg-open, graph-ui.js) is the deliberate
      // exception and nothing here closes it: its lock is ADDITIVE with both of
      // these. Three classes each setting overflow: hidden compose without a
      // release-order hazard — see the z-index ledger in style.css. Do not
      // "fix" that by closing the pane from here. The pane's trigger (#dxgOpen)
      // lives inside <nav id="nav">, so on mobile the drawer has to be open to
      // reach it and nav-open + dxg-open together is a normal state.
      function setDrawer(open, opener) {
        var wasOpen = document.body.classList.contains('nav-open');
        if (open) {
          cancelFocusRestore();
          if (opener) { drawerFocusReturn = opener; }
        }
        if (open) { closeCommentPanel(); }
        document.body.classList.toggle('nav-open', open);
        if (navToggle) { navToggle.setAttribute('aria-expanded', String(open)); }
        if (!open && wasOpen) {
          cancelSearchFocus();
          var sidebarEl = document.getElementById('sidebar') || document.querySelector('.sidebar');
          var active = document.activeElement;
          if (sidebarEl && active && sidebarEl.contains(active) && typeof active.blur === 'function') {
            active.blur();
          }
          restoreFocus(drawerFocusReturn, '#navToggle');
          drawerFocusReturn = null;
        }
      }

      // ================================================================
      // Comment UI (Phase 5) — probe-gated composer + thread actions
      // ================================================================
      //
      // ESCAPING CONTRACT (XSS): innerHTML is assigned EXACTLY ONE kind of value
      // in this whole block — a `body_html` field the server produced with
      // internal/render/markdown (the only markdown renderer), reached via
      // setBodyHTML(). EVERYTHING else — author roles, ids, timestamps, the
      // optimistic placeholder typed into the composer, and every toast/error
      // string — goes through textContent. The optimistic placeholder is
      // deliberately plain text and is replaced by the server's escaped body_html
      // when the write resolves, so a hostile body renders inert before AND after
      // the round-trip.

      var mounted = false;          // true once /api/ping confirms a live serve
      var currentClaimID = null;    // claim id of the open panel, or null
      // part-b skip state: the claim id + JSON signature of the threads currently
      // rendered in the rail. A reload whose freshly-fetched threads match this
      // signature (e.g. an SSE "changed" for a DIFFERENT claim, or a no-op tick)
      // skips the destructive rebuild, so unsent composer/reply/edit text and the
      // reader's focus survive. Only ever read/written in renderPanelFromAPI.
      var lastRenderedClaimID = null;
      var lastRenderedThreadsJSON = null;
      var rail = document.getElementById('commentsPanel');
      var railBody = document.getElementById('commentsRailBody');
      var railTitle = document.getElementById('commentsRailTitle');
      var railSubtitle = document.getElementById('commentsRailSubtitle');
      var railCount = document.getElementById('commentsRailCount');
      var railClose = document.getElementById('commentsRailClose');
      var composerSlot = document.getElementById('commentsComposerSlot');
      var commentsOverlay = document.getElementById('commentsOverlay');
      var toastEl = document.getElementById('commentsToast');
      var toastTimer = null;

      function el(tag, cls) {
        var n = document.createElement(tag);
        if (cls) { n.className = cls; }
        return n;
      }
      function textEl(tag, cls, text) {
        var n = el(tag, cls);
        if (text != null) { n.textContent = text; } // textContent, never innerHTML
        return n;
      }

      // Spec 14 §4.4 (456-0/457-0/459-0), R-J.7: the "N resolved" disclosure
      // gets its own stroked chevron rather than the browser's native
      // disclosure-closed bullet (::-webkit-details-marker is hidden in CSS)
      // or the shared dx-icon sprite (this glyph is drawn at a distinct
      // stroke-width/color the sprite's currentColor convention doesn't
      // carry). It rotates via CSS on `.comments-resolved[open]`. Built via
      // innerHTML (same technique as dxIcon above) rather than
      // document.createElementNS: an explicit SVG namespace URI string trips
      // TestNoNetworkReferencesAnywhereInEngine's http:// scan, and an
      // HTML-parsed <svg> tag gets the right namespace for free.
      function buildResolvedChevron() {
        var wrap = document.createElement('span');
        wrap.innerHTML = '<svg class="comments-resolved-chevron" viewBox="0 0 24 24" width="13" height="13" fill="none" aria-hidden="true"><path d="m9 18 6-6-6-6"></path></svg>';
        return wrap.firstChild;
      }
      // setBodyHTML is the SOLE innerHTML sink. Its only caller passes a server
      // body_html value; centralizing it keeps the escaping contract auditable.
      function setBodyHTML(node, bodyHTML) {
        node.innerHTML = bodyHTML || '';
      }

      function toast(msg) {
        if (!toastEl) { return; }
        toastEl.textContent = msg; // textContent — error text is never trusted markup
        toastEl.hidden = false;
        toastEl.classList.add('comments-toast--show');
        if (toastTimer) { window.clearTimeout(toastTimer); }
        toastTimer = window.setTimeout(function () {
          toastEl.classList.remove('comments-toast--show');
          toastEl.hidden = true;
        }, 4000);
      }

      var ERR_TEXT = {
        rights_denied: 'you can only act on your own comments',
        thread_resolved: 'that thread is already resolved',
        thread_open: 'that thread is already open',
        claim_file_changed: 'the claim changed on disk — reload the page',
        read_only: 'this viewer is read-only',
        empty_body: 'the comment body is empty',
        thread_not_found: 'that thread no longer exists',
        reply_not_found: 'that reply no longer exists',
        claim_not_found: 'that claim no longer exists'
      };
      function errMsg(prefix, err) {
        var code = err && err.code;
        if (code && ERR_TEXT[code]) { return prefix + ': ' + ERR_TEXT[code]; }
        return prefix + '.';
      }

      // ---- API helpers ------------------------------------------------
      function apiGet(path) {
        return fetch(path, { headers: { 'Accept': 'application/json' } }).then(function (res) {
          if (!res.ok) {
            var e = new Error('http ' + res.status);
            e.status = res.status;
            throw e;
          }
          return res.json();
        });
      }
      // apiSend issues a mutating request. POST/PATCH carry a JSON body (and thus
      // a Content-Type the admission middleware requires); DELETE carries none
      // (its actor rides a query param), matching the server handlers. The
      // browser stamps Origin + Sec-Fetch-Site same-origin automatically, so a
      // same-origin fetch passes admission.
      function apiSend(method, path, body) {
        var opts = { method: method, headers: {} };
        if (method === 'POST' || method === 'PATCH') {
          opts.headers['Content-Type'] = 'application/json';
          opts.body = JSON.stringify(body || {});
        }
        return fetch(path, opts).then(function (res) {
          return res.json().catch(function () { return {}; }).then(function (data) {
            if (!res.ok) {
              var e = new Error((data && data.error) ? data.error : ('http ' + res.status));
              e.code = data && data.error;
              e.status = res.status;
              throw e;
            }
            return data;
          });
        });
      }
      function claimPath(claimID) {
        return '/api/claims/' + encodeURIComponent(claimID) + '/comments';
      }

      // ---- chip / card state fan-out ----------------------------------
      // Every claim renders its chip once per DOM copy (overview claims render N
      // times), so state fan-out iterates ALL chips carrying the claim id via
      // data-claim-id and never getElementById — the canonical copy is the only
      // one with an id. Each chip's owning card is reached with closest('.claim').
      function chipsFor(claimID) {
        var out = [];
        document.querySelectorAll('.comment-chip').forEach(function (chip) {
          if (chip.getAttribute('data-claim-id') === claimID) { out.push(chip); }
        });
        return out;
      }
      // 14 §4.6 D14.7 / OD14.3: a THIRD, independent fact from --open/--empty/
      // --resolved — this claim's rail is the one currently showing, distinct
      // from "has open threads" (a claim can have open threads with its rail
      // shut, or an open rail on a claim with zero threads). Driven here
      // rather than folded into --open so the two facts never collide.
      function setChipExpanded(claimID, expanded) {
        chipsFor(claimID).forEach(function (chip) {
          chip.setAttribute('aria-expanded', String(expanded));
          chip.classList.toggle('comment-chip--active', expanded);
        });
      }
      function updateChips(claimID, openCount, totalCount) {
        chipsFor(claimID).forEach(function (chip) {
          var open = openCount > 0;
          var empty = totalCount === 0;
          var shown = open ? openCount : totalCount;
          var countEl = chip.querySelector('.comment-chip-count');
          if (countEl) { countEl.textContent = String(shown); }
          chip.classList.toggle('comment-chip--open', open);
          chip.classList.toggle('comment-chip--empty', empty);
          // The three chip states are mutually exclusive: --open (unsettled
          // threads), --resolved (threads, all settled), --empty (no thread has
          // ever been opened). "no comments" must NOT fall through to --resolved,
          // whose muted "0" would read as "everything raised was settled".
          chip.classList.toggle('comment-chip--resolved', !open && !empty);
          chip.setAttribute('aria-label', empty
            ? 'add the first comment on this claim'
            : (open
              ? 'view ' + openCount + ' open comment thread(s)'
              : 'view ' + totalCount + ' comment thread(s), all resolved'));
          var card = chip.closest('.claim');
          if (card) { card.classList.toggle('claim-card--commented', open); }
          // PROBE-AWARE hiding. An empty chip is hidden ONLY when no live comment
          // API answered — the static file:// case, where there is no composer to
          // reach and the chip would open a rail with nothing in it and no way to
          // add anything. Against a confirmed serve it stays visible, which is what
          // lets the user open the FIRST comment on a claim at all: buildPanel calls
          // updateChips(claimID, 0, 0) on exactly the empty claim whose chip was just
          // clicked, and a bare `totalCount === 0` here would hide that chip out from
          // under the click that opened it.
          var slot = chip.closest('.claim-comments-slot');
          if (slot) { slot.hidden = (empty && !mounted); }
        });
      }
      // syncEmptyChips reveals (or re-hides) every zero-thread chip on the page as
      // a group, matching the server's hidden-by-default markup to what the probe
      // actually found. It runs on probe success and at the end of every
      // initViewer — a Phase 5c fragment swap ships freshly server-rendered chips,
      // hidden again, and only this pass knows the API is live. Chips that carry
      // threads are untouched; their <span> slot is never hidden.
      function syncEmptyChips() {
        document.querySelectorAll('.comment-chip--empty').forEach(function (chip) {
          var slot = chip.closest('.claim-comments-slot');
          if (slot) { slot.hidden = !mounted; }
        });
      }
      // recomputeChipsFromPanel refreshes chip state from the threads currently
      // in the rail (used for optimistic updates before the authoritative
      // refetch): open = threads without the resolved class, total = all.
      function recomputeChipsFromPanel(claimID) {
        if (!railBody) { return; }
        var all = railBody.querySelectorAll('.comment-thread');
        var total = all.length;
        var open = 0;
        all.forEach(function (t) {
          if (!t.classList.contains('comment-thread--resolved')) { open++; }
        });
        updateChips(claimID, open, total);
      }

      // claimTitleFor reads the human title off the claim's own head line
      // (.k[data-claim-id] > .label > .k-title) rather than the slug, for the
      // rail's subtitle (OD14.5, board rule 47E-0: "the rail always names the
      // claim it belongs to"). Falls back to the claim id itself when no
      // matching head is on the page (e.g. a chip inside a collapsed overview
      // whose canonical copy id was stripped elsewhere in the DOM tree).
      function claimTitleFor(claimID) {
        var head = document.querySelector('.k[data-claim-id="' + cssAttr(claimID) + '"]');
        var titleEl = head && head.querySelector('.k-title');
        return (titleEl && titleEl.textContent.trim()) || claimID;
      }

      // ---- elapsed time (R10.1/R10.3/R10.4) ----------------------------
      // Computed client-side from the machine `datetime`, never baked into the
      // generated HTML — a relative phrase baked at generation time is wrong
      // forever. One unit, never two; the exact stamp stays on the element's
      // own datetime/title attributes for a reader who needs it.
      function elapsedPhrase(iso) {
        var then = new Date(iso).getTime();
        if (isNaN(then)) { return ''; }
        var mins = Math.max(0, Math.floor((Date.now() - then) / 60000));
        if (mins < 60) { return (mins <= 1 ? '1 minute ago' : mins + ' minutes ago'); }
        var hours = Math.floor(mins / 60);
        if (hours < 24) { return (hours === 1 ? '1 hour ago' : hours + ' hours ago'); }
        var days = Math.floor(hours / 24);
        return (days === 1 ? '1 day ago' : days + ' days ago');
      }
      function decorateElapsedTime(timeEl) {
        var iso = timeEl.getAttribute('datetime');
        if (!iso) { return; }
        var phrase = elapsedPhrase(iso);
        if (!phrase) { return; }
        timeEl.textContent = phrase;
        if (!timeEl.hasAttribute('title')) { timeEl.setAttribute('title', iso); }
      }
      function decorateElapsedTimes(root) {
        (root || document).querySelectorAll('time.comment-time[datetime]').forEach(decorateElapsedTime);
      }

      // ---- panel open / close -----------------------------------------
      function commentPanelOpen() {
        return document.body.classList.contains('comments-open');
      }
      function openCommentPanel(claimID) {
        currentClaimID = claimID;
        setDrawer(false); // mutual exclusion: opening comments closes the nav
        document.body.classList.add('comments-open');
        if (commentsOverlay) { commentsOverlay.hidden = false; }
        // OD14.5: two lines, "Comments" / "on <title>" — the slug the pre-
        // revamp single line carried ('Comments — ' + claimID) is available on
        // title= hover only, on the rail element itself.
        if (railTitle) { railTitle.textContent = 'Comments'; }
        if (railSubtitle) { railSubtitle.textContent = 'on ' + claimTitleFor(claimID); }
        if (rail) { rail.title = claimID; }
        if (railCount) { railCount.textContent = ''; } // cleared here; renderPanel below fills it in
        if (rail) {
          rail.hidden = false;
          // Desktop: a non-modal complementary right rail. Mobile: a modal
          // bottom sheet. The role/aria-modal pair is set per viewport here
          // because an attribute cannot be media-queried in CSS.
          if (window.matchMedia('(max-width: 860px)').matches) {
            rail.setAttribute('role', 'dialog');
            rail.setAttribute('aria-modal', 'true');
          } else {
            rail.setAttribute('role', 'complementary');
            rail.removeAttribute('aria-modal');
          }
        }
        setChipExpanded(claimID, true);
        renderPanel(claimID);
        if (rail) { rail.focus(); }
      }
      function closeCommentPanel() {
        if (!commentPanelOpen()) { return; }
        document.body.classList.remove('comments-open');
        if (commentsOverlay) { commentsOverlay.hidden = true; }
        if (rail) { rail.hidden = true; }
        if (currentClaimID) { setChipExpanded(currentClaimID, false); }
        currentClaimID = null;
      }

      // ---- panel rendering --------------------------------------------
      function renderPanel(claimID) {
        if (!railBody) { return; }
        if (mounted) {
          renderPanelFromAPI(claimID);
        } else {
          renderPanelReadOnly(claimID);
        }
      }

      // buildReadOnlyNote renders R-J.6/14a state 3's explanation into the
      // composer's slot: "a control with nowhere to POST is not disabled, it
      // is not built" (note 46N-0) — so this re-tenants the slot rather than
      // showing a disabled composer.
      function buildReadOnlyNote() {
        return textEl('p', 'comments-readonly-note',
          'Read only. This viewer was opened as a file, so there is nothing to write back to.');
      }

      // Read-only (file://): clone the baked-in server-rendered .comments-panel
      // threads for this claim into the rail. Cloning a server node is DOM copy,
      // not innerHTML parsing, so it stays within the escaping contract; no
      // composer or action controls are added.
      function renderPanelReadOnly(claimID) {
        railBody.textContent = '';
        var baked = null;
        document.querySelectorAll('.comments-panel').forEach(function (p) {
          if (!baked && p.getAttribute('data-claim-id') === claimID) { baked = p; }
        });
        var list = el('div', 'comments-threads');
        if (baked) {
          baked.querySelectorAll(':scope > .comment-thread, :scope > .comments-resolved').forEach(function (node) {
            list.appendChild(node.cloneNode(true));
          });
        }
        if (!list.children.length) {
          list.appendChild(textEl('p', 'comments-empty', 'No comments.'));
        }
        railBody.appendChild(list);
        decorateElapsedTimes(list);
        if (composerSlot) {
          composerSlot.textContent = '';
          composerSlot.appendChild(buildReadOnlyNote());
        }
        if (railCount) {
          var openN = list.querySelectorAll(':scope > .comment-thread:not(.comment-thread--resolved)').length;
          var totalN = list.querySelectorAll('.comment-thread').length;
          railCount.textContent = String(openN > 0 ? openN : totalN);
        }
      }

      // Serve mode: fetch the authoritative thread list and rebuild the rail with
      // live controls + composer. GET /api/comments returns every thread; we
      // filter to this claim client-side (there is no per-claim GET endpoint).
      function renderPanelFromAPI(claimID) {
        if (!railBody.querySelector('.comments-threads')) {
          railBody.textContent = '';
          railBody.appendChild(textEl('p', 'comments-loading', 'Loading…'));
        }
        apiGet('/api/comments').then(function (data) {
          if (currentClaimID !== claimID) { return; } // panel switched/closed while loading
          var threads = ((data && data.comments) || []).filter(function (c) {
            return c.claim_id === claimID;
          });
          // A JSON signature of exactly what buildPanel would render for this
          // claim. The server serializes deterministically, so two fetches of an
          // unchanged claim produce byte-identical signatures.
          var sig = JSON.stringify(threads);
          // sameClaimRendered: the rail currently shows THIS claim's threads (not a
          // different claim we just switched from, and not an error/loading state).
          var sameClaimRendered = (lastRenderedClaimID === claimID &&
            !!railBody.querySelector('.comments-threads'));
          // (part b) Nothing this claim shows has changed -> skip the destructive
          // rebuild entirely, preserving any dirty draft and the reader's focus.
          if (sameClaimRendered && lastRenderedThreadsJSON === sig) { return; }
          // (part a) A rebuild is needed. Capture dirty drafts to restore across it
          // — but ONLY when the rail already shows this claim, never on a claim
          // switch (that would leak one claim's draft into another's panel).
          var drafts = sameClaimRendered ? captureDirtyDrafts() : emptyDrafts();
          lastRenderedClaimID = claimID;
          lastRenderedThreadsJSON = sig;
          buildPanel(claimID, threads, drafts);
        }).catch(function () {
          if (currentClaimID !== claimID) { return; }
          railBody.textContent = '';
          railBody.appendChild(textEl('p', 'comments-error', 'Could not load comments.'));
          if (composerSlot) {
            composerSlot.textContent = '';
            composerSlot.appendChild(buildComposer(claimID));
          }
        });
      }

      // ---- dirty-draft capture (part a) -------------------------------
      // A "draft" is user-typed, non-empty text sitting in the composer, a reply
      // composer, or an open inline edit form when a rebuild is about to wipe the
      // rail. captureDirtyDrafts snapshots those values keyed so buildPanel can
      // put each one back on the freshly-built node it belongs to. Reply drafts key
      // by owning thread id; edit drafts key by (thread id, reply id) so a root
      // edit and a reply edit never collide. The three source classes are disjoint
      // (.comment-composer / .comment-reply-composer / .comment-edit-form), so a
      // textarea is captured into exactly one bucket.
      function emptyDrafts() { return { composer: null, replies: {}, edits: {} }; }
      function editKey(threadID, replyID) { return String(threadID) + '\n' + String(replyID || ''); }
      function captureDirtyDrafts() {
        var drafts = emptyDrafts();
        if (!railBody) { return drafts; }
        // The root composer now lives in #commentsComposerSlot (R-J.5: pinned,
        // a sibling of the scrolling body), not inside railBody.
        var comp = composerSlot && composerSlot.querySelector('.comment-composer .comment-composer-input');
        if (comp && comp.value.trim()) { drafts.composer = comp.value; }
        railBody.querySelectorAll('.comment-reply-composer .comment-composer-input').forEach(function (ta) {
          if (!ta.value.trim()) { return; }
          var thread = ta.closest('.comment-thread');
          if (thread) { drafts.replies[thread.getAttribute('data-thread-id')] = ta.value; }
        });
        railBody.querySelectorAll('.comment-edit-form .comment-composer-input').forEach(function (ta) {
          if (!ta.value.trim()) { return; }
          var thread = ta.closest('.comment-thread');
          var reply = ta.closest('.comment-reply');
          var tid = thread ? thread.getAttribute('data-thread-id') : '';
          var rid = reply ? reply.getAttribute('data-reply-id') : '';
          drafts.edits[editKey(tid, rid)] = ta.value;
        });
        return drafts;
      }

      function buildPanel(claimID, threads, drafts) {
        drafts = drafts || emptyDrafts();
        railBody.textContent = '';
        var open = threads.filter(function (t) { return t.status === 'open'; });
        var resolved = threads.filter(function (t) { return t.status !== 'open'; });
        var list = el('div', 'comments-threads');
        open.forEach(function (t) { list.appendChild(buildThread(claimID, t, drafts)); });
        if (resolved.length) {
          var details = el('details', 'comments-resolved');
          var summary = el('summary', null);
          summary.appendChild(buildResolvedChevron());
          summary.appendChild(textEl('span', null, resolved.length + ' resolved'));
          details.appendChild(summary);
          resolved.forEach(function (t) { details.appendChild(buildThread(claimID, t, drafts)); });
          list.appendChild(details);
        }
        // An empty claim's rail is a first-class destination since v0.3.0 (every
        // claim carries a chip), so this is no longer a corner case: the reader
        // arrives here precisely to write the FIRST comment. The empty-state line
        // plus buildComposer below are what they find.
        syncEmptyLine(list, open.length > 0 || resolved.length > 0);
        railBody.appendChild(list);
        decorateElapsedTimes(list);
        if (composerSlot) {
          composerSlot.textContent = '';
          composerSlot.appendChild(buildComposer(claimID, drafts.composer));
        }
        updateChips(claimID, open.length, threads.length);
        // R-J.4: the sheet header's mono count is a slot the rail leaves
        // empty (CSS keeps it display:none above 860px) — "shown" mirrors the
        // chip's own open-else-total rule (updateChips above).
        if (railCount) {
          var shown = open.length > 0 ? open.length : threads.length;
          railCount.textContent = String(shown);
        }
        // (part a) Now the whole panel is attached to the DOM, size every textarea
        // that carries a RESTORED draft (composer, reply composers, re-opened edit
        // forms) to fit — a programmatic .value fires no 'input' event, and
        // scrollHeight is only meaningful once laid out, so this post-attach pass is
        // what actually grows a restored draft instead of leaving it clipped to one
        // row (VJF-1). Empty textareas are skipped, staying at their one-row default.
        growRestoredDrafts();
      }

      // syncEmptyLine keeps the rail's empty-state line in step with whether the
      // thread list actually holds anything: hasThreads removes it, !hasThreads
      // ensures exactly one is present. Three callers need it — buildPanel (the
      // rail opened on a claim with no threads), doAdd (the first optimistic
      // thread displaces it), and doAdd's rollback (a failed first write must not
      // leave a rail that is both threadless and silent) — so the invariant lives
      // in one place instead of three inline appends that could drift.
      function syncEmptyLine(list, hasThreads) {
        if (!list) { return; }
        var line = list.querySelector('.comments-empty');
        if (hasThreads) {
          if (line) { line.remove(); }
        } else if (!line) {
          list.appendChild(textEl('p', 'comments-empty', 'No comments yet — add the first one below.'));
        }
      }

      // growRestoredDrafts grows every non-empty composer/reply/edit textarea now
      // attached under the rail to fit its content. It is idempotent and cheap
      // (a handful of textareas), and safe on an empty rail (querySelectorAll on a
      // missing element is guarded). The root composer lives in
      // #commentsComposerSlot (R-J.5), a sibling of railBody, so both are
      // walked rather than railBody alone.
      function growRestoredDrafts() {
        [railBody, composerSlot].forEach(function (root) {
          if (!root) { return; }
          root.querySelectorAll('.comment-composer-input').forEach(function (ta) {
            if (ta.value) { growNow(ta); }
          });
        });
      }

      function buildThread(claimID, t, drafts) {
        drafts = drafts || emptyDrafts();
        var art = el('article', 'comment-thread' + (t.status !== 'open' ? ' comment-thread--resolved' : ''));
        art.setAttribute('data-thread-id', t.id);
        art.appendChild(buildMessage(claimID, t.id, '', t, drafts));
        (t.replies || []).forEach(function (r) {
          var rep = el('div', 'comment-reply');
          rep.setAttribute('data-reply-id', r.id);
          rep.appendChild(buildMessage(claimID, t.id, r.id, r, drafts));
          art.appendChild(rep);
        });
        if (mounted) {
          art.appendChild(buildThreadActions(claimID, t));
          if (t.status === 'open') {
            var replyDraft = drafts.replies[t.id];
            var replyForm = buildReplyComposer(claimID, t.id, replyDraft);
            // OD14.8: the boards draw no composer under a thread at rest —
            // it is a REVEAL target for the bare `Reply` label (see
            // buildThreadActions), not mounted visible. The one exception is
            // a dirty draft carried across a panel rebuild: the reader was
            // already replying, so it stays revealed instead of hiding the
            // text they were mid-sentence on. The element itself is always
            // appended (never conditionally mounted) so a click on `Reply`
            // has something to unhide, and so fix3_test.go's
            // TestReplyRepopulatesOnResolvedConflict still finds the node.
            if (!replyDraft) {
              replyForm.hidden = true;
            }
            art.appendChild(replyForm);
          }
        }
        return art;
      }

      // buildMessage renders one message (thread root when replyID==="" else a
      // reply). The body is the ONLY innerHTML sink (server body_html). Edit and
      // delete controls mount only in serve mode and only on own-role (human)
      // messages — the browser composer's actor is fixed to human; the server
      // enforces the same rights and is the real backstop.
      function buildMessage(claimID, threadID, replyID, m, drafts) {
        var wrap = el('div', 'comment-message');
        var meta = el('div', 'comment-meta');
        meta.appendChild(textEl('span', 'comment-role comment-role--' + m.author, m.author));
        // R10.1/R10.3/R10.4: elapsed, one unit, computed here rather than
        // baked — decorateElapsedTime reads the datetime it sets below and
        // leaves the absolute stamp reachable on title= for the rare reader
        // who needs it.
        var time = el('time', 'comment-time');
        time.setAttribute('datetime', m.created);
        decorateElapsedTime(time);
        meta.appendChild(time);
        if (m.edited) { meta.appendChild(textEl('span', 'comment-edited', '(edited)')); }
        if (mounted && m.author === 'human') {
          var edit = iconButton('comment-action comment-edit', 'pencil', 'Edit comment', function () {
            startEdit(claimID, threadID, replyID, m, body);
          });
          var del = iconButton('comment-action comment-delete', 'x',
            replyID ? 'Delete reply' : 'Delete thread', function () {
              doDelete(claimID, threadID, replyID);
            });
          meta.appendChild(edit);
          meta.appendChild(del);
        }
        wrap.appendChild(meta);
        var body = el('div', 'comment-body');
        setBodyHTML(body, m.body_html);
        wrap.appendChild(body);
        // (part a) Restore an in-progress edit: if this message had a dirty edit
        // draft when the rebuild started, re-open its inline editor prefilled with
        // that draft (only own-role human messages ever carry an editor).
        var editDraft = drafts && drafts.edits ? drafts.edits[editKey(threadID, replyID)] : undefined;
        if (mounted && m.author === 'human' && editDraft != null) {
          startEdit(claimID, threadID, replyID, m, body, editDraft);
        }
        return wrap;
      }

      // OD14.8: Resolve is a labelled pill (tick + word) and Reply a bare
      // accent label, replacing the pre-revamp icon-only button with no Reply
      // control at all. Reply does not build a second composer — the reply
      // composer for an open thread is already mounted (buildThread below) so
      // fix3_test.go's TestReplyRepopulatesOnResolvedConflict can wait for it
      // without a click — it only focuses that existing field.
      function buildThreadActions(claimID, t) {
        var row = el('div', 'comment-thread-actions');
        if (t.status === 'open') {
          row.appendChild(labelledButton('comment-action comment-resolve', 'check', 'Resolve', 'Resolve thread', function () {
            doResolve(claimID, t.id);
          }));
          var reply = textEl('button', 'comment-action-text comment-reply-trigger', 'Reply');
          reply.type = 'button';
          reply.addEventListener('click', function () {
            var art = threadNode(t.id);
            var form = art && art.querySelector('.comment-reply-composer');
            var ta = form && form.querySelector('.comment-composer-input');
            // OD14.8: `Reply` REVEALS the composer buildThread already
            // mounted hidden, rather than building a second one — unhiding
            // is idempotent, so a reply already open just refocuses.
            if (form) { form.hidden = false; }
            if (ta) { ta.focus(); }
          });
          row.appendChild(reply);
        } else {
          row.appendChild(labelledButton('comment-action comment-reopen', 'rotate-ccw', 'Reopen', 'Reopen thread', function () {
            doReopen(claimID, t.id);
          }));
        }
        return row;
      }

      function iconButton(cls, icon, label, onClick) {
        var b = el('button', cls);
        b.type = 'button';
        b.setAttribute('aria-label', label);
        b.appendChild(dxIcon(icon));
        b.addEventListener('click', onClick);
        return b;
      }

      // labelledButton is iconButton plus a visible text label (R-J.7's
      // Resolve/Reopen pill: tick or rotate-ccw glyph + word).
      function labelledButton(cls, icon, text, ariaLabel, onClick) {
        var b = iconButton(cls, icon, ariaLabel, onClick);
        b.appendChild(textEl('span', 'comment-action-label', text));
        return b;
      }

      // growNow sizes a textarea to fit its current .value in one shot. It is the
      // body of autoGrow's input handler, exposed so that a PROGRAMMATIC .value
      // assignment — restoring a dirty draft across an SSE rebuild, or rolling a
      // failed add/reply back into the composer — can grow the field to fit too:
      // setting .value fires no 'input' event, so without an explicit growNow the
      // restored text renders CLIPPED to the one-row default (VJF-1).
      function growNow(ta) {
        ta.style.height = 'auto';
        ta.style.height = ta.scrollHeight + 'px';
      }

      function autoGrow(ta) {
        ta.addEventListener('input', function () { growNow(ta); });
      }

      // R-J.5: field on its own row, then a footer row (caption left, Comment
      // button right, per the OD14.7 caption below) — the root composer's own
      // shape, distinct from the plain textarea+button row a reply/edit form
      // uses (buildReplyComposer, startEdit).
      function buildComposer(claimID, draftValue) {
        var form = el('form', 'comment-composer');
        var ta = el('textarea', 'comment-composer-input');
        ta.setAttribute('aria-label', 'Add a comment');
        ta.rows = 1;
        ta.placeholder = 'Add a comment…';
        autoGrow(ta);
        if (draftValue) { ta.value = draftValue; } // (part a) restore an unsent draft across a rebuild; buildPanel's post-attach growRestoredDrafts() sizes it to fit (scrollHeight needs layout, so it can't grow while detached here)
        var footer = el('div', 'comment-composer-footer');
        // OD14.7: serve-only copy — a static export mounts no composer at all
        // for this caption to sit beside (renderPanelReadOnly never calls
        // buildComposer), so it never needs its own file://-guard here.
        footer.appendChild(textEl('span', 'comment-composer-caption', 'Saved to the served viewer, not to this file.'));
        var btn = textEl('button', 'comment-composer-submit', 'Comment');
        btn.type = 'submit';
        footer.appendChild(btn);
        form.appendChild(ta);
        form.appendChild(footer);
        form.addEventListener('submit', function (e) {
          e.preventDefault();
          var body = ta.value.trim();
          if (!body) { return; }
          doAdd(claimID, body, ta);
        });
        return form;
      }

      function buildReplyComposer(claimID, threadID, draftValue) {
        var form = el('form', 'comment-reply-composer');
        var ta = el('textarea', 'comment-composer-input');
        ta.setAttribute('aria-label', 'Reply to thread');
        ta.rows = 1;
        ta.placeholder = 'Reply…';
        autoGrow(ta);
        if (draftValue) { ta.value = draftValue; } // (part a) restore an unsent reply across a rebuild; buildPanel's post-attach growRestoredDrafts() sizes it to fit (scrollHeight needs layout, so it can't grow while detached here)
        // Labelled "Send", not "Reply": the bare `Reply` label that reveals
        // this form (buildThreadActions) stays on screen once it is open, so
        // a second, submit-side "Reply" would be a duplicate control name
        // for two different actions (reveal vs. send) — no board draws this
        // field, so the boards don't pin a literal for it.
        var btn = textEl('button', 'comment-composer-submit', 'Send');
        btn.type = 'submit';
        form.appendChild(ta);
        form.appendChild(btn);
        form.addEventListener('submit', function (e) {
          e.preventDefault();
          var body = ta.value.trim();
          if (!body) { return; }
          doReply(claimID, threadID, body, ta);
        });
        return form;
      }

      // optimisticMessage builds a plain-text placeholder message. The body is
      // set with textContent, so even a hostile body renders as inert text until
      // the server's escaped body_html replaces the whole panel on refresh.
      function optimisticMessage(body) {
        var wrap = el('div', 'comment-message');
        var meta = el('div', 'comment-meta');
        meta.appendChild(textEl('span', 'comment-role comment-role--human', 'human'));
        meta.appendChild(textEl('span', 'comment-pending', 'sending…'));
        wrap.appendChild(meta);
        wrap.appendChild(textEl('div', 'comment-body', body));
        return wrap;
      }

      // ---- mutating ops (optimistic + rollback + toast) ---------------
      function threadNode(tid) {
        if (!railBody) { return null; }
        var found = null;
        railBody.querySelectorAll('.comment-thread').forEach(function (a) {
          if (!found && a.getAttribute('data-thread-id') === tid) { found = a; }
        });
        return found;
      }

      function doAdd(claimID, body, ta) {
        var list = railBody.querySelector('.comments-threads');
        var placeholder = el('article', 'comment-thread comment-thread--optimistic');
        placeholder.appendChild(optimisticMessage(body));
        if (list) {
          // On a claim with no threads yet the list holds only the empty-state
          // line; drop it as the first optimistic thread goes in, so the rail
          // never reads "No comments yet." directly above a comment. The catch
          // below puts it back if the write fails, since nothing else rebuilds
          // the list on that path.
          syncEmptyLine(list, true);
          list.insertBefore(placeholder, list.firstChild);
        }
        ta.value = '';
        ta.style.height = 'auto';
        recomputeChipsFromPanel(claimID);
        apiSend('POST', claimPath(claimID), { as: 'human', body: body })
          .then(function () { renderPanelFromAPI(claimID); })
          .catch(function (err) {
            if (placeholder.parentNode) { placeholder.parentNode.removeChild(placeholder); }
            if (list) { syncEmptyLine(list, list.querySelector('.comment-thread') !== null); }
            recomputeChipsFromPanel(claimID);
            // The input was cleared optimistically when the placeholder went in;
            // a failed write must restore the user's text, never discard it — and
            // grow it back to fit (a bare .value assignment fires no input event,
            // so without growNow a multi-line draft renders clipped to one row).
            ta.value = body;
            growNow(ta);
            toast(errMsg('Could not add comment', err));
          });
      }

      function doReply(claimID, tid, body, ta) {
        var art = threadNode(tid);
        var placeholder = null;
        if (art) {
          placeholder = el('div', 'comment-reply comment-reply--optimistic');
          placeholder.appendChild(optimisticMessage(body));
          var actions = art.querySelector('.comment-thread-actions');
          if (actions) { art.insertBefore(placeholder, actions); } else { art.appendChild(placeholder); }
        }
        ta.value = '';
        ta.style.height = 'auto';
        apiSend('POST', '/api/claims/' + encodeURIComponent(claimID) + '/comments/' + encodeURIComponent(tid) + '/replies', { as: 'human', body: body })
          .then(function () { renderPanelFromAPI(claimID); })
          .catch(function (err) {
            if (placeholder && placeholder.parentNode) { placeholder.parentNode.removeChild(placeholder); }
            // Restore the user's text (cleared optimistically above) so a failed
            // reply is not silently lost — and grow it back to fit (a bare .value
            // assignment fires no input event, so without growNow a multi-line
            // draft renders clipped to one row).
            ta.value = body;
            growNow(ta);
            toast(errMsg('Could not reply', err));
          });
      }

      function doResolve(claimID, tid) {
        var art = threadNode(tid);
        if (art) { art.classList.add('comment-thread--resolved'); }
        recomputeChipsFromPanel(claimID);
        apiSend('POST', '/api/claims/' + encodeURIComponent(claimID) + '/comments/' + encodeURIComponent(tid) + '/resolve', { as: 'human' })
          .then(function () { renderPanelFromAPI(claimID); })
          .catch(function (err) {
            if (art) { art.classList.remove('comment-thread--resolved'); }
            recomputeChipsFromPanel(claimID);
            toast(errMsg('Could not resolve thread', err));
          });
      }

      function doReopen(claimID, tid) {
        var art = threadNode(tid);
        if (art) { art.classList.remove('comment-thread--resolved'); }
        recomputeChipsFromPanel(claimID);
        apiSend('POST', '/api/claims/' + encodeURIComponent(claimID) + '/comments/' + encodeURIComponent(tid) + '/reopen', { as: 'human' })
          .then(function () { renderPanelFromAPI(claimID); })
          .catch(function (err) {
            if (art) { art.classList.add('comment-thread--resolved'); }
            recomputeChipsFromPanel(claimID);
            toast(errMsg('Could not reopen thread', err));
          });
      }

      function doDelete(claimID, tid, rid) {
        var wholeThread = !rid;
        if (wholeThread && !window.confirm('Delete this whole comment thread? This cannot be undone.')) {
          return;
        }
        var node;
        if (wholeThread) {
          node = threadNode(tid);
        } else {
          var art = threadNode(tid);
          node = art ? art.querySelector('.comment-reply[data-reply-id="' + cssAttr(rid) + '"]') : null;
        }
        if (node) { node.classList.add('comment-thread--deleting'); }
        var path = '/api/claims/' + encodeURIComponent(claimID) + '/comments/' + encodeURIComponent(tid);
        if (rid) { path += '?reply=' + encodeURIComponent(rid); }
        apiSend('DELETE', path)
          .then(function () {
            if (node && node.parentNode) { node.parentNode.removeChild(node); }
            recomputeChipsFromPanel(claimID);
            renderPanelFromAPI(claimID);
          })
          .catch(function (err) {
            if (node) { node.classList.remove('comment-thread--deleting'); }
            toast(errMsg('Could not delete', err));
          });
      }

      // cssAttr escapes a value for use inside an attribute selector's quoted
      // string (only backslash and the double-quote need escaping there). Reply
      // ids are engine-minted "r-<hex>" tokens, so this is defensive.
      function cssAttr(v) {
        return String(v).replace(/\\/g, '\\\\').replace(/"/g, '\\"');
      }

      // startEdit swaps a message body for an inline textarea prefilled with the
      // RAW body (server-provided m.body), and PATCHes on save. Cancel restores
      // the original rendered body node; a successful save rebuilds the panel.
      function startEdit(claimID, tid, rid, m, bodyNode, initialValue) {
        if (!bodyNode || !bodyNode.parentNode) { return; }
        var form = el('form', 'comment-edit-form');
        var ta = el('textarea', 'comment-composer-input');
        ta.setAttribute('aria-label', 'Edit comment');
        // Prefill with an explicit initialValue when re-opening after a rebuild
        // (part a — the captured in-progress draft), else the message's raw body.
        ta.value = (initialValue != null ? initialValue : (m.body || ''));
        autoGrow(ta);
        var save = textEl('button', 'comment-composer-submit', 'Save');
        save.type = 'submit';
        var cancel = textEl('button', 'comment-action-text', 'Cancel');
        cancel.type = 'button';
        form.appendChild(ta);
        form.appendChild(save);
        form.appendChild(cancel);
        bodyNode.replaceWith(form);
        // Grow to fit the prefilled body/draft AFTER the form is in the DOM — a
        // .value assignment fires no 'input' event, and scrollHeight is only
        // meaningful once laid out (a live edit-click attaches here; an edit form
        // re-opened during a rebuild is instead grown by buildPanel's post-attach
        // growRestoredDrafts, since this subtree is still detached then).
        growNow(ta);
        ta.focus();
        cancel.addEventListener('click', function () { form.replaceWith(bodyNode); });
        form.addEventListener('submit', function (e) {
          e.preventDefault();
          var newBody = ta.value.trim();
          if (!newBody) { return; }
          var path = '/api/claims/' + encodeURIComponent(claimID) + '/comments/' + encodeURIComponent(tid);
          if (rid) { path += '?reply=' + encodeURIComponent(rid); }
          apiSend('PATCH', path, { as: 'human', body: newBody })
            .then(function () {
              // Close the editor BEFORE the authoritative rebuild so the rebuild's
              // draft-capture does not mistake a just-saved edit for one still in
              // progress and re-open it; the rebuild renders the saved body.
              if (form.parentNode) { form.replaceWith(bodyNode); }
              renderPanelFromAPI(claimID);
            })
            .catch(function (err) {
              // Keep the edit form open with the user's revision (do NOT revert to
              // the rendered body) so a failed edit does not discard the text.
              toast(errMsg('Could not edit', err));
            });
        });
      }

      // ---- status strip (lock-ledger integrity + lint) ------------------
      //
      // WHY THIS EXISTS AT ALL. v0.3.0's promise is that a LOCKED claim cannot
      // change without an approval record, and everything that enforces it —
      // the lock ledger, the gate in internal/check, "check --staged", the
      // pre-commit hook, CI — speaks to an AGENT through a JSON envelope or a
      // terminal. The human's only surface is this page. So a project whose
      // ledger had been tampered with rendered here exactly like a sound one:
      // every claim readable, every lock icon in place, nothing said. The agent
      // could see the refusal and the human could not, which inverts who the
      // promise is for.
      //
      // The server-side half is deliberate and stays that way: check.Status
      // REPORTS the gate where check.Run REFUSES, so a disputed ledger never
      // blanks the viewer — a reader must be able to open the very claim that
      // is under dispute and read it. This strip is the other half of that
      // bargain. The page keeps rendering; it just stops rendering silently.
      //
      // PRESENTATION follows the lint-error shape (rule/name, claim id,
      // message, one row each) because a reader already knows how to scan it —
      // but integrity findings are a separate, first group with their own
      // heading, and they alone set the strip's alarm state. Folding them into
      // the lint list would make "nobody approved this lock" look like "this
      // claim has no edges".

      var stripEl = document.getElementById('statusStrip');
      // retry fix 21 (this lane's third attempt): #statusStripNote (the
      // "Critical 0 · Needs you 3 · ..." tally) is REMOVED per the
      // coordinator's ruling — no 02/03/04 board draws it, the chips
      // already carry every count it repeated — and dropped from shell.html
      // along with its element lookup here.
      //
      // THE HEAD HAS TWO FORMS, ONE PER WIDTH, and both are written on every
      // paint. #statusStripToggle is the desktop band — one summary sentence
      // with "Show issues" beside it — and #statusStripCard is the phone form
      // (Paper EH1-0), a card with one ROW PER FINDING. CSS shows one; see
      // shell.html for why the choice is a media query and not a width read
      // in here.
      //
      // Both are filled unconditionally rather than behind a matchMedia
      // check. The cost is a few DOM nodes nobody sees; the alternative is a
      // resize listener and a breakpoint duplicated out of the stylesheet,
      // which is one more thing to keep in step for no gain.
      var stripToggle = document.getElementById('statusStripToggle');
      var stripTitle = document.getElementById('statusStripTitle');
      var stripCard = document.getElementById('statusStripCard');
      var stripCardCount = document.getElementById('statusStripCardCount');
      var stripFindings = document.getElementById('statusStripFindings');
      var stripBody = document.getElementById('statusStripBody');
      var lastStatusData = null;

      function countLabel(n, word) {
        return n + ' ' + word + (n === 1 ? '' : 's');
      }

      function humanRule(name) {
        return String(name || '').split('-').filter(Boolean).map(function (part) {
          return part.charAt(0).toUpperCase() + part.slice(1);
        }).join(' ');
      }

      function activeFacetClaimIDs() {
        var ids = Object.create(null);
        // The Build order section holds diagrams, not claim cards: with it
        // active the id set is empty and only project-wide findings render.
        var section = document.querySelector('.module-section:not([hidden]):not(.build-order-section)');
        var group = section && section.querySelector(':scope > .claim-group:not([hidden])');
        if (!group) { return ids; }
        var collect = function (root) {
          root.querySelectorAll('.claim').forEach(function (claim) {
            var id = claim.dataset.claimId || claim.id;
            if (id) { ids[id] = true; }
          });
        };
        collect(group);
        var tmpl = group.querySelector(':scope > template.dossierx-surface-template');
        if (tmpl && tmpl.content) { collect(tmpl.content); }
        return ids;
      }

      function findingsForActiveFacet(findings, claimIDs) {
        return findings.filter(function (finding) {
// A finding without a claim id is project-wide and therefore applies
// in every facet. Claim-specific findings belong only beside the
// facet that contains that claim.
          return !finding.claim_id || claimIDs[finding.claim_id] === true;
        });
      }

      var STATUS_SEVERITIES = [
        { id: 'critical', label: 'Critical' },
        { id: 'needs_you', label: 'Needs you' },
        { id: 'blocker', label: 'Blocker' },
        { id: 'check', label: 'Check' },
        { id: 'later', label: 'Later' }
      ];
      var stripSeverityFilter = '';

      function addStatusGroup(groups, key, fields) {
        if (!groups[key]) {
          groups[key] = {
            key: key,
            severity: fields.severity,
            title: fields.title,
            kind: fields.kind || '',
            // retry fix 20 / R08.2 / 04 §8 item 3: `origin` is the finding's
            // KIND — which array it came from — never its severity. Before
            // this fix the APPROVAL RECORD group was selected by
            // `severity === 'critical'`, which only happened to work because
            // collectStatusGroups was the sole producer of that severity; a
            // future critical-severity LINT finding would have been filed
            // under APPROVAL RECORD by accident, and § 8 item 1's Critical
            // ROWS could never sit in their own module group. Only the
            // ledger call site below passes 'ledger'; every other call site
            // defaults to 'readiness'.
            origin: fields.origin || 'readiness',
            // ownerModuleClaimID is the claim whose MODULE the Issues screen
            // groups this finding under (04 §2: "which module do I chase").
            // For a readiness blocker/review cause that is the dependency
            // the reader must go fix, not the claim it is blocking — those
            // two call sites pass it explicitly; every other kind of finding
            // is about its own claim, so it falls back to fields.claimID.
            ownerModuleClaimID: fields.ownerModuleClaimID || fields.claimID || '',
            claimIDs: {},
            count: 0
          };
        }
        var group = groups[key];
        if (fields.claimID) { group.claimIDs[fields.claimID] = true; }
        group.count += 1;
        return group;
      }

      function collectStatusGroups(readiness, claimIDs, ledger, lintErrors, lintWarnings, checkClaimIDs) {
        var groups = {};
        ledger.forEach(function (finding) {
          addStatusGroup(groups, 'critical:' + (finding.rule || 'ledger') + ':' + (finding.claim_id || ''), {
            severity: 'critical',
            title: humanRule(finding.rule) || 'Approval record issue',
            kind: finding.rule || 'ledger',
            origin: 'ledger',
            claimID: finding.claim_id
          });
        });
        Object.keys(readiness || {}).sort().forEach(function (id) {
          if (claimIDs[id] !== true) { return; }
          var assessment = readiness[id] || {};
          (assessment.dependency_conditions || assessment.conditions || []).forEach(function (condition) {
            var dep = condition.dependency_id || (condition.path || [])[(condition.path || []).length - 1] || condition.kind || 'dependency';
            addStatusGroup(groups, 'blocker:' + (condition.kind || 'dependency') + ':' + dep, {
              severity: 'blocker',
              title: readinessFactLabel(condition, 'condition', id),
              kind: condition.kind || 'dependency',
              claimID: id,
              ownerModuleClaimID: dep
            });
          });
          (assessment.review_causes || assessment.causes || []).forEach(function (cause) {
            var owner = cause.direct ? id : (cause.dependency_id || id);
            addStatusGroup(groups, 'needs_you:' + (cause.kind || 'review') + ':' + owner, {
              severity: 'needs_you',
              title: readinessFactLabel(cause, 'cause', id),
              kind: cause.kind || 'review',
              claimID: id,
              ownerModuleClaimID: owner
            });
          });
        });
        checkClaimIDs.forEach(function (id) {
          addStatusGroup(groups, 'check:conformance:' + id, {
            severity: 'check',
            title: 'Implementation checks are not ready',
            kind: 'conformance',
            claimID: id
          });
        });
        lintErrors.forEach(function (finding) {
          addStatusGroup(groups, 'check:lint:' + (finding.lint || 'lint'), {
            severity: 'check',
            title: humanRule(finding.lint) || 'Check issue',
            kind: finding.lint || 'lint',
            claimID: finding.claim_id
          });
        });
        lintWarnings.forEach(function (finding) {
          addStatusGroup(groups, 'later:lint:' + (finding.lint || 'lint'), {
            severity: 'later',
            title: humanRule(finding.lint) || 'Later warning',
            kind: finding.lint || 'lint',
            claimID: finding.claim_id
          });
        });
        return Object.keys(groups).sort().map(function (key) { return groups[key]; });
      }

      function statusGroupClaimCount(group) {
        return Object.keys(group.claimIDs).length;
      }

      function uniqueClaimCount(groups) {
        var ids = Object.create(null);
        groups.forEach(function (group) {
          Object.keys(group.claimIDs).forEach(function (id) { ids[id] = true; });
        });
        return Object.keys(ids).length;
      }

      function countSeverity(groups, id) {
        return uniqueClaimCount(groups.filter(function (group) { return group.severity === id; }));
      }

      function blockerHeadline(groups) {
        var blockers = groups.filter(function (group) { return group.severity === 'blocker'; });
        if (!blockers.length) { return ''; }
        var byKind = {};
        blockers.forEach(function (group) {
          if (!byKind[group.kind]) { byKind[group.kind] = Object.create(null); }
          Object.keys(group.claimIDs).forEach(function (id) { byKind[group.kind][id] = true; });
        });
        var topKind = Object.keys(byKind).sort(function (a, b) {
          return Object.keys(byKind[b]).length - Object.keys(byKind[a]).length;
        })[0];
        var n = Object.keys(byKind[topKind]).length;
        var activeIDs = activeFacetClaimIDs();
        var total = Object.keys(activeIDs).length;
        var activeTab = document.querySelector('.module-section:not([hidden]) > .sub-nav .subtab.on .sec-tab__label');
        var facetLabel = activeTab ? activeTab.textContent.trim().toLowerCase() + ' ' : '';
        var subject = (total > 0 && n === total ? 'All ' : '') + n + ' ' + facetLabel + 'claim' + (n === 1 ? '' : 's');
        if (topKind === 'dependency_unapproved') {
          var activeModule = document.querySelector('.module-section:not([hidden])');
          var outside = !!activeModule && blockers.filter(function (group) {
            return group.kind === topKind;
          }).every(function (group) {
            var owner = ownerModuleID(group);
            return owner && owner !== activeModule.id;
          });
          return subject + (n === 1 ? ' is' : ' are') + ' blocked by unapproved dependencies' + (outside ? ' outside this module' : '');
        }
        return subject + (n === 1 ? ' is' : ' are') + ' blocked';
      }

      function conformanceNotReadyIDs(claimIDs) {
        var ids = [];
        document.querySelectorAll('.claim-conformance[data-implementation-ready="false"]').forEach(function (panel) {
          var id = panel.getAttribute('data-claim-id');
          if (id && claimIDs[id] === true) { ids.push(id); }
        });
        return ids;
      }

      // retry fix 23/27 (this lane's third attempt, 04 §4.8 "Text column" /
      // dot): the row is exactly three flex children — the dot, a text
      // column carrying the title and (for a single-claim group) its
      // demoted slug, and the fixed-width count slot — rather than three
      // loose wrapping siblings. See style.css's .status-finding-dot /
      // .status-finding-text for the layout this DOM shape enables.
      function renderStatusGroup(group) {
        var row = el('li', 'status-finding status-finding--group');
        row.setAttribute('data-severity', group.severity);
        row.appendChild(el('span', 'status-finding-dot'));
        var text = el('span', 'status-finding-text');
        text.appendChild(textEl('span', 'status-finding-rule', group.title || 'Issue'));
        var ids = Object.keys(group.claimIDs).sort();
        // Paper 1KX-0: every row carries a second, demoted line under its
        // title holding the dotted claim id of the claim the reader must go
        // FIX — not one of the claims it blocks. For a blocker/needs_you
        // group that claim is group.ownerModuleClaimID (the dependency_id or
        // review owner the group key is built from, so it is exact for the
        // whole group however many claims it blocks). Lint groups key on the
        // lint rule rather than a claim, so their ownerModuleClaimID is only
        // whichever claim created them; those keep the narrower single-claim
        // reading. Mobile hides this line entirely — Paper 5XH-0 draws no
        // slug at phone width.
        var ownerID = (group.severity === 'blocker' || group.severity === 'needs_you')
          ? group.ownerModuleClaimID
          : (ids.length === 1 ? ids[0] : '');
        if (ownerID) {
          text.appendChild(textEl('span', 'status-finding-claim', ownerID));
        }
        row.appendChild(text);
        // retry fix 7: 04 §4.8 "Count label" / §6 "Row count" promotes the
        // phrase INTO the count slot as a bare `N claims`, never `blocks N
        // claim(s)` — that longer phrase was written for a full-width third
        // line, which the fixed 104px slot (style.css) no longer is.
        var n = ids.length || group.count;
        row.appendChild(textEl('span', 'status-finding-msg', n + ' claim' + (n === 1 ? '' : 's')));
        return row;
      }

// The status endpoint is project-wide, but the reader's orientation is
// module-local. Place the notice after the active facet controls, where it
// can be seen before reading without interrupting the module → facet →
// action sequence. The section fallback covers the brief
// interval before system-record.js has enhanced a fresh SSE fragment.
      function positionStatusStrip() {
        if (!stripEl) { return; }
        // Never the Build order section: it falls through to the .content-area
        // branch rather than taking the strip as the diagrams' first child.
        var section = document.querySelector('.module-section:not([hidden]):not(.build-order-section)');
        if (!section) {
          var content = document.querySelector('.content-area');
          if (content && stripEl.parentNode !== content) { content.insertBefore(stripEl, content.firstChild); }
          return;
        }
        var canvas = section.querySelector(':scope > .reading-canvas:not([hidden])');
        if (canvas) {
          if (canvas.firstElementChild !== stripEl) { canvas.insertBefore(stripEl, canvas.firstChild); }
          return;
        }
        var subNav = section.querySelector(':scope > .sub-nav');
        var header = section.querySelector(':scope > .system-record-head, :scope > .track-head');
        if (subNav) {
          if (subNav.nextElementSibling !== stripEl) { subNav.insertAdjacentElement('afterend', stripEl); }
        } else if (header) {
          if (header.nextElementSibling !== stripEl) { header.insertAdjacentElement('afterend', stripEl); }
        } else if (section.firstElementChild !== stripEl) {
          section.insertBefore(stripEl, section.firstChild);
        }
      }
      window.dossierxPositionStatusStrip = positionStatusStrip;
      // Exposed for the same reason dossierxPositionStatusStrip is: a browser
      // test needs a way to paint an APPROVAL RECORD verdict (04 §8 item 3 /
      // R08.2) without corrupting a real lock ledger to produce one, since
      // ledger_findings only ever arrives from a live GET /api/status.
      window.dossierxRenderStatusStrip = function (data) { renderStatusStrip(data); };

      // ownerModuleID (04 §2, §9 item 6) resolves a group's ownerModuleClaimID
      // to the .module-section id that owns it, via the same claimToFacet /
      // facetToModule maps initViewer() already builds for deep linking — read
      // only, never written here, exactly as readinessClaimLabel already reads
      // .claim/.k .label elsewhere in this file. A claim id the maps do not
      // know (a project-wide finding with no claim_id at all) resolves to ''
      // and moduleLabel below renders it as the "Other" catch-all group.
      function ownerModuleID(group) {
        var facetID = claimToFacet[group.ownerModuleClaimID];
        return facetID ? facetToModule[facetID] : '';
      }

      // moduleLabel reads the sidebar's own .sec-tab label text for a module
      // id — the label a lane agent must not duplicate as a second literal,
      // per tokens.md's single-source rule. R09.6 already forbids adding an
      // Issues nav tab, so this is read-only lookup, never a written one.
      function moduleLabel(moduleID) {
        if (!moduleID) { return 'Other'; }
        var tab = document.querySelector('.sec-tab[data-target="#' + moduleID.replace(/"/g, '') + '"] .sec-tab__label');
        return tab ? (tab.textContent || '').trim() : moduleID;
      }

      // findingGroup renders one heading + its finding list. `note`, when
      // given, is 04 §6's `blocks <N> of <M> claims here` weight phrase — a
      // second span so it can be styled and (§5 M6) stacked separately from
      // the module name, never concatenated into one string.
      function findingGroup(heading, rows, note) {
        var group = el('div', 'status-group');
        var head = el('p', 'status-group-head');
        head.appendChild(textEl('span', 'status-group-head-name', heading));
        // retry fix 25 (this lane's third attempt, 04 §4.7 "Filler rule"):
        // the hairline only exists between a name and a weight phrase — a
        // heading with no `note` (APPROVAL RECORD) gets neither.
        if (note) {
          head.appendChild(el('span', 'status-group-head-rule'));
          head.appendChild(textEl('span', 'status-group-head-note', note));
        }
        group.appendChild(head);
        var list = el('ul', 'status-finding-list');
        rows.forEach(function (r) { list.appendChild(r); });
        group.appendChild(list);
        return group;
      }

      // readinessClaimLabel derives a claim's reading-view title from its own
      // rendered .k .label, for use as a dependency's readable name here.
      // G18(b): the label's three children are .k-title, .pill and .k-id
      // (card.html); the pill and the collapse chevron were already
      // stripped, but .k-id (the mono id line) was not, so the machine id
      // used to leak into every derived readiness title. All three are
      // stripped now, so the id never rides along with a readable label.
      function readinessClaimLabel(id) {
        var card = id ? document.getElementById(id) : null;
        var label = card && card.querySelector('.k .label');
        if (label) {
          var copy = label.cloneNode(true);
          copy.querySelectorAll('.pill, .k-id, .claim-collapse-chevron').forEach(function (node) { node.remove(); });
          var rendered = (copy.textContent || '').replace(/\s+/g, ' ').trim();
          if (rendered) { return rendered; }
        }
        var part = String(id || 'unknown dependency').split('.').pop();
        return part.replace(/[-_]+/g, ' ').replace(/\b\w/g, function (c) { return c.toUpperCase(); });
      }

      function readinessFactLabel(record, type, rootID) {
        var path = record.path || [];
        var targetID = record.dependency_id || path[path.length - 1] || rootID;
        var target = readinessClaimLabel(targetID);
        var labels = type === 'condition' ? {
          dependency_unapproved: target + ' is not locally approved',
          missing_dependency: target + ' is missing',
          unreadable_dependency: target + ' cannot be read',
          retired_dependency: target + ' is retired',
          unknown_historical_baseline: 'The historical approval baseline is unknown',
          dependency_cycle: 'A required dependency cycle reaches ' + target
        } : {
          direct_dependency_change: target + ' changed after approval',
          upstream_dependency_review: target + ' requires upstream review',
          own_thread: 'This claim has an open review thread',
          own_flag: 'This claim has an active review flag',
          approval_content_drift: 'This claim changed after approval',
          approval_missing: 'This claim has no approval record',
          approval_released: 'This claim\'s approval was released',
          approval_unknown: 'This claim\'s approval state is unknown',
          // Deliberately not "changed after approval" (approval_content_drift's
          // wording): that names a LOCKED claim whose bytes no longer match a
          // STANDING approval, which is the tamper finding. This names the
          // honest, intended act — unlock, rewrite, re-lock — caught mid-way.
          unapproved_edit: 'This claim was approved, then rewritten'
        };
        return labels[record.kind] || String(record.kind || 'Readiness obstacle').replace(/_/g, ' ');
      }

      // readinessTargetID is the SAME "final dependency" identity
      // readinessFactLabel already reads — the id a blocker row's target
      // slug names and the id 06 §8 item 11 requires stays exactly one hop
      // long even when the record's own representative path is longer
      // (upstream) or cyclical.
      function readinessTargetID(record, rootID) {
        var path = record.path || [];
        return record.dependency_id || path[path.length - 1] || rootID;
      }

      // readinessHopLabel is the PER-BLOCKER-ROW pill: "direct - 1 hop" /
      // "upstream - 2 hops" (06 §4.4, §6). readinessHopCount is the same
      // arithmetic as a bare number, for module grouping/sorting and for
      // the module row's OWN "nearest N hop(s)" wording (06 §4.4 / 03
      // §4.10), which never carries "direct"/"upstream".
      function readinessHopLabel(record, type) {
        var path = record.path || [];
        var hops = Math.max(0, path.length - 1);
        var relation = (type === 'cause' && record.direct) || hops <= 1 ? 'direct' : 'upstream';
        return relation + ' · ' + hops + ' ' + (hops === 1 ? 'hop' : 'hops');
      }

      function readinessHopCount(record, type) {
        if (type === 'cause' && record.direct) { return 0; }
        var path = record.path || [];
        return Math.max(0, path.length - 1);
      }

      function readinessNearestHopLabel(hops) {
        return 'nearest ' + hops + ' ' + (hops === 1 ? 'hop' : 'hops');
      }

      // readinessModuleKey groups a fact by "the module that owns the fix"
      // (06 §2/§7.6: NOT the representative-route claim the old
      // readinessRouteKey grouped by). A dependency condition's fix lives
      // wherever its unapproved target lives; a direct review cause's fix
      // is this claim's own review, so it groups under this claim's own
      // module. claimToFacet/facetToModule/moduleLabel are the same maps
      // and function the status strip already builds (initViewer, above) —
      // read-only here, never duplicated.
      function readinessModuleKey(item, rootID) {
        var targetID = (item.type === 'cause' && item.record.direct) ? rootID : readinessTargetID(item.record, rootID);
        var facetID = claimToFacet[targetID];
        return facetID ? facetToModule[facetID] : '';
      }

      // readinessGroupByModule sorts groups per 06 §9 open decision 2 (the
      // board's own tie-break, not stated in the rules): nearest hop
      // distance ascending, then blocker count descending, then module name
      // ascending. Only the FIRST group is opened by the caller.
      function readinessGroupByModule(facts, rootID) {
        var byKey = {};
        var groups = [];
        facts.forEach(function (item) {
          var key = readinessModuleKey(item, rootID);
          if (!Object.prototype.hasOwnProperty.call(byKey, key)) {
            byKey[key] = { key: key, label: moduleLabel(key), items: [], minHops: Infinity };
            groups.push(byKey[key]);
          }
          var group = byKey[key];
          group.items.push(item);
          var hops = readinessHopCount(item.record, item.type);
          if (hops < group.minHops) { group.minHops = hops; }
        });
        groups.sort(function (a, b) {
          if (a.minHops !== b.minHops) { return a.minHops - b.minHops; }
          if (b.items.length !== a.items.length) { return b.items.length - a.items.length; }
          return a.label < b.label ? -1 : (a.label > b.label ? 1 : 0);
        });
        return groups;
      }

      // readinessBlockerRow builds ONE <li>: a title, an OPTIONAL authored
      // detail line, a hop pill, and a dependency path of EXACTLY two slugs
      // (06 §2/§8 item 11/§8 item 12 — never truncated with an ellipsis,
      // since a slug is machine identity). R09.8: the row leads with the
      // title, never the claim id; the id appears only inside the path,
      // where it is machine identity.
      //
      // RETRY FIX (wave-B2 fix list item 1, IMPLEMENTED WITH A DISPUTE
      // against the probe evidence): record.detail is
      // rendered only for an `own_flag` review cause. 06 §8 item 3 and
      // R09.8 ("the authored detail is not one of the eight demotions;
      // nothing is deleted from the data") require the reviewer's own
      // review-flag reason to be readable in the row itself, not only
      // inside the closed Raw diagnostics <pre>.
      //
      // The fix list's own suggested test — "render when detail is present
      // AND its text is not a restatement of the title (compare against
      // readinessFactLabel's output)" — does NOT hold against
      // internal/readiness/readiness.go, which this lane reads as its
      // authoritative source (a probe of the DATA, not of a rendering).
      // Every DependencyCondition kind's `Detail` is a FIXED, engine-
      // generated string with no per-instance information: dependency_
      // unapproved's is literally the string constant "required dependency
      // is not locally approved" (readiness.go:292), missing_dependency's
      // is "required dependency is missing" (:361), retired_dependency's is
      // "required dependency is retired" (:539), unreadable_dependency's is
      // "required dependency is unreadable" (:541), unknown_historical_
      // baseline's is "no historical content baseline is available" (:553).
      // None of these strings is a textual match for readinessFactLabel's
      // per-target title ("<target> is not locally approved" etc.), so the
      // fix list's literal text-equality check does NOT suppress them — it
      // would print the same boilerplate sentence under every single
      // dependency_unapproved row in the Cutainly corpus (31,673
      // instances), which is exactly the "engine vocabulary describing its
      // own grouping choice" class of noise R09.8 exists to demote, not
      // restore. Cause kinds are similarly mixed: own_flag's Detail is
      // `flag.Reason`, genuinely authored human text (readiness.go:510);
      // own_thread's is a comma-joined list of raw thread ids, not prose
      // (:503); direct_dependency_change's and approval_content_drift's are
      // BOTH the same fixed string, "dependency content differs from the
      // reviewed baseline" (:326/:348/:538/:559); the three approval_*
      // causes are system-generated sentences naming the failure mode, not
      // authored either (:466-478) — and LANES.md records that none of the
      // three has any fixture anywhere, so this branch is never exercised
      // against real data. own_flag is the ONLY kind whose Detail is
      // genuinely the reviewer's own words, so it is the only one this
      // lane renders. Verified live: TestReadinessTreatsFlagDetailsAsText
      // (own_flag, hostile-markup escaping) passes; probed against the
      // rendered Cutainly client that a dependency_unapproved blocker row
      // (e.g. sharing-observation.contract.the-command-surface's own
      // panel) renders NO boilerplate detail line, only title/hop/path.
      function readinessBlockerRow(item, rootID) {
        var li = el('li', 'claim-readiness-blocker');

        var titleText = readinessFactLabel(item.record, item.type, rootID);
        var titleGroup = el('div', 'claim-readiness-blocker-title-group');
        titleGroup.appendChild(textEl('strong', 'claim-readiness-blocker-title', titleText));

        var isAuthoredDetail = item.type === 'cause' && item.record.kind === 'own_flag';
        var detailText = isAuthoredDetail ? (item.record.detail || '').replace(/\s+/g, ' ').trim() : '';
        if (detailText) {
          titleGroup.appendChild(textEl('p', 'claim-readiness-blocker-detail', detailText));
        }
        li.appendChild(titleGroup);

        li.appendChild(textEl('span', 'claim-readiness-relation', readinessHopLabel(item.record, item.type)));

        var path = el('div', 'claim-readiness-path-chips');
        path.appendChild(textEl('span', 'claim-readiness-slug claim-readiness-slug--source', rootID));
        var targetLine = el('span', 'claim-readiness-path-target-line');
        var chevron = el('span', 'claim-readiness-path-chevron');
        chevron.setAttribute('aria-hidden', 'true');
        chevron.appendChild(dxIcon('chevron-right'));
        targetLine.appendChild(chevron);
        targetLine.appendChild(textEl('span', 'claim-readiness-slug claim-readiness-slug--target', readinessTargetID(item.record, rootID)));
        path.appendChild(targetLine);
        li.appendChild(path);

        return li;
      }

      // READINESS_VISIBLE_CAP is the within-module truncation 06 §4.4's own
      // arithmetic fixes: "Show 9 more in this module" against an 11-item
      // Capability-support group means 2 rendered up front. The remaining
      // rows are built and appended ONLY when "Show N more" is clicked
      // (06 §8 item 14: in-place expansion, never navigation) — lazily,
      // the same way the retired inline Mermaid trace it replaces used to
      // defer its own SVG (readinessScaleBudgets's DOM-node ceiling is a
      // real constraint against a project whose fan-out runs to thousands
      // of facts in one module, same as the trace's old lazy-render
      // rationale). The complete list stays fully QUERYABLE from the
      // engine's own /api payload (nothing is deleted from the data); it is
      // deferred from the DOM, not withheld from the reader.
      var READINESS_VISIBLE_CAP = 2;

      // MODULES_VISIBLE_CAP is 06 §8 item 9's module-list truncation: "the
      // board shows exactly four [modules]... the remainder are one-line
      // rows in ascending hop order, and the list is subject to the same
      // Show N more treatment the within-module list gets." RETRY FIX
      // (verifier item 12): the Cutainly corpus has claims fanning out
      // across up to 23 modules (voice.contract.what-this-module-does-not-
      // own), so an uncapped module list drew 23 full <details> rows.
      var MODULES_VISIBLE_CAP = 4;

      // readinessModuleRowFlat is the "one-line row" §8 item 9 names for a
      // module beyond the cap: name, nearest-hop note and count pill, same
      // geometry as an open module's own summary line but never a
      // <details> — it is not itself expandable, only a locator, so the
      // reviewer still sees every module's name and count without paying
      // for 19 extra disclosure widgets up front.
      function readinessModuleRowFlat(group) {
        var row = el('div', 'claim-readiness-module-summary claim-readiness-module-row-flat');
        row.appendChild(textEl('span', 'claim-readiness-module-name', group.label));
        row.appendChild(textEl('span', 'claim-readiness-module-hop', readinessNearestHopLabel(group.minHops)));
        row.appendChild(el('span', 'claim-readiness-module-spacer'));
        var pill = el('span', 'claim-readiness-module-count');
        pill.appendChild(document.createTextNode(String(group.items.length)));
        row.appendChild(pill);
        return row;
      }

      // readinessResolutionSentence is 03 §4.10 / 05 §4.9's "Resolution
      // note" (RETRY FIX, verifier item 6): "Both sit in capability-support.
      // One approval there clears this claim." The wording is mechanical
      // from the single group's own key (its module id, the same
      // lowercase-hyphenated form the quoted example uses — group.label is
      // the sidebar's Title Case display name, a different string), never
      // invented prose, and this is called only when there is exactly one
      // group to name (renderClaimReadiness enforces that).
      function readinessResolutionSentence(group) {
        var count = group.items.length;
        var subject = count === 1 ? 'It sits' : (count === 2 ? 'Both sit' : 'All ' + count + ' sit');
        return subject + ' in ' + (group.key || group.label) + '. One approval there clears this claim.';
      }

      // readinessPanelFooter is the panel's OWN footer — "See in claims
      // graph", plus the resolution sentence when there is exactly one
      // module to resolve. RETRY FIX (verifier item 5): this used to be
      // built once PER MODULE inside readinessModule and appended to every
      // module's own body, so two open modules on one claim drew the link
      // twice. It is now built once and appended to the panel itself, after
      // .claim-readiness-modules, per components-00 §B ("Footer · every
      // panel ends with 'See in claims graph' hard right") and 06 §4.4's
      // row 3B6-0, which sits at the SECTION's end, after all module rows.
      function readinessPanelFooter(singleGroup) {
        var footer = el('div', 'claim-readiness-module-footer');
        if (singleGroup) {
          footer.appendChild(textEl('p', 'claim-readiness-resolution', readinessResolutionSentence(singleGroup)));
        }
        // The reverse of graph-ui.js's own data-dxg-open-claim link
        // (graph-ui.js:3585-3594): that gap — "no API for open the graph
        // focused on claim X" — is recorded, not solved, per 06 §9 open
        // decision 7. [data-dxg-open] is the SAME delegated trigger the
        // sidebar button uses (graph-ui.js:68, :481-486), so this link
        // opens the real pane rather than shipping a dead affordance; it
        // just cannot focus it on rootID yet.
        var link = el('a', 'claim-readiness-graph-link');
        link.href = '#';
        link.setAttribute('data-dxg-open', '');
        link.appendChild(dxIcon('git-branch'));
        link.appendChild(textEl('span', '', 'See in claims graph'));
        footer.appendChild(link);
        return footer;
      }

      // readinessModule builds one module's disclosure: a summary line
      // (chevron, module name, "nearest N hop(s)", a count pill) and, once
      // open, its blocker list and an optional "Show N more in this
      // module". The panel-level "See in claims graph" footer is built
      // once by readinessPanelFooter, not per module (RETRY FIX, verifier
      // item 5).
      function readinessModule(group, rootID, index) {
        var details = el('details', 'claim-readiness-module');
        if (index === 0) { details.open = true; }

        var summary = el('summary', 'claim-readiness-module-summary');
        var chevron = el('span', 'claim-readiness-module-chevron');
        chevron.setAttribute('aria-hidden', 'true');
        chevron.appendChild(dxIcon('chevron-right'));
        summary.appendChild(chevron);
        summary.appendChild(textEl('span', 'claim-readiness-module-name', group.label));
        summary.appendChild(textEl('span', 'claim-readiness-module-hop', readinessNearestHopLabel(group.minHops)));
        summary.appendChild(el('span', 'claim-readiness-module-spacer'));
        var pill = el('span', 'claim-readiness-module-count');
        pill.appendChild(document.createTextNode(String(group.items.length)));
        summary.appendChild(pill);
        details.appendChild(summary);

        var body = el('div', 'claim-readiness-module-body');
        var list = el('ul', 'claim-readiness-blockers');
        var visible = group.items.slice(0, READINESS_VISIBLE_CAP);
        var rest = group.items.slice(READINESS_VISIBLE_CAP);
        visible.forEach(function (item) { list.appendChild(readinessBlockerRow(item, rootID)); });
        body.appendChild(list);

        if (rest.length) {
          var more = el('button', 'claim-readiness-more');
          more.type = 'button';
          var moreChevron = el('span', 'claim-readiness-more-chevron');
          moreChevron.setAttribute('aria-hidden', 'true');
          moreChevron.appendChild(dxIcon('chevron-down'));
          more.appendChild(moreChevron);
          more.appendChild(textEl('span', '', 'Show ' + rest.length + ' more in this module'));
          more.addEventListener('click', function () {
            rest.forEach(function (item) { list.appendChild(readinessBlockerRow(item, rootID)); });
            more.remove();
          });
          body.appendChild(more);
        }

        details.appendChild(body);
        return details;
      }

      // readinessRawDiagnostics is R09.8's demotion, not a deletion: the
      // exact engine fields and representative path arrays stay reachable
      // (viewer-tests' escaping and live-refresh assertions read them) as
      // the quietest, closed-by-default thing in the panel.
      function readinessRawDiagnostics(assessment) {
        var details = el('details', 'claim-readiness-raw');
        details.appendChild(textEl('summary', '', 'Raw diagnostics'));
        details.appendChild(textEl('p', '', 'Exact engine fields and representative path arrays.'));
        details.appendChild(textEl('pre', '', JSON.stringify({
          claim_id: assessment.claim_id,
          policy_version: assessment.policy_version,
          local_approved: assessment.local_approved,
          dependency_ready: assessment.dependency_ready,
          ready: assessment.ready,
          review_pending: assessment.review_pending,
          local_reasons: assessment.local_reasons || [],
          local_approval_issue: assessment.local_approval_issue || '',
          dependency_conditions: assessment.dependency_conditions || assessment.conditions || [],
          review_causes: assessment.review_causes || assessment.causes || []
        }, null, 2)));
        return details;
      }

      // renderClaimReadiness reorganizes the policy engine's facts for
      // review. It never re-derives a verdict, merges independent facts,
      // walks the dependency graph, or treats a representative path as
      // exclusive cause ownership. The complete records remain available in
      // list and raw form.
      //
      // R09.1-R09.3: the door (.claim-readiness-door, a native
      // <details name="claim-footer-<id>">) and its panel
      // (.claim-readiness.claim-footer-panel, the door's next sibling — the
      // same shape components.EdgesHTMLWithLinks uses for its own two
      // doors) are rebuilt as a pair on every call, exactly as the single
      // .claim-readiness section used to be, so a live poll's replacement
      // stays atomic. The door shares its `name` with the relationships and
      // sources doors that same function already writes, joining their
      // native "one open at a time" group at no extra cost; it force-closes
      // any of them still marked open from a stale server-side signal
      // before opening itself, because R09.3's auto-open — reserved for a
      // BLOCKED claim — outranks the relationships door's own
      // drifted/review_pending auto-open signal.
      // bannerShowing is supplied by renderStatusStrip, which is the only
      // caller that knows whether the facet-level banner will be on screen
      // this pass. R09.3's auto-open is per-claim and is suppressed while
      // that banner shows (screens/02 section 9.1): on a facet where every
      // claim is blocked the banner already says so once, and opening a door
      // on every card buries the prose it is meant to annotate.
      function renderClaimReadiness(assessments, bannerShowing) {
        // Project by the rendered card's canonical id. Claim ids also occur
        // on graph links, comment controls, and edge references, so
        // heading/link scans are not a reliable card inventory after the
        // live mount.
        document.querySelectorAll('.claim[id]').forEach(function (card) {
          var id = card.id;
          var assessment = assessments && assessments[id];
          if (!assessment) { return; }

          var conditions = assessment.dependency_conditions || assessment.conditions || [];
          var causes = assessment.review_causes || assessment.causes || [];
          var facts = conditions.map(function (record) { return { type: 'condition', record: record }; })
            .concat(causes.map(function (record) { return { type: 'cause', record: record }; }));
          var blocked = facts.length > 0;
          var state = assessment.ready
            ? 'Ready'
            : (assessment.review_pending
              ? 'Review required'
              : (!assessment.local_approved && assessment.dependency_ready ? 'Approval required' : 'Dependencies not ready'));

          var footerName = 'claim-footer-' + id;
          var door = blocked
            ? el('details', 'claim-readiness-door')
            : el('span', 'claim-readiness-empty claim-footer-chip claim-footer-chip--readiness claim-footer-chip--empty');
          if (blocked) { door.setAttribute('name', footerName); }
          door.setAttribute('data-readiness-state', state);
          // Preserve the authoritative assessment total on the disclosure
          // itself. The visible blocker rows are progressively disclosed and
          // therefore cannot serve as a total for sibling navigation UI.
          door.setAttribute('data-readiness-fact-count', String(facts.length));
          var summary = blocked
            ? el('summary', 'claim-footer-chip claim-footer-chip--readiness claim-footer-chip--blocked')
            : door;
          if (blocked) {
            summary.appendChild(el('span', 'claim-readiness-chip-dot'));
            summary.appendChild(textEl('span', 'claim-footer-chip-label', 'Blocked'));
            summary.appendChild(textEl('span', 'claim-footer-chip-count', facts.length + ' ' + (facts.length === 1 ? 'blocker' : 'blockers')));
            summary.appendChild(footerChevron());
            door.appendChild(summary);
          } else {
            summary.appendChild(textEl('span', 'claim-footer-chip-label', 'No blockers'));
          }
          if (blocked && !bannerShowing) {
            // Force-close any sibling in this name group a stale
            // server-rendered `open` attribute left open before this claim's
            // own blocked state claims the group, per R09.3's priority over
            // that signal. components.go no longer emits its own open
            // attribute (Z7-0: relationships never opens by default), so this
            // now only guards against a deep-linked sibling.
            document.querySelectorAll('details[name="' + footerName.replace(/"/g, '\\"') + '"][open]').forEach(function (other) { other.open = false; });
            door.open = true;
          }

          var panel = el('div', 'claim-readiness claim-footer-panel' + (blocked ? ' claim-readiness--blocked' : ''));
          panel.setAttribute('aria-label', 'Claim readiness');

          // RETRY FIX (verifier item 8): local approval reasons used to be
          // appended FIRST, ahead of the READINESS BLOCKERS eyebrow, so a
          // blocked claim's panel opened with a bare "is not locally
          // approved" bullet instead of the section head. No board (06
          // §4.4, 03 §4.10, 05 §4.9, components-00 §B) draws content above
          // that eyebrow. localNotes is still computed here (it needs
          // assessment before either branch below), but the actual
          // .claim-readiness-local append moves below the module list /
          // summary paragraph, whichever this claim takes.
          var localNotes = (assessment.local_reasons || []).slice();
          // local_approval_issue is a concise alias and may repeat the
          // reason already present in local_reasons. Collapse that one
          // presentation duplicate while retaining both exact fields in
          // Raw diagnostics.
          if (assessment.local_approval_issue && localNotes.indexOf(assessment.local_approval_issue) < 0) {
            localNotes.push(assessment.local_approval_issue);
          }

          if (facts.length) {
            var groups = readinessGroupByModule(facts, id);
            var head = el('div', 'claim-readiness-section-head');
            head.appendChild(textEl('span', 'claim-readiness-eyebrow', 'Readiness blockers'));
            head.appendChild(el('span', 'claim-readiness-eyebrow-rule'));
            var moduleWord = groups.length === 1 ? 'module' : 'modules';
            head.appendChild(textEl('span', 'claim-readiness-scope', facts.length + ' across ' + groups.length + ' ' + moduleWord));
            panel.appendChild(head);

            var modules = el('div', 'claim-readiness-modules');
            var visibleGroups = groups.slice(0, MODULES_VISIBLE_CAP);
            var restGroups = groups.slice(MODULES_VISIBLE_CAP);
            visibleGroups.forEach(function (group, index) { modules.appendChild(readinessModule(group, id, index)); });
            panel.appendChild(modules);

            if (restGroups.length) {
              // 06 §8 item 9: the remainder are "one-line rows in ascending
              // hop order", subject to the same Show N more treatment the
              // within-module list gets — built and appended only on click.
              var moreModules = el('button', 'claim-readiness-more claim-readiness-more--modules');
              moreModules.type = 'button';
              var moreModulesChevron = el('span', 'claim-readiness-more-chevron');
              moreModulesChevron.setAttribute('aria-hidden', 'true');
              moreModulesChevron.appendChild(dxIcon('chevron-down'));
              moreModules.appendChild(moreModulesChevron);
              moreModules.appendChild(textEl('span', '', 'Show ' + restGroups.length + ' more modules'));
              moreModules.addEventListener('click', function () {
                restGroups.slice().sort(function (a, b) { return a.minHops - b.minHops; })
                  .forEach(function (group) { modules.appendChild(readinessModuleRowFlat(group)); });
                moreModules.remove();
              });
              panel.appendChild(moreModules);
            }

            // 05 §4.9 / 03 §4.10's resolution sentence sits only when every
            // blocker sits in one module — components-00 §B: "The sentence
            // to its left appears only when all blockers sit in one
            // module, because only then is one approval enough to clear
            // the claim." (RETRY FIX, verifier item 6.)
            panel.appendChild(readinessPanelFooter(groups.length === 1 ? groups[0] : null));
          } else {
            var localSentence = assessment.local_approved ? 'This claim is locally approved.' : 'This claim is not locally approved.';
            var dependencySentence = assessment.dependency_ready ? 'Its required dependency chain is ready.' : 'Its dependency chain is not ready.';
            panel.appendChild(textEl('p', 'claim-readiness-summary', localSentence + ' ' + dependencySentence));
          }

          if (localNotes.length) {
            var local = el('div', 'claim-readiness-local');
            var localList = el('ul');
            localNotes.forEach(function (note) { localList.appendChild(textEl('li', '', note)); });
            local.appendChild(localList);
            panel.appendChild(local);
          }

          panel.appendChild(readinessRawDiagnostics(assessment));

          var existingDoor = card.querySelector('.claim-readiness-door, .claim-readiness-empty');
          var existingPanel = card.querySelector('.claim-readiness');
          var links = card.querySelector('.claim-links');
          var footer = card.querySelector('.claim-footer');
          if (!blocked) {
            if (existingPanel) { existingPanel.remove(); }
            if (existingDoor && existingDoor.parentNode) { existingDoor.replaceWith(door); }
            else if (footer) { footer.insertBefore(door, footer.firstChild); }
            else { card.appendChild(door); }
            return;
          }
          if (existingPanel && existingPanel.parentNode) { existingPanel.replaceWith(panel); }
          else if (links && links.parentNode) { links.parentNode.insertBefore(panel, links); }
          else if (footer) { footer.insertBefore(panel, footer.firstChild); }
          else { card.appendChild(panel); }
          if (existingDoor && existingDoor.parentNode) { existingDoor.replaceWith(door); }
          else { panel.parentNode.insertBefore(door, panel); }
        });
      }

      // A checked-in/offline viewer has no /api/status endpoint. The
      // rendered graph payload carries the same readiness projection as
      // .catalog.json, so turn its node data back into the claim-id map the
      // card renderer accepts. A live status response below always
      // replaces this snapshot.
      function offlineReadiness() {
        var node = document.getElementById('dossierx-graph');
        if (!node) { return {}; }
        try {
          var payload = JSON.parse(node.textContent || '{}');
          var out = {};
          (payload.nodes || []).forEach(function (graphNode) {
            if (graphNode.id && graphNode.readiness) { out[graphNode.id] = graphNode.readiness; }
          });
          return out;
        } catch (_) { return {}; }
      }

      // ---- Unapproved edits -------------------------------------------------
      //
      // The state these three functions draw is the ordinary one, and it was
      // the one the viewer could not show. A claim is locked; someone unlocks
      // it, rewrites it, and has not locked it again yet. The new wording
      // replaced the old in the file, so the reading view showed text with
      // nothing to compare it against and a chip reading DRAFT — the same chip
      // a claim nobody ever approved carries. "Never written" and "approved,
      // then moved" are different jobs for a reviewer, and one word was being
      // used for both.
      //
      // The engine now answers both questions: readiness emits an
      // `unapproved_edit` cause (so the claim reaches the Issues screen on its
      // own, without needing a dependent to notice for it), and the graph
      // payload carries the line diff from the approved body to the current
      // one. Everything below is presentation of those two facts. It computes
      // nothing about whether an edit is allowed, and never writes.

      // offlineApprovedEdits reads the per-claim approvaledit.Change records
      // off the SAME rendered graph payload offlineReadiness reads. There is
      // deliberately no /api/status path for this: a checked-in file:// export
      // has no endpoint to ask, and a panel that appeared only under
      // "dossierx serve" would be missing exactly where a reviewer reads a
      // repository they did not build.
      function offlineApprovedEdits() {
        var node = document.getElementById('dossierx-graph');
        if (!node) { return {}; }
        try {
          var payload = JSON.parse(node.textContent || '{}');
          var out = {};
          (payload.nodes || []).forEach(function (graphNode) {
            if (graphNode.id && graphNode.approved_edit) { out[graphNode.id] = graphNode.approved_edit; }
          });
          return out;
        } catch (_) { return {}; }
      }

      // editDateLabel turns an RFC3339 instant into the board's own short
      // form ("12 Sep"). It is deliberately not a full date: the panel's job
      // is to place the approval in the reader's recent memory, and a
      // four-part date would be the widest thing on a row that is otherwise
      // a sentence.
      function editDateLabel(iso) {
        if (!iso) { return ''; }
        var d = new Date(iso);
        if (isNaN(d.getTime())) { return String(iso).slice(0, 10); }
        return d.getDate() + ' ' + ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
          'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'][d.getMonth()];
      }

      // approvedEditHasDiff reports whether there is a second wording to show
      // at all.
      //
      // Two states look like an edit and have nothing to put behind a
      // "Changes" tab. A record written before the approved wording was kept
      // proves the text moved and does not carry what it moved from. A claim
      // whose hash moved because its rests_on or build_role moved has an
      // approved wording identical to its current one. In both, the panel has
      // one sentence to say and no second version of the claim — so there is
      // no switch, and the claim's own body stays on screen.
      //
      // Offering the switch anyway is what this replaces, and it was worse
      // than a useless control: choosing "Changes" hid the body and put the
      // sentence in its place, so the claim rendered with no content at all.
      function approvedEditHasDiff(change) {
        return !!(change.content_retained &&
          (change.hunks || []).length &&
          (change.changed_passages || 0) > 0);
      }

      // approvedEditDiff builds the passage-by-passage view: unchanged
      // passages as ordinary prose, and each changed one as the approved
      // passage above the current one, tinted and ruled.
      //
      // The HTML is produced by the ENGINE (internal/approvaledit), not here,
      // because internal/render/markdown is this project's only markdown
      // renderer and the viewer has none. Shipping the source and parsing it
      // in the browser would mean growing a second renderer that could
      // disagree with the first about the same claim, on the same page.
      function approvedEditDiff(change) {
        var body = el('div', 'claim-edit-diff');
        (change.hunks || []).forEach(function (hunk) {
          var cls = hunk.op === 'remove' ? 'claim-edit-passage claim-edit-passage--removed'
            : hunk.op === 'add' ? 'claim-edit-passage claim-edit-passage--added'
            : 'claim-edit-passage';
          var block = el('div', cls);
          // innerHTML, and only here. The value came from
          // internal/render/markdown, which is the escaping boundary every
          // other claim body in this viewer crosses — the same function, over
          // the same author bytes, reached through the same payload as the
          // readiness projection beside it. It is exactly as trusted as the
          // body it sits above, and no more.
          block.innerHTML = hunk.html || '';
          if ((hunk.changed || []).length) {
            block.setAttribute('aria-label',
              (hunk.op === 'remove' ? 'Removed since approval: ' : 'Added since approval: ') +
              hunk.changed.join(', '));
          }
          body.appendChild(block);
        });
        var fields = approvedEditFields(change);
        if (fields) { body.appendChild(fields); }
        return body;
      }

      // approvedEditFields draws the non-body fields that moved, each as its
      // approved YAML above its current YAML with the words that moved marked.
      //
      // It is here because "Also changed: embodiment, audit_notes." was not an
      // answer. For a claim whose prose never moved that sentence WAS the whole
      // panel, and it named the change without showing it — the reader was told
      // a field they cannot see had moved in a way they cannot see, and had to
      // go and read the file to find out what they were being asked about. Six
      // of the fourteen claims this feature was built for are in exactly that
      // state.
      //
      // The blocks are drawn as text and not as prose. A field is YAML, so it
      // is escaped by the engine (internal/approvaledit/fields.go) and shown in
      // the monospace the file itself would show; running it through the
      // markdown renderer would turn a list into a list and a `*` into
      // emphasis, and stop it being the thing on disk.
      function approvedEditFields(change) {
        var changes = change.field_changes || [];
        if (!changes.length) { return null; }
        var wrap = el('div', 'claim-edit-fields');

        var toggle = el('button', 'claim-edit-fields-toggle');
        toggle.type = 'button';
        var list = el('div', 'claim-edit-fields-list');
        var open = true;
        function paint() {
          toggle.setAttribute('aria-expanded', String(open));
          toggle.textContent = (open ? 'Hide' : 'Show') + ' the ' +
            (changes.length === 1 ? 'field' : changes.length + ' fields') + ' that moved';
          list.hidden = !open;
        }
        toggle.addEventListener('click', function (event) {
          event.preventDefault();
          open = !open;
          paint();
        });

        changes.forEach(function (field) {
          var row = el('div', 'claim-edit-field');
          row.appendChild(textEl('span', 'claim-edit-field-name', field.field));
          if (field.truncated) {
            row.appendChild(textEl('p', 'claim-edit-field-note',
              'This field moved. It is too large to show here; read it in the claim file.'));
            list.appendChild(row);
            return;
          }
          (field.hunks || []).forEach(function (hunk) {
            var cls = hunk.op === 'remove' ? 'claim-edit-field-hunk claim-edit-field-hunk--removed'
              : hunk.op === 'add' ? 'claim-edit-field-hunk claim-edit-field-hunk--added'
              : 'claim-edit-field-hunk';
            var block = el('pre', cls);
            // innerHTML over engine-escaped text. internal/approvaledit's
            // field renderer HTML-escapes the YAML before it marks anything,
            // so the only tags that can be in here are the mark spans it
            // wrote itself — a field holding <script> arrives as text.
            block.innerHTML = hunk.html || '';
            if ((hunk.changed || []).length) {
              block.setAttribute('aria-label',
                (hunk.op === 'remove' ? 'Removed from ' : 'Added to ') + field.field + ': ' +
                hunk.changed.join(', '));
            }
            row.appendChild(block);
          });
          list.appendChild(row);
        });

        paint();
        wrap.appendChild(toggle);
        wrap.appendChild(list);
        return wrap;
      }

      // approvedEditNote is what a claim with no second wording says instead of
      // offering a switch. It sits under the bar, beside the claim's own body
      // rather than in place of it.
      function approvedEditNote(change) {
        var wrap = el('div', 'claim-edit-note');
        if (!change.content_retained) {
          wrap.appendChild(textEl('p', 'claim-edit-note-text',
            'This claim was approved before the approved wording was kept on the record, so there is nothing to compare against. The approval is released and the text has since changed.'));
          wrap.appendChild(textEl('p', 'claim-edit-note-hint',
            'Run dossierx claim recover-approved-content to look for the approved revision in this project\u2019s git history.'));
          return wrap;
        }
        var fields = change.other_fields || [];
        wrap.appendChild(textEl('p', 'claim-edit-note-text', fields.length
          ? 'The wording is unchanged since approval. ' +
            (fields.length === 1 ? '1 field moved.' : fields.length + ' fields moved.')
          : 'The wording is unchanged since approval.'));
        var diff = approvedEditFields(change);
        if (diff) { wrap.appendChild(diff); }
        return wrap;
      }

      // approvedEditBar is the control the board draws in place of the old
      // disclosure: a tinted row above the body that says what the reader is
      // looking at, and a two-segment switch between the two states.
      //
      // It is a BAR AND NOT A DISCLOSURE, and the difference is which state
      // is the default. A disclosure says the diff is an aside a reader opens
      // if they want it; the board says the opposite — a claim that was
      // approved and then rewritten opens SHOWING what moved, because that is
      // the thing about it a reader has to know before they read a word of
      // it. "Current" is the escape hatch, not the resting state.
      function approvedEditBar(card, change, showingChanges) {
        var bar = el('div', 'claim-edit-bar');
        var approved = editDateLabel(change.approved_at);
        var hasDiff = approvedEditHasDiff(change);
        bar.appendChild(textEl('span', 'claim-edit-bar-title',
          !hasDiff || showingChanges ? 'Edited since it was approved' : 'Showing the current wording'));

        var meta;
        if (!hasDiff || showingChanges) {
          meta = 'approved ' + approved + (change.approved_by ? ' \u00b7 ' + change.approved_by : '');
        } else {
          var passages = change.changed_passages || 0;
          meta = passages === 1
            ? '1 passage differs from the approval of ' + approved
            : passages + ' passages differ from the approval of ' + approved;
        }
        bar.appendChild(textEl('span', 'claim-edit-bar-meta', meta));

        // No second wording, no switch. A control whose two states show the
        // same thing is not a control, and the one that used to be here did
        // worse than nothing: picking "Changes" hid the claim's body to make
        // room for a diff that did not exist.
        if (!hasDiff) { return bar; }

        var group = el('div', 'claim-edit-switch');
        group.setAttribute('role', 'group');
        group.setAttribute('aria-label', 'Which wording to show');
        [['Changes', true], ['Current', false]].forEach(function (pair) {
          var seg = el('button', 'claim-edit-switch-seg');
          seg.type = 'button';
          seg.textContent = pair[0];
          seg.setAttribute('aria-pressed', String(showingChanges === pair[1]));
          seg.addEventListener('click', function (event) {
            event.preventDefault();
            paintApprovedEdit(card, change, pair[1]);
          });
          group.appendChild(seg);
        });
        bar.appendChild(group);
        return bar;
      }

      // paintApprovedEdit puts one claim card into one of the two states, and
      // is the only thing that writes either of them, so the bar and the body
      // can never disagree about which one is showing.
      function paintApprovedEdit(card, change, showingChanges) {
        var hasDiff = approvedEditHasDiff(change);
        showingChanges = hasDiff && showingChanges;
        var existingBar = card.querySelector(':scope > .claim-edit-bar');
        // The diff may already be wrapped: the four-line disclosure wraps it
        // exactly as it wraps a claim body (system-record.js's BODY_LIKE), so
        // what has to be taken out is the wrapper when there is one.
        var existingDiff = card.querySelector(
          ':scope > .claim-edit-diff, :scope > .claim-edit-note, :scope > .claim-body-disclosure--edit');

        var bar = approvedEditBar(card, change, showingChanges);
        if (existingBar) { existingBar.replaceWith(bar); }
        else {
          var head = card.querySelector(':scope > .k');
          if (head && head.parentNode) { head.parentNode.insertBefore(bar, head.nextSibling); }
          else { card.insertBefore(bar, card.firstChild); }
        }
        if (existingDiff) { existingDiff.remove(); }
        if (!hasDiff) {
          bar.parentNode.insertBefore(approvedEditNote(change), bar.nextSibling);
        } else if (showingChanges) {
          bar.parentNode.insertBefore(approvedEditDiff(change), bar.nextSibling);
          // Hand the new diff to the disclosure so it is measured and clamped
          // like any other body. Without this the diff renders at full height
          // while the claim beside it truncates at four lines, which is the
          // same claim behaving as two components.
          if (typeof window.dossierxEnhanceSystemRecord === 'function') {
            window.dossierxEnhanceSystemRecord();
          }
        }
        // The claim's own body is hidden by a class ON THE CARD, and not by
        // setting `hidden` on the body itself.
        //
        // Which element the body IS moves: the long-body disclosure wraps
        // .claim-body inside .claim-body-disclosure after first paint, so a
        // handle taken before the wrap and a handle taken after it are
        // different nodes. Hiding whichever one a selector matched at the
        // time left the first one hidden and the second one shown, and the
        // switch stopped working on exactly the claims long enough to be
        // wrapped. A class on the card is one state, in one place, whatever
        // the body is wearing.
        //
        // It is also why the body is hidden rather than removed: it carries
        // the source-note clamps and that disclosure, and tearing it out
        // would take their state with it every time the reader flipped.
        card.classList.toggle('claim--showing-changes', showingChanges);
      }

      // renderApprovedEdits upgrades each edited claim's chip and puts its
      // card into the Changes state. It is driven from mountSurface (so the
      // surface reaches its final height before anything measures or scrolls
      // it) and again from renderStatusStrip, so everything it creates is
      // either guarded or left alone on a repaint.
      function renderApprovedEdits() {
        var edits = offlineApprovedEdits();
        Object.keys(edits).forEach(function (id) {
          var card = document.getElementById(id);
          if (!card) { return; }
          var change = edits[id];

          // The chip. It stays .pill.pv — the claim IS a draft, and inventing
          // a fourth colour for it would say the state is unrelated to the
          // one the reader already knows. Only the word changes, because only
          // the word was wrong: DRAFT is true and incomplete, and a reviewer
          // deciding what to open next needs the part it leaves out.
          var pill = card.querySelector('.k .label .pill');
          if (pill && pill.getAttribute('data-dx-edited') !== '1') {
            pill.setAttribute('data-dx-edited', '1');
            pill.setAttribute('title', 'This claim held an approval, which was released when it was unlocked. What is written now differs from what was approved.');
            var icon = pill.querySelector('.dx-icon');
            pill.textContent = '';
            if (icon) { pill.appendChild(icon); }
            pill.appendChild(textEl('span', '', 'Edited \u00b7 was approved'));
          }

          // The bar and the body state, painted only when the bar is not
          // already there: a repaint must not take the reader's own "Current"
          // choice away from them on the next poll.
          if (!card.querySelector(':scope > .claim-edit-bar')) {
            paintApprovedEdit(card, change, true);
          }
        });
      }

      // approvedEditIDsIn returns the claims in the id set that have been
      // rewritten since the approval their unlock released.
      //
      // It is scoped to the facet on screen, by the SAME id set the rest of
      // the strip scopes itself with — a count of claims on a page the reader
      // is not looking at is a count they cannot act on. The set is a
      // null-prototype object keyed by id (activeFacetClaimIDs), so this
      // indexes it rather than calling Set or Array methods it does not have.
      function approvedEditIDsIn(claimIDs) {
        var ids = claimIDs || {};
        return Object.keys(offlineApprovedEdits()).filter(function (id) { return ids[id]; });
      }

      // renderStatusStrip paints one /api/status payload. An empty verdict hides
      // the strip entirely rather than showing a green badge: the viewer already
      // reads as "fine" by default, and a persistent all-clear chip would be one
      // more thing to stop noticing.
      function renderStatusStrip(data) {
        if (!stripEl || !stripBody || !stripCard || !stripFindings || !stripTitle) { return; }
        lastStatusData = data || {};
        var claimIDs = activeFacetClaimIDs();
        var ledger = findingsForActiveFacet(lastStatusData.ledger_findings || [], claimIDs);
        var lintErrors = findingsForActiveFacet(lastStatusData.lint_errors || [], claimIDs);
        var lintWarnings = findingsForActiveFacet(lastStatusData.lint_warnings || [], claimIDs);
        var groups = collectStatusGroups(
          lastStatusData.readiness || offlineReadiness(),
          claimIDs,
          ledger,
          lintErrors,
          lintWarnings,
          conformanceNotReadyIDs(claimIDs)
        );

        var actionable = groups.some(function (group) { return group.severity !== 'later'; });
        // The banner's own visibility decides whether R09.3's readiness
        // auto-open is suppressed (screens/02 section 9.1). It is computed
        // here, BEFORE the doors are built, and passed down: reading
        // stripEl.hidden inside renderClaimReadiness would sample the
        // PREVIOUS pass's state, since this function only writes it below.
        var bannerShowing = groups.length > 0 && actionable;
        renderClaimReadiness(lastStatusData.readiness || offlineReadiness(), bannerShowing);
        renderApprovedEdits();
        if (!groups.length || !actionable) {
          stripEl.hidden = true;
          stripEl.classList.remove('status-strip--integrity', 'status-strip--lint');
          stripBody.textContent = '';
          return;
        }

        var visible = groups.filter(function (group) {
          return !stripSeverityFilter || group.severity === stripSeverityFilter;
        });

        stripBody.textContent = '';
        // 04 §8 item 11 / §9 item 5: shell.html:265-271 already states this
        // strip exists only against a live serve; readiness and lint groups
        // still render statically from the embedded graph payload
        // (offlineReadiness), but an APPROVAL RECORD verdict never can — a
        // baked one would be exactly the "stale green strip nobody checked"
        // that comment forbids. Stated here, not silently omitted.
        if (((window.location && window.location.protocol) || '') === 'file:') {
          stripBody.appendChild(textEl('p', 'status-strip-static-note',
            'Approval record findings need a live dossierx serve; open with dossierx serve to see them.'));
        }
        var filters = el('div', 'status-strip-filters');
        STATUS_SEVERITIES.forEach(function (item) {
          var count = countSeverity(groups, item.id);
          // retry fix 32 (this lane's third attempt, COORDINATOR RULING):
          // a severity at zero emits NO chip — R08.1 makes a FILLED pill
          // mean "nothing above this", which only means something when the
          // filled (Critical) chip's absence is itself the "nothing above
          // this" signal. A zero-count chip for every severity, always
          // shown, said nothing a reader could act on.
          if (count === 0) { return; }
          var pressed = stripSeverityFilter === item.id;
          // 04 §4.4 / R08.1: a scoped chip, never claim-status's shared
          // .pill — that class is a different vocabulary (LOCKED / DRAFT /
          // review_pending) and restyling it here would restyle every claim
          // chip in the document too.
          var chip = el('button', 'status-severity-chip status-severity-chip--' + item.id);
          chip.type = 'button';
          chip.appendChild(el('span', 'status-severity-chip__dot'));
          chip.appendChild(document.createTextNode(item.label + ' '));
          chip.appendChild(textEl('span', 'status-severity-chip__count', String(count)));
          // 04 §8 item 2: pressed is the R-F.2 "open" treatment (an accent
          // border + weight step), not a second fill — the severity hues are
          // already spent on meaning and cannot also mean "selected".
          chip.setAttribute('aria-pressed', String(pressed));
          chip.addEventListener('click', function (event) {
            event.preventDefault();
            event.stopPropagation();
            stripSeverityFilter = pressed ? '' : item.id;
            renderStatusStrip(lastStatusData);
          });
          filters.appendChild(chip);
        });
        stripBody.appendChild(filters);

        // 04 §2 / §9 item 6: the promoted Issues body groups by the module
        // that owns the blocking claim, never by severity — severity is the
        // filter strip above, not the organising axis. R09.8 / §9 item 7
        // keeps "Later" out of the default list; a reader opts in by
        // pressing its chip. R08.2 / §8 item 3: integrity ("ledger")
        // findings are a different KIND of finding, not a severity among
        // severities — they sort into their own APPROVAL RECORD group,
        // first, ahead of every module group, regardless of weight.
        var withoutLater = visible.filter(function (group) {
          return group.severity !== 'later' || stripSeverityFilter === 'later';
        });
        // retry fix 20: keyed off `origin` (the finding's KIND), not
        // `severity` — a critical-severity LINT row must still be able to
        // sit in its own module group (§ 8 item 1), and only a `ledger`
        // finding may ever join APPROVAL RECORD.
        var approvalRecord = withoutLater.filter(function (group) { return group.origin === 'ledger'; });
        var byModule = {};
        var moduleOrder = [];
        withoutLater.filter(function (group) { return group.origin !== 'ledger'; }).forEach(function (group) {
          var moduleID = ownerModuleID(group);
          if (!Object.prototype.hasOwnProperty.call(byModule, moduleID)) {
            byModule[moduleID] = [];
            moduleOrder.push(moduleID);
          }
          byModule[moduleID].push(group);
        });
        var facetTotal = uniqueClaimCount(groups.filter(function (group) { return group.origin !== 'ledger'; }));
        moduleOrder.sort(function (a, b) {
          return uniqueClaimCount(byModule[b]) - uniqueClaimCount(byModule[a]);
        });
        if (approvalRecord.length) {
          var recordEl = findingGroup('APPROVAL RECORD', approvalRecord.slice().sort(function (a, b) {
            return statusGroupClaimCount(b) - statusGroupClaimCount(a);
          }).map(renderStatusGroup));
          recordEl.classList.add('status-group--approval-record');
          stripBody.appendChild(recordEl);
        }
        moduleOrder.forEach(function (moduleID) {
          var rows = byModule[moduleID].slice().sort(function (a, b) {
            return statusGroupClaimCount(b) - statusGroupClaimCount(a);
          });
          var weight = uniqueClaimCount(rows);
          var note = 'blocks ' + weight + ' of ' + facetTotal + ' claim' + (facetTotal === 1 ? '' : 's') + ' here';
          stripBody.appendChild(findingGroup(moduleLabel(moduleID), rows.map(renderStatusGroup), note));
        });

        // retry fix 21: stripNote (the "Critical 0 · Needs you 3 · ..."
        // tally) is removed from the collapsed banner per the coordinator's
        // ruling — the severity chips above already carry every count this
        // used to repeat as a second string. statusChipLine, the function
        // that built that string, is now dead (no other call site) and is
        // deleted.
        //
        // Both forms of the head, every paint. The desktop band carries the
        // single headline sentence it always did; the phone card carries one
        // row per finding. They are derived from the SAME groups, ledger and
        // lint lists a line apart, so the two can differ in shape and never
        // in what they say.
        var rows = statusStripRows(groups, ledger, lintErrors, claimIDs);
        renderStatusStripCard(rows);
        // The sentence is the first row's — the rows are already in the order
        // that puts the most serious finding first, so the band leads with
        // the same fact the card's top row does.
        stripTitle.textContent = rows[0].text;

        stripEl.classList.toggle('status-strip--integrity', ledger.length > 0);
        stripEl.classList.toggle('status-strip--lint', ledger.length === 0);
        // #statusStripBody is populated above and stays hidden: the Issues
        // screen MOVES its children into itself (issuesSyncFromStrip), so it
        // must exist and must not be shown here.
        if (stripBody) { stripBody.hidden = true; }
        positionStatusStrip();
        stripEl.hidden = false;
      }

      // statusStripRows turns the facet's findings into the card's rows —
      // one row per KIND of thing waiting, never one per claim.
      //
      // The rows are the summary sentences the strip already computed, each
      // now carrying its own hue instead of four of them competing to be the
      // one headline. `tone` is the row's colour family, and it is the only
      // thing the card's tint rule reads: 'alarm' for anything the approval
      // record or the dependency chain is refusing, 'draft' for work in
      // progress that nobody has approved yet.
      function statusStripRows(groups, ledger, lintErrors, claimIDs) {
        var rows = [];
        if (ledger.length) {
          rows.push({
            tone: 'alarm', severity: 'critical',
            text: countLabel(ledger.length, 'approval record issue') + ' in this facet need' +
              (ledger.length === 1 ? 's' : '') + ' attention'
          });
        }
        var headline = blockerHeadline(groups);
        if (headline) {
          rows.push({ tone: 'alarm', severity: 'blocker', text: headline });
        }
        if (lintErrors.length) {
          rows.push({
            tone: 'alarm', severity: 'critical',
            text: countLabel(lintErrors.length, 'issue') + ' in this facet need' +
              (lintErrors.length === 1 ? 's' : '') + ' attention'
          });
        }
        // Claims rewritten since approval. This is the row the old
        // single-sentence head had no space for, and the reason it became a
        // card: it is amber where everything above it is red, and one tinted
        // surface cannot make two severity claims at once.
        var edited = approvedEditIDsIn(claimIDs);
        if (edited.length) {
          rows.push({
            tone: 'draft', severity: 'needs_you',
            text: countLabel(edited.length, 'claim') + ' ' + (edited.length === 1 ? 'has' : 'have') +
              ' edits that have not been approved'
          });
        }
        if (!rows.length) {
          rows.push({
            tone: 'alarm', severity: '',
            text: countLabel(groups.length, 'grouped issue') + ' in this facet'
          });
        }
        return rows;
      }

      // renderStatusStripCard paints the head. Tinted while every row shares
      // a tone, neutral the moment they disagree — Paper EH1-0's own rule,
      // and the reason it is a rule: a tinted surface IS a severity claim,
      // so a card holding a red fact and an amber one has to stop making it
      // and let the dots carry the colour instead.
      function renderStatusStripCard(rows) {
        if (!stripCard || !stripFindings) { return; }
        var tones = {};
        rows.forEach(function (row) { tones[row.tone] = true; });
        var toneKeys = Object.keys(tones);
        var tinted = toneKeys.length === 1 ? toneKeys[0] : '';
        stripCard.classList.toggle('status-strip-card--alarm', tinted === 'alarm');
        stripCard.classList.toggle('status-strip-card--draft', tinted === 'draft');
        stripCard.classList.toggle('status-strip-card--neutral', !tinted);
        if (stripCardCount) { stripCardCount.textContent = String(rows.length); }

        stripFindings.textContent = '';
        rows.forEach(function (row) {
          // A button, not a div with a handler: each row is a way into the
          // Issues screen and has to be reachable by keyboard and named to a
          // screen reader like the one control it is.
          var el_ = el('button', 'status-strip-finding');
          el_.type = 'button';
          el_.setAttribute('data-tone', row.tone);
          var dot = el('span', 'status-strip-finding-dot');
          dot.setAttribute('aria-hidden', 'true');
          el_.appendChild(dot);
          el_.appendChild(textEl('span', 'status-strip-finding-text', row.text));
          var chev = el('span', 'status-strip-finding-chevron');
          chev.setAttribute('aria-hidden', 'true');
          chev.appendChild(dxIcon('chevron-right'));
          el_.appendChild(chev);
          el_.addEventListener('click', function (event) {
            event.preventDefault();
            // Filtered to the row's own severity. On a real corpus the
            // unfiltered screen is hundreds of dependency rows and the thing
            // this row named is somewhere inside them; a way in that lands a
            // reader in a list they then have to search has not answered the
            // question it asked. Same stripSeverityFilter the chips set, so
            // the chip is pressed on arrival and one more click clears it.
            stripSeverityFilter = row.severity || '';
            renderStatusStrip(lastStatusData);
            openIssuesView();
          });
          stripFindings.appendChild(el_);
        });
      }

      // refreshStatus polls the endpoint. A FAILED poll deliberately leaves the
      // last verdict on screen untouched: a transient fetch failure (the server
      // restarting mid-reload) must never be able to clear an integrity warning,
      // because "the warning went away" and "the warning was fixed" would then
      // look identical to the reader.
      function refreshStatus() {
        if (!mounted) { return; }
        apiGet('/api/status').then(function (data) {
          renderStatusStrip(data);
        }, function () { /* keep the last verdict; the next tick recovers */ });
      }

      // R09.6 (Paper ZP-0): "Issues is its own screen, not an expansion. The
      // banner is the only way in." No board in the group draws an expanded
      // banner, and the string "Hide issues" does not exist anywhere in the
      // design file, so the strip has no inline disclosure at all.
      //
      // EH1-0 then replaced the one sentence with one row per finding, and
      // the entry point went with it: each row opens the Issues screen,
      // filtered to that row's own severity. "The banner is the only way in"
      // still holds — the banner simply has more than one door now, because
      // a facet routinely has more than one kind of thing waiting.
      //
      // #statusStripBody is never emptied, removed or left unpopulated:
      // renderStatusStrip still builds into it, issuesSyncFromStrip still
      // MOVES its children into the Issues screen, and closeIssuesView still
      // moves them back. It is simply never shown in place.

      // ================================================================
      // Issues view (04-issues-screen.md) — lane L8a
      // ================================================================
      //
      // The screen's own chrome around lane L8's status-strip vocabulary:
      // breadcrumb, title/subtitle, scope and sort controls, the findings
      // card and the "WHERE THE BLOCKERS LIVE" rail (04 §3, §4.1-4.3, §4.5,
      // §4.6, §4.9). R09.6: reached ONLY from a row of the banner card, each
      // of which wires openIssuesView itself as it is built.
      //
      // #issuesFilters and #issuesFindingsCard never build their own copy of
      // the severity chips or the grouped findings list: issuesSyncFromStrip
      // MOVES #statusStripBody's current children into them (never clones),
      // so every click handler and every future renderStatusStrip() re-paint
      // L8 already wired keeps working untouched — composed, not restyled.
      var issuesViewEl = document.getElementById('issuesView');
      var issuesBackBtn = document.getElementById('issuesBack');
      var issuesBreadcrumbFacetEl = document.getElementById('issuesBreadcrumbFacet');
      var issuesBreadcrumbSuffixEl = document.getElementById('issuesBreadcrumbSuffix');
      var issuesSubtitleEl = document.getElementById('issuesSubtitle');
      var issuesScopeEl = document.getElementById('issuesScope');
      var issuesCaveatEl = document.getElementById('issuesCaveat');
      var issuesFiltersHost = document.getElementById('issuesFilters');
      var issuesSortControl = document.getElementById('issuesSortControl');
      var issuesFindingsCard = document.getElementById('issuesFindingsCard');
      var issuesRailPanelEl = document.getElementById('issuesRailPanel');
      var issuesRailCaveatEl = document.getElementById('issuesRailCaveat');
      var issuesRailRowsEl = document.getElementById('issuesRailRows');
      var issuesViewOpen = false;
      // 04 §8 item 7: This facet / Module / Project. Selecting a wider scope
      // changes the denominator (and the rail's own claim-id basis) and
      // nothing else — the numerator per module, read straight off L8's own
      // grouping, never changes with scope.
      var issuesScope = 'facet';
      var ISSUES_RAIL_MAX_ROWS = 4;

      // issuesScopedClaimIDs answers 04 §8 item 7's "the current scope" for
      // any of the three positions, read-only over the SAME lookup maps
      // initViewer() builds for deep linking (never written here).
      function issuesScopedClaimIDs(scope) {
        if (scope === 'project') {
          var all = Object.create(null);
          Object.keys(claimToFacet).forEach(function (id) { all[id] = true; });
          return all;
        }
        if (scope === 'module') {
          var section = document.querySelector('.module-section:not([hidden]):not(.build-order-section)');
          var moduleID = section ? section.id : '';
          var ids = Object.create(null);
          Object.keys(claimToFacet).forEach(function (id) {
            if (facetToModule[claimToFacet[id]] === moduleID) { ids[id] = true; }
          });
          return ids;
        }
        return activeFacetClaimIDs();
      }

      // issuesGroupsForScope reruns L8's own pure grouping functions
      // (collectStatusGroups et al. — called, never edited) over a broader
      // claim-id set than the active facet, so the rail and the denominator
      // can answer Module/Project scope without a second data source.
      function issuesGroupsForScope(scope) {
        var ids = issuesScopedClaimIDs(scope);
        var data = lastStatusData || {};
        var ledger = findingsForActiveFacet(data.ledger_findings || [], ids);
        var lintErrors = findingsForActiveFacet(data.lint_errors || [], ids);
        var lintWarnings = findingsForActiveFacet(data.lint_warnings || [], ids);
        var groups = collectStatusGroups(
          data.readiness || offlineReadiness(), ids, ledger, lintErrors, lintWarnings,
          conformanceNotReadyIDs(ids)
        );
        // Mirrors renderStatusStrip's own severity-filter + APPROVAL RECORD /
        // Later exclusions (viewer-runtime.js, above) so the rail and the
        // denominator always describe the SAME rows the card is showing.
        var visible = groups.filter(function (g) {
          return !stripSeverityFilter || g.severity === stripSeverityFilter;
        });
        return visible.filter(function (g) {
          return (g.severity !== 'later' || stripSeverityFilter === 'later') && g.origin !== 'ledger';
        });
      }

      function issuesModuleWeights(scope) {
        var withoutLater = issuesGroupsForScope(scope);
        var byModule = {};
        var order = [];
        withoutLater.forEach(function (g) {
          var id = ownerModuleID(g);
          if (!Object.prototype.hasOwnProperty.call(byModule, id)) { byModule[id] = []; order.push(id); }
          byModule[id].push(g);
        });
        var rows = order.map(function (id) {
          return { id: id, label: moduleLabel(id), weight: uniqueClaimCount(byModule[id]) };
        });
        rows.sort(function (a, b) { return b.weight - a.weight; });
        return {
          rows: rows,
          claims: uniqueClaimCount(withoutLater),
          paths: rows.reduce(function (sum, r) { return sum + r.weight; }, 0)
        };
      }

      // issuesApplyScopeDenominator retexts the ALREADY-RENDERED weight
      // phrase (".status-group-head-note", L8's own node, moved in whole)
      // rather than re-running findingGroup — 04 §8 item 7: "changing scope
      // changes the denominator and nothing else", so the numerator this
      // phrase already carries is left untouched and only the trailing "of
      // <M>" is rewritten.
      function issuesApplyScopeDenominator() {
        if (!issuesFindingsCard) { return; }
        var m = issuesModuleWeights(issuesScope).claims;
        issuesFindingsCard.querySelectorAll('.status-group-head-note').forEach(function (note) {
          var match = /^blocks (\d+) of \d+ claims? here$/.exec(note.textContent || '');
          if (!match) { return; }
          note.textContent = 'blocks ' + match[1] + ' of ' + m + ' claim' + (m === 1 ? '' : 's') + ' here';
        });
      }

      // renderIssuesRail (04 §4.9): a ranking with bars, proportional to the
      // facet's blocked-claim TOTAL (§4.9's own measured widths — retry fix
      // 4, coordinator ruling: "rail bars use the spec's rounding basis" —
      // round(N / M, 1px) against data.claims, the SAME M
      // issuesApplyScopeDenominator already writes into every group header's
      // "blocks N of M claims here" phrase (§8 item 7/9: the rail figures
      // must agree with the group headers at the same scope). §4.9's table
      // measures round(92%, 1px) for a count of 24 against a facet total of
      // 26 (24/26 = 92.3%), NOT 100% for the largest row — §2 and §4.9's
      // prose ("proportions of the largest count") describes the board's
      // words, not its pixels, and the pixels are the spec (tokens.md's
      // standing rule: a measured value wins over paraphrased prose).
      // Truncated rather than scrolled past four rows, with the same
      // double-counting caveat sentence mirrored under the mobile subtitle
      // (§5 M2) once the rail itself is gone. The caveat is DERIVED here,
      // not the fixture's hand-authored literal, so it stays correct at any
      // corpus size or scope.
      function renderIssuesRail() {
        if (!issuesRailRowsEl) { return; }
        var data = issuesModuleWeights(issuesScope);
        issuesRailRowsEl.textContent = '';
        var shown = data.rows.slice(0, ISSUES_RAIL_MAX_ROWS);
        var total = data.claims;
        shown.forEach(function (row) {
          var wrap = el('div', 'issues-rail-row');
          var labelRow = el('div', 'issues-rail-row-label');
          labelRow.appendChild(textEl('span', 'issues-rail-row-name', row.label));
          labelRow.appendChild(textEl('span', 'issues-rail-row-count', String(row.weight)));
          wrap.appendChild(labelRow);
          var track = el('div', 'issues-rail-bar-track');
          var fill = el('div', 'issues-rail-bar-fill');
          fill.style.width = (total ? Math.round((row.weight / total) * 100) : 0) + '%';
          track.appendChild(fill);
          wrap.appendChild(track);
          issuesRailRowsEl.appendChild(wrap);
        });
        if (data.rows.length > ISSUES_RAIL_MAX_ROWS) {
          issuesRailRowsEl.appendChild(textEl('p', 'issues-rail-more',
            (data.rows.length - ISSUES_RAIL_MAX_ROWS) + ' more not shown'));
        }
        var caveat = data.claims > 0
          ? (countLabel(data.claims, 'claim') + ' blocked, ' + countLabel(data.paths, 'path') +
             ' — a claim blocked through two modules counts under both.')
          : '';
        if (issuesRailCaveatEl) { issuesRailCaveatEl.textContent = caveat; }
        if (issuesCaveatEl) { issuesCaveatEl.textContent = caveat; }
        // §8 item 5: an empty ranking is a ranking of nothing — the panel is
        // OMITTED, not rendered empty.
        if (issuesRailPanelEl) { issuesRailPanelEl.hidden = data.rows.length === 0; }
      }

      // issuesSyncHeader reads the breadcrumb (04 §4.1) and subtitle
      // (§4.2) off the SAME active-module/active-facet DOM the rest of the
      // reading view already maintains — read-only, no new state.
      function issuesSyncHeader() {
        var moduleTab = document.querySelector('.sec-tab.on .sec-tab__label');
        var activeSection = document.querySelector('.module-section:not([hidden]):not(.build-order-section)');
        var subtab = activeSection && activeSection.querySelector('.subtab.on .sec-tab__label');
        if (issuesBreadcrumbFacetEl) {
          issuesBreadcrumbFacetEl.textContent = moduleTab ? moduleTab.textContent.trim() : '';
        }
        if (issuesBreadcrumbSuffixEl) {
          issuesBreadcrumbSuffixEl.textContent = subtab ? ('· ' + subtab.textContent.trim()) : '';
        }
        if (issuesSubtitleEl) {
          var groups = collectStatusGroups(
            (lastStatusData && lastStatusData.readiness) || offlineReadiness(),
            activeFacetClaimIDs(), [], [], [], []
          );
          issuesSubtitleEl.textContent = blockerHeadline(groups) || 'Nothing in this facet is blocked.';
        }
      }

      // issuesSyncFromStrip is the one function that touches L8's DOM: it
      // MOVES (appendChild on an already-mounted node detaches it from its
      // old parent) #statusStripBody's current children — the severity
      // filter row and every .status-group / .status-strip-static-note —
      // into this view's own containers, in place of the two containers'
      // stale content from the previous sync. The MutationObserver declared
      // below calls this again every time renderStatusStrip (L8's function)
      // repaints #statusStripBody, so a poll, a severity-chip click or a
      // facet change all keep this view's copy live without this file ever
      // calling into or editing renderStatusStrip itself.
      //
      // It disconnects that observer before moving anything and reconnects
      // after: appendChild-ing stripBody's OWN children elsewhere is ALSO a
      // childList mutation of stripBody (a removal), and the observer would
      // otherwise queue a second, self-triggered call for it — which runs as
      // a microtask AFTER this synchronous function has already returned, by
      // which point stripBody is genuinely empty (this function just moved
      // everything out of it), so that second call would find "no groups"
      // and overwrite the correct result just produced with the empty-state
      // message. A plain re-entrancy flag cannot fix this: it would already
      // be reset by the time the queued microtask ran.
      function issuesSyncFromStrip() {
        if (!stripBody || !issuesFindingsCard || !issuesFiltersHost) { return; }
        if (issuesStripObserver) { issuesStripObserver.disconnect(); }
        issuesFiltersHost.textContent = '';
        issuesFindingsCard.textContent = '';
        Array.prototype.slice.call(stripBody.children).forEach(function (node) {
          if (node.classList && node.classList.contains('status-strip-filters')) {
            issuesFiltersHost.appendChild(node);
          } else {
            issuesFindingsCard.appendChild(node);
          }
        });
        // 04 §8 items 5/6: nothing blocked in this facet, or a severity
        // filter that matches nothing — state it in one line rather than
        // showing zero groups silently. Never hide the filter strip that
        // caused an empty result (§8 item 6).
        if (!issuesFindingsCard.querySelector('.status-group')) {
          issuesFindingsCard.appendChild(textEl('p', 'issues-findings-empty', stripSeverityFilter
            ? 'No open findings match this filter in this facet.'
            : 'Nothing in this facet is blocked.'));
        }
        issuesApplyScopeDenominator();
        renderIssuesRail();
        if (issuesStripObserver) { issuesStripObserver.observe(stripBody, { childList: true }); }
      }

      function setIssuesScope(scope) {
        issuesScope = scope;
        if (issuesScopeEl) {
          issuesScopeEl.querySelectorAll('.issues-scope-seg').forEach(function (btn) {
            btn.setAttribute('aria-pressed', String(btn.getAttribute('data-scope') === scope));
          });
        }
        issuesApplyScopeDenominator();
        renderIssuesRail();
      }

      // #systemFacetToc (group 02/system-record.js's right-hand claim list,
      // "ON THIS FACET") is `position: fixed` — a sibling of .layout, not a
      // descendant of .content-area — so hiding .content-area alone leaves
      // it floating over this view's own right rail at the same screen
      // edge. Toggling its OWN `hidden` property from here does not stick:
      // system-record.js's layout MutationObserver (its own documented,
      // pre-existing re-render-on-any-.layout-mutation behaviour — see
      // renderToc/updateTocActive) reacts to the very childList mutations
      // issuesSyncFromStrip makes inside #issuesView (a .layout descendant)
      // and calls renderToc() again on the next frame, which sets
      // `toc.hidden = false` unconditionally whenever a facet is active —
      // undoing this lane's `= true` almost immediately. A CSS rule keyed
      // off #issuesView's own hidden state (below, this lane's appended
      // section) is not fighting that same reactive loop and always wins.

      function openIssuesView() {
        if (!issuesViewEl) { return; }
        // issuesSyncFromStrip MOVES #statusStripBody's children in here, so
        // those nodes live in exactly ONE place at a time. Re-entering while
        // this view is already open would clear both hosts and then find
        // stripBody already empty, printing "Nothing in this facet is
        // blocked." beside a rail that is still counting the real blockers.
        if (issuesViewOpen) { return; }
        var contentArea = document.querySelector('.content-area');
        if (contentArea) { contentArea.hidden = true; }
        issuesViewEl.hidden = false;
        issuesViewOpen = true;
        // The reading view this replaces can be scrolled deep into an
        // 828-claim facet; this view has its own internal scroll region
        // (.issues-body) and starts at its own top regardless of where the
        // page the reader is leaving happened to be.
        window.scrollTo(0, 0);
        issuesSyncHeader();
        issuesSyncFromStrip();
        issuesViewEl.focus({ preventScroll: true });
      }

      // closeIssuesView is this screen's only exit besides ordinary
      // navigation (a sidebar tab, a citation link — both close it via the
      // capture-phase listener below): R09.6 puts no second control on the
      // boards, so the breadcrumb's own back chevron is this lane's choice
      // for "leave the way you came".
      function closeIssuesView() {
        if (!issuesViewEl || issuesViewEl.hidden) { return; }
        issuesViewEl.hidden = true;
        issuesViewOpen = false;
        var contentArea = document.querySelector('.content-area');
        if (contentArea) { contentArea.hidden = false; }
        // Hand the status strip back the children issuesSyncFromStrip MOVED
        // out of it. Leaving #statusStripBody drained has two visible costs:
        // the reading view shows an EXPANDED but blank strip until the next
        // renderStatusStrip repaints it, and the next openIssuesView syncs
        // from an empty source and renders the empty state while
        // renderIssuesRail — which reads lastStatusData, not the DOM — still
        // reports the true count. That is one screen saying both "Nothing in
        // this facet is blocked." and "26 claims blocked, 59 paths".
        //
        // issuesViewOpen is already false, so the observer callback these
        // moves would queue is a no-op; it is disconnected around them
        // anyway, for the same reason issuesSyncFromStrip disconnects.
        if (!stripBody || !issuesFiltersHost || !issuesFindingsCard) { return; }
        if (issuesStripObserver) { issuesStripObserver.disconnect(); }
        Array.prototype.slice.call(issuesFindingsCard.children).forEach(function (node) {
          // This view's OWN node, never the strip's — it must not travel back.
          if (node.classList && node.classList.contains('issues-findings-empty')) {
            node.remove();
            return;
          }
          stripBody.appendChild(node);
        });
        // renderStatusStrip writes the filter row ahead of every group, so
        // restore it to that same position: a later re-open must be
        // indistinguishable from a first.
        var filterRow = issuesFiltersHost.firstElementChild;
        if (filterRow) {
          var firstGroup = stripBody.querySelector('.status-group');
          if (firstGroup) { stripBody.insertBefore(filterRow, firstGroup); }
          else { stripBody.appendChild(filterRow); }
        }
        if (issuesStripObserver) { issuesStripObserver.observe(stripBody, { childList: true }); }
      }

      if (issuesBackBtn) {
        issuesBackBtn.addEventListener('click', closeIssuesView);
      }
      if (issuesScopeEl) {
        issuesScopeEl.addEventListener('click', function (event) {
          var btn = event.target.closest('.issues-scope-seg');
          if (btn) { setIssuesScope(btn.getAttribute('data-scope')); }
        });
      }
      // 04 §8 item 13 / §9 Open decision 9: only one sort value exists, so
      // the control is box-only — no menu to open — until a second value
      // does. It is left focusable and pressable rather than disabled (04
      // §8 item 12 reasons the same way about a zero-count severity chip).
      if (issuesSortControl) {
        issuesSortControl.addEventListener('click', function (event) { event.preventDefault(); });
      }
      // R09.6's "the banner is the only way in" holds for both forms. The
      // desktop band has one door, bound here; the phone card has one per
      // finding, each wired as it is built (see renderStatusStripCard)
      // because each carries its own severity filter.
      //
      // The band's door opens the screen UNFILTERED, and that is not an
      // oversight: its sentence names one finding but the band stands for the
      // whole facet, so narrowing to that one row's severity would hide the
      // others behind a filter the reader did not choose. A card row names
      // exactly what it filters to, which is what earns it the filter.
      if (stripToggle) {
        stripToggle.addEventListener('click', function () {
          stripSeverityFilter = '';
          renderStatusStrip(lastStatusData);
          openIssuesView();
        });
      }
      // Any ordinary navigation — a sidebar tab, a subtab, a citation link —
      // leaves this screen the same way the back chevron does. Capture phase
      // so it runs before the target's own handler, and a click INSIDE the
      // view (the scope control, the back button itself) never matches these
      // selectors.
      document.addEventListener('click', function (event) {
        if (!issuesViewOpen) { return; }
        var navHit = event.target.closest &&
          event.target.closest('.sec-tab, .subtab, a.claim-cite, .claim-links a[href^="#"]');
        if (navHit) { closeIssuesView(); }
      }, true);
      window.addEventListener('keydown', function (event) {
        if (event.key === 'Escape' && issuesViewOpen) { closeIssuesView(); }
      });
      // #statusStripBody is a stable node outside .layout (like #statusStrip
      // itself), so one observer registered once at load survives every SSE
      // fragment swap — nothing here needs re-arming from initViewer(). It is
      // declared here (not anonymous) because issuesSyncFromStrip itself
      // disconnects and re-observes around its own moves: appendChild-ing
      // stripBody's children elsewhere is ALSO a childList mutation of
      // stripBody, and MutationObserver callbacks run as a microtask AFTER
      // the synchronous sync already returned — a plain re-entrancy guard
      // reset at the end of that same synchronous call would already be
      // false by the time the queued callback ran, so the self-triggered
      // second sync would still fire and find stripBody freshly emptied by
      // the first one, overwriting its correct result with the empty state.
      var issuesStripObserver = (stripBody && window.MutationObserver)
        ? new MutationObserver(function () {
            if (issuesViewOpen) { issuesSyncFromStrip(); }
          })
        : null;
      if (issuesStripObserver) { issuesStripObserver.observe(stripBody, { childList: true }); }

      // ---- source-note three-line clamp ---------------------------------
      // A source's supports/does_not_support line is authored prose with no
      // natural length, and a claim carrying several long ones buries the
      // evidence it was meant to present. Each note is therefore clamped to
      // three lines with a show more/show less control — but ONLY the notes
      // that actually run past three lines get one, because a control on a
      // one-line note is the clutter the clamp exists to remove.
      //
      // "Actually runs past three lines" is not knowable when the HTML is
      // written: it moves with the viewport, the font and the reader's zoom. So
      // the server ships every note WHOLE with its button hidden (see
      // components/sources.go), and this block applies the clamp, measures, and
      // reveals the control where it is earned. Two consequences, both wanted:
      //
      //   - With no script — or on an engine without ResizeObserver — every
      //     note stays whole and no button appears. A clamp whose control
      //     cannot run would hide a citation's stated LIMIT behind dead chrome,
      //     and the limit is the half of a citation most worth reading.
      //   - Measurement waits for a real box. A note inside a closed <details>
      //     or a hidden facet has zero width and is not judged then; the
      //     observer fires when it gains one, which is the same event as the
      //     reader opening the footer, switching tabs, resizing the window, or
      //     a webfont landing late. One mechanism covers all four.
      //
      // NO LOOP, and this is the part to read before editing. Applying the
      // clamp changes the note's HEIGHT, which re-fires the observer; the guard
      // is that a decision is keyed on the note's WIDTH, which the clamp does
      // not touch. Same width, already judged, return.
      var noteObserver = null;
      var noteState = (typeof WeakMap === 'function') ? new WeakMap() : null;

      // sourceNoteParts resolves an observed body back to the three elements a
      // decision needs, or null if the markup is not the shape sources.go
      // writes. Nothing here repairs an unexpected shape — it declines it, and
      // the note keeps the whole text it already had.
      function sourceNoteParts(body) {
        var note = body.parentNode;
        if (!note || !note.classList || !note.classList.contains('claim-source-note')) { return null; }
        var btn = note.querySelector('.claim-source-note-toggle');
        if (!btn) { return null; }
        return { note: note, body: body, btn: btn };
      }

      // judgeSourceNote is only ever called with .is-clamped applied, so the
      // ordinary scrollHeight-beats-clientHeight overflow test is the whole
      // question. The 1px slack absorbs sub-pixel line heights, which otherwise
      // report a three-line note as overflowing its own three lines.
      function judgeSourceNote(p) {
        if (p.body.scrollHeight > p.body.clientHeight + 1) {
          p.btn.hidden = false;
          return;
        }
        p.note.classList.remove('is-clamped');
        p.btn.hidden = true;
        p.btn.setAttribute('aria-expanded', 'false');
      }

      function onSourceNoteResize(entries) {
        for (var i = 0; i < entries.length; i++) {
          var p = sourceNoteParts(entries[i].target);
          if (!p) { continue; }
          // A reader who opened this note owns it until they close it again.
          // Re-judging here would either yank it shut or force a flash of the
          // clamp to measure behind their back.
          if (p.btn.getAttribute('aria-expanded') === 'true') { continue; }
          // A BOX THAT HAS WIDTH IS NOT YET A BOX THAT HAS BEEN LAID OUT, and
          // judging one is how this went wrong before the guard existed. A note
          // inside a closed <details> — content-visibility: hidden — can report
          // the containing block's width while its own contents have no
          // geometry at all, and the pair that comes back then is scrollHeight
          // from the content and clientHeight of zero. That reads as "overflows
          // by 19px" for a note of a single short line, so every one-line note
          // in the document was handed a "show more" that did nothing when
          // pressed: precisely the clutter the clamp exists to remove, applied
          // to precisely the notes that never needed it.
          //
          // Requiring BOTH dimensions is the whole fix, and it needs no timer:
          // a laid-out note is at least one line tall, and skipping without
          // recording the width leaves the next delivery — the one with real
          // geometry — free to decide.
          var w = Math.round(p.body.clientWidth);
          if (w === 0 || p.body.clientHeight === 0) { continue; }
          var st = noteState.get(p.note);
          if (!st || st.width === w) { continue; }
          st.width = w;
          // Re-apply before measuring: a previous judgement at a wider box may
          // have removed the clamp, and the test above assumes it is on.
          p.note.classList.add('is-clamped');
          judgeSourceNote(p);
        }
      }

      // mountSourceNoteClamps is idempotent and re-armed from initViewer(): the
      // observer is disconnected first, so an SSE fragment swap can never leave
      // it holding detached nodes from the document it replaced.
      function mountSourceNoteClamps() {
        if (typeof window.ResizeObserver !== 'function' || !noteState) { return; }
        if (!noteObserver) { noteObserver = new window.ResizeObserver(onSourceNoteResize); }
        noteObserver.disconnect();
        var bodies = document.querySelectorAll('.claim-source-note > .claim-source-note-body');
        for (var i = 0; i < bodies.length; i++) {
          var p = sourceNoteParts(bodies[i]);
          if (!p) { continue; }
          // width -1 means "never judged", so the first observation at any real
          // width decides. The expand label is captured from the server's own
          // button text, so the two states have one author (sources.go) and
          // this script invents neither.
          noteState.set(p.note, { width: -1, expandLabel: p.btn.textContent });
          p.note.classList.add('is-clamped');
          noteObserver.observe(p.body);
        }
      }

      // ---- reachability probe -----------------------------------------
      // A relative fetch('/api/ping') with a ~1s AbortController timeout, wrapped
      // so its rejection can NEVER abort the rest of init. Success requires
      // res.ok AND a JSON content type AND body.dossierx === "serve"; only then
      // do the write controls mount. On file:// the fetch rejects and is
      // swallowed — the read-only panel is the correct, intended outcome.
      function probeAndMount() {
        // A file:// page has no server to ask, and a relative fetch there does
        // not merely fail quietly: it logs net::ERR_FILE_NOT_FOUND as a console
        // error on a document whose whole premise is that it works offline with
        // no network. Ask the protocol instead of asking the network.
        var proto = (window.location && window.location.protocol) || '';
        if (proto !== 'http:' && proto !== 'https:') { return; }
        var controller = null;
        var timer = null;
        try { controller = new AbortController(); } catch (e) { controller = null; }
        var opts = { headers: { 'Accept': 'application/json' } };
        if (controller) {
          opts.signal = controller.signal;
          timer = window.setTimeout(function () {
            try { controller.abort(); } catch (e) { /* ignore */ }
          }, 1000);
        }
        fetch('/api/ping', opts).then(function (res) {
          if (timer) { window.clearTimeout(timer); }
          var ct = res.headers.get('Content-Type') || '';
          if (!res.ok || ct.indexOf('application/json') === -1) { return null; }
          return res.json();
        }).then(function (body) {
          if (body && body.dossierx === 'serve') {
            mounted = true;
            document.body.classList.add('comments-live');
            // A reachable comment API is exactly what makes "add the first
            // comment" a real action, so this is where the zero-thread chips the
            // server rendered hidden become visible. It must follow `mounted =
            // true` — syncEmptyChips reads it.
            syncEmptyChips();
            // Same reasoning for the status strip: /api/status only exists
            // behind a live serve, so this is the first moment the project's
            // lint health and lock-ledger verdict can be known at all. It must
            // follow `mounted = true` — refreshStatus reads it.
            refreshStatus();
            // A confirmed live serve is the ONLY place live reload is wired: the
            // EventSource needs a server behind the page. startLiveReload is
            // idempotent and no-ops on a hook-less (marker-absent) shell.
            startLiveReload();
            // If a panel was opened during the (brief) probe window, re-render it
            // now with the live controls.
            if (currentClaimID) { renderPanel(currentClaimID); }
          }
        }).catch(function () {
          if (timer) { window.clearTimeout(timer); }
          // file:// or unreachable — swallow. Controls simply never mount.
        });
      }

      // ================================================================
      // SSE live reload (Phase 5c) — fragment-swap without losing view state
      // ================================================================
      //
      // Against a confirmed live serve the viewer opens an EventSource on
      // /api/events and, on each "changed", re-fetches the two swap subtrees from
      // /api/fragment and replaces <main class="content-area"> and <nav id="nav">
      // in place, then re-runs the idempotent initViewer() against the fresh DOM.
      //
      // Crucially this is the RESTORE-VIEW path, deliberately SEPARATE from a
      // deep-link jump: it re-applies the CURRENT module/facet from the hash but
      // SKIPS showModuleFacet's scrollIntoView+highlight block, restores the
      // reader's captured scroll position, and re-opens an open comment panel by
      // its claim/thread id. Re-running the deep-link path on every server tick
      // would yank the viewport out from under the reader — so it must not.
      //
      // The handler attaches ONLY when the effective shell carries the runtime
      // marker: a hook-less shell.html override (marker absent) is served
      // read-only and must not attempt live reload. To bound the HTTP/1.1
      // six-connections-per-host limit across many open tabs, the stream is
      // closed when the tab is hidden or unloading and reopened when it is shown.

      var sse = null;               // the live EventSource, or null when closed
      var liveReloadAttached = false;
      var reloadInFlight = false;   // a fragment fetch + swap is running
      var reloadPending = false;    // a "changed" arrived mid-swap; re-fetch after

      // hasViewerRuntime mirrors render.ShellHasViewerRuntime on the client: the
      // default shell tags <head> with the viewer-runtime marker <meta> (its
      // content is "comments-sse"), and a custom shell.html override that drops
      // that tag is served read-only. The client detects the tag by its CONTENT
      // value on purpose — not by the name token the server scans for — so that
      // marker string still occurs in exactly ONE place (the tag itself). If this
      // check duplicated that token, the server's bytes.Contains scan would find
      // it here too and never recognize a stripped-marker override as read-only.
      function hasViewerRuntime() {
        return !!document.querySelector('meta[content="comments-sse"]');
      }

      // startLiveReload wires the EventSource + tab-visibility lifecycle exactly
      // once. It is called only from the probe success path, so it runs only
      // against a confirmed live serve; it no-ops (read-only degradation) when the
      // shell lacks the runtime marker or the browser has no EventSource. The
      // comments-livereload body class records the attach DECISION synchronously
      // (so a marker-absent shell is observably NOT wired), distinct from
      // comments-sse-open which marks the stream actually connecting below.
      function startLiveReload() {
        if (liveReloadAttached) { return; }
        if (!hasViewerRuntime()) { return; }              // hook-less override: no live reload
        if (typeof window.EventSource !== 'function') { return; }
        liveReloadAttached = true;
        document.body.classList.add('comments-livereload');
        openSSE();
        document.addEventListener('visibilitychange', function () {
          if (document.visibilityState === 'hidden') {
            closeSSE();
          } else {
            // Re-open the stream AND catch up. While hidden the stream was closed,
            // so any change in that window was broadcast to no subscriber, and the
            // reconnect below replays nothing (the server only re-sends
            // ": connected" on subscribe). Fire one onServerChanged() to re-fetch
            // /api/fragment and fold in whatever changed while we were away — a
            // fragment fetch always reads the freshest render, so this is a full
            // catch-up regardless of the (now closed) SSE having missed the signal.
            openSSE();
            onServerChanged();
          }
        });
        // pagehide fires on navigation-away / bfcache entry; drop the stream so a
        // backgrounded page stops holding one of the six per-host connections.
        window.addEventListener('pagehide', closeSSE);
      }

      function openSSE() {
        if (sse) { return; }                              // already open (idempotent)
        try {
          sse = new EventSource('/api/events');
        } catch (e) {
          sse = null;
          return;
        }
        // onopen marks the stream live only AFTER the server has registered this
        // subscriber (handleEvents subscribes before writing its response head),
        // so a change made once comments-sse-open is set is guaranteed delivered.
        sse.onopen = function () {
          document.body.classList.add('comments-sse-open');
        };
        sse.addEventListener('changed', function () { onServerChanged(); });
        // EventSource reconnects transient drops itself (readyState CONNECTING);
        // only a permanently CLOSED stream is nulled so a later visibility reopen
        // can re-create it.
        sse.onerror = function () {
          if (sse && sse.readyState === EventSource.CLOSED) {
            sse = null;
            document.body.classList.remove('comments-sse-open');
          }
        };
      }

      function closeSSE() {
        if (!sse) { return; }
        try { sse.close(); } catch (e) { /* ignore */ }
        sse = null;
        document.body.classList.remove('comments-sse-open');
      }

      // onServerChanged fetches the fresh fragment and swaps it in. Overlapping
      // "changed" events (rare — the watcher debounces and the hub channel is
      // capacity-1) are coalesced into one in-flight swap plus at most one
      // trailing re-fetch, so two events can never interleave two swaps.
      function onServerChanged() {
        // Re-poll the strip on every tick, independently of the fragment swap.
        // A claim file changing on disk is EXACTLY the event that turns a sound
        // ledger into a disputed one — hand-editing a locked claim is the tamper
        // the gate exists to catch — so the verdict has to be re-read whenever
        // the claims do. It is a separate request on purpose: it must still run
        // when the fragment fetch is coalesced away below, and a failing status
        // poll must not abort the re-render (or the reverse).
        refreshStatus();
        if (reloadInFlight) { reloadPending = true; return; }
        reloadInFlight = true;
        apiGet('/api/fragment').then(function (frag) {
          applyFragment(frag);
        }).catch(function () {
          // A failed fragment fetch (server briefly away mid-reload) leaves the
          // current DOM intact; the next changed (or a manual refresh) recovers.
        }).then(function () {
          reloadInFlight = false;
          if (reloadPending) { reloadPending = false; onServerChanged(); }
        });
      }

      // applyFragment swaps the two subtrees and restores the view. ESCAPING
      // CONTRACT: the outerHTML sinks below receive ONLY the server-rendered
      // fragment from /api/fragment (frag.content / frag.nav) — the very same
      // trusted HTML the initial page shipped, produced by the one server-side
      // renderer — which is exactly the second permitted innerHTML-family value
      // alongside an API body_html. No user-derived string is ever assigned here.
      function applyFragment(frag) {
        if (!frag || typeof frag.content !== 'string' || typeof frag.nav !== 'string') { return; }
        var oldContent = document.querySelector('.content-area');
        var oldNav = document.getElementById('nav');
        if (!oldContent || !oldNav) { return; }

        // ---- capture RESTORE-VIEW state BEFORE the swap ----
        // The default desktop layout scrolls the WINDOW (the sidebar is sticky and
        // .content-area is flex:1 with no overflow of its own), so the reader's
        // scroll lives on the document; a template override or the mobile layout
        // could scroll .content-area instead, so BOTH are captured and restored.
        var savedWinScroll = window.pageYOffset || document.documentElement.scrollTop || 0;
        var savedContentScroll = oldContent.scrollTop;
        var reopenClaimID = commentPanelOpen() ? currentClaimID : null;
        // Browser scroll anchoring reacts to the temporary height difference
        // between the old enhanced subtree and the fresh static fragment. Hold
        // it off until the readiness panel has been restored and layout has
        // settled, then put the reader back at the exact captured coordinate.
        var htmlRoot = document.documentElement;
        var savedOverflowAnchor = htmlRoot.style.overflowAnchor;
        htmlRoot.style.overflowAnchor = 'none';

        // ---- swap both subtrees (server fragment — a permitted sink) ----
        oldContent.outerHTML = frag.content;   // replaces <main class="content-area">
        oldNav.outerHTML = frag.nav;           // replaces <nav id="nav">

        // The Build order tab's renderer (the vendored mermaid build and
        // build-order-ui.js) sits OUTSIDE .content-area and is only emitted
        // for a project with a locked order, so a fragment swap cannot deliver
        // it: a project that locks its FIRST build order while this page is
        // open receives a .build-order-section with no renderer. One full
        // reload, once, on that zero-to-one transition; every later swap has
        // the renderer and takes the observer path. It is a one-time event
        // only because the section is inside the same guard as the scripts.
        // The test is the section's CLASS, which no module section carries,
        // never its id: a module's section id is slugify(module), and a
        // module named "build-order" once matched a getElementById here on
        // every swap, turning each one into a full reload.
        if (document.querySelector('.build-order-section') && typeof window.mermaid === 'undefined') {
          window.location.reload();
          return;
        }

        // ---- re-point every lookup map at the fresh DOM ----
        initViewer();

        // ---- RESTORE view (NOT a deep-link jump) ----
        // Re-apply the current module/facet from the hash, but pass NO claim in
        // opts, so showModuleFacet's scrollIntoView+highlight block does not run
        // and the viewport is not yanked. skipHash keeps the URL untouched.
        var target = resolve(hashId());
        showModuleFacet(target.module, target.facet, { skipHash: true });
        // Small corpora remount every surface after a swap so hidden-facet
        // witnesses (live-reload tests, cross-facet getElementById) keep
        // working. Large corpora stay soft-mounted: only the active surface
        // above is live, matching first-paint behaviour.
        if (!softMountEnabled()) { mountAllSurfaces(); }
        // initViewer ran syncEmptyChips before hosts were filled; re-run now
        // that claim cards (and their zero-thread chips) are in the live DOM.
        syncEmptyChips();
        if (typeof window.dossierxEnhanceSystemRecord === 'function') {
          window.dossierxEnhanceSystemRecord();
        }
        // refreshStatus races the fragment fetch; if it painted the old DOM
        // before this swap, re-apply the last verdict onto the fresh cards.
        if (lastStatusData) {
          // renderStatusStrip rebuilds the readiness doors itself, with the
          // banner state they have to be gated on. Calling renderClaimReadiness
          // separately here would paint them once un-gated first.
          renderStatusStrip(lastStatusData);
        }

        // ---- restore the reader's scroll AFTER the active section is un-hidden
        // above, so the document has regained its full height ----
        var freshContent = document.querySelector('.content-area');
        if (freshContent && savedContentScroll) { freshContent.scrollTop = savedContentScroll; }
        function restoreWindowScroll() {
          // html uses smooth scrolling for reader navigation, but restoration
          // is state, not navigation. Applying a smooth scroll here lets a
          // taller replacement subtree leave the reader between positions
          // when the refresh completes. Disable it for this synchronous write.
          var rootScrollBehavior = htmlRoot.style.scrollBehavior;
          htmlRoot.style.scrollBehavior = 'auto';
          window.scrollTo(0, savedWinScroll);
          htmlRoot.style.scrollBehavior = rootScrollBehavior;
        }
        restoreWindowScroll();
        window.requestAnimationFrame(function () {
          restoreWindowScroll();
          htmlRoot.style.overflowAnchor = savedOverflowAnchor;
        });

        // ---- re-open the panel by (claim/thread) id. The panel node itself lives
        // OUTSIDE the swapped subtree so it survived, but its chips were replaced,
        // so re-mark the fresh chips expanded and refresh the thread list (which
        // also picks up whatever change triggered this reload). ----
        if (reopenClaimID) {
          setChipExpanded(reopenClaimID, true);
          renderPanel(reopenClaimID);
        }
      }

      // ================================================================
      // Delegated listeners (attached ONCE, on surviving nodes) + init
      // ================================================================
      //
      // Tab/subtab/chip clicks are delegated on document rather than bound
      // per-element, so a later SSE fragment swap that replaces the tabs/cards
      // keeps working without re-binding (and initViewer can be re-run freely).
      document.addEventListener('click', function (e) {
        var graphTrigger = e.target.closest('[data-dxg-open]');
        if (graphTrigger) {
          if (document.body.classList.contains('dxg-open')) {
            restoreFocus(graphTrigger, '#dxgOpen');
          } else {
            graphFocusReturn = graphTrigger;
          }
        }
        if (e.target.closest('[data-dxg-close]') && graphFocusReturn) {
          restoreFocus(graphFocusReturn, '#dxgOpen');
        }
        var chip = e.target.closest('.comment-chip');
        if (chip) {
          e.preventDefault();
          var chipClaim = chip.getAttribute('data-claim-id');
          if (commentPanelOpen() && currentClaimID === chipClaim) {
            closeCommentPanel();
          } else {
            openCommentPanel(chipClaim);
          }
          return;
        }
        // The source-note show more/show less control. It carries no id (a
        // claim is rendered a second time inside any track that owns it, and
        // ids may not be), so its handle on the note it governs is the parent
        // it sits in — which is what makes both copies work independently.
        var noteToggle = e.target.closest('.claim-source-note-toggle');
        if (noteToggle) {
          var noteEl = noteToggle.parentNode;
          var opening = noteToggle.getAttribute('aria-expanded') !== 'true';
          noteToggle.setAttribute('aria-expanded', String(opening));
          noteEl.classList.toggle('is-clamped', !opening);
          var noteSt = noteState ? noteState.get(noteEl) : null;
          noteToggle.textContent = opening
            ? (noteToggle.getAttribute('data-collapse-label') || 'show less')
            : ((noteSt && noteSt.expandLabel) || 'show more');
          return;
        }
        var secTab = e.target.closest('.sec-tab');
        if (secTab) {
          var moduleID = (secTab.dataset.target || '').replace(/^#/, '');
          var facetID = (secTab.dataset.defaultTarget || '').replace(/^#/, '');
          showModuleFacet(moduleID, facetID);
          setDrawer(false);
          return;
        }
        var subtab = e.target.closest('.subtab');
        if (subtab) {
          var subFacet = (subtab.dataset.target || '').replace(/^#/, '');
          showModuleFacet(facetToModule[subFacet], subFacet);
          setDrawer(false);
          return;
        }
      });

      window.addEventListener('hashchange', function () {
        // A hash change is a request for a CLAIM in the reading view: the
        // graph pane's "back to this claim", a facet-TOC row, the browser's
        // own Back button, a shared link. The Issues screen hides
        // .content-area, so leaving it open strands the reader on Issues
        // while showModuleFacet silently re-syncs the findings card and the
        // rail to the NEW facet behind a breadcrumb and subtitle that still
        // name the old one (issuesSyncHeader runs only from openIssuesView).
        // Closing first also un-hides .content-area before showModuleFacet
        // scrolls the deep-linked card into view — scrollIntoView on a hidden
        // ancestor is a silent no-op.
        closeIssuesView();
        showFromHash({ skipHash: true });
      });

      if (navToggle) {
        navToggle.addEventListener('click', function () {
          setDrawer(!document.body.classList.contains('nav-open'), navToggle);
        });
      }
      var mobileSearchToggle = document.getElementById('mobileSearchToggle');
      if (mobileSearchToggle) {
        mobileSearchToggle.addEventListener('click', function () {
          setDrawer(true, mobileSearchToggle);
          var search = document.getElementById('navSearch');
          if (search) {
            cancelSearchFocus();
            searchFocusFrame = window.requestAnimationFrame(function () {
              searchFocusFrame = 0;
              if (document.body.classList.contains('nav-open')) { search.focus(); }
            });
          }
        });
      }
      var navSearch = document.getElementById('navSearch');
      if (navSearch) {
        navSearch.addEventListener('input', function () {
          var query = navSearch.value.trim().toLowerCase();
          document.querySelectorAll('.system-nav-group').forEach(function (group) {
            var rows = Array.prototype.slice.call(group.querySelectorAll('.sec-tab'));
            var matches = rows.filter(function (row) {
              var visible = !query || row.textContent.toLowerCase().indexOf(query) !== -1;
              row.hidden = !visible;
              return visible;
            });
            group.hidden = query !== '' && matches.length === 0;
          });
        });
      }
      if (navOverlay) {
        navOverlay.addEventListener('click', function () { setDrawer(false); });
      }
      var navDrawerClose = document.getElementById('navDrawerClose');
      if (navDrawerClose) {
        navDrawerClose.addEventListener('click', function () { setDrawer(false); });
      }
      if (commentsOverlay) {
        commentsOverlay.addEventListener('click', function () { closeCommentPanel(); });
      }
      if (railClose) {
        railClose.addEventListener('click', function () { closeCommentPanel(); });
      }

      // This file's one window keydown Escape listener: an open comment panel
      // closes first (and returns, so the same press does not also toggle the
      // nav); otherwise Escape closes the nav drawer as before.
      //
      // It is NOT the only Escape handler on the page any more. graph-ui.js
      // registers a document-level keydown that closes the graph pane, and a
      // document-level event bubbles on to window — so with the pane open one
      // press runs closePane() and then falls through to setDrawer(false) here.
      // That is harmless today (closing an already-closed drawer is a no-op),
      // but anything added to this handler that is not idempotent has to check
      // the pane's state too.
      window.addEventListener('keydown', function (event) {
        if (event.key === 'Escape') {
          if (commentPanelOpen()) { closeCommentPanel(); return; }
          var target = event.target;
          if (graphFocusReturn && target && typeof target.closest === 'function' && target.closest('#dxgPane')) {
            restoreFocus(graphFocusReturn, '#dxgOpen');
            return;
          }
          if (document.body.classList.contains('nav-open')) { event.preventDefault(); }
          setDrawer(false);
        }
      });

      // Initial render. This script tag sits at the end of <body>, so the DOM is
      // already parsed by the time this IIFE runs — no DOMContentLoaded needed.
      initViewer();
      showFromHash({ skipHash: true });
      // Soft-mount is for Curtainly-scale corpora. Ordinary fixtures mount
      // every surface up front so print, theme parity, and getElementById
      // witnesses behave as they did with eager DOM.
      if (!softMountEnabled()) { mountAllSurfaces(); }
      // Static file:// viewers never receive /api/status. Paint the assessment
      // carried by the graph payload now; a live response replaces it later.
      renderStatusStrip({ readiness: offlineReadiness() });
      probeAndMount();
    })();
