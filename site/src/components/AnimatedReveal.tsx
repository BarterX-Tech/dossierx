import { motion, useReducedMotion } from "framer-motion";
import type { ReactNode } from "react";

interface AnimatedRevealProps {
  children: ReactNode;
  /** Stagger delay in seconds. */
  delay?: number;
  /** Slide-in distance in px (default 24). */
  y?: number;
  className?: string;
}

/**
 * Reveals its children on scroll-into-view via framer-motion's whileInView.
 * Animation runs once; respects reduced-motion by keeping the distance small.
 */
export function AnimatedReveal({
  children,
  delay = 0,
  y = 24,
  className,
}: AnimatedRevealProps) {
  const reduce = useReducedMotion();

  return (
    <motion.div
      className={className}
      initial={reduce ? { opacity: 1, y: 0 } : { opacity: 0, y }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: "0px 0px -80px 0px" }}
      transition={
        reduce
          ? { duration: 0 }
          : { duration: 0.5, delay, ease: [0.22, 1, 0.36, 1] }
      }
    >
      {children}
    </motion.div>
  );
}
