import { AnimatedReveal } from "../components/AnimatedReveal";
import { SectionContainer } from "../components/SectionContainer";
import { SectionHeader } from "../components/SectionHeader";
import { ReleaseTimeline, type Release } from "../components/ReleaseTimeline";
import { getSection } from "./section-utils";

interface VersionsData {
  releases: Release[];
}

/**
 * Landing-page teaser: the latest release only, plus a CTA to the full
 * release-history page (releases.html) — the complete timeline used to live
 * here but was pulled out to its own page so the landing page stays a pitch,
 * not a changelog.
 */
export function Versions() {
  const section = getSection("versions");
  const data = section.data as unknown as VersionsData;

  return (
    <SectionContainer id={section.id} alt>
      <SectionHeader
        eyebrow="Release ledger"
        title={section.title}
        contentMd={section.contentMd}
      />

      <ReleaseTimeline releases={data.releases} limit={1} />

      <AnimatedReveal className="versions__cta-row" delay={0.08} y={16}>
        <a
          className="button button--ghost"
          href={`${import.meta.env.BASE_URL}releases.html`}
        >
          See the full release history →
        </a>
      </AnimatedReveal>
    </SectionContainer>
  );
}
