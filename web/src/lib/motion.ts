import type { Transition } from "motion/react";

// Motion tokens (docs/16 §4). Animate only transform and opacity.
export const duration = { fast: 0.12, base: 0.2, slow: 0.32 } as const;
export const ease = { standard: [0.2, 0, 0, 1] } as const;
export const spring = {
  snappy: { type: "spring", stiffness: 500, damping: 32 },
} as const satisfies Record<string, Transition>;
export const STAGGER = 0.03;
export const STAGGER_MAX = 10;
