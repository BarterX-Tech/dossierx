import { useEffect, useState } from "react";
import {
  AnimatePresence,
  motion,
  useReducedMotion,
  type Variants,
} from "framer-motion";
import { contentSpec, latestVersion } from "../content";
import { getSection } from "./section-utils";

interface HeroData {
  ctas: { label: string; href: string }[];
  pipeline: string[];
  roles: { id: "agent" | "human"; who: string; surface: string }[];
}

const container: Variants = {
  hidden: {},
  show: {
    transition: { staggerChildren: 0.08, delayChildren: 0.1 },
  },
};

const rise: Variants = {
  hidden: { opacity: 1, y: 22 },
  show: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.6, ease: [0.22, 1, 0.36, 1] },
  },
};

const wordRise: Variants = {
  hidden: { opacity: 1, y: "0.5em" },
  show: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.55, ease: [0.22, 1, 0.36, 1] },
  },
};

// The final word of the title gets the editorial accent treatment.
function splitTitle(title: string) {
  const words = title.split(" ");
  return words.map((w, i) => ({ w, accent: i >= words.length - 1 }));
}

/** One line of a card's revealed claim snippet — plain, or a diff add/del. */
interface ClaimLine {
  text: string;
  kind?: "add" | "del";
}

/**
 * The hero-scale folder cascade: the LOCK LIFECYCLE dramatized, NOT the four
 * facets (those are the Claims section's FacetCascade — reusing them here would
 * dupe). Five overlapping paper "claims" drop into a stack, each tab-labelled
 * with a lifecycle state, walking the same example claim
 * (widget.contract.overview) all the way around the loop: draft -> locked ->
 * edited (a dependency's content changes, shown as a highlighted diff) ->
 * review_pending (DetectStale flags it) -> locked again (reaudit --confirm).
 * Clicking a card reveals that state's claim text below the stack. This reuses
 * the .cascade visual language (per-variant --fill/--label/--role vars, the
 * rounded-notch tab, the sheet shadow + paper-grain overlay) at hero scale
 * under a separate .herostack namespace; it does NOT touch .cascade.
 */
const STACK_FOLDERS: {
  key: string;
  label: string;
  role: string;
  variant: "plain" | "accent" | "accent2" | "warn";
  tabLeft: string;
  rot: number;
  lines: ClaimLine[];
}[] = [
  {
    key: "draft",
    label: "draft",
    role: "unreviewed · may still change",
    variant: "plain",
    tabLeft: "6%",
    rot: -0.6,
    lines: [
      { text: "id: widget.contract.overview" },
      { text: "status: draft" },
      { text: "freely editable — nothing trusts it yet." },
    ],
  },
  {
    key: "locked-1",
    label: "locked",
    role: "reviewed · frozen truth",
    variant: "accent",
    tabLeft: "26%",
    rot: 0.5,
    lines: [
      { text: "status: draft", kind: "del" },
      { text: "status: locked", kind: "add" },
      { text: "a human reviewed it. now it is a trust assertion." },
    ],
  },
  {
    key: "edited",
    label: "edited",
    role: "a dependency just changed",
    variant: "warn",
    tabLeft: "14%",
    rot: -0.45,
    lines: [
      {
        text: "A widget is the smallest unit this project documents.",
        kind: "del",
      },
      {
        text: "A widget is the smallest reviewable unit this project documents.",
        kind: "add",
      },
      { text: "a claim this one depends on just moved." },
    ],
  },
  {
    key: "review_pending",
    label: "review_pending",
    role: "a dependency moved · recheck",
    variant: "accent2",
    tabLeft: "30%",
    rot: -0.5,
    lines: [
      { text: "review_pending: true", kind: "add" },
      { text: "reason: dependency content hash changed" },
      { text: "DetectStale flagged it automatically. drift made loud." },
    ],
  },
  {
    key: "locked-2",
    label: "locked",
    role: "claim reaudit --confirm · re-frozen",
    variant: "accent",
    tabLeft: "10%",
    rot: 0.4,
    lines: [
      { text: "review_pending: false", kind: "add" },
      { text: "audit_notes: + reaudited — confirmed match", kind: "add" },
      { text: "reviewed again, on purpose. re-frozen." },
    ],
  },
];

const stackContainer: Variants = {
  hidden: {},
  show: { transition: { staggerChildren: 0.12, delayChildren: 0.2 } },
};
// rot is passed via `custom` and held constant across states, so folders settle
// (translate + scale + fade) without spinning — same technique as .cascade.
const stackFolder: Variants = {
  hidden: (rot: number) => ({ opacity: 1, y: -28, scale: 0.985, rotate: rot }),
  show: (rot: number) => ({
    opacity: 1,
    y: 0,
    scale: 1,
    rotate: rot,
    transition: { type: "spring", stiffness: 260, damping: 26 },
  }),
};

