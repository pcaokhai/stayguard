"use client";

import { useEffect, useState, type ReactNode } from "react";
import { worker } from "./browser";

// Holds the page back until the worker is ready, so no request escapes to a missing API.
// Strict mode runs effects twice in dev; start the worker once per page load.
let starting: Promise<unknown> | undefined;
const startOnce = () => (starting ??= worker.start({ onUnhandledFrame: "bypass" }));

export default function BrowserMocks({ children }: { children: ReactNode }) {
  const [ready, setReady] = useState(false);
  const [failure, setFailure] = useState("");
  useEffect(() => {
    startOnce()
      .then(() => setReady(true))
      .catch((e: unknown) => setFailure(String(e)));
  }, []);
  if (failure) return <p role="alert">Mock worker failed to start: {failure}</p>;
  return ready ? children : null;
}
