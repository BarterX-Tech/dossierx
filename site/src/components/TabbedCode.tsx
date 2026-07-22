import { useState } from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import clsx from "clsx";
import { CodeBlock } from "./CodeBlock";
import type { CodeExample } from "../content";

interface TabbedCodeProps {
  examples: CodeExample[];
  className?: string;
}

/**
 * Renders a set of code examples as a single tabbed panel. Falls back to a
 * plain stack of one when only a single example is present.
 */
export function TabbedCode({ examples, className }: TabbedCodeProps) {
  const [active, setActive] = useState(0);
  const reduce = useReducedMotion();
  if (examples.length === 0) return null;

  const current = examples[Math.min(active, examples.length - 1)];

  return (
    <div className={clsx("codetabs", className)}>
      {examples.length > 1 && (
        <div
          className="codetabs__tabs"
          role="tablist"
          aria-label="Code examples"
        >
          {examples.map((ex, i) => (
            <button
              key={ex.title}
              type="button"
              role="tab"
              id={`codetab-${ex.title}`}
              aria-selected={i === active}
              aria-controls={`codepanel-${ex.title}`}
              className={clsx(
                "codetabs__tab",
                i === active && "codetabs__tab--active",
              )}
              onClick={() => setActive(i)}
            >
              {ex.title}
            </button>
          ))}
        </div>
      )}
      <AnimatePresence mode="wait">
        <motion.div
          key={current.title}
          role={examples.length > 1 ? "tabpanel" : undefined}
          id={examples.length > 1 ? `codepanel-${current.title}` : undefined}
          aria-labelledby={
            examples.length > 1 ? `codetab-${current.title}` : undefined
          }
          initial={reduce ? { opacity: 1 } : { opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          exit={reduce ? { opacity: 0 } : { opacity: 0, y: -8 }}
          transition={{ duration: reduce ? 0 : 0.22, ease: "easeOut" }}
        >
          <CodeBlock
            code={current.code}
            lang={current.lang}
            title={examples.length > 1 ? undefined : current.title}
          />
        </motion.div>
      </AnimatePresence>
    </div>
  );
}
