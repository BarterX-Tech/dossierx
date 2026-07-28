import { motion, useReducedMotion } from "framer-motion";
import { AnimatedReveal } from "./AnimatedReveal";
import { DRAW } from "../sections/motion-tokens";

export interface Surface {
  id: "human" | "agent";
  role: string;
  who: string;
  surface: string;
  command: string;
  commandNote: string;
  can: string[];
  cannot: string[];
  footnote: string;
}

export interface Boundary {
  label: string;
  crossings: { from: "human" | "agent"; text: string; note: string }[];
}

/* ---- Visual (a): the release's core idea, drawn once ----
 *
 * Two panels, one per role, with the boundary between them made literal: a
 * dashed rule down the middle carrying the words "the approval boundary", and
 * beneath it the only two things that legitimately cross it — a sentence in
 * chat each way.
 *
 * It is DOM + CSS rather than one big <svg> on purpose. The content is two
 * lists of prose that must reflow, wrap, be selectable, and be read by a screen
 * reader in a sane order; an SVG would have frozen the line breaks at one
 * viewport width and turned nine sentences into <text> nodes. The only drawn
 * element is the rule itself, which is a 1px line and needs no vector at all.
 *
 * Human is warm (--accent), agent is green (--ok) — the same two hues the
 * viewer's author-role pills and this site's .role-pill already use for the same
 * two actors, so the mapping is learned once and holds everywhere.
 */

/** The affirmative marker on a "Can" row. */
function CanMark() {
  return (
    <svg
      className="surface__mark"
      viewBox="0 0 14 14"
      fill="none"
      aria-hidden="true"
    >
      <path
        d="M2.5 7.5 5.5 10.5 11.5 3.5"
        stroke="currentColor"
        strokeWidth="1.9"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

/** The negative marker on a "Cannot" row — a circle-slash, not an ✗, because
 * these are things the surface does not OFFER rather than things that failed. */
function CannotMark() {
  return (
    <svg
      className="surface__mark"
      viewBox="0 0 14 14"
      fill="none"
      aria-hidden="true"
    >
      <circle cx="7" cy="7" r="5" stroke="currentColor" strokeWidth="1.6" />
      <path
        d="M3.8 10.2 10.2 3.8"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
      />
    </svg>
  );
}

function SurfacePanel({ surface, delay }: { surface: Surface; delay: number }) {
  return (
    <AnimatedReveal delay={delay} y={18}>
      <article className={`surface surface--${surface.id}`}>
        <header className="surface__head">
          <span className="surface__role">{surface.role}</span>
          <h3 className="surface__who">{surface.who}</h3>
          <p className="surface__where">{surface.surface}</p>
          <p className="surface__cmd">
            <code>{surface.command}</code>
            <span className="surface__cmd-note">{surface.commandNote}</span>
          </p>
        </header>

        <div className="surface__cols">
          <div className="surface__col surface__col--can">
            <span className="surface__col-label">Can</span>
            <ul>
              {surface.can.map((item) => (
                <li key={item}>
                  <CanMark />
                  <span>{item}</span>
                </li>
              ))}
            </ul>
          </div>
          <div className="surface__col surface__col--cannot">
            <span className="surface__col-label">Cannot</span>
            <ul>
              {surface.cannot.map((item) => (
                <li key={item}>
                  <CannotMark />
                  <span>{item}</span>
                </li>
              ))}
            </ul>
          </div>
        </div>

        <p className="surface__foot">{surface.footnote}</p>
      </article>
    </AnimatedReveal>
  );
}

export function TwoSurfaces({
  surfaces,
  boundary,
}: {
  surfaces: Surface[];
  boundary: Boundary;
}) {
  const reduce = useReducedMotion();
  const human = surfaces.find((s) => s.id === "human");
  const agent = surfaces.find((s) => s.id === "agent");
  if (!human || !agent) return null;

  return (
    <figure
      className="surfaces"
      aria-label="The two DossierX surfaces: a human reviewer in the viewer, and an agent operator on the CLI, separated by the approval boundary"
    >
      <div className="surfaces__grid">
        <SurfacePanel surface={human} delay={0} />

        {/* The boundary. Its line is drawn (scaleY from the top) so the divider
            arrives as a gesture rather than being simply present — and it is a
            transform, not a height animation, so nothing around it reflows.
            The label is vertical on wide viewports and horizontal once the grid
            collapses to one column; see .surfaces__rail in index.css. */}
        <div className="surfaces__rail" aria-hidden="true">
          <motion.span
            className="surfaces__rail-line"
            initial={reduce ? { scaleY: 1 } : { scaleY: 0 }}
            whileInView={{ scaleY: 1 }}
            viewport={{ once: true, margin: "0px 0px -80px 0px" }}
            transition={reduce ? { duration: 0 } : { ...DRAW, duration: 0.7 }}
          />
          <span className="surfaces__rail-label">{boundary.label}</span>
          <motion.span
            className="surfaces__rail-line"
            initial={reduce ? { scaleY: 1 } : { scaleY: 0 }}
            whileInView={{ scaleY: 1 }}
            viewport={{ once: true, margin: "0px 0px -80px 0px" }}
            transition={reduce ? { duration: 0 } : { ...DRAW, duration: 0.7 }}
          />
        </div>

        <SurfacePanel surface={agent} delay={0.08} />
      </div>

      <figcaption className="surfaces__crossings">
        <span className="surfaces__crossings-label">
          What crosses the boundary
        </span>
        <ul>
          {boundary.crossings.map((c) => (
            <li key={c.text} className={`crossing crossing--${c.from}`}>
              {/* The arrow points the way the sentence travels: the human's line
                  runs left-to-right toward the agent, the agent's answer comes
                  back. On a single-column layout both read top-down, so the
                  glyph is decorative and the role pill carries the meaning. */}
              <span className="crossing__dir" aria-hidden="true">
                {c.from === "human" ? "→" : "←"}
              </span>
              <span className="crossing__body">
                <span className="crossing__text">{c.text}</span>
                <span className="crossing__note">{c.note}</span>
              </span>
            </li>
          ))}
        </ul>
      </figcaption>
    </figure>
  );
}
