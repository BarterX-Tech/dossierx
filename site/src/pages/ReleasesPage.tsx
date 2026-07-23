import { AnimatedReveal } from "../components/AnimatedReveal";
import { SectionContainer } from "../components/SectionContainer";
import { SectionHeader } from "../components/SectionHeader";
import { ReleaseTimeline, type Release } from "../components/ReleaseTimeline";
import { BrandMark } from "../components/BrandMark";
import { Footer } from "../sections/Footer";
import { getSection } from "../sections/section-utils";

interface VersionsData {
  releases: Release[];
}

/** Standalone full release-history page (releases.html) — a genuine second
 * static entry point (see vite.config.ts's build.rollupOptions.input), not a
 * client-side route, since the site deploys as static GitHub Pages output
 * with no SPA rewrite. Reuses the same "versions" content-spec section and
 * ReleaseTimeline rendering the landing-page teaser uses, uncapped. */
export function ReleasesPage() {
  const section = getSection("versions");
  const data = section.data as unknown as VersionsData;

  return (
    <>
      <header className="releases-header">
        <div className="section__inner releases-header__inner">
          <a
            className="releases-header__brand"
            href={import.meta.env.BASE_URL}
          >
            <BrandMark className="releases-header__mark" />
            DossierX
          </a>
          <a className="releases-header__back" href={import.meta.env.BASE_URL}>
            ← Back to home
          </a>
        </div>
      </header>

      <main id="main">
        <SectionContainer id="releases">
          <AnimatedReveal y={16}>
            <SectionHeader
              eyebrow="Release ledger"
              title="Full release history."
              contentMd={section.contentMd}
            />
          </AnimatedReveal>

          <ReleaseTimeline releases={data.releases} />
        </SectionContainer>
      </main>

      <Footer />
    </>
  );
}
