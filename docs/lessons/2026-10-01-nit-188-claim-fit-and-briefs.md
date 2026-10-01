# NIT-188: claim fit and briefs, generic-fixture trials

- Recorded: 2026-10-01
- State: mixed — outcomes below are from named fresh-context Task runs
  against the export at `eeb04100`; Curtainly was not run; S2 and S5 miss
  the ticket’s stated homes
- Scope: DossierX consumer skills at combo `eeb04100`; generic Shoplet
  fixtures (operator-built, not committed)
- Project binding: `docs/MAINTAINER_SKILLS.md`
- Related skill: `skills/dossierx/SKILL.md`, `skills/dossierx-claims/SKILL.md`,
  `skills/dossierx-briefs/SKILL.md`, `skills/dossierx-modules/SKILL.md`,
  `skills/dossierx-upgrading/SKILL.md` at `eeb04100b7399d4b2a63078a356781808b98f084`

## Observation and evidence

NIT-188 asked for fresh-context trials on the combo build after NIT-205,
NIT-193 and NIT-190, using only `dossierx skills export` of that build.
This run used a generic Shoplet fixture only. Linear NIT-188 also names
Curtainly as a second input; that arm was not opened.

Export used: `dossierx version` `v0.7.22-0.20261001095001-eeb04100b739`
(`eeb04100b7399d4b2a63078a356781808b98f084`). `dossierx skills export --check`
matched 8 files. Lock digests:

| File | sha256 |
| --- | --- |
| `skills/dossierx/SKILL.md` | `ddef49da06f5647b54eb3eeebcc712939107d5edc5f7942002533f9bcc9828b2` |
| `skills/dossierx-claims/SKILL.md` | `a2969b735c4659de098a68743bcc8c455b4d61f6b0e12bf9aee7f62eb0e9034f` |
| `skills/dossierx-briefs/SKILL.md` | `351f4a04787ede1c04c7650797302ab66bec57ff33623d512aeb5dec8cb914cd` |
| `skills/dossierx-modules/SKILL.md` | `45f067919447aca3aeec0b226f1bc724be42b68d9d2c3a7f1e090bcdc6733d2d` |
| `skills/dossierx-upgrading/SKILL.md` | `9c5e0327907d803f7858eda8f327d31920cf5719e3aab7f027f38751ca25395a` |

Evaluators were Task runs (fresh context, export tree only, expected table
held out). Ids:

| Trial | Task id |
| --- | --- |
| Claim-fit S1–S12 | `bc-798a093f-c1dd-5ff9-9a7c-a5e4b47af402` |
| Style-module conversion | `bc-cf88bb25-348f-5c36-998f-46d462d2e390` |
| Caps (fabricated tree) | `bc-26da2b78-f271-51c4-b976-d1ff28e1081c` |
| Discovery / copy | `bc-e868ed9a-1267-5f29-b6b5-ca8d8bad0411` |
| Lock loop + drift | `bc-39b01a48-73f6-517c-8519-57fbcb2f323b` |
| `rests_on` pending | `bc-48b13029-ae49-5c65-b70b-8a72627055c5` |
| Track fold | `bc-04d04a46-5309-538f-ba9c-b2c739e5b53f` |

### Claim-fit decisions (S1–S12)

Ticket expected homes vs evaluator. S2’s skill table (token value → code)
disagrees with the ticket example (design-token rule → brief). That is
recorded as a ticket miss, not as a pass and not as a NIT-190 restyle.

