import {
  Highlight,
  type Language,
  type PrismTheme,
} from "prism-react-renderer";
import clsx from "clsx";

interface CodeBlockProps {
  code: string;
  lang?: string;
  title?: string;
  className?: string;
}

// Map the content spec's informal lang labels onto Prism language ids.
const LANG_ALIAS: Record<string, Language> = {
  yaml: "yaml",
  yml: "yaml",
  bash: "bash",
  sh: "bash",
  shell: "bash",
  python: "python",
  py: "python",
  go: "go",
  text: "markup",
  txt: "markup",
};

const dossierTheme: PrismTheme = {
  plain: { color: "var(--code-text)", backgroundColor: "var(--code-bg)" },
  styles: [
    {
      types: ["comment", "prolog", "doctype", "cdata"],
      style: { color: "var(--code-text-faint)", fontStyle: "italic" },
    },
    {
      types: ["punctuation", "operator"],
      style: { color: "var(--code-text-dim)" },
    },
    {
      types: ["property", "tag", "constant", "symbol", "deleted"],
      style: { color: "var(--code-accent)" },
    },
    {
      types: [
        "boolean",
        "number",
        "selector",
        "attr-name",
        "string",
        "char",
        "builtin",
        "inserted",
      ],
      style: { color: "var(--code-ok)" },
    },
    {
      types: ["atrule", "attr-value", "keyword"],
      style: { color: "var(--code-accent-strong)" },
    },
    { types: ["function", "class-name"], style: { color: "var(--code-text)" } },
    {
      types: ["regex", "important", "variable"],
      style: { color: "var(--code-accent)" },
    },
  ],
};

/**
 * Syntax-highlighted code block backed by prism-react-renderer (lightweight,
 * no global CSS, tokenizes at render time). Falls back to plain markup for
 * unknown languages such as `text`.
 */
export function CodeBlock({
  code,
  lang = "text",
  title,
  className,
}: CodeBlockProps) {
  const language = LANG_ALIAS[lang.toLowerCase()] ?? "markup";

  return (
    <figure className={clsx("codeblock", className)}>
      {title && (
        <figcaption className="codeblock__title">
          <span className="codeblock__lang">{lang}</span>
          {title}
        </figcaption>
      )}
      <Highlight code={code.trimEnd()} language={language} theme={dossierTheme}>
        {({ className: cls, style, tokens, getLineProps, getTokenProps }) => (
          <pre className={clsx("codeblock__pre", cls)} style={style}>
            <code>
              {tokens.map((line, i) => (
                <span
                  key={i}
                  {...getLineProps({ line })}
                  className="codeblock__line"
                >
                  <span className="codeblock__lineno">{i + 1}</span>
                  <span className="codeblock__linecontent">
                    {line.map((token, key) => (
                      <span key={key} {...getTokenProps({ token })} />
                    ))}
                  </span>
                </span>
              ))}
            </code>
          </pre>
        )}
      </Highlight>
    </figure>
  );
}