function HeroStack() {
  const reduce = useReducedMotion();
  // On narrow viewports the whisper of hand-placed rotation is dropped so a tilt
  // can never clip the edge; the vertical stagger is kept (mirrors .cascade).
  const [narrow, setNarrow] = useState(false);
  useEffect(() => {
    const mq = window.matchMedia("(max-width: 640px)");
    const sync = () => setNarrow(mq.matches);
    sync();
    mq.addEventListener("change", sync);
    return () => mq.removeEventListener("change", sync);
  }, []);

  // Hovering (or focusing) a card previews its claim snippet in the panel
  // below the stack; clicking pins it so it stays visible after the mouse
  // leaves (this is also how touch/keyboard select a card).
  const STEP = 76;
  // An opened card rises and grows taller than one STEP — enough to reach over
  // the next TWO tabs below it. Without correction that swallows their tabs
  // entirely (they'd stay same z-index, buried under the open sheet), so every
  // card below whichever one is active gets pushed down by PUSH, clearing the
  // open sheet's reach. Cards above are untouched; nothing before the active
  // one ever overlaps it.
  const PUSH = 130;
  const [pinnedKey, setPinnedKey] = useState<string | null>(null);
  const [hoverKey, setHoverKey] = useState<string | null>(null);
  const activeKey = hoverKey ?? pinnedKey;
  const activeIndex = STACK_FOLDERS.findIndex((f) => f.key === activeKey);
  const togglePin = (key: string) =>
    setPinnedKey((cur) => (cur === key ? null : key));
  const tailPush =
    activeIndex >= 0 && activeIndex < STACK_FOLDERS.length - 1 ? PUSH : 0;

  return (
    <div className="herostack">
      <motion.div
        className="herostack__stage"
        variants={stackContainer}
        initial={reduce ? "show" : "hidden"}
        animate="show"
        style={{
          height: `${10 + (STACK_FOLDERS.length - 1) * STEP + 180 + tailPush}px`,
        }}
      >
        {STACK_FOLDERS.map((f, i) => {
          const isActive = activeKey === f.key;
          const pushed = activeIndex >= 0 && i > activeIndex;
          return (
            <motion.div
              key={f.key}
              className={`herostack__folder herostack__folder--${f.variant}${isActive ? " herostack__folder--active" : ""}`}
              style={{
                top: `${10 + i * STEP + (pushed ? PUSH : 0)}px`,
                zIndex: isActive ? 100 : 10 + i * 10,
                ["--tab-left" as string]: f.tabLeft,
              }}
              variants={stackFolder}
              custom={narrow ? 0 : f.rot}
            >
              <motion.div
                className="herostack__sheet"
                role="button"
                tabIndex={0}
                aria-pressed={pinnedKey === f.key}
                aria-expanded={isActive}
                aria-label={`${f.label} — ${f.role}. Show its claim text.`}
                onMouseEnter={() => setHoverKey(f.key)}
                onMouseLeave={() => setHoverKey(null)}
                onFocus={() => setHoverKey(f.key)}
                onBlur={() => setHoverKey(null)}
                onClick={() => togglePin(f.key)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    togglePin(f.key);
                  }
                }}
                animate={{ y: reduce ? 0 : isActive ? -26 : 0 }}
                transition={{ type: "spring", stiffness: 300, damping: 26 }}
              >
                <div className="herostack__tab">
                  <span className="herostack__label">{f.label}</span>
                </div>
                <span className="herostack__role">{f.role}</span>

                {/* Like pulling this claim up out of the drawer to read it: the
                  whole card rises (animated above) and jumps in front of the
                  stack, then the document itself slides up from the bottom of
                  the card into view — in place, not a panel elsewhere. */}
                <AnimatePresence>
                  {isActive && (
                    <motion.pre
                      className="herostack__opendoc"
                      initial={reduce ? { opacity: 1 } : { opacity: 0, y: 26 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={reduce ? { opacity: 0 } : { opacity: 0, y: 26 }}
                      transition={{ duration: 0.3, ease: [0.22, 1, 0.36, 1] }}
                    >
                      {f.lines.map((line, li) => (
                        <span
                          key={li}
                          className={`herostack__diff-line${line.kind ? ` herostack__diff-line--${line.kind}` : ""}`}
                        >
                          {line.kind === "del" && (
                            <span className="herostack__diff-sign">−</span>
                          )}
                          {line.kind === "add" && (
                            <span className="herostack__diff-sign">+</span>
                          )}
                          {line.text}
                        </span>
                      ))}
                    </motion.pre>
                  )}
                </AnimatePresence>

                {/* Post-settle beat 1: the padlock stamps in on each locked folder. */}
                {f.key.startsWith("locked") && (
                  <motion.span
                    className="herostack__lock"
                    initial={
                      reduce
                        ? { opacity: 1, scale: 1 }
                        : { opacity: 0, scale: 1.2 }
                    }
                    animate={{ opacity: 1, scale: 1 }}
                    transition={{
                      type: "spring",
                      stiffness: 420,
                      damping: 12,
                      delay: 0.9,
                    }}
                  >
                    <svg
                      width="22"
                      height="22"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="var(--surface)"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    >
                      <rect x="5" y="11" width="14" height="9" rx="2" />
                      <path d="M8 11V8a4 4 0 0 1 8 0v3" />
                    </svg>
                  </motion.span>
                )}

                {/* Post-settle beat 2: the amber flag rotates up on review_pending, */}
                {/* then its pulse ring loops — the eye-magnet drift alarm. */}
                {f.key === "review_pending" && (
                  <>
                    <motion.span
                      className="herostack__flag"
                      initial={
                        reduce
                          ? { opacity: 1, rotate: 0 }
                          : { opacity: 0, rotate: 28 }
                      }
                      animate={{ opacity: 1, rotate: 0 }}
                      transition={{
                        duration: 0.35,
                        ease: [0.22, 1, 0.36, 1],
                        delay: 1.3,
                      }}
                    >
                      <svg
                        width="22"
                        height="27"
                        viewBox="0 0 24 30"
                        fill="none"
                      >
                        <line
                          x1="5"
                          y1="2"
                          x2="5"
                          y2="28"
                          stroke="var(--warn)"
                          strokeWidth="2"
                          strokeLinecap="round"
                        />
                        <path
                          d="M5 3 L19 6 L14.5 10.5 L19 15 L5 15 Z"
                          fill="var(--warn)"
                        />
                      </svg>
                    </motion.span>
                    {!reduce && <span className="herostack__pulse" />}
                  </>
                )}
              </motion.div>
            </motion.div>
          );
        })}
      </motion.div>
    </div>
  );
}