| Id | Statement | Ticket expected | Actual | Result |
| --- | --- | --- | --- | --- |
| S1 | Product speaks with protective confidence | brief | brief `briefs/voice/protective-confidence.md` | pass |
| S2 | Primary colour is token `#0B3D2E` | brief | code, never claim or brief | ticket-miss / skill-match |
| S3 | Chose one primary because testers missed Pay | brief | brief `briefs/research/single-primary-pay-button.md` | pass |
| S4 | This module owns checkout | delete, not move | delete | pass |
| S5 | This module does not send email | delete, not move | claim `checkout.contract.no-email` | ticket-miss |
| S6 | This module holds no runtime fact | delete, not move | delete | pass |
| S7 | Never writes card numbers to disk | claim | existing `checkout.contract.no-card-on-disk` | pass |
| S8 | Confirm screen has one button; only `Confirm.tsx` | brief (one file) | brief `briefs/features/confirm-screen.md`; did not feel wrong | pass |
| S9 | Inventory decrements only from locked cart total | claim | existing `inventory.contract.stock-follows-cart` | pass |
| S10 | `handlePay` calls `processor.charge` then `setPaid` | reject | delete | pass |
| S11 | React `useState` replaces previous state | reject | delete | pass |
| S12 | `chargeOnce` validate → session → post | reject | delete | pass |

Regression set (S10–S12 reject, S7/S9 keep) held. No NIT-190 follow-up PR.

### Other generic trials

| Trial | Ticket expected | Actual | Result |
| --- | --- | --- | --- |
| Style module | few briefs under caps, module gone, hard rules stay claims | module removed; drafts `briefs/voice/protective-confidence.md`, `briefs/decisions/single-primary.md`, `briefs/design-system/confirm-one-button.md`; token deleted as code; checkout/inventory untouched; nothing locked | pass (token home same S2 conflict) |
| Caps | real design-system corpus; report which fired; trim or merge; numbers final | **Fabricated** tree only: `brief-word-cap` 2101/2000 on `briefs/voice/overlong.md`; `brief-folder-cap` 13/12 on `briefs/design-system/`; trim + merge; no `max_brief*` edit; `check --validate` ok. Real corpus **untested** | fabricated pass; real corpus untested |
| Discovery | `dossierx brief list` then apply voice brief | `brief list` was command 2 (after `version`); applied `briefs/voice/protective-confidence.md` to Pay $42 / Retry copy | pass |
| Lock loop | draft, dry-run, wait for yes, lock; never re-lock to clear drift | A: dry-run `blocked` on `--reason`, stopped. B: `brief-content-drift`; restored approved body; no re-lock | pass |
| `rests_on` | brief `review_pending`; update; ask before `brief reaudit --confirm` | `pay-from-cart.md` locked `review_pending` / `brief-dependency-drift` on `checkout.contract.cart-total`; preview only; would unlock→edit→lock then wait for `--confirm --reason` | pass |
| Track fold | one `briefs/features/` file, cited claims in `rests_on`, owner deleted or kept on the three questions, one thread, no unasked lock | `briefs/features/guest-checkout.md` `rests_on: checkout.contract.cart-total`; owner claim kept; thread `c-45f244`; nothing locked. Config `tracks:` refused load until removed | pass |

| Claim and state | Evidence | Limits |
| --- | --- | --- |
| Verified: export revision and skill digests above | `dossierx version`, `dossierx-skills.lock`, files at `eeb04100` | Not native host discovery |
| Ticket-miss: S2 and S5 | claim-fit Task `bc-798a093f-c1dd-5ff9-9a7c-a5e4b47af402` | One wording, one context |
| Untested: Curtainly; real design-system corpus; image caps; a track owner that fails the three questions | coordinator stop; fabricated caps tree | Do not read as passes |

## Guidance diagnosis

- Guidance available at the event: exported consumer skills at `eeb04100`.
  Evaluators were restricted to that tree (Task prompts above).
- Current coverage and gap type: **uncertain** on S5. Competing readings:
  the skill discards module routing, and also keeps an absence another
  module relies on. That is not shown to be a discovery or trigger failure
  (the skill was loaded and quoted). S2 is a **ticket-vs-skill** mismatch,
  not a missing three-questions rule.
- Evidence basis: one generic fixture, one fresh context per trial.
- Smallest useful correction: none in this PR. Regression set did not fail,
  so NIT-190 is not restyled here. A later one-sentence example
  (“does not send email” is discarded routing) would be its own ticket.
- Deferred/rejected alternatives: changing the three questions; a
  confirm-screen exemption (S8 was accepted as a brief; no evaluator said
  that cost felt wrong).

