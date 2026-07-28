import { motion, useReducedMotion } from "framer-motion";
import { AnimatedReveal } from "../components/AnimatedReveal";
import { SectionContainer } from "../components/SectionContainer";
import { SectionHeader } from "../components/SectionHeader";
import { getSection } from "./section-utils";
import { POP, DRAW } from "./motion-tokens";

interface State {
  id: string;
  label: string;
  desc: string;
}
interface Transition {
  from: string;
  to: string;
  trigger: string;
  /** Who decides the transition happens — the v0.3.0 half of the story. */
  mandate: string;
  /** Who actually runs the command (or clicks the button) that performs it. */
  execute: string;
  note: string;
}
interface LifecycleData {
  states: State[];
  transitions: Transition[];
  invariant: string;
}

/* ---- SVG state diagram: draft -> locked -> review_pending -> reaudit -> locked ----
 *
 * Ground-up rebuild. One fixed grid, perfect L/R symmetry about center-line CY=130.
 *   viewBox 0 0 760 260. Three identical 164x76 boxes; column pitch a constant 264.
 *   Forward edges (lock, flag) bow UP above the row; return edges (unlock, reaudit)
 *   bow DOWN below it — a mirrored lens between each adjacent pair, reflected across
 *   CY. Every edge attaches to a side FACE (y=112 forward, y=148 return), never a
 *   corner. Arrowheads are explicit filled-triangle <path>s (not SVG markers) so
 *   they can be held hidden until the stroke finishes drawing. */

// Layout constants (viewBox user units).
const NY = 92; // node top
const NW = 164; // node width
const NH = 76; // node height

type StateColor = "var(--text-dim)" | "var(--ok)" | "var(--warn)";

interface DiagNode {
  id: "draft" | "locked" | "review";
  x: number;
  center: [number, number];
  label: string;
  sub: string;
  color: StateColor;
  inDelay: number;
}
interface DiagEdge {
  id: string;
  d: string;
  color: string;
  dashed?: boolean;
  deemph?: boolean;
  tip: [number, number];
  angle: number;
  label: string;
  lx: number;
  ly: number;
  drawAt: number;
  kind: "draw" | "wipe";
}

// Single source of truth for the narrative timeline (seconds from whileInView).
const DELAYS = {
  draftNode: 0.0,
  lockDraw: 0.45,
  lockedNode: 1.05,
  lockedGlow1: 1.15,
  flagDraw: 1.65,
  reviewNode: 2.25,
  reviewGlow: 2.35,
  reauditDraw: 2.8,
  resolveDraw: 3.2,
  lockedGlow2: 3.4,
  unlockDraw: 3.9,
} as const;

// Per-edge offsets off drawAt (uniform, deterministic):
//   arrowhead lands 0.45s after the draw starts (~as the 0.55s line completes),
//   label names the transition 0.50s after.
const ARROW_OFFSET = 0.45;
const LABEL_OFFSET = 0.5;

// Springs. NODE settles the state boxes; ARROW pops the arrowheads (= POP token).
const NODE = {
  type: "spring",
  stiffness: 260,
  damping: 24,
  mass: 0.9,
} as const;
const ARROW = POP;

const NODES: DiagNode[] = [
  {
    id: "draft",
    x: 34,
    center: [116, 130],
    label: "draft",
    sub: "editable · untrusted",
    color: "var(--text-dim)",
    inDelay: DELAYS.draftNode,
  },
  {
    id: "locked",
    x: 298,
    center: [380, 130],
    label: "locked",
    sub: "reviewed · trusted",
    color: "var(--ok)",
    inDelay: DELAYS.lockedNode,
  },
  {
    id: "review",
    x: 562,
    center: [644, 130],
    label: "review_pending",
    sub: "locked · flagged",
    color: "var(--warn)",
    inDelay: DELAYS.reviewNode,
  },
];

