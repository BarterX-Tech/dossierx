interface TreeNode {
  name: string;
  kind: "dir" | "file" | "engine-file";
  note?: string;
  children?: TreeNode[];
}

const TREE: TreeNode[] = [
  {
    name: "project.config.yaml",
    kind: "file",
    note: "the only project-specific input the engine reads",
  },
  {
    name: "claims/",
    kind: "dir",
    note: "claims_dir — path is configurable, layout carries no meaning to the engine",
    children: [
      {
        name: "<module>/",
        kind: "dir",
        children: [
          {
            name: "<facet>/",
            kind: "dir",
            children: [
              { name: "01-role-and-scope.yaml", kind: "file" },
              { name: "02-workflows-lifecycle.yaml", kind: "file" },
            ],
          },
        ],
      },
    ],
  },
  {
    name: ".catalog.json",
    kind: "engine-file",
    note: "deterministic, alphabetical-by-id — reviewable in a diff",
  },
  {
    name: ".dossierx-lock-store.json",
    kind: "engine-file",
    note: "content-hash baseline for DetectStale",
  },
  {
    name: ".dossierx-flag-store.json",
    kind: "engine-file",
    note: "agent-initiated docs-flag drift notes",
  },
  {
    name: "viewer/",
    kind: "dir",
    children: [
      {
        name: "index.html",
        kind: "file",
        note: "self-contained, static — the render output",
      },
    ],
  },
];

function FolderIcon() {
  return (
    <svg
      className="filetree__icon"
      viewBox="0 0 20 16"
      fill="none"
      aria-hidden="true"
    >
      <path
        d="M1 2.5C1 1.67 1.67 1 2.5 1H7l1.6 2H17.5c.83 0 1.5.67 1.5 1.5v9c0 .83-.67 1.5-1.5 1.5h-15C1.67 15 1 14.33 1 13.5v-11Z"
        stroke="currentColor"
        strokeWidth="1.3"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function FileIcon({ engine }: { engine?: boolean }) {
  return (
    <svg
      className="filetree__icon"
      viewBox="0 0 14 18"
      fill="none"
      aria-hidden="true"
    >
      <path
        d="M1.5 1.5h7L12.5 5.5v11a1 1 0 0 1-1 1h-10a1 1 0 0 1-1-1v-14a1 1 0 0 1 1-1Z"
        stroke="currentColor"
        strokeWidth="1.2"
        strokeLinejoin="round"
        fill={engine ? "currentColor" : "none"}
        fillOpacity={engine ? 0.12 : 0}
      />
      <path
        d="M8.5 1.5v4h4"
        stroke="currentColor"
        strokeWidth="1.2"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function TreeList({ nodes }: { nodes: TreeNode[] }) {
  return (
    <ul className="filetree__list">
      {nodes.map((n) => (
        <li key={n.name} className="filetree__item">
          <span
            className={`filetree__row${n.kind === "engine-file" ? " filetree__row--engine" : ""}`}
          >
            {n.kind === "dir" ? (
              <FolderIcon />
            ) : (
              <FileIcon engine={n.kind === "engine-file"} />
            )}
            <code className="filetree__name">{n.name}</code>
            {n.note && <span className="filetree__note">{n.note}</span>}
          </span>
          {n.children && <TreeList nodes={n.children} />}
        </li>
      ))}
    </ul>
  );
}

/**
 * A real on-disk layout for a project consuming DossierX, grounded in the
 * dossierx/ directory of a private, multi-module production app that had been
 * burned by silent documentation drift — not an invented example.
 */
export function FileTree() {
  return (
    <div className="filetree">
      <div className="filetree__frame">
        <div className="filetree__header">
          <span className="filetree__kicker">file structure</span>
          <span className="filetree__kicker filetree__kicker--right">
            on disk
          </span>
        </div>
        <p className="filetree__title">A project consuming DossierX</p>
        <TreeList nodes={TREE} />
        <p className="filetree__footer">
          <span>project.config.yaml + claims_dir in, static viewer/ out</span>
          <span className="filetree__arrow" aria-hidden="true">
            →
          </span>
        </p>
      </div>
    </div>
  );
}
