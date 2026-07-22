import { contentSpec, type Section } from "../content";

/** Look up a section from the content spec by id, or throw if missing. */
export function getSection(id: string): Section {
  const section = contentSpec.sections.find((s) => s.id === id);
  if (!section) throw new Error(`Unknown section id: ${id}`);
  return section;
}
