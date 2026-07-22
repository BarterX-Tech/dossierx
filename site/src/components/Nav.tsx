import { useEffect, useState } from "react";
import { motion, useReducedMotion, useScroll, useSpring } from "framer-motion";
import clsx from "clsx";
import type { NavItem } from "../content";
import { BrandMark } from "./BrandMark";

interface NavProps {
  siteTitle: string;
  items: NavItem[];
}

/** Height of the sticky header, kept in sync with --nav-height in index.css. */
const NAV_OFFSET = 64;

/**
 * Scrolls to a section by id, compensating for the sticky nav height, then
 * syncs the URL hash without pushing a new history entry.
 */
function scrollToSection(id: string) {
  const el = document.getElementById(id);
  if (!el) return;
  const target = Math.max(
    0,
    el.getBoundingClientRect().top + window.scrollY - NAV_OFFSET,
  );
  history.replaceState(null, "", `#${id}`);
  window.scrollTo({ top: target, behavior: "instant" });
}

/**
 * Sticky anchor-based navigation for the single scrolling page. Tracks the
 * active section with an IntersectionObserver; no router involved.
 */
export function Nav({ siteTitle, items }: NavProps) {
  const [active, setActive] = useState<string>(items[0]?.id ?? "");
  const [open, setOpen] = useState(false);
  const reduce = useReducedMotion();

  // Page scroll progress (0 → 1), driving the accent bar under the nav. When
  // reduce-motion is on we bind the raw value; otherwise a light spring smooths
  // the fill so it eases rather than snaps.
  const { scrollYProgress } = useScroll();
  const smooth = useSpring(scrollYProgress, {
    stiffness: 120,
    damping: 30,
    mass: 0.3,
  });
  const progress = reduce ? scrollYProgress : smooth;

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((e) => e.isIntersecting)
          .sort((a, b) => b.intersectionRatio - a.intersectionRatio)[0];
        if (visible) setActive(visible.target.id);
      },
      { rootMargin: "-40% 0px -55% 0px", threshold: [0, 0.25, 0.5, 1] },
    );
    for (const item of items) {
      const el = document.getElementById(item.id);
      if (el) observer.observe(el);
    }
    return () => observer.disconnect();
  }, [items]);

  return (
    <header className="nav">
      <div className="nav__inner">
        <a
          href="#hero"
          className="nav__brand"
          onClick={(e) => {
            e.preventDefault();
            setOpen(false);
            scrollToSection("hero");
          }}
        >
          <BrandMark className="nav__brand-mark" />
          {siteTitle}
        </a>

        <button
          type="button"
          className="nav__toggle"
          aria-expanded={open}
          aria-controls="nav-links"
          aria-label="Toggle navigation"
          onClick={() => setOpen((v) => !v)}
        >
          <span />
          <span />
          <span />
        </button>

        <nav
          id="nav-links"
          className={clsx("nav__links", open && "nav__links--open")}
          aria-label="Section navigation"
        >
          {items.map((item, index) => (
            <a
              key={item.id}
              href={`#${item.id}`}
              className={clsx(
                "nav__link",
                active === item.id && "nav__link--active",
              )}
              aria-current={active === item.id ? "true" : undefined}
              onClick={(e) => {
                e.preventDefault();
                setOpen(false);
                scrollToSection(item.id);
              }}
            >
              <span className="nav__index">
                {String(index + 1).padStart(2, "0")}
              </span>
              {item.label}
            </a>
          ))}
        </nav>
      </div>
      <motion.div
        className="nav__progress"
        style={{ scaleX: progress }}
        aria-hidden="true"
      />
    </header>
  );
}
