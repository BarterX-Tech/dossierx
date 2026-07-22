import { Fragment, type ReactNode } from "react";

interface MarkdownProps {
  children: string;
  className?: string;
}

// Minimal inline formatter: **bold**, *italic*, `code`, and [text](href).
// Deliberately tiny — enough for the content spec's prose, no external lib.
function renderInline(text: string, keyPrefix: string): ReactNode[] {
  const nodes: ReactNode[] = [];
  const pattern = /(\*\*[^*]+\*\*|\*[^*]+\*|`[^`]+`|\[[^\]]+\]\([^)]+\))/g;
  let last = 0;
  let match: RegExpExecArray | null;
  let i = 0;

  while ((match = pattern.exec(text)) !== null) {
    if (match.index > last) {
      nodes.push(text.slice(last, match.index));
    }
    const tok = match[0];
    const key = `${keyPrefix}-${i++}`;
    if (tok.startsWith("**")) {
      nodes.push(<strong key={key}>{tok.slice(2, -2)}</strong>);
    } else if (tok.startsWith("`")) {
      nodes.push(<code key={key}>{tok.slice(1, -1)}</code>);
    } else if (tok.startsWith("[")) {
      const m = /\[([^\]]+)\]\(([^)]+)\)/.exec(tok);
      if (m) {
        nodes.push(
          <a key={key} href={m[2]}>
            {m[1]}
          </a>,
        );
      }
    } else {
      nodes.push(<em key={key}>{tok.slice(1, -1)}</em>);
    }
    last = match.index + tok.length;
  }
  if (last < text.length) nodes.push(text.slice(last));
  return nodes;
}

/**
 * Renders a small subset of Markdown: paragraphs, ordered/unordered lists,
 * and inline bold/italic/code/links. Blocks are split on blank lines.
 */
export function Markdown({ children, className }: MarkdownProps) {
  const blocks = children.split(/\n{2,}/);

  return (
    <div className={className}>
      {blocks.map((block, bi) => {
        const lines = block.split("\n");
        const isOrdered = lines.every((l) => /^\d+\.\s/.test(l));
        const isUnordered = lines.every((l) => /^[-*]\s/.test(l));

        if (isOrdered) {
          return (
            <ol key={bi}>
              {lines.map((l, li) => (
                <li key={li}>
                  {renderInline(l.replace(/^\d+\.\s/, ""), `${bi}-${li}`)}
                </li>
              ))}
            </ol>
          );
        }
        if (isUnordered) {
          return (
            <ul key={bi}>
              {lines.map((l, li) => (
                <li key={li}>
                  {renderInline(l.replace(/^[-*]\s/, ""), `${bi}-${li}`)}
                </li>
              ))}
            </ul>
          );
        }
        return (
          <p key={bi}>
            {lines.map((l, li) => (
              <Fragment key={li}>
                {li > 0 && <br />}
                {renderInline(l, `${bi}-${li}`)}
              </Fragment>
            ))}
          </p>
        );
      })}
    </div>
  );
}
