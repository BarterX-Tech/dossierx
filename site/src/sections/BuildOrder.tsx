import { motion, useReducedMotion } from "framer-motion";
import { AnimatedReveal } from "../components/AnimatedReveal";
import { SectionContainer } from "../components/SectionContainer";
import { SectionHeader } from "../components/SectionHeader";
import { getSection } from "./section-utils";
import { BEAT, POP, DRAW } from "./motion-tokens";

interface Phase {
  phase: string;
  role: string;
  ordering: string;
}
interface Subcommand {
  cmd: string;
  desc: string;
}
interface BuildOrderData {
  phases: Phase[];
  subcommands: Subcommand[];
}

/* ---- Animated DAG: orientation -> schema -> behavior -> api -> verification ---- */
function BuildFlow({ phases }: { phases: Phase[] }) {
  const reduce = useReducedMotion();
  const flow = phases.filter((p) => p.phase !== "out-of-scope");
  const excluded = phases.find((p) => p.phase === "out-of-scope");

  return (
    <div className="buildflow-wrap">
      <motion.div
        className="buildflow"
        initial="hidden"
        whileInView="show"
        viewport={{ once: true, margin: "0px 0px -80px 0px" }}
        aria-hidden="true"
      >
        {flow.map((p, i) => {
          const topo = p.ordering.includes("topolog");
          return (
            <div className="buildflow__node" key={p.phase}>
              {i < flow.length - 1 && (
                <svg
                  className="buildflow__connector"
                  viewBox="0 0 100 20"
                  preserveAspectRatio="none"
                  aria-hidden="true"
                >
                  {/* Wobble baked into the path geometry (fixed alternating
                      cubic segments) instead of a live feTurbulence filter.
                      Drawn via pathLength so the hand-drawn squiggle reveals
                      along its length with zero geometric distortion — no
                      scaleX rubber-banding on the preserveAspectRatio=none svg. */}
                  <motion.path
                    d="M0 10 C 12 5, 20 6, 28 9 C 36 12, 44 14, 52 10 C 60 6, 70 7, 78 10 C 86 13, 94 12, 100 10"
                    variants={{
                      hidden: { pathLength: reduce ? 1 : 0, opacity: 0 },
                      show: {
                        pathLength: 1,
                        opacity: 1,
                        transition: reduce
                          ? { duration: 0.2 }
                          : {
                              ...DRAW,
                              delay: (2 * i + 1) * BEAT,
                              opacity: {
                                duration: 0.01,
                                delay: (2 * i + 1) * BEAT,
                              },
                            },
                      },
                    }}
                  />
                </svg>
              )}
              <motion.span
                className="buildflow__dot"
                variants={{
                  hidden: { opacity: 0, scale: reduce ? 1 : 0.6 },
                  show: {
                    opacity: 1,
                    scale: 1,
                    transition: reduce
                      ? { duration: 0.2 }
                      : { ...POP, delay: 2 * i * BEAT },
                  },
                }}
              >
                {i + 1}
              </motion.span>
              <motion.span
                className="buildflow__phase"
                variants={{
                  hidden: { opacity: 0, y: reduce ? 0 : 8 },
                  show: {
                    opacity: 1,
                    y: 0,
                    transition: reduce
                      ? { duration: 0.2 }
                      : {
                          duration: 0.35,
                          ease: [0.22, 1, 0.36, 1],
                          delay: 2 * i * BEAT + 0.12,
                        },
                  },
                }}
              >
                {p.phase}
                <span className="buildflow__ord">
                  {topo ? "topo-sorted" : "stable order"}
                </span>
              </motion.span>
            </div>
          );
        })}
      </motion.div>
      {excluded && (
        <p className="diagram-legend" style={{ marginTop: "0.75rem" }}>
          <span>
            <i
              className="legend-swatch"
              style={{ background: "var(--text-faint)" }}
            />
            <code>out-of-scope</code> — {excluded.role}
          </span>
        </p>
      )}
    </div>
  );
}

export function BuildOrder() {
  const section = getSection("build-order");
  const data = section.data as unknown as BuildOrderData;

  return (
    <SectionContainer id={section.id}>
      <SectionHeader
        eyebrow="Build order"
        title={section.title}
        contentMd={section.contentMd}
      />

      <BuildFlow phases={data.phases} />

      <ol className="phases">
        {data.phases.map((p, i) => {
          const excluded = p.phase === "out-of-scope";
          return (
            <AnimatedReveal key={p.phase} delay={0.04 * i}>
              <li className={`phase${excluded ? " phase--excluded" : ""}`}>
                <span className="phase__num">{excluded ? "—" : i + 1}</span>
                <div>
                  <code className="phase__name">{p.phase}</code>
                  <p className="phase__role">{p.role}</p>
                  <span className="phase__ordering">{p.ordering}</span>
                </div>
              </li>
            </AnimatedReveal>
          );
        })}
      </ol>

      <div className="subcommands">
        {data.subcommands.map((s) => (
          <AnimatedReveal key={s.cmd}>
            <div className="subcommand">
              <code className="subcommand__cmd">{s.cmd}</code>
              <p>{s.desc}</p>
            </div>
          </AnimatedReveal>
        ))}
      </div>
    </SectionContainer>
  );
}
