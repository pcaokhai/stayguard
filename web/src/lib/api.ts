import createClient from "openapi-fetch";
import type { paths } from "../api/generated/schema";

// The only door to /v1 (docs/10 §40): a typed client over the generated schema.
export const createApiClient = (baseUrl: string) => createClient<paths>({ baseUrl });

// Same origin: the API container serves the static export and /v1 (docs/02 §8).
export const api = createApiClient("");
