import { AnimatedReveal } from "./AnimatedReveal";

export interface Release {
  version: string;
  date: string;
  title: string;
  tag: string;
  highlights: string[];
  // There is deliberately NO `commit` field. It held the tagged release's short
  // sha, it could not converge (writing the sha is itself a commit, so the value
  // was stale the moment it landed), it named the wrong sha for two releases
  // running, and it disagreed with the binary by construction — seven characters
  // against the forty GoReleaser stamps into `main.commit`. The data, its one
  // reader and the release step that wrote it are all gone.
  //
  // The DECLARATION is what outlived them, and it is why this comment exists
  // rather than nothing. An optional field on the interface is a standing
  // invitation: it makes `commit: "abc1234"` on an entry type-check, so the
  // field comes back silently and only then acquires a reader. Do not re-add it.
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
