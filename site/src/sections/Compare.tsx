import { AnimatedReveal } from "../components/AnimatedReveal";
import { SectionContainer } from "../components/SectionContainer";
import { SectionHeader } from "../components/SectionHeader";
import { getSection } from "./section-utils";

interface CompareRow {
  property: string;
  wiki: string;
  adr: string;
  markdown: string;
  dossierx: string;
}
interface CompareData {
  columns: string[];
  rows: CompareRow[];
}

export function Compare() {
  const section = getSection("compare");
  const data = section.data as unknown as CompareData;

  return (
    <SectionContainer id={section.id} className="section--reading">
      <SectionHeader
        eyebrow="The comparison"
        title={section.title}
        contentMd={section.contentMd}
      />

      <AnimatedReveal>
        <div className="compare__scroll">
          <table className="compare">
            <thead>
              <tr>
                {data.columns.map((c) => (
                  <th key={c}>{c}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {data.rows.map((r) => (
                <tr key={r.property}>
                  <th scope="row">{r.property}</th>
                  <td>{r.wiki}</td>
                  <td>{r.adr}</td>
                  <td>{r.markdown}</td>
                  <td className="compare__dossierx">{r.dossierx}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </AnimatedReveal>
    </SectionContainer>
  );
}
