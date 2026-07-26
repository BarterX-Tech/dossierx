export type Role = "human" | "agent" | "engine";

/**
 * A role tag — human / agent / engine.
 *
 * It lives in its own file because four different visuals now stamp who acted
 * (the review loop, the terminal/viewer pair, the actor legend, the thread
 * panel), and the whole point of the vocabulary is that the same word wears the
 * same colour everywhere. The hues match the rendered viewer's own author-role
 * pills — human warm, agent green — so a reader who has seen the real tool
 * recognises them here.
 */
export function RolePill({ role }: { role: Role }) {
  return <span className={`role-pill role-pill--${role}`}>{role}</span>;
}
