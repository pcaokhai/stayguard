"use client";

import { useEffect, useState, type ReactNode } from "react";
import { worker } from "./browser";

// Holds the page back until the worker is ready, so no request escapes to a missing API.
export default function BrowserMocks({ children }: { children: ReactNode }) {
  const [ready, setReady] = useState(false);
  const [failure, setFailure] = useState("");
  useEffect(() => {
    worker
      .start({ onUnhandledFrame: "bypass" })
      .then(() => setReady(true))
      .catch((e: unknown) => setFailure(String(e)));
  }, []);
  if (failure) return <p role="alert">Mock worker failed to start: {failure}</p>;
  return ready ? children : null;
}
