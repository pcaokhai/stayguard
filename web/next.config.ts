import type { NextConfig } from "next";

// Turbopack resolves every import it sees, even in dead branches, so the mock layer is chosen
// by alias at build time: NEXT_PUBLIC_MOCK=1 bundles it, the default build ships none of it.
const mockLayer =
  process.env.NEXT_PUBLIC_MOCK === "1"
    ? "./src/mocks/setup/mock-on.tsx"
    : "./src/mocks/setup/mock-off.tsx";

const config: NextConfig = {
  output: "export",
  turbopack: { resolveAlias: { "mock-layer": mockLayer } },
};

export default config;
