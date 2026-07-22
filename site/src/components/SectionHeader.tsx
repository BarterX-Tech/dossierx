import { AnimatedReveal } from "./AnimatedReveal";
import { Markdown } from "./Markdown";

interface SectionHeaderProps {
  eyebrow: string;
  title: string;
  contentMd: string;
}

/** A shared editorial lead-in so every section follows one typographic system. */
export function SectionHeader({
  eyebrow,
  title,
  contentMd,
}: SectionHeaderProps) {
  return (
    <header className="section__intro">
      <AnimatedReveal className="section__rail" y={10}>
        <p className="section__eyebrow">{eyebrow}</p>
      </AnimatedReveal>
      <AnimatedReveal className="section__heading" y={16}>
        <h2 className="section__title">{title}</h2>
      </AnimatedReveal>
      <AnimatedReveal className="section__lede" delay={0.04} y={16}>
        <Markdown className="prose">{contentMd}</Markdown>
      </AnimatedReveal>
    </header>
  );
}