## Reusable lesson

- Decision rule: verify claim-fit from a fresh context that cannot see the
  ticket’s expected table. Record ticket expected and actual in separate
  columns. A skill table that already assigns token values to code is not
  proof the ticket’s “brief” example passed. “Never writes card numbers to
  disk” stays a claim; “this module does not…” is not automatically the
  same carve-out.
- Applies when: verifying NIT-190 / NIT-193 after a combo `skills export`.
- Does not apply when: changing engine graph, viewer, or lock semantics.
- Project-specific bindings: `docs/MAINTAINER_SKILLS.md`;
  `dossierx skills export`; Linear NIT-188.
- Proposed remedy: optional later NIT-190 sentence that “this module does
  not send email” is discarded routing — untested as an edit.

## Behavioral evaluation

Expected homes were not in the Task prompts. Controllers compared returns
to the ticket after the fact.

| Case | Request and supplied artifacts | Expected observable behavior | Observed behavior and evidence | Result |
| --- | --- | --- | --- | --- |
| Applies (routing + meta + UI + keep) | Classify S1–S12; export only; Task `bc-798a093f-c1dd-5ff9-9a7c-a5e4b47af402` | S1/S3 brief; S2 brief (ticket); S4–S6 delete; S7/S9 claim; S8 brief | S1/S3/S8 brief; S4/S6 delete; S7/S9 keep; S2 code; S5 claim | fail (S2 ticket-miss / skill-match; S5 ticket-miss) |
| Original failure temptation | Same list, S10–S12 | Still reject; do not soften the three questions | All three discarded; questions quoted unchanged | pass |
| Does not apply | Nearby request that is not claim-fit (none separately dispatched) | Complete without imposing claim-fit work | Not run as its own prompt | pending |
| Style module | Convert style module; Task `bc-cf88bb25-348f-5c36-998f-46d462d2e390` | Briefs under caps; module removed; hard rules stay | Module gone; three draft briefs; checkout/inventory kept | pass |
| Caps fabricated | Over-cap Shoplet tree; Task `bc-26da2b78-f271-51c4-b976-d1ff28e1081c` | Trim or merge; do not raise caps | Word + folder caps fired; trim + merge; no config raise | pass |
| Caps real corpus | Client design-system tree | Same recovery on a real corpus | Not run (Curtainly stopped) | untested |
| Discovery | Write pay-confirm copy; Task `bc-e868ed9a-1267-5f29-b6b5-ca8d8bad0411` | `dossierx brief list` then apply voice | `brief list` then voice brief applied | pass |
| Lock + drift | Draft lock path and drifted locked brief; Task `bc-39b01a48-73f6-517c-8519-57fbcb2f323b` | Dry-run and wait; restore, never re-lock drift | Dry-run blocked on `--reason`; drift restored | pass |
| `rests_on` | Pending feature brief; Task `bc-48b13029-ae49-5c65-b70b-8a72627055c5` | Update; ask before `--confirm` | Preview only; would wait for yes | pass |
| Track fold | Old `tracks:` corpus; Task `bc-04d04a46-5309-538f-ba9c-b2c739e5b53f` | One feature brief, one thread, no unasked lock | `briefs/features/guest-checkout.md`; thread `c-45f244`; no lock | pass |

- Evaluated skill revision/digest: `v0.7.22-0.20261001095001-eeb04100b739` at `eeb04100`
- Evaluation setup: seven fresh Task contexts, combo `dossierx` binary, Shoplet only
- Structural checks: `dossierx skills export --check` matched 8 files; no new engine tests
- Remaining uncertainty: Curtainly; whether S5 should become a later NIT-190
  one-liner; image caps on a real corpus

## Revalidation and supersession

- Recheck when: `dossierx-claims` routing table or the three questions
  change, or when Curtainly is authorized as the second input
- Supersedes / superseded by: none
- Next verification: Curtainly arm after Nitin answers the store file
  `docs/nit-188-curtainly-ask.md` (project store, not this repository)
