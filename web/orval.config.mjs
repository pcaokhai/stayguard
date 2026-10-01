// MSW handlers from the contract (SG-002). The typed client comes from openapi-typescript;
// orval is used only for mocks, and its own fetch client output stays inside the generated mocks directory.
const config = {
  stayguard: {
    input: { target: "../contracts/openapi.yaml" },
    output: {
      mode: "single",
      target: "src/mocks/generated/api.ts",
      client: "fetch",
      mock: { generators: [{ type: "msw", useExamples: true }] },
    },
  },
};

export default config;
