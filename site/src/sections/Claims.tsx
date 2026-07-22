import { AnimatedReveal } from "../components/AnimatedReveal";
import { TabbedCode } from "../components/TabbedCode";
import { SectionContainer } from "../components/SectionContainer";
import { SectionHeader } from "../components/SectionHeader";
import { FileTree } from "../components/FileTree";
import { getSection } from "./section-utils";

interface Field {
  name: string;
  desc: string;
}
interface EdgeType {
  name: string;
  semantics: string;
}
interface ClaimsData {
  requiredFields: Field[];
  optionalFields: Field[];
  engineManagedFields: Field[];
  edgeTypes: EdgeType[];
}

function FieldList({ title, fields }: { title: string; fields: Field[] }) {
  return (
    <div className="fieldlist">
      <h3 className="fieldlist__title">{title}</h3>
      <dl className="fieldlist__dl">
        {fields.map((f) => (
          <div key={f.name} className="fieldlist__row">
            <dt>
              <code>{f.name}</code>
            </dt>
            <dd>{f.desc}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

function IdAnatomy() {
  const segs = [
    { cls: "id-seg--module", val: "module", label: "configured" },
    { cls: "id-seg--facet", val: "facet", label: "configured" },
    { cls: "id-seg--slug", val: "slug", label: "kebab-case" },
  ];
  return (
    <div className="id-anatomy">
      <p className="id-anatomy__title">
        Every claim id is exactly three segments
      </p>
      <div className="id-anatomy__row">
        {segs.map((s, i) => (
          <span
            key={s.val}
            style={{
              display: "inline-flex",
              alignItems: "flex-start",
              gap: "0.35rem",
            }}
          >
            <span className={`id-seg ${s.cls}`}>
              <span className="id-seg__val">{s.val}</span>
              <span className="id-seg__label">{s.label}</span>
            </span>
            {i < segs.length - 1 && <span className="id-dot">.</span>}
          </span>
        ))}
      </div>
    </div>
  );
}

export function Claims() {
  const section = getSection("claims");
  const data = section.data as unknown as ClaimsData;

  return (
    <SectionContainer id={section.id}>
      <SectionHeader
        eyebrow="The claim"
        title={section.title}
        contentMd={section.contentMd}
      />

      <AnimatedReveal>
        <IdAnatomy />
      </AnimatedReveal>

      {section.codeExamples && (
        <AnimatedReveal>
          <TabbedCode examples={section.codeExamples} />
        </AnimatedReveal>
      )}

      <AnimatedReveal>
        <FileTree />
      </AnimatedReveal>

      <div className="claims__fields">
        <AnimatedReveal>
          <FieldList title="Required fields" fields={data.requiredFields} />
        </AnimatedReveal>
        <AnimatedReveal delay={0.05}>
          <FieldList title="Optional fields" fields={data.optionalFields} />
        </AnimatedReveal>
        <AnimatedReveal delay={0.1}>
          <FieldList
            title="Engine-managed fields"
            fields={data.engineManagedFields}
          />
        </AnimatedReveal>
      </div>

      <AnimatedReveal>
        <div className="edgetypes">
          <h3 className="fieldlist__title">
            Three deliberately distinct edge types
          </h3>
          <div className="edgetypes__grid">
            {data.edgeTypes.map((e) => (
              <div key={e.name} className="edgetype">
                <code className="edgetype__name">{e.name}</code>
                <p>{e.semantics}</p>
              </div>
            ))}
          </div>
        </div>
      </AnimatedReveal>
    </SectionContainer>
  );
}
