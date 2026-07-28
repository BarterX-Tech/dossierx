import { AnimatedReveal } from "../components/AnimatedReveal";
import { SectionContainer } from "../components/SectionContainer";
import { SectionHeader } from "../components/SectionHeader";
import {
  TwoSurfaces,
  type Boundary,
  type Surface,
} from "../components/TwoSurfaces";
import { getSection } from "./section-utils";

interface Enforcement {
  strength: string;
  title: string;
  body: string;
}
interface RolesData {
  surfaces: Surface[];
  boundary: Boundary;
  gate: string;
  enforcement: Enforcement[];
}

/**
 * "Who runs what" — deliberately the FIRST section after the hero, and
 * deliberately before any command appears anywhere on the page.
 *
 * The misconception this section exists to kill is that a person sits down and
 * drives DossierX. Every later section is written assuming the reader already
 * knows they don't: the CLI section is a machine contract rather than a
 * tutorial, the Lifecycle table names who mandates a transition and who
 * executes it, and the review-loop section spends its whole length on the
 * human's two gestures. If a reader arrives at any of those without this page,
 * the copy reads as needlessly indirect.
 *
 * The strengths grid at the end is the honest half. "Hard" means the code makes
 * it impossible, "Blocked" means git refuses the commit, "Detected" means check
 * reports it after the fact, and "Convention" means exactly that — a required
 * --reason that makes an unprompted lock loud rather than impossible. Claiming
 * all four were the same thing would be the easiest and worst thing this
 * section could do.
 */
export function Roles() {
  const section = getSection("roles");
  const data = section.data as unknown as RolesData;

  return (
    <SectionContainer id={section.id} alt>
      <SectionHeader
        eyebrow="Who runs what"
        title={section.title}
        contentMd={section.contentMd}
      />

      <TwoSurfaces surfaces={data.surfaces} boundary={data.boundary} />

      <AnimatedReveal>
        <p className="invariant">
          <strong>What the gate is for.</strong> {data.gate}
        </p>
      </AnimatedReveal>

      <div className="strengths">
        {data.enforcement.map((e, i) => (
          <AnimatedReveal key={e.strength} delay={0.05 * i}>
            <div className={`strength strength--${e.strength.toLowerCase()}`}>
              <span className="strength__badge">{e.strength}</span>
              <h3 className="strength__title">{e.title}</h3>
              <p className="strength__body">{e.body}</p>
            </div>
          </AnimatedReveal>
        ))}
      </div>
    </SectionContainer>
  );
}
