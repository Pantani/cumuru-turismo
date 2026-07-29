import createClient from "openapi-fetch";

import type { components, paths } from "../../generated/schema";

type HealthStatus = components["schemas"]["HealthStatus"];
type ReadinessStatus = components["schemas"]["ReadinessStatus"];
type Problem = components["schemas"]["Problem"];

type FetchImplementation = (request: Request) => Promise<Response>;

interface PlatformClientOptions {
  baseUrl?: string;
  fetch?: FetchImplementation;
}

export class PlatformApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "PlatformApiError";
    this.status = status;
  }
}

function safeProblemTitle(error: Problem | undefined) {
  return error?.title ?? "Não foi possível consultar o serviço.";
}

export function createPlatformClient({
  baseUrl = "/api/v1",
  fetch = (request) => globalThis.fetch(request),
}: PlatformClientOptions = {}) {
  const client = createClient<paths>({
    baseUrl,
    credentials: "same-origin",
    fetch,
  });

  return {
    async getHealth(signal?: AbortSignal): Promise<HealthStatus> {
      const options = signal ? { signal } : {};
      const { data, error, response } = await client.GET(
        "/platform/health",
        options,
      );

      if (data) {
        return data;
      }

      throw new PlatformApiError(response.status, safeProblemTitle(error));
    },

    async getReadiness(signal?: AbortSignal): Promise<ReadinessStatus> {
      const options = signal ? { signal } : {};
      const { data, error, response } = await client.GET(
        "/platform/readiness",
        options,
      );

      if (data) {
        return data;
      }

      throw new PlatformApiError(response.status, safeProblemTitle(error));
    },
  };
}

export function resolveApiBaseUrl(configuredUrl: string | undefined) {
  if (configuredUrl) {
    return configuredUrl;
  }

  return "/api/v1";
}

export const platformClient = createPlatformClient({
  baseUrl: resolveApiBaseUrl(import.meta.env.VITE_API_BASE_URL),
});
