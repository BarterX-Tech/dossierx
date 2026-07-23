import { AnimatedReveal } from "./AnimatedReveal";

export interface Release {
  version: string;
  date: string;
  title: string;
  tag: string;
  highlights: string[];
}

interface ReleaseTimelineProps {
  releases: Release[];
  /** Cap the number of entries rendered (most-recent-first is handled by the caller). */
  limit?: number;
}

/**
 * The release ledger's `<ol>` timeline markup, shared by the compact landing-page
 * teaser (Versions.tsx, capped) and the full releases page (ReleasesPage.tsx,
 * uncapped) so both read from one rendering — the release data itself still
 * lives once in content.ts.
 */
export function ReleaseTimeline({ releases, limit }: ReleaseTimelineProps) {
  const latestIndex = releases.length - 1;
  const shown =
    limit === undefined ? releases : releases.slice(latestIndex - limit + 1);

  return (
    <ol className="timeline">
      {shown.map((r) => {
        const i = releases.indexOf(r);
        return (
          <li
            key={r.version}
            className={`tl-item${i === latestIndex ? " tl-item--latest" : ""}`}
          >
            <span className="tl-item__dot" aria-hidden="true" />
            <AnimatedReveal delay={0.04 * (i - (latestIndex - shown.length + 1))} y={16}>
              <div className="release">
                <div className="release__head">
                  <span className="release__version">{r.version}</span>
                  <time className="release__date">{r.date}</time>
                  {i === latestIndex && (
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
        );
      })}
    </ol>
  );
}