const EDGES: DiagEdge[] = [
  // forward: lock (draft -> locked), bows up, apex ~65.5 at x=248; end tangent 55.9°
  {
    id: "lock",
    d: "M 198 112 C 240 50 256 50 298 112",
    color: "var(--accent)",
    tip: [298, 112],
    angle: 55.9,
    // Every label on this diagram names the command as an AGENT would type it
    // — the noun included, the --reason that carries the human's approval
    // included. "lock" alone was a v0.2.0 verb and no longer exists.
    label: "claim lock --reason",
    lx: 248,
    ly: 42,
    drawAt: DELAYS.lockDraw,
    kind: "draw",
  },
  // forward: flag (locked -> review), dashed & clip-wiped, apex ~65.5 at x=512
  {
    id: "flag",
    d: "M 462 112 C 504 50 520 50 562 112",
    color: "var(--warn)",
    dashed: true,
    tip: [562, 112],
    angle: 55.9,
    label: "drift · claim flag · new thread",
    lx: 512,
    ly: 42,
    drawAt: DELAYS.flagDraw,
    kind: "wipe",
  },
  // return: reaudit (review -> locked), bows down, apex ~194.5 at x=512; end tangent 235.9°
  {
    id: "reaudit",
    d: "M 562 148 C 520 210 504 210 462 148",
    color: "var(--ok)",
    tip: [462, 148],
    angle: 235.9,
    label: "claim reaudit --confirm",
    lx: 512,
    ly: 218,
    drawAt: DELAYS.reauditDraw,
    kind: "draw",
  },
  // return: Resolve (review -> locked), the SECOND clearer, nested just inside
  // the reaudit arc; apex ~175.5 at x=513; end tangent 223.4°. This is the one
  // edge on the diagram a HUMAN both mandates and executes — it is a click in
  // the viewer, not a command — which is why it keeps the human accent colour.
  {
    id: "resolve",
    d: "M 558 150 C 522 184 504 184 468 150",
    color: "var(--accent)",
    tip: [468, 150],
    angle: 223.4,
    label: "you click Resolve",
    lx: 512,
    ly: 188,
    drawAt: DELAYS.resolveDraw,
    kind: "draw",
  },
  // return: unlock (locked -> draft), the de-emphasized secondary escape, drawn LAST
  {
    id: "unlock",
    d: "M 298 148 C 256 210 240 210 198 148",
    color: "var(--text-faint)",
    deemph: true,
    tip: [198, 148],
    angle: 235.9,
    label: "claim unlock --reason",
    lx: 248,
    ly: 218,
    drawAt: DELAYS.unlockDraw,
    kind: "draw",
  },
];

type Reduce = boolean | null;

function StateBox({ node, reduce }: { node: DiagNode; reduce: Reduce }) {
  return (
    <motion.g
      style={{ transformOrigin: `${node.center[0]}px ${node.center[1]}px` }}
      variants={{
        hidden: reduce ? { opacity: 0 } : { opacity: 0, scale: 0.92, y: 8 },
        show: {
          opacity: 1,
          scale: 1,
          y: 0,
          transition: reduce
            ? { duration: 0.25 }
            : { ...NODE, delay: node.inDelay },
        },
      }}
    >
      <rect
        x={node.x}
        y={NY}
        width={NW}
        height={NH}
        rx={10}
        fill="var(--surface)"
        stroke={node.color}
        strokeWidth={1.75}
      />
      {/* left accent rule — 8px inset top/bottom so it never fights the rx=10 corners */}
      <rect
        x={node.x}
        y={104}
        width={3}
        height={52}
        rx={1.5}
        fill={node.color}
      />
      <text
        className="node-label"
        x={node.center[0]}
        y={123}
        textAnchor="middle"
      >
        {node.label}
      </text>
      <text className="node-sub" x={node.center[0]} y={143} textAnchor="middle">
        {node.sub}
      </text>
    </motion.g>
  );
}

function Glow({
  color,
  center,
  delay,
  reduce,
}: {
  color: string;
  center: [number, number];
  delay: number;
  reduce: Reduce;
}) {
  if (reduce) return null;
  const x = center[0] - NW / 2;
  return (
    <motion.rect
      className="lc-glow"
      x={x}
      y={NY}
      width={NW}
      height={NH}
      rx={10}
      stroke={color}
      strokeWidth={2}
      style={{ transformOrigin: `${center[0]}px ${center[1]}px` }}
      variants={{
        hidden: { opacity: 0 },
        show: {
          opacity: [0, 0.55, 0],
          scale: [1, 1.09],
          transition: { duration: 0.6, ease: [0.22, 1, 0.36, 1], delay },
        },
      }}
    />
  );
}

