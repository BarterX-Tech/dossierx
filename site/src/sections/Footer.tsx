import { Markdown } from "../components/Markdown";
import { BrandMark } from "../components/BrandMark";
import { getSection } from "./section-utils";

interface FooterData {
  links: { label: string; href: string }[];
  note: string;
}

// The footer is shared by index.html and releases.html, but the four in-page
// anchors it carries — #hero, #roles, #comments, #cli — are sections only
// index.html renders. Emitted bare, they resolve against whatever page is
// showing, so on releases.html all four were dead links to itself. They are
// rewritten to index.html there and left bare on index, where a bare fragment
// is a same-page scroll rather than a navigation.
function resolveHref(href: string): string {
  if (!href.startsWith("#")) return href;
  const onIndex =
    typeof window === "undefined" ||
    !window.location.pathname.endsWith("releases.html");
  return onIndex ? href : `index.html${href}`;
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
            <a key={l.href} href={resolveHref(l.href)}>
              {l.label}
            </a>
          ))}
        </nav>

        <p className="footer__note">{data.note}</p>
      </div>
    </footer>
  );
}
