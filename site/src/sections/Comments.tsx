import { AnimatedReveal } from "../components/AnimatedReveal";
import { SectionContainer } from "../components/SectionContainer";
import { SectionHeader } from "../components/SectionHeader";
import { CodeBlock } from "../components/CodeBlock";
import { getSection } from "./section-utils";

type Role = "human" | "agent" | "engine";

interface WorkflowStep {
  role: Role;
  title: string;
  body: string;
}
interface Reply {
  id: string;
  role: Role;
  created: string;
  body: string;
}
interface Thread {
  id: string;
  role: Role;
  status: string;
  created: string;
  body: string;
  replies: Reply[];
}
interface DemoCard {
  id: string;
  module: string;
  facet: string;
  status: string;
  panelTitle: string;
  thread: Thread;
  resolvedCount: number;
}
interface CommentsData {
  roles: { id: Role; label: string }[];
  workflow: WorkflowStep[];
  card: DemoCard;
  terminal: { lang: string; code: string };
}

/** A role tag — human / agent / engine — matching the viewer's author-role
 * pill vocabulary but styled in the site's warm palette. */
function RolePill({ role }: { role: Role }) {
  return <span className={`role-pill role-pill--${role}`}>{role}</span>;
}

/** The animated review loop: six steps, each stamped with the actor (human,
 * agent, or the engine's automatic gate) that drives it. */
function Workflow({ steps }: { steps: WorkflowStep[] }) {
  return (
    <ol className="cwf">
      {steps.map((s, i) => (
        <AnimatedReveal key={i} delay={0.05 * i} y={16}>
          <li className="cwf__step">
            <span className="cwf__num" aria-hidden="true">
              {i + 1}
            </span>
            <div className="cwf__content">
              <div className="cwf__head">
                <RolePill role={s.role} />
                <span className="cwf__title">{s.title}</span>
              </div>
              <p className="cwf__text">{s.body}</p>
            </div>
          </li>
        </AnimatedReveal>
      ))}
    </ol>
  );
}

/** A React recreation of the viewer's claim card + baked-in comment thread
 * panel (no screenshots): the 💬 chip on a commented card, the thread with its
 * author-role pill and an indented reply, and the "N resolved" collapse. */
function UiVignette({ card }: { card: DemoCard }) {
  const { thread } = card;
  return (
    <figure className="cmt-vignette" aria-label="Viewer claim card with a comment thread">
      <div className="cmt-card cmt-card--commented">
        <div className="cmt-card__head">
          <code className="cmt-card__id">{card.id}</code>
          <span className="cmt-card__status">{card.status}</span>
        </div>
        <p className="cmt-card__meta">
          {card.module} · {card.facet} · table · blocked from locking
        </p>
        <div className="cmt-card__rows" aria-hidden="true">
          <div className="cmt-card__row">
            <span>event_name</span>
            <span>string</span>
          </div>
          <div className="cmt-card__row cmt-card__row--flagged">
            <span>severity</span>
            <span>enum</span>
          </div>
        </div>
        <div className="cmt-card__footer">
          <span
            className="cmt-chip cmt-chip--open"
            role="img"
            aria-label="1 open comment thread"
          >
            <span aria-hidden="true">💬</span>
            <span className="cmt-chip__count">1</span>
          </span>
        </div>
      </div>

      <div className="cmt-panel">
        <p className="cmt-panel__title">{card.panelTitle}</p>

        <div className="cmt-thread">
          <div className="cmt-meta">
            <RolePill role={thread.role} />
            <span className="cmt-time">{thread.created}</span>
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
        </div>

        {card.resolvedCount > 0 && (
          <details className="cmt-resolved">
            <summary>{card.resolvedCount} resolved</summary>
            <div className="cmt-thread cmt-thread--resolved">
              <div className="cmt-meta">
                <RolePill role="human" />
                <span className="cmt-time">2026-07-23 · 16:04</span>
              </div>
              <p className="cmt-body">
                Confirmed the envelope field names against the contract facet.
              </p>
            </div>
          </details>
        )}
      </div>
    </figure>
  );
}

export function Comments() {
  const section = getSection("comments");
  const data = section.data as unknown as CommentsData;

  return (
    <SectionContainer id={section.id}>
      <SectionHeader
        eyebrow="Comments"
        title={section.title}
        contentMd={section.contentMd}
      />

      <AnimatedReveal className="cmt-roles" y={10}>
        <span className="cmt-roles__label">Three actors</span>
        {data.roles.map((r) => (
          <RolePill key={r.id} role={r.id} />
        ))}
      </AnimatedReveal>

      <Workflow steps={data.workflow} />

      <div className="cmt-showcase">
        <div className="cmt-showcase__ui">
          <p className="cmt-showcase__label">In the viewer</p>
          <AnimatedReveal y={16}>
            <UiVignette card={data.card} />
          </AnimatedReveal>
        </div>
        <div className="cmt-showcase__term">
          <p className="cmt-showcase__label">At the terminal</p>
          <AnimatedReveal y={16} delay={0.06}>
            <CodeBlock
              code={data.terminal.code}
              lang={data.terminal.lang}
              title="dossierx serve · the lock gate"
            />
          </AnimatedReveal>
        </div>
      </div>
    </SectionContainer>
  );
}
