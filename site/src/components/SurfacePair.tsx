import type { ReactNode } from "react";
import { AnimatedReveal } from "./AnimatedReveal";
import { RolePill, type Role } from "./RolePill";

export interface PairReply {
  id: string;
  role: Role;
  created: string;
  body: string;
}
export interface PairThread {
  id: string;
  role: Role;
  status: string;
  created: string;
  body: string;
  replies: PairReply[];
}
export interface PairCard {
  id: string;
  module: string;
  facet: string;
  status: string;
  statusNote: string;
  panelTitle: string;
  rows: { field: string; type: string; flagged?: boolean }[];
  thread: PairThread;
  resolvedCount: number;
}
export interface SurfacePairData {
  claimID: string;
  terminal: { title: string; url: string; code: string; note: string };
  viewer: { title: string; url: string; note: string };
  card: PairCard;
}

/* ---- Visual (d): one claim, both surfaces, side by side ----
 *
 * The argument the whole release rests on is that these two pictures are the
 * same claim. So they are drawn together, at the same scale, sharing the claim
 * id and the thread id: on the left the agent's terminal with a real `comment
 * inbox` envelope, on the right the card and thread that same YAML renders in
 * the reviewer's browser.
 *
 * Neither half is a screenshot. Screenshots go stale silently, cannot be read by
 * a screen reader, and would be the one thing on a page about drift that drifts.
 * The terminal is a <pre> with a hand-rolled JSON tint; the browser is the same
 * card markup the Comments section already recreates, given a chrome bar.
 *
 * The terminal half is the site's ONE deliberately dark surface (--code-*), the
 * viewer half is paper (--surface). That contrast is the point: they are two
 * different places, and the eye should not have to be told twice.
 */

/* One line of terminal output, tinted.
 *
 * The site already has prism-react-renderer for real code blocks, and it is
 * deliberately not used here: this is a TERMINAL, where the prompt line and the
 * program's answer have to look like different things, and a JSON tokenizer
 * knows nothing about "$ ". So the prompt is handled first, and the remaining
 * lines get a five-token tint — key, string, number, literal, punctuation —
 * which is every token JSON has. Tokens become React elements, never HTML
 * strings; nothing here goes near dangerouslySetInnerHTML. */
const JSON_TOKEN =
  /("(?:[^"\\]|\\.)*")(\s*:)?|\b(true|false|null)\b|(-?\d+(?:\.\d+)?)/g;

function tintJson(line: string, keyPrefix: string): ReactNode[] {
  const nodes: ReactNode[] = [];
  let last = 0;
  let i = 0;
  let match: RegExpExecArray | null;

  JSON_TOKEN.lastIndex = 0;
  while ((match = JSON_TOKEN.exec(line)) !== null) {
    if (match.index > last) nodes.push(line.slice(last, match.index));
    const key = `${keyPrefix}-${i++}`;
    const [whole, str, colon, literal, num] = match;

    if (str !== undefined) {
      // A string immediately followed by a colon is a KEY; the same bytes in
      // any other position are a value. That one lookahead is the only piece of
      // grammar this needs to know.
      nodes.push(
        <span key={key} className={colon ? "vterm__key" : "vterm__str"}>
          {str}
        </span>,
      );
      if (colon) nodes.push(<span key={`${key}-c`}>{colon}</span>);
    } else if (literal !== undefined) {
      nodes.push(
        <span key={key} className="vterm__lit">
          {literal}
        </span>,
      );
    } else if (num !== undefined) {
      nodes.push(
        <span key={key} className="vterm__num">
          {num}
        </span>,
      );
    } else {
      nodes.push(whole);
    }
    last = match.index + whole.length;
  }
  if (last < line.length) nodes.push(line.slice(last));
  return nodes;
}

function Terminal({ title, code }: { title: string; code: string }) {
  return (
    <div className="vterm">
      <div className="term__bar">
        <span className="term__dots">
          <i />
          <i />
          <i />
        </span>
        <span className="term__title">{title}</span>
      </div>
      <pre className="vterm__body">
        {code.split("\n").map((line, i) =>
          line.startsWith("$ ") ? (
            <div key={i} className="vterm__prompt">
              {line.slice(2)}
            </div>
          ) : (
            <div key={i} className="vterm__out">
              {tintJson(line, String(i))}
            </div>
          ),
        )}
      </pre>
    </div>
  );
}

