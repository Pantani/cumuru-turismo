import { useCallback, useEffect, useState } from "react";

import type { components } from "../../generated/schema";
import { useAuthSession } from "../../shared/auth/AuthSession";
import { OperationStatus } from "../../shared/forms/FieldFeedback";
import { InviteQr, type QrPresentation } from "../invite/InviteQr";
import { PRIVACY_NOTICE_VERSION } from "../operator/stay-commands";
import { entityTagFor } from "../operator/stay-lifecycle";
import { useOperation } from "../operator/use-operation";
import { isNotFound, versionFromEtag } from "./accommodation-etag";

type AccommodationInviteStatus =
  components["schemas"]["AccommodationInviteStatus"];

interface PosterInvitePanelProps {
  accommodationId: string;
  onVersionChange: (version: number) => void;
  version: number;
}

const posterPresentation: QrPresentation = {
  title: "Cartaz pronto para impressão",
  label: "Código QR do cartaz de autocadastro",
  discardLabel: "Ocultar cartaz",
  description: (
    <p>
      Imprima este código. O endereço não é exibido nem copiado, e o token viaja
      no fragmento, que nunca chega ao servidor.
    </p>
  ),
};

function usageSummary(status: AccommodationInviteStatus) {
  if (status.max_uses === null) {
    return `${status.use_count} cadastros recebidos, sem limite de uso`;
  }
  return `${status.use_count} de ${status.max_uses} usos`;
}

function PosterStatus({ status }: { status: AccommodationInviteStatus | null }) {
  if (status === null) {
    return (
      <p className="empty-note">
        Nenhum cartaz ativo. Emita um para receber autocadastros.
      </p>
    );
  }
  return (
    <dl className="context-summary">
      <div>
        <dt>Uso</dt>
        <dd>{usageSummary(status)}</dd>
      </div>
      <div>
        <dt>Validade</dt>
        <dd>{status.expires_at.slice(0, 10)}</dd>
      </div>
    </dl>
  );
}

function usePosterStatus(accommodationId: string) {
  const { selfServiceClient } = useAuthSession();
  const [status, setStatus] = useState<AccommodationInviteStatus | null>(null);

  const reload = useCallback(async () => {
    try {
      const result =
        await selfServiceClient.getAccommodationInvite(accommodationId);
      setStatus(result.data);
    } catch (error) {
      if (!isNotFound(error)) {
        throw error;
      }
      setStatus(null);
    }
  }, [accommodationId, selfServiceClient]);

  useEffect(() => {
    void reload().catch(() => setStatus(null));
  }, [reload]);

  return { reload, setStatus, status };
}

export function PosterInvitePanel({
  accommodationId,
  onVersionChange,
  version,
}: PosterInvitePanelProps) {
  const { selfServiceClient } = useAuthSession();
  const operation = useOperation();
  const { run } = operation;
  const { reload, setStatus, status } = usePosterStatus(accommodationId);
  const [posterUrl, setPosterUrl] = useState<string | null>(null);

  const issue = useCallback(async () => {
    const result = await run("Emitindo o cartaz", () =>
      selfServiceClient.createAccommodationInvite(
        accommodationId,
        { privacy_notice_version: PRIVACY_NOTICE_VERSION, max_uses: null },
        entityTagFor(version),
        crypto.randomUUID(),
      ),
    );
    if (result !== null) {
      applyVersion(result.etag, onVersionChange);
      setPosterUrl(result.data.url);
      await reload();
    }
  }, [accommodationId, onVersionChange, reload, run, selfServiceClient, version]);

  const revoke = useCallback(async () => {
    const result = await run("Revogando o cartaz", () =>
      selfServiceClient.revokeAccommodationInvite(
        accommodationId,
        entityTagFor(version),
        crypto.randomUUID(),
      ),
    );
    if (result !== null) {
      applyVersion(result.etag, onVersionChange);
      setPosterUrl(null);
      setStatus(result.data);
    }
  }, [accommodationId, onVersionChange, run, selfServiceClient, setStatus, version]);

  return (
    <section className="poster-panel" aria-labelledby="poster-panel-title">
      <h2 id="poster-panel-title">Cartaz de autocadastro</h2>
      <p className="queue-hint">
        Um único código QR, exposto na parede, que qualquer hóspede pode ler
        para iniciar o próprio cadastro. Cada cadastro depende de aprovação sua.
      </p>

      <PosterStatus status={status} />
      <OperationStatus operation={operation} />

      <div className="button-row">
        <button
          type="button"
          className="primary-action"
          disabled={operation.busy}
          onClick={() => void issue()}
        >
          {status === null ? "Emitir cartaz" : "Trocar o cartaz"}
        </button>
        {status === null ? null : (
          <button
            type="button"
            className="quiet-action"
            disabled={operation.busy}
            onClick={() => void revoke()}
          >
            Revogar cartaz
          </button>
        )}
      </div>
      <p className="disclaimer">
        Trocar o cartaz invalida imediatamente o anterior: o papel que já estiver
        na parede deixa de funcionar e precisa ser reimpresso.
      </p>

      {posterUrl === null ? null : (
        <InviteQr
          url={posterUrl}
          presentation={posterPresentation}
          onDiscard={() => setPosterUrl(null)}
        />
      )}
    </section>
  );
}

function applyVersion(
  etag: string | null,
  onVersionChange: (version: number) => void,
) {
  const next = versionFromEtag(etag);
  if (next !== null) {
    onVersionChange(next);
  }
}
