import { AnimatedReveal } from "../components/AnimatedReveal";
import { SectionContainer } from "../components/SectionContainer";
import { SectionHeader } from "../components/SectionHeader";
import { getSection } from "./section-utils";

interface PhilosophyData {
  principles: { title: string; body: string }[];
}

export function Philosophy() {
  const section = getSection("philosophy");
  const data = section.data as unknown as PhilosophyData;

  return (
    <SectionContainer id={section.id} className="section--reading" alt>
      <SectionHeader
        eyebrow="Why it exists"
        title={section.title}
        contentMd={section.contentMd}
      />

      <div className="card-grid">
        {data.principles.map((p, i) => (
          <AnimatedReveal key={p.title} delay={0.05 * i}>
            <div className="card">
              <span className="card__index">
                {String(i + 1).padStart(2, "0")}
              </span>
              <h3 className="card__title">{p.title}</h3>
              <p className="card__body">{p.body}</p>
            </div>
          </AnimatedReveal>
        ))}
      </div>
    </SectionContainer>
  );
}