export function Hero() {
  const section = getSection("hero");
  const data = section.data as unknown as HeroData;
  const reduce = useReducedMotion();
  const words = splitTitle(section.title);

  return (
    <section id={section.id} className="hero">
      <motion.div
        className="hero__content"
        variants={container}
        initial={reduce ? "show" : "hidden"}
        animate="show"
      >
        <div className="hero__main">
          <div className="hero__col">
            <motion.div className="hero__kicker" variants={rise}>
              <span>Open-source documentation engine</span>
              <span className="hero__kicker-meta">{latestVersion} · Go 1.26</span>
            </motion.div>

            <h1 className="hero__title">
              {words.map(({ w, accent }, i) => (
                <motion.span
                  key={i}
                  className={`hero__title-word${accent ? " hero__title-accent" : ""}`}
                  variants={wordRise}
                >
                  {w}
                </motion.span>
              ))}
            </h1>

            <motion.p className="hero__subhead" variants={rise}>
              {contentSpec.tagline}
            </motion.p>

            {/* The two roles, named before anything else on the page. The whole
                release is this distinction, and a reader who scrolls no further
                than the fold should still leave with it. */}
            <motion.dl className="hero__roles" variants={rise}>
              {data.roles.map((r) => (
                <div className={`hero__role hero__role--${r.id}`} key={r.id}>
                  <dt>{r.who}</dt>
                  <dd>{r.surface}</dd>
                </div>
              ))}
            </motion.dl>

            <motion.div className="hero__ctas" variants={rise}>
              {data.ctas.map((cta, i) => (
                <a
                  key={cta.href}
                  href={cta.href}
                  className={i === 0 ? "button" : "button button--ghost"}
                  {...(cta.href.startsWith("http")
                    ? { target: "_blank", rel: "noreferrer" }
                    : {})}
                >
                  {cta.label}
                  {i === 0 && <span aria-hidden="true">↗</span>}
                </a>
              ))}
              <a href="#cli" className="button button--text">
                Read the machine contract <span aria-hidden="true">↓</span>
              </a>
            </motion.div>
          </div>

          <div className="hero__visual">
            <div className="hero__visual-head">
              <span>One claim</span>
              <span>five states</span>
            </div>
            <HeroStack />
            <p className="hero__visual-note">
              Hover or focus a tab to inspect the claim as trust changes.
            </p>
          </div>
        </div>

        <motion.div className="hero__ledger" variants={rise}>
          <div className="hero__workflow" aria-label="The stages inside dossierx check">
            {/* These five are STAGES of one command, not five commands — three
                of them were verbs in v0.2.0 and were deleted for being
                packaging artifacts. They survive as the values check reports in
                the envelope's stopped_at, which is why an agent still cares
                about them: "stopped at lint" and "stopped at scan" call for
                different next moves. */}
            <span className="hero__ledger-label">
              Inside <code>dossierx check</code>
            </span>
            <div className="hero__pipeline">
              {data.pipeline.map((step, index) => (
                <span className="hero__pipeline-step" key={step}>
                  <code>{step}</code>
                  {index < data.pipeline.length - 1 && (
                    <span aria-hidden="true">→</span>
                  )}
                </span>
              ))}
            </div>
          </div>
          <dl className="hero__facts">
            <div>
              <dt>20</dt>
              <dd>commands your agent runs</dd>
            </div>
            <div>
              <dt>1</dt>
              <dd>command you run</dd>
            </div>
            <div>
              <dt>0</dt>
              <dd>silent changes to a locked claim</dd>
            </div>
          </dl>
        </motion.div>
      </motion.div>
    </section>
  );
}
