import {cleanup, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { renderWithSession } from "../test/session";
import AnalyticsQualityPage from "./AnalyticsQualityPage";

function renderPage(signedIn = false) {
  return renderWithSession(<AnalyticsQualityPage />, { signedIn });
}

describe("rota interna de qualidade", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("falha fechada e não chama a API sem sessão", () => {
    const fetcher = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetcher);

    renderPage();

    expect(
      screen.getByRole("heading", { name: "Acesso institucional necessário" }),
    ).toBeInTheDocument();
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("usa a sessão existente para consultar a rota protegida", async () => {
    const fetcher = vi.fn<typeof fetch>().mockImplementation(async (input) => {
      const request = input as Request;
      expect(request.headers.get("Authorization")).toBe("Bearer cms_test-session-token");
      // A página passou a ler duas rotas protegidas: qualidade e funil. A
      // asserção continua sendo sobre a sessão, e agora vale para as duas.
      if (new URL(request.url).pathname.endsWith("/funnel")) {
        return Response.json(
          {
            window: "last_30_days",
            as_of: "2026-07-28T12:00:00Z",
            invite: {
              issued: 0,
              submitted: 0,
              expired_unused: 0,
              revoked: 0,
            },
            survey: {
              issued: 0,
              completed: 0,
              expired_unanswered: 0,
              revoked: 0,
            },
            self_registration: {
              started: 0,
              pending: 0,
              approved: 0,
              rejected: 0,
              expired: 0,
            },
          },
          {
            headers: {
              "Cache-Control": "no-store",
              "X-Request-ID": "request-funnel-page-test",
            },
          },
        );
      }
      return Response.json(
        {
          window: "last_30_days",
          updated_at: "2026-07-28T12:00:00Z",
          incomplete_stays: { status: "available", value: 0 },
          overdue_planned_departures: { status: "available", value: 0 },
          silent_accommodations: { status: "available", value: 0 },
          aggregation_failures: { status: "available", value: 0 },
          suspected_duplicates: {
            status: "not_available",
            reason_code: "pseudonym_not_approved",
          },
          fnrh_failures: {
            status: "not_available",
            reason_code: "not_implemented",
          },
          coverage_by_category: [],
        },
        {
          headers: {
            "Cache-Control": "no-store",
            "X-Request-ID": "request-quality-page-test",
          },
        },
      );
    });
    vi.stubGlobal("fetch", fetcher);

    renderPage(true);

    expect(
      await screen.findByRole("heading", {
        name: "Resumo dos últimos 30 dias",
      }),
    ).toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(2);
    expect(document.body.textContent).not.toContain("cms_test-session-token");
  });
});
