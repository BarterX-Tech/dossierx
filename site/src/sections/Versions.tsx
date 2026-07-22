import { AnimatedReveal } from "../components/AnimatedReveal";
import { SectionContainer } from "../components/SectionContainer";
import { SectionHeader } from "../components/SectionHeader";
import { getSection } from "./section-utils";

interface Release {
  version: string;
  date: string;
  title: string;
  tag: string;
  highlights: string[];
}
interface VersionsData {
  releases: Release[];
}

export function Versions() {
  const section = getSection("versions");
  const data = section.data as unknown as VersionsData;
  const latest = data.releases.length - 1;

  return (
    <SectionContainer id={section.id} alt>
      <SectionHeader
        eyebrow="Release ledger"
        title={section.title}
        contentMd={section.contentMd}
      />

      <ol className="timeline">
        {data.releases.map((r, i) => (
          <li
            key={r.version}
            className={`tl-item${i === latest ? " tl-item--latest" : ""}`}
          >
            <span className="tl-item__dot" aria-hidden="true" />
            <AnimatedReveal delay={0.04 * i} y={16}>
              <div className="release">
                <div className="release__head">
                  <span className="release__version">{r.version}</span>
                  <time className="release__date">{r.date}</time>
                  {i === latest && (
                    <span className="release__latest">latest</span>
                  )}
                </div>
                <h3 className="release__title">{r.title}</h3>
                <p className="release__tag">{r.tag}</p>
                <ul className="release__highlights">
                  {r.highlights.map((h, hi) => (
                    <li key={hi}>{h}</li>
                  ))}
                </ul>
              </div>
            </AnimatedReveal>
          </li>
        ))}
      </ol>
    </SectionContainer>
  );
}
