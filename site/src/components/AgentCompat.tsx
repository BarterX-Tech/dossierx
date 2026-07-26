import { motion, useReducedMotion } from "framer-motion";
import { AnimatedReveal } from "./AnimatedReveal";
import { DRAW } from "../sections/motion-tokens";

export interface CompatData {
  title: string;
  lede: string;
  source: { label: string; detail: string };
  targets: { harness: string; path: string; note: string }[];
  harnesses: string[];
  gate: {
    label: string;
    hook: { title: string; body: string };
    ci: { title: string; body: string };
  };
}

/* ---- Visual (c): harness independence, in two movements ----
 *
 * Movement one, fanning OUT: one markdown bundle compiled into the binary, and
 * three places it can land. Nothing in DossierX detects a harness or ships an
 * adapter — the skills are markdown, and a tool that loads markdown loads them.
 *
 * Movement two, converging IN: whichever tool made the change, the change
 * arrives as a git commit, and the gate sits there. That is the whole argument
 * for putting enforcement in a pre-commit hook rather than in per-tool config:
 * six lanes, one gate, no adapters — and CI behind it, because clean merges,
 * rebases, cherry-picks, reverts and --no-verify never fire pre-commit at all.
 *
 * Every name on this page is set in the site's own type. No third-party logo is
 * fetched, embedded or redrawn: a wordmark in Plex Sans states the fact
 * ("this works with Codex") without borrowing anyone's trade dress, and it keeps
 * the artifact self-contained, which the site's zero-external-asset rule
 * requires anyway.
 *
 * The two connector fans are SVG with preserveAspectRatio="none" — the same
 * technique BuildOrder's squiggle uses — so they stretch to whatever width the
 * grid ends up at, and they are revealed with pathLength rather than a width
 * animation so nothing reflows while they draw.
 */

const FAN_OUT = [
  "M150 0 C 150 22, 50 20, 50 46",
  "M150 0 L 150 46",
  "M150 0 C 150 22, 250 20, 250 46",
];

// Six lanes in, one gate out — one lane per wordmark above it. The x positions
// are six evenly spaced points across the 300-unit box rather than an attempt to
// land under each chip: the chips are centre-flowed and wrap, so any exact
// alignment would hold at one width and be wrong at every other. What has to
// read is "many in, one out", and evenly spaced lanes say that at every width.
const FAN_IN = [25, 75, 125, 175, 225, 275].map(
  (x) => `M${x} 0 C ${x} 24, 150 22, 150 46`,
);

function ConnectorFan({
  paths,
  label,
  reduce,
}: {
  paths: string[];
  label: string;
  reduce: boolean | null;
}) {
  return (
    <svg
      className="compat__fan"
      viewBox="0 0 300 46"
      preserveAspectRatio="none"
      role="img"
      aria-label={label}
    >
      {paths.map((d, i) => (
        <motion.path
          key={d}
          className="compat__fan-path"
          d={d}
          initial={{ pathLength: reduce ? 1 : 0, opacity: reduce ? 1 : 0 }}
          whileInView={{ pathLength: 1, opacity: 1 }}
          viewport={{ once: true, margin: "0px 0px -60px 0px" }}
          transition={
            reduce
              ? { duration: 0 }
              : {
                  ...DRAW,
                  delay: 0.08 * i,
                  opacity: { duration: 0.01, delay: 0.08 * i },
                }
          }
        />
      ))}
    </svg>
  );
}

export function AgentCompat({ data }: { data: CompatData }) {
  const reduce = useReducedMotion();

  return (
    <section className="compat" aria-label={data.title}>
      <header className="compat__head">
        <h3 className="compat__title">{data.title}</h3>
        <p className="compat__lede">{data.lede}</p>
      </header>

      <AnimatedReveal y={14}>
        <div className="compat__source">
          <span className="compat__source-label">{data.source.label}</span>
          <span className="compat__source-detail">{data.source.detail}</span>
        </div>
      </AnimatedReveal>

      <ConnectorFan
        paths={FAN_OUT}
        label="One embedded markdown bundle, exported into three places"
        reduce={reduce}
      />

      <div className="compat__targets">
        {data.targets.map((t, i) => (
          <AnimatedReveal key={t.harness} delay={0.05 * i} y={14}>
            <div className="compat__target">
              <span className="compat__harness">{t.harness}</span>
              <code className="compat__path">{t.path}</code>
              <p className="compat__note">{t.note}</p>
            </div>
          </AnimatedReveal>
        ))}
      </div>

      <div className="compat__rule">
        <span>and whatever wrote the change, it commits through git</span>
      </div>

      <AnimatedReveal y={14}>
        <ul className="compat__marks">
          {data.harnesses.map((h) => (
            <li key={h} className="compat__mark">
              {h}
            </li>
          ))}
        </ul>
      </AnimatedReveal>

      <ConnectorFan
        paths={FAN_IN}
        label="Every tool's commit arrives at the same git gate"
        reduce={reduce}
      />

      <AnimatedReveal y={14}>
        <div className="compat__gate">
          <span className="compat__gate-label">
            <code>{data.gate.label}</code>
          </span>
          <div className="compat__gate-grid">
            <div className="compat__gate-card">
              <h4>{data.gate.hook.title}</h4>
              <p>{data.gate.hook.body}</p>
            </div>
            <div className="compat__gate-card compat__gate-card--authority">
              <h4>{data.gate.ci.title}</h4>
              <p>{data.gate.ci.body}</p>
            </div>
          </div>
        </div>
      </AnimatedReveal>
    </section>
  );
}
