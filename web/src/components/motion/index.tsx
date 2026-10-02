"use client";

import { Children, type ComponentProps, type ReactNode } from "react";
import { motion } from "motion/react";
import NumberFlow from "@number-flow/react";
import { duration, ease, spring, STAGGER, STAGGER_MAX } from "@/lib/motion";

const enter = { duration: duration.slow, ease: ease.standard };

// Opacity plus an 8 px rise; with reduced motion MotionConfig drops the transform and keeps the fade.
export function FadeIn({
  delay = 0,
  ...props
}: ComponentProps<typeof motion.div> & { delay?: number }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ ...enter, delay }}
      {...props}
    />
  );
}

// Children fade in one after another, at most the first 10.
export function StaggerList({ children, ...props }: ComponentProps<typeof motion.div>) {
  return (
    <motion.div {...props}>
      {Children.toArray(children as ReactNode).map((child, i) => (
        <FadeIn key={i} delay={Math.min(i, STAGGER_MAX) * STAGGER}>
          {child}
        </FadeIn>
      ))}
    </motion.div>
  );
}

// Shared layoutId pill that slides behind the active tab, chip or segment.
export function SlidingPill({ id, className }: { id: string; className?: string }) {
  return (
    <motion.span
      layoutId={id}
      transition={spring.snappy}
      className={className}
      aria-hidden="true"
    />
  );
}

export function PressScale(props: ComponentProps<typeof motion.div>) {
  return (
    <motion.div whileTap={{ scale: 0.97 }} transition={{ duration: duration.fast }} {...props} />
  );
}

// Counts and dashboard totals only; never amounts on pay, bill, receipt or checkout screens.
export function RollingNumber({ value }: { value: number }) {
  return <NumberFlow value={value} locales="vi-VN" />;
}

// SVG check drawn from path length 0 to 1 in 0.4 s.
export function SuccessTick({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.4"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      aria-hidden="true"
    >
      <motion.path
        d="M5 12.5l4.5 4.5L19 7.5"
        initial={{ pathLength: 0 }}
        animate={{ pathLength: 1 }}
        transition={{ duration: 0.4, ease: ease.standard }}
      />
    </svg>
  );
}

// One-time scale 1 -> 1.04 -> 1; change `trigger` to run it again.
export function Pulse({
  trigger,
  ...props
}: ComponentProps<typeof motion.div> & { trigger?: unknown }) {
  return (
    <motion.div
      key={String(trigger)}
      initial={{ scale: 1 }}
      animate={{ scale: [1, 1.04, 1] }}
      transition={{ duration: duration.slow }}
      {...props}
    />
  );
}

// Shakes twice (x: 0 -> -6 -> 6 -> 0, 0.2 s) each time `trigger` changes to a new truthy value.
export function Shake({
  trigger,
  ...props
}: ComponentProps<typeof motion.div> & { trigger: number }) {
  return (
    <motion.div
      key={trigger}
      animate={trigger ? { x: [0, -6, 6, -6, 6, 0] } : undefined}
      transition={{ duration: 0.2 }}
      {...props}
    />
  );
}
