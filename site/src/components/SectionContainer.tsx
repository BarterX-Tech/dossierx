import clsx from "clsx";
import type { ReactNode } from "react";

interface SectionContainerProps {
  id: string;
  children: ReactNode;
  /** Optional eyebrow label rendered above the content. */
  eyebrow?: string;
  className?: string;
  /** Alternate background band for visual rhythm. */
  alt?: boolean;
}

/**
 * Standard section shell: an anchor target (id), a max-width content column,
 * and consistent vertical rhythm. Sections compose their own inner layout.
 */
export function SectionContainer({
  id,
  children,
  eyebrow,
  className,
  alt = false,
}: SectionContainerProps) {
  return (
    <section
      id={id}
      className={clsx("section", alt && "section--alt", className)}
    >
      <div className="section__inner">
        {eyebrow && <span className="sr-only">{eyebrow}</span>}
        {children}
      </div>
    </section>
  );
}
