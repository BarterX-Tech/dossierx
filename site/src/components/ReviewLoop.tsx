import { motion, useReducedMotion } from "framer-motion";
import { AnimatedReveal } from "./AnimatedReveal";
import { RolePill, type Role } from "./RolePill";
import { POP } from "../sections/motion-tokens";

export interface LoopStep {
  role: Role;
  /** Where the step physically happens: a browser, a terminal, chat, disk. */
  surface: string;
  title: string;
  body: string;
  /** The one step that is the approval. Exactly one step sets this. */
  emphasis?: boolean;
}

/* ---- Visual (b): the review loop as a two-lane sequence ----
 *
 * The old version of this was a grid of six equal cards, which is the one shape
 * that cannot say the thing the loop is about: that the work ALTERNATES between
 * two parties, and that one specific step is an approval the next step depends
 * on. So: a spine down the middle, the human's steps in the left lane, the
 * agent's in the right, and the engine's spanning the middle because it belongs
 * to neither. Reading the zigzag IS reading the handoffs.
 *
 * Two beats are called out. Step 6 — the Resolve click — carries an "approval"
 * stamp, because it is the load-bearing gesture in the whole design: claim lock
 * refuses a claim with an open thread, so that click is literally what unblocks
 * locking. And the final step names --reason, because that is where the human's
 * words end up on the record.
 *
 * Motion: the spine draws top-down once on scroll-in, and each step reveals on
 * the shared AnimatedReveal with an index delay, so the sequence plays as a
 * sequence. Under prefers-reduced-motion the spine is simply present and every
 * step is at rest — useReducedMotion here, plus the global reduce block in
 * index.css for the CSS-driven pulse.
 */
export function ReviewLoop({ steps }: { steps: LoopStep[] }) {
  const reduce = useReducedMotion();

  return (
    <div className="rloop-wrap">
      {/* The spine is a sibling of the list rather than an ::before on it, so it
          can be animated by framer without touching the list's layout at all. */}
      <motion.span
        className="rloop__spine"
        aria-hidden="true"
        initial={reduce ? { scaleY: 1 } : { scaleY: 0 }}
        whileInView={{ scaleY: 1 }}
        viewport={{ once: true, margin: "0px 0px -120px 0px" }}
        transition={
          reduce
            ? { duration: 0 }
            : { duration: 1.1, ease: [0.22, 1, 0.36, 1] }
        }
      />

      <ol className="rloop">
        {steps.map((s, i) => (
          <li
            key={s.title}
            className={`rloop__step rloop__step--${s.role}${
              s.emphasis ? " rloop__step--approval" : ""
            }`}
          >
            <span className="rloop__marker" aria-hidden="true">
              <span className="rloop__num">{i + 1}</span>
            </span>

            <AnimatedReveal className="rloop__reveal" delay={0.05 * i} y={14}>
              <div className="rloop__card">
                <div className="rloop__head">
                  <RolePill role={s.role} />
                  <span className="rloop__surface">{s.surface}</span>
                </div>
                <h3 className="rloop__title">{s.title}</h3>
                <p className="rloop__text">{s.body}</p>

                {s.emphasis && (
                  <motion.span
                    className="rloop__stamp"
                    initial={
                      reduce
                        ? { opacity: 1, scale: 1 }
                        : { opacity: 0, scale: 1.25, rotate: -3 }
                    }
                    whileInView={{ opacity: 1, scale: 1, rotate: -3 }}
                    viewport={{ once: true, margin: "0px 0px -80px 0px" }}
                    transition={reduce ? { duration: 0 } : { ...POP, delay: 0.2 }}
                  >
                    this is the approval
                  </motion.span>
                )}
              </div>
            </AnimatedReveal>
          </li>
        ))}
      </ol>
    </div>
  );
}
