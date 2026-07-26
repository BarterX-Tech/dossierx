import { AnimatedReveal } from "../components/AnimatedReveal";
import { SectionContainer } from "../components/SectionContainer";
import { SectionHeader } from "../components/SectionHeader";
import { RolePill, type Role } from "../components/RolePill";
import { ReviewLoop, type LoopStep } from "../components/ReviewLoop";
import { SurfacePair, type SurfacePairData } from "../components/SurfacePair";
import { getSection } from "./section-utils";

interface CommentsData {
  roles: { id: Role; label: string }[];
  loop: LoopStep[];
  surfacePair: SurfacePairData;
  gateNote: string;
}

/**
 * The review loop, written from the HUMAN's side.
 *
 * v0.2.0's version of this section described a feature ("comments exist, and an
 * unresolved thread blocks locking"). That is true but it is not the loop, and a
 * reader finished it still not knowing what they personally would do. This one
 * follows one disagreement all the way around: you comment, the engine writes it
 * into the claim and re-renders, you say "I left comments", the agent finds them
 * in one call, fixes and replies, you click Resolve — which is the approval —
 * and only then does the agent lock, with your words in --reason.
 *
 * Two visuals carry it: the stepped two-lane loop (who acts, and where), and the
 * terminal/viewer pair (the same claim from both surfaces at once).
 */
export function Comments() {
  const section = getSection("comments");
  const data = section.data as unknown as CommentsData;

  return (
    <SectionContainer id={section.id}>
      <SectionHeader
        eyebrow="The review loop"
        title={section.title}
        contentMd={section.contentMd}
      />

      <AnimatedReveal className="cmt-roles" y={10}>
        <span className="cmt-roles__label">Who acts</span>
        {data.roles.map((r) => (
          <RolePill key={r.id} role={r.id} />
        ))}
      </AnimatedReveal>

      <ReviewLoop steps={data.loop} />

      <SurfacePair data={data.surfacePair} note={data.gateNote} />
    </SectionContainer>
  );
}