function EdgeLine({ edge, reduce }: { edge: DiagEdge; reduce: Reduce }) {
  const targetOpacity = edge.deemph ? 0.55 : 1;

  if (edge.kind === "wipe") {
    // Dashed flag: pathLength manages dasharray internally and would eat the visible
    // "7 6" dashes, so reveal it with a L->R clip wipe instead (x is monotonic
    // 462->562). After the wipe, march the dashes forever = the "automatic" signal.
    const marchDelay = edge.drawAt + 0.55;
    return (
      <>
        <clipPath id="flagClip">
          <motion.rect
            x={455}
            y={50}
            width={120}
            height={75}
            style={{ transformOrigin: "455px 88px" }}
            variants={{
              hidden: { scaleX: reduce ? 1 : 0 },
              show: {
                scaleX: 1,
                transition: reduce
                  ? { duration: 0 }
                  : {
                      duration: 0.55,
                      ease: [0.22, 1, 0.36, 1],
                      delay: edge.drawAt,
                    },
              },
            }}
          />
        </clipPath>
        <motion.path
          className="edge-path"
          d={edge.d}
          stroke={edge.color}
          strokeDasharray="7 6"
          clipPath="url(#flagClip)"
          variants={{
            hidden: { opacity: 1, strokeDashoffset: 0 },
            show: reduce
              ? { opacity: 1, strokeDashoffset: 0 }
              : {
                  opacity: 1,
                  strokeDashoffset: [0, -13],
                  transition: {
                    strokeDashoffset: {
                      duration: 0.9,
                      ease: "linear",
                      repeat: Infinity,
                      delay: marchDelay,
                    },
                  },
                },
          }}
        />
      </>
    );
  }

  return (
    <motion.path
      className="edge-path"
      d={edge.d}
      stroke={edge.color}
      variants={{
        hidden: {
          pathLength: reduce ? 1 : 0,
          opacity: reduce ? targetOpacity : 0,
        },
        show: {
          pathLength: 1,
          opacity: targetOpacity,
          transition: reduce
            ? { duration: 0.25 }
            : {
                ...DRAW,
                duration: 0.55,
                delay: edge.drawAt,
                opacity: { duration: 0.01, delay: edge.drawAt },
              },
        },
      }}
    />
  );
}

function ArrowHead({ edge, reduce }: { edge: DiagEdge; reduce: Reduce }) {
  const targetOpacity = edge.deemph ? 0.55 : 1;
  const delay = edge.drawAt + ARROW_OFFSET;
  // Outer <g> places the tip exactly on the target face at the true end tangent;
  // inner motion.path only pops scale/opacity about its own fill-box center.
  return (
    <g
      transform={`translate(${edge.tip[0]} ${edge.tip[1]}) rotate(${edge.angle})`}
    >
      <motion.path
        className="lc-arrow"
        d="M0 0 L-11 -5 L-3.5 0 L-11 5 Z"
        fill={edge.color}
        style={{ transformBox: "fill-box", transformOrigin: "center" }}
        variants={{
          hidden: { opacity: 0, scale: reduce ? 1 : 0.4 },
          show: {
            opacity: targetOpacity,
            scale: 1,
            transition: reduce ? { duration: 0.25 } : { ...ARROW, delay },
          },
        }}
      />
    </g>
  );
}

function EdgeLabel({ edge, reduce }: { edge: DiagEdge; reduce: Reduce }) {
  const delay = edge.drawAt + LABEL_OFFSET;
  const targetOpacity = edge.id === "unlock" ? 0.7 : 1;
  return (
    <motion.text
      className="edge-label"
      x={edge.lx}
      y={edge.ly}
      textAnchor="middle"
      fill={edge.color}
      variants={{
        hidden: { opacity: 0, y: reduce ? 0 : 4 },
        show: {
          opacity: targetOpacity,
          y: 0,
          transition: reduce
            ? { duration: 0.25 }
            : { duration: 0.28, ease: [0.22, 1, 0.36, 1], delay },
        },
      }}
    >
      {edge.label}
    </motion.text>
  );
}

