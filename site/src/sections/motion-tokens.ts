/* ---- Shared diagram motion tokens ----
 * One rhythm and two easing curves drive both the Lifecycle and BuildOrder
 * diagrams so they read as a single motion system:
 *   - anything that APPEARS springs in on POP (nodes, dots, labels)
 *   - anything that DRAWS uses pathLength on the site's signature cubic (DRAW)
 *   - terminal dots settle on the snappier SETTLE spring as the line arrives
 * BEAT is the shared unit of narrative time; every element is placed on an
 * explicit integer beat so the sequence is deterministic, not index-magic. */
export const BEAT = 0.26;

// Site signature spring — for anything that appears.
export const POP = { type: "spring", stiffness: 300, damping: 26 } as const;

// Snappier spring — for terminal dots landing at the end of a drawn line.
export const SETTLE = { type: "spring", stiffness: 500, damping: 22 } as const;

// Signature decelerating cubic — for pathLength draws.
export const DRAW = { duration: 0.5, ease: [0.22, 1, 0.36, 1] } as const;

// Same cubic, slightly longer — for text labels naming a transition.
export const LABEL = { duration: 0.35, ease: [0.22, 1, 0.36, 1] } as const;
