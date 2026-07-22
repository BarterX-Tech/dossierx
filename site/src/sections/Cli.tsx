import { useMemo, useState } from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { AnimatedReveal } from "../components/AnimatedReveal";
import { SectionContainer } from "../components/SectionContainer";
import { SectionHeader } from "../components/SectionHeader";
import { getSection } from "./section-utils";

interface Command {
  name: string;
  usage: string;
  summary: string;
  detail: string;
  example: string;
}
interface Group {
  group: string;
  commands: Command[];
}
interface CliData {
  groups: Group[];
  globalFlag: { name: string; desc: string };
}

interface FlatCommand extends Command {
  group: string;
}

// Render a command's example transcript with prompt/output styling. Lines that
// begin with "$ " are shell prompts; everything else is program output.
function Transcript({ text }: { text: string }) {
  return (
    <pre className="term-out">
      {text.split("\n").map((line, i) => {
        const isPrompt = line.startsWith("$ ");
        return (
          <div
            key={i}
            className={
              isPrompt ? "term-out__line--prompt" : "term-out__line--out"
            }
          >
            {isPrompt ? line.slice(2) : line}
          </div>
        );
      })}
    </pre>
  );
}

export function Cli() {
  const section = getSection("cli");
  const data = section.data as unknown as CliData;
  const [query, setQuery] = useState("");
  const [activeGroup, setActiveGroup] = useState<string | null>(null);
  const [open, setOpen] = useState<string | null>(null);
  const reduce = useReducedMotion();

  const all: FlatCommand[] = useMemo(
    () =>
      data.groups.flatMap((g) =>
        g.commands.map((c) => ({ ...c, group: g.group })),
      ),
    [data.groups],
  );

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return all.filter((c) => {
      if (activeGroup && c.group !== activeGroup) return false;
      if (!q) return true;
      return (
        c.name.toLowerCase().includes(q) ||
        c.usage.toLowerCase().includes(q) ||
        c.summary.toLowerCase().includes(q) ||
        c.detail.toLowerCase().includes(q)
      );
    });
  }, [all, query, activeGroup]);

  return (
    <SectionContainer id={section.id}>
      <SectionHeader
        eyebrow="Command line"
        title={section.title}
        contentMd={section.contentMd}
      />

      <AnimatedReveal>
        <div className="cli">
          <div className="term__bar">
            <span className="term__dots">
              <i />
              <i />
              <i />
            </span>
            <span className="term__title">dossierx · command reference</span>
          </div>

          <div className="cli__search">
            <span className="cli__prompt">❯</span>
            <input
              className="cli__input"
              type="text"
              value={query}
              placeholder="filter commands… (lock, build-order, drift)"
              aria-label="Filter CLI commands"
              onChange={(e) => setQuery(e.target.value)}
            />
            <span className="cli__count" aria-live="polite">
              <span className="sr-only">Showing </span>
              {filtered.length}/{all.length}
              <span className="sr-only"> commands</span>
            </span>
          </div>

          <div
            className="cli__filters"
            role="group"
            aria-label="Filter commands by group"
          >
            <button
              type="button"
              className={`cli__chip${activeGroup === null ? " cli__chip--active" : ""}`}
              aria-pressed={activeGroup === null}
              onClick={() => setActiveGroup(null)}
            >
              all
            </button>
            {data.groups.map((g) => (
              <button
                key={g.group}
                type="button"
                className={`cli__chip${activeGroup === g.group ? " cli__chip--active" : ""}`}
                aria-pressed={activeGroup === g.group}
                onClick={() =>
                  setActiveGroup((cur) => (cur === g.group ? null : g.group))
                }
              >
                {g.group}
              </button>
            ))}
          </div>

          <div className="cli__list">
            {filtered.length === 0 && (
              <p className="cli__empty">no command matches “{query}”.</p>
            )}
            {filtered.map((c) => {
              const isOpen = open === c.name;
              return (
                <div className="cmd" key={c.name}>
                  <button
                    type="button"
                    className="cmd__head"
                    aria-expanded={isOpen}
                    onClick={() => setOpen(isOpen ? null : c.name)}
                  >
                    <span
                      className={`cmd__caret${isOpen ? " cmd__caret--open" : ""}`}
                    >
                      ▶
                    </span>
                    <span className="cmd__name">{c.name}</span>
                    <span className="cmd__group">{c.group}</span>
                    <span className="cmd__summary">{c.summary}</span>
                  </button>
                  <AnimatePresence initial={false}>
                    {isOpen && (
                      <motion.div
                        className="cmd__body"
                        initial={
                          reduce
                            ? { height: "auto", opacity: 1 }
                            : { height: 0, opacity: 0 }
                        }
                        animate={{ height: "auto", opacity: 1 }}
                        exit={
                          reduce ? { opacity: 0 } : { height: 0, opacity: 0 }
                        }
                        transition={
                          reduce
                            ? { duration: 0 }
                            : { duration: 0.28, ease: [0.22, 1, 0.36, 1] }
                        }
                      >
                        <div className="cmd__body-inner">
                          <code className="cmd__usage">{c.usage}</code>
                          <p className="cmd__detail">{c.detail}</p>
                          <Transcript text={c.example} />
                        </div>
                      </motion.div>
                    )}
                  </AnimatePresence>
                </div>
              );
            })}
          </div>
        </div>
      </AnimatedReveal>

      <AnimatedReveal>
        <p className="note">
          <code>{data.globalFlag.name}</code> — {data.globalFlag.desc}
        </p>
      </AnimatedReveal>
    </SectionContainer>
  );
}