function LifecycleDiagram() {
  const reduce = useReducedMotion();
  return (
    <div className="diagram-wrap">
      <motion.svg
        className="diagram-svg"
        viewBox="0 0 760 260"
        preserveAspectRatio="xMidYMid meet"
        role="img"
        aria-label="Lifecycle state diagram. A human mandates each transition and the agent executes it: draft locks to locked via claim lock --reason; locked flags to review_pending automatically on dependency drift, or when the agent runs claim flag, or when a comment thread opens; claim reaudit --confirm or the human clicking Resolve returns review_pending to locked; claim unlock --reason returns locked to draft."
        initial="hidden"
        whileInView="show"
        viewport={{ once: true, margin: "0px 0px -80px 0px" }}
      >
        {/* (1) glows behind everything */}
        <Glow
          color="var(--ok)"
          center={[380, 130]}
          delay={DELAYS.lockedGlow1}
          reduce={reduce}
        />
        <Glow
          color="var(--warn)"
          center={[644, 130]}
          delay={DELAYS.reviewGlow}
          reduce={reduce}
        />
        <Glow
          color="var(--ok)"
          center={[380, 130]}
          delay={DELAYS.lockedGlow2}
          reduce={reduce}
        />

        {/* (2) edge strokes */}
        {EDGES.map((e) => (
          <EdgeLine key={e.id} edge={e} reduce={reduce} />
        ))}

        {/* (3) arrowheads */}
        {EDGES.map((e) => (
          <ArrowHead key={e.id} edge={e} reduce={reduce} />
        ))}

        {/* (4) state boxes on top so they always overdraw any edge tail */}
        {NODES.map((n) => (
          <StateBox key={n.id} node={n} reduce={reduce} />
        ))}

        {/* (5) labels last — always readable */}
        {EDGES.map((e) => (
          <EdgeLabel key={e.id} edge={e} reduce={reduce} />
        ))}
      </motion.svg>

      <div className="diagram-legend">
        <span>
          <i
            className="legend-swatch"
            style={{ background: "var(--accent)" }}
          />{" "}
          you mandate it
        </span>
        <span>
          <i className="legend-swatch" style={{ background: "var(--warn)" }} />{" "}
          raised automatically — nobody decided (dashed)
        </span>
        <span>
          <i className="legend-swatch" style={{ background: "var(--ok)" }} />{" "}
          the agent executes it, with --reason on the record
        </span>
      </div>
    </div>
  );
}

export function Lifecycle() {
  const section = getSection("lifecycle");
  const data = section.data as unknown as LifecycleData;

  return (
    <SectionContainer id={section.id} alt>
      <SectionHeader
        eyebrow="Lifecycle"
        title={section.title}
        contentMd={section.contentMd}
      />

      <LifecycleDiagram />

      <div className="states">
        {data.states.map((s, i) => (
          <AnimatedReveal key={s.id} delay={0.05 * i}>
            <div className="state">
              <span className="state__label">{s.label}</span>
              <p className="state__desc">{s.desc}</p>
            </div>
          </AnimatedReveal>
        ))}
      </div>

      <AnimatedReveal>
        <div className="compare__scroll">
          {/* From and To are one column now. They were two, and the two extra
              columns this table needed — who mandates, who executes — are worth
              more than the split: the transition itself is the row's identity,
              the parties are the thing v0.3.0 actually documents. */}
          <table className="transitions">
            <thead>
              <tr>
                <th>Transition</th>
                <th>Mandated by</th>
                <th>Executed by</th>
                <th>How</th>
                <th>Note</th>
              </tr>
            </thead>
            <tbody>
              {data.transitions.map((t, i) => (
                <tr key={i}>
                  <td className="transitions__edge">
                    {t.from} <span aria-hidden="true">→</span> {t.to}
                  </td>
                  <td>{t.mandate}</td>
                  <td>{t.execute}</td>
                  <td>
                    <code>{t.trigger}</code>
                  </td>
                  <td>{t.note}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </AnimatedReveal>

      <AnimatedReveal>
        <p className="invariant">
          <strong>Invariant.</strong> {data.invariant}
        </p>
      </AnimatedReveal>
    </SectionContainer>
  );
}
