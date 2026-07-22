import { AnimatedReveal } from "../components/AnimatedReveal";
import { TabbedCode } from "../components/TabbedCode";
import { SectionContainer } from "../components/SectionContainer";
import { SectionHeader } from "../components/SectionHeader";
import { getSection } from "./section-utils";

interface Channel {
  channel: string;
  about: string;
  owner: string;
  trigger: string;
}
interface CodeLinksData {
  channels: Channel[];
  unlinkedNote: string;
}

export function CodeLinks() {
  const section = getSection("code-links");
  const data = section.data as unknown as CodeLinksData;

  return (
    <SectionContainer id={section.id} alt>
      <SectionHeader
        eyebrow="Code links"
        title={section.title}
        contentMd={section.contentMd}
      />

      <div className="channels">
        {data.channels.map((c, i) => (
          <AnimatedReveal key={c.channel} delay={0.05 * i}>
            <div
              className={`channel${c.channel.startsWith("B") ? " channel--b" : ""}`}
            >
              <h3 className="channel__title">{c.channel}</h3>
              <dl>
                <dt>About</dt>
                <dd>{c.about}</dd>
                <dt>Owner</dt>
                <dd>{c.owner}</dd>
                <dt>Trigger</dt>
                <dd>{c.trigger}</dd>
              </dl>
            </div>
          </AnimatedReveal>
        ))}
      </div>

      {section.codeExamples && (
        <AnimatedReveal>
          <TabbedCode examples={section.codeExamples} />
        </AnimatedReveal>
      )}

      <AnimatedReveal>
        <p className="note">{data.unlinkedNote}</p>
      </AnimatedReveal>
    </SectionContainer>
  );
}