function ViewerCard({ url, card }: { url: string; card: PairCard }) {
  const { thread } = card;
  return (
    <div className="vbrowser">
      {/* Browser chrome. The URL is a localhost address with a random high port
          because that is what serve actually binds — a reviewer who has run it
          recognises the shape, and it quietly says "this is not on the
          internet". It is text, not an input: nothing here is operable. */}
      <div className="vbrowser__bar">
        <span className="vbrowser__dots">
          <i />
          <i />
          <i />
        </span>
        <span className="vbrowser__url">{url}</span>
      </div>

      <div className="vbrowser__page">
        <div className="cmt-card cmt-card--commented">
          <div className="cmt-card__head">
            <code className="cmt-card__id">{card.id}</code>
            <span
              className="cmt-chip cmt-chip--open"
              role="img"
              aria-label="1 open comment thread"
            >
              <span aria-hidden="true">💬</span>
              <span className="cmt-chip__count">1</span>
            </span>
            <span className="cmt-card__status">{card.status}</span>
          </div>
          <p className="cmt-card__meta">
            {card.module} · {card.facet} · table · {card.statusNote}
          </p>
          <div className="cmt-card__rows" aria-hidden="true">
            {card.rows.map((r) => (
              <div
                key={r.field}
                className={`cmt-card__row${
                  r.flagged ? " cmt-card__row--flagged" : ""
                }`}
              >
                <span>{r.field}</span>
                <span>{r.type}</span>
              </div>
            ))}
          </div>
        </div>

        <div className="cmt-panel">
          <p className="cmt-panel__title">{card.panelTitle}</p>

          <div className="cmt-thread">
            <div className="cmt-meta">
              <RolePill role={thread.role} />
              <span className="cmt-time">{thread.created}</span>
              <code className="cmt-tid">{thread.id}</code>
            </div>
            <p className="cmt-body">{thread.body}</p>

            {thread.replies.map((r) => (
              <div className="cmt-reply" key={r.id}>
                <div className="cmt-meta">
                  <RolePill role={r.role} />
                  <span className="cmt-time">{r.created}</span>
                </div>
                <p className="cmt-body">{r.body}</p>
              </div>
            ))}

            {/* The button that is the approval. Rendered as inert markup
                (aria-hidden, no handler) because this is a picture of the
                viewer, not the viewer — but drawn at full strength, because
                every other control in this vignette is deliberately absent and
                this one being prominent IS the message. */}
            <div className="cmt-actions" aria-hidden="true">
              <span className="vw-btn vw-btn--primary">Resolve</span>
              <span className="vw-btn">Reply</span>
            </div>
          </div>

          {card.resolvedCount > 0 && (
            <details className="cmt-resolved">
              <summary>{card.resolvedCount} resolved</summary>
              <div className="cmt-thread cmt-thread--resolved">
                <div className="cmt-meta">
                  <RolePill role="human" />
                  <span className="cmt-time">2026-07-25 · 16:04</span>
                </div>
                <p className="cmt-body">
                  Confirmed the envelope field names against the contract facet.
                </p>
              </div>
            </details>
          )}
        </div>
      </div>
    </div>
  );
}

export function SurfacePair({
  data,
  note,
}: {
  data: SurfacePairData;
  note: string;
}) {
  return (
    <figure
      className="pair"
      aria-label={`One claim, ${data.claimID}, seen from both surfaces: the agent's terminal running dossierx comment inbox, and the same thread rendered in the reviewer's browser`}
    >
      <div className="pair__grid">
        <div className="pair__half">
          <p className="pair__label">
            <RolePill role="agent" />
            {data.terminal.title}
          </p>
          <AnimatedReveal y={16}>
            <Terminal title={data.terminal.url} code={data.terminal.code} />
          </AnimatedReveal>
          <p className="pair__note">{data.terminal.note}</p>
        </div>

        <div className="pair__half">
          <p className="pair__label">
            <RolePill role="human" />
            {data.viewer.title}
          </p>
          <AnimatedReveal y={16} delay={0.06}>
            <ViewerCard url={data.viewer.url} card={data.card} />
          </AnimatedReveal>
          <p className="pair__note">{data.viewer.note}</p>
        </div>
      </div>

      <figcaption className="pair__caption">
        <code>{data.claimID}</code>
        <span>{note}</span>
      </figcaption>
    </figure>
  );
}
