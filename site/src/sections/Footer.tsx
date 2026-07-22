import { Markdown } from "../components/Markdown";
import { BrandMark } from "../components/BrandMark";
import { getSection } from "./section-utils";

interface FooterData {
  links: { label: string; href: string }[];
  note: string;
}

export function Footer() {
  const section = getSection("footer");
  const data = section.data as unknown as FooterData;

  return (
    <footer id={section.id} className="footer">
      <div className="section__inner">
        <h2 className="footer__title">
          <BrandMark className="footer__mark" />
          {section.title}
        </h2>
        <Markdown className="prose footer__body">{section.contentMd}</Markdown>

        <nav className="footer__links">
          {data.links.map((l) => (
            <a key={l.href} href={l.href}>
              {l.label}
            </a>
          ))}
        </nav>

        <p className="footer__note">{data.note}</p>
      </div>
    </footer>
  );
}
