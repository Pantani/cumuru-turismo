import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { Phase4Client } from "../shared/api/phase4-client";
import PublicFoundationPage from "./PublicFoundationPage";

const pending = new Promise<never>(() => undefined);
const client: Phase4Client = {
  getSummary: vi.fn(() => pending),
  getPresence: vi.fn(() => pending),
  getPreferences: vi.fn(() => pending),
  getMethodology: vi.fn(() => pending),
  getQuality: vi.fn(() => pending),
};

describe("página pública do observatório", () => {
  it("introduz o dashboard como protótipo não censitário", () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <PublicFoundationPage client={client} />
      </QueryClientProvider>,
    );

    expect(
      screen.getByRole("heading", {
        level: 1,
        name: "Observatório Turístico de Cumuruxatiba",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText(/somente dados fictícios/i)).toBeInTheDocument();
    expect(
      screen.getByText(/não substitui estatística oficial nem censo/i),
    ).toBeInTheDocument();
  });
});
