import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, expect, test } from "vitest";
import { handlers } from "../mocks/setup/handlers";
import { createApiClient } from "./api";

const server = setupServer(...handlers);
beforeAll(() => server.listen({ onUnhandledFrame: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

// SG-002 AC3: the typed client talks to the generated mock handlers, no API running.
test("MockClient_SG002_AC3: typed client reads a generated mock handler", async () => {
  const api = createApiClient("http://localhost");
  const { data, error, response } = await api.GET("/v1/services");

  expect(error).toBeUndefined();
  expect(response.status).toBe(200);
  expect(data).toBeDefined();
});
