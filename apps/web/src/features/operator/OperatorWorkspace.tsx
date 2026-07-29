import {
  type FormEvent,
  type RefObject,
  useCallback,
  useRef,
  useState,
} from "react";

import type { components } from "../../generated/schema";
import { InviteQr } from "../invite/InviteQr";
import {
  Phase2ApiError,
  strongEtagPattern,
} from "../../shared/api/phase2-client";
import { useAuthSession } from "../../shared/auth/AuthSession";
import { createUuidV7 } from "../../shared/identity/uuid-v7";
import { setSurveyCapability } from "../../shared/security/survey-capability";
import {
  type ValidationIssue,
  validateCreateStay,
  validateSubmitGroup,
} from "../../shared/validation/phase2-validation";
import {
  createVisitor,
  VisitorEditor,
  type VisitorEditorHandle,
} from "../visitors/VisitorEditor";

type MembershipRole = components["schemas"]["MembershipRole"];
type StayStatus = components["schemas"]["StayStatus"];
type SubmitGroupRequest = components["schemas"]["SubmitGroupRequest"];

function operationError(error: unknown) {
  if (error instanceof Phase2ApiError) {
    const retry =
      error.retryAfterSeconds === null
        ? ""
        : ` Tente novamente em ${error.retryAfterSeconds} segundos.`;
    return `${error.problem.title}${retry}`;
  }
  return "Não foi possível alcançar o serviço. Nenhum dado foi descartado.";
}

function useOperationStatus() {
  const [status, setStatus] = useState("");
  const [busy, setBusy] = useState(false);
  const run = useCallback(
    async <T,>(
      label: string,
      execute: () => Promise<T>,
      onSuccess?: (result: T) => void,
    ) => {
      setBusy(true);
      setStatus(`${label}: processando…`);
      try {
        const result = await execute();
        onSuccess?.(result);
        setStatus(`${label}: concluído.`);
        return true;
      } catch (error) {
        setStatus(operationError(error));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [],
  );
  return { busy, fail: setStatus, run, status };
}

function makeKeys() {
  return {
    membership: crypto.randomUUID(),
    stay: crypto.randomUUID(),
    group: crypto.randomUUID(),
    invite: crypto.randomUUID(),
    checkIn: crypto.randomUUID(),
    checkOut: crypto.randomUUID(),
    cancel: crypto.randomUUID(),
    noShow: crypto.randomUUID(),
  };
}

function isUnavailable(busy: boolean, ...requiredValues: string[]) {
  return busy || requiredValues.some((value) => value.length === 0);
}

type IdempotencyKeys = ReturnType<typeof makeKeys>;

function rotateKey(keys: IdempotencyKeys, name: keyof IdempotencyKeys) {
  keys[name] = crypto.randomUUID();
}

function validStrongEtag(value: string) {
  return strongEtagPattern.test(value);
}

function captureEtag(
  etag: string | null,
  setEtag: (value: string) => void,
) {
  if (etag !== null) {
    setEtag(etag);
  }
}

function focusIssue(
  field: string,
  targets: Record<string, RefObject<HTMLInputElement | null>>,
) {
  targets[field]?.current?.focus();
}

function AccommodationOperations() {
  const { client } = useAuthSession();
  const operation = useOperationStatus();
  const keys = useRef(makeKeys());
  const [accommodationId, setAccommodationId] = useState("");
  const [accommodationName, setAccommodationName] = useState("");
  const [accommodationEtag, setAccommodationEtag] = useState("");
  const [membershipId, setMembershipId] = useState("");
  const [membershipEtag, setMembershipEtag] = useState("");
  const [issuer, setIssuer] = useState("");
  const [subject, setSubject] = useState("");
  const [role, setRole] = useState<MembershipRole>("operator");
  const [projection, setProjection] = useState(
    "Nenhuma acomodação ou vínculo consultado.",
  );

  function updateAccommodation(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void operation.run(
      "Atualizar acomodação",
      () =>
        client.updateAccommodation(
          accommodationId,
          { name: accommodationName },
          accommodationEtag,
        ),
      (result) => {
        captureEtag(result.etag, setAccommodationEtag);
        setProjection(
          `Acomodação ${result.data.id}: ${result.data.status}, versão ${result.data.version}.`,
        );
      },
    );
  }

  function createMembership(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void operation.run(
      "Adicionar vínculo",
      () =>
        client.createAccommodationMembership(
          accommodationId,
          { oidc_issuer: issuer, oidc_subject: subject, role },
          keys.current.membership,
      ),
      (result) => {
        captureEtag(result.etag, setMembershipEtag);
        setProjection(
          `Vínculo ${result.data.id}: papel ${result.data.role}, versão ${result.data.version}.`,
        );
        rotateKey(keys.current, "membership");
      },
    );
  }

  return (
    <section className="operation-card" aria-labelledby="accommodation-title">
      <h3 id="accommodation-title">Acomodações e vínculos</h3>
      <div className="field-grid">
        <label>
          ID da acomodação
          <input
            value={accommodationId}
            required
            onChange={(event) => setAccommodationId(event.target.value)}
          />
        </label>
        <label>
          ETag da acomodação
          <input
            value={accommodationEtag}
            required
            aria-invalid={!validStrongEtag(accommodationEtag)}
            pattern={'^"[1-9][0-9]*"$'}
            onChange={(event) => setAccommodationEtag(event.target.value)}
          />
        </label>
        <label>
          ETag do vínculo
          <input
            value={membershipEtag}
            required
            aria-invalid={!validStrongEtag(membershipEtag)}
            pattern={'^"[1-9][0-9]*"$'}
            onChange={(event) => setMembershipEtag(event.target.value)}
          />
        </label>
      </div>
      <div className="button-row">
        <button
          type="button"
          disabled={operation.busy}
          onClick={() =>
            void operation.run(
              "Listar acomodações",
              () => client.listAccommodations(),
              (result) =>
                setProjection(
                  `${result.data.items.length} acomodação(ões) disponível(is).`,
                ),
            )
          }
        >
          Listar acomodações
        </button>
        <button
          type="button"
          disabled={operation.busy || accommodationId.length === 0}
          onClick={() =>
            void operation.run(
              "Consultar acomodação",
              () => client.getAccommodation(accommodationId),
              (result) => {
                captureEtag(result.etag, setAccommodationEtag);
                setProjection(
                  `Acomodação ${result.data.id}: ${result.data.status}, versão ${result.data.version}.`,
                );
              },
            )
          }
        >
          Consultar acomodação
        </button>
        <button
          type="button"
          disabled={operation.busy || accommodationId.length === 0}
          onClick={() =>
            void operation.run(
              "Listar vínculos",
              () => client.listAccommodationMemberships(accommodationId),
              (result) =>
                setProjection(
                  `${result.data.items.length} vínculo(s) institucional(is).`,
                ),
            )
          }
        >
          Listar vínculos
        </button>
      </div>
      <form onSubmit={updateAccommodation}>
        <label>
          Nome operacional da acomodação
          <input
            value={accommodationName}
            minLength={1}
            maxLength={200}
            required
            onChange={(event) => setAccommodationName(event.target.value)}
          />
        </label>
        <button
          type="submit"
          disabled={
            operation.busy ||
            accommodationId.length === 0 ||
            !validStrongEtag(accommodationEtag)
          }
        >
          Atualizar acomodação
        </button>
      </form>
      <form onSubmit={createMembership}>
        <h4>Novo vínculo OIDC institucional</h4>
        <div className="field-grid">
          <label>
            Emissor OIDC
            <input
              type="url"
              value={issuer}
              maxLength={2048}
              required
              onChange={(event) => setIssuer(event.target.value)}
            />
          </label>
          <label>
            Subject institucional
            <input
              value={subject}
              maxLength={255}
              required
              onChange={(event) => setSubject(event.target.value)}
            />
          </label>
          <label>
            Papel
            <select
              value={role}
              onChange={(event) =>
                setRole(event.target.value as MembershipRole)
              }
            >
              <option value="operator">Operador</option>
              <option value="manager">Gestor</option>
            </select>
          </label>
        </div>
        <button
          type="submit"
          disabled={operation.busy || accommodationId.length === 0}
        >
          Adicionar vínculo
        </button>
      </form>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          void operation.run(
            "Atualizar vínculo",
            () =>
              client.updateAccommodationMembership(
                accommodationId,
                membershipId,
                { role },
                membershipEtag,
              ),
            (result) => {
              captureEtag(result.etag, setMembershipEtag);
              setProjection(
                `Vínculo ${result.data.id}: papel ${result.data.role}, versão ${result.data.version}.`,
              );
            },
          );
        }}
      >
        <label>
          ID do vínculo
          <input
            value={membershipId}
            required
            onChange={(event) => setMembershipId(event.target.value)}
          />
        </label>
        <button
          type="submit"
          disabled={
            operation.busy ||
            accommodationId.length === 0 ||
            membershipId.length === 0 ||
            !validStrongEtag(membershipEtag)
          }
        >
          Atualizar vínculo
        </button>
      </form>
      <p role="status" aria-live="polite">
        {operation.status}
      </p>
      <output className="result-projection" aria-live="polite">
        {projection}
      </output>
    </section>
  );
}

interface StayIdentityFieldsProps {
  accommodationId: string;
  accommodationInputRef: RefObject<HTMLInputElement | null>;
  etag: string;
  etagInvalid: boolean;
  invalidFields: ReadonlySet<string>;
  onAccommodationId: (value: string) => void;
  onEtag: (value: string) => void;
  onStayId: (value: string) => void;
  stayId: string;
}

function StayIdentityFields(props: StayIdentityFieldsProps) {
  return (
    <div className="field-grid">
      <label>
        ID da acomodação
        <input
          ref={props.accommodationInputRef}
          value={props.accommodationId}
          aria-invalid={props.invalidFields.has("accommodation_id")}
          onChange={(event) => props.onAccommodationId(event.target.value)}
        />
      </label>
      <label>
        ID da estadia
        <input
          value={props.stayId}
          onChange={(event) => props.onStayId(event.target.value)}
        />
      </label>
      <label>
        ETag da estadia
        <input
          value={props.etag}
          required
          pattern={'^"[1-9][0-9]*"$'}
          aria-invalid={props.etagInvalid}
          onChange={(event) => props.onEtag(event.target.value)}
        />
      </label>
    </div>
  );
}

function StayOperations() {
  const { client } = useAuthSession();
  const operation = useOperationStatus();
  const keys = useRef(makeKeys());
  const [accommodationId, setAccommodationId] = useState("");
  const [stayId, setStayId] = useState("");
  const [etag, setEtag] = useState("");
  const [arrival, setArrival] = useState("");
  const [departure, setDeparture] = useState("");
  const [guestCount, setGuestCount] = useState(1);
  const [status, setStatus] = useState<StayStatus>("draft");
  const [projection, setProjection] = useState(
    "Nenhuma estadia consultada.",
  );
  const [issues, setIssues] = useState<ValidationIssue[]>([]);
  const submissionId = useRef(createUuidV7());
  const accommodationInputRef = useRef<HTMLInputElement>(null);
  const arrivalInputRef = useRef<HTMLInputElement>(null);
  const departureInputRef = useRef<HTMLInputElement>(null);
  const guestCountInputRef = useRef<HTMLInputElement>(null);

  function createStay(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const body = {
      accommodation_id: accommodationId,
      planned_arrival_on: arrival,
      planned_departure_on: departure,
      expected_guest_count: guestCount,
      client_submission_id: submissionId.current,
    };
    const validationIssues = validateCreateStay(body);
    setIssues(validationIssues);
    if (validationIssues.length > 0) {
      operation.fail(
        validationIssues[0]?.message ?? "Revise os dados da estadia.",
      );
      focusIssue(validationIssues[0]?.field ?? "", {
        accommodation_id: accommodationInputRef,
        planned_arrival_on: arrivalInputRef,
        planned_departure_on: departureInputRef,
        expected_guest_count: guestCountInputRef,
      });
      return;
    }
    void operation.run(
      "Criar estadia",
      () => client.createStay(body, keys.current.stay),
      (result) => {
        captureEtag(result.etag, setEtag);
        setProjection(
          `Estadia ${result.data.id}: ${result.data.status}, versão ${result.data.version}.`,
        );
        rotateKey(keys.current, "stay");
        submissionId.current = createUuidV7();
      },
    );
  }

  const invalidFields = new Set(issues.map((issue) => issue.field));

  return (
    <section className="operation-card" aria-labelledby="stay-title">
      <h3 id="stay-title">Estadias</h3>
      <StayIdentityFields
        accommodationId={accommodationId}
        accommodationInputRef={accommodationInputRef}
        etag={etag}
        etagInvalid={!validStrongEtag(etag)}
        invalidFields={invalidFields}
        stayId={stayId}
        onAccommodationId={setAccommodationId}
        onEtag={setEtag}
        onStayId={setStayId}
      />
      <div className="field-grid">
        <label>
          Situação para filtro
          <select
            value={status}
            onChange={(event) => setStatus(event.target.value as StayStatus)}
          >
            <option value="draft">Rascunho</option>
            <option value="invited">Convidada</option>
            <option value="pre_registered">Pré-registrada</option>
            <option value="checked_in">Check-in</option>
            <option value="checked_out">Check-out</option>
            <option value="cancelled">Cancelada</option>
            <option value="no_show">Não compareceu</option>
          </select>
        </label>
        <label>
          Chegada prevista
          <input
            ref={arrivalInputRef}
            type="date"
            value={arrival}
            required
            aria-invalid={invalidFields.has("planned_arrival_on")}
            onChange={(event) => setArrival(event.target.value)}
          />
        </label>
        <label>
          Saída prevista
          <input
            ref={departureInputRef}
            type="date"
            value={departure}
            required
            aria-invalid={invalidFields.has("planned_departure_on")}
            onChange={(event) => setDeparture(event.target.value)}
          />
        </label>
        <label>
          Quantidade esperada
          <input
            ref={guestCountInputRef}
            type="number"
            value={guestCount}
            min={1}
            max={100}
            required
            aria-invalid={invalidFields.has("expected_guest_count")}
            onChange={(event) => setGuestCount(event.target.valueAsNumber)}
          />
        </label>
      </div>
      <form noValidate onSubmit={createStay}>
        <button type="submit" disabled={operation.busy}>
          Criar estadia
        </button>
      </form>
      <div className="button-row">
        <button
          type="button"
          disabled={operation.busy}
          onClick={() =>
            void operation.run(
              "Listar estadias",
              () => client.listStays(accommodationId || undefined, status),
              (result) =>
                setProjection(
                  `${result.data.items.length} estadia(s) no filtro ${status}.`,
                ),
            )
          }
        >
          Listar estadias
        </button>
        <button
          type="button"
          disabled={isUnavailable(operation.busy, stayId)}
          onClick={() =>
            void operation.run(
              "Consultar estadia",
              () => client.getStay(stayId),
              (result) => {
                captureEtag(result.etag, setEtag);
                setProjection(
                  `Estadia ${result.data.id}: ${result.data.status}, ${result.data.visitor_count} visitante(s), versão ${result.data.version}.`,
                );
              },
            )
          }
        >
          Consultar estadia
        </button>
        <button
          type="button"
          aria-disabled={
            isUnavailable(operation.busy, stayId) ||
            !validStrongEtag(etag)
          }
          onClick={() =>
            void operation.run(
              "Atualizar estadia",
              () =>
                client.updateStay(
                  stayId,
                  {
                    planned_arrival_on: arrival,
                    planned_departure_on: departure,
                    expected_guest_count: guestCount,
                  },
                  etag,
                ),
              (result) => {
                captureEtag(result.etag, setEtag);
                setProjection(
                  `Estadia ${result.data.id}: ${result.data.status}, versão ${result.data.version}.`,
                );
              },
            )
          }
          disabled={
            isUnavailable(operation.busy, stayId) || !validStrongEtag(etag)
          }
        >
          Atualizar estadia
        </button>
      </div>
      <p role="status" aria-live="polite">
        {operation.status}
      </p>
      <output className="result-projection" aria-live="polite">
        {projection}
      </output>
    </section>
  );
}

function GroupAndLifecycleOperations() {
  const { client } = useAuthSession();
  const operation = useOperationStatus();
  const keys = useRef(makeKeys());
  const [stayId, setStayId] = useState("");
  const [etag, setEtag] = useState("");
  const [privacyVersion, setPrivacyVersion] = useState("");
  const [visitors, setVisitors] = useState(() => [
    createVisitor("responsible"),
  ]);
  const [issues, setIssues] = useState<ValidationIssue[]>([]);
  const [showInvite, setShowInvite] = useState(false);
  const [projection, setProjection] = useState(
    "Nenhum grupo ou evento consultado.",
  );
  const inviteUrlRef = useRef<string | null>(null);
  const visitorEditorRef = useRef<VisitorEditorHandle>(null);
  const submissionId = useRef(createUuidV7());

  function groupPayload(): SubmitGroupRequest {
    return {
      client_submission_id: submissionId.current,
      privacy_notice_version: privacyVersion,
      visitors,
    };
  }

  async function createInvite() {
    const result = await client.createStayInvite(
      stayId,
      { privacy_notice_version: privacyVersion },
      etag,
      keys.current.invite,
    );
    inviteUrlRef.current = result.data.url;
    setShowInvite(true);
    captureEtag(result.etag, setEtag);
    setProjection(
      `Convite ${result.data.invite_id} válido até ${result.data.expires_at}.`,
    );
    rotateKey(keys.current, "invite");
  }

  function submitAssistedGroup() {
    const body = groupPayload();
    const validationIssues = validateSubmitGroup(body);
    setIssues(validationIssues);
    if (validationIssues.length > 0) {
      const first = validationIssues[0];
      operation.fail(first?.message ?? "Revise os dados do grupo.");
      visitorEditorRef.current?.focus(
        first?.field === "visitors" ? "visitors.0.role" : (first?.field ?? ""),
      );
      return;
    }
    void operation.run(
      "Enviar grupo assistido",
      () =>
        client.submitAssistedStayGroup(
          stayId,
          body,
          etag,
          keys.current.group,
        ),
      (result) => {
        captureEtag(result.etag, setEtag);
        setSurveyCapability(result.surveyCapability);
        setProjection(
          `Submissão ${result.data.submission_id}: ${result.data.status}. Pesquisa voluntária liberada nesta sessão.`,
        );
        rotateKey(keys.current, "group");
        submissionId.current = createUuidV7();
      },
    );
  }

  function projectLifecycle(
    result: Awaited<ReturnType<typeof client.checkInStay>>,
    key: keyof IdempotencyKeys,
  ) {
    captureEtag(result.etag, setEtag);
    setProjection(
      `Estadia ${result.data.id}: ${result.data.status}, versão ${result.data.version}.`,
    );
    rotateKey(keys.current, key);
  }

  function discardInvite() {
    inviteUrlRef.current = null;
    setShowInvite(false);
  }

  return (
    <section className="operation-card" aria-labelledby="lifecycle-title">
      <h3 id="lifecycle-title">Grupo, convite e ciclo da estadia</h3>
      <div className="field-grid">
        <label>
          ID da estadia
          <input
            value={stayId}
            required
            onChange={(event) => setStayId(event.target.value)}
          />
        </label>
        <label>
          ETag da estadia
          <input
            value={etag}
            required
            pattern={'^"[1-9][0-9]*"$'}
            aria-invalid={!validStrongEtag(etag)}
            onChange={(event) => setEtag(event.target.value)}
          />
        </label>
        <label>
          Versão do aviso de privacidade
          <input
            value={privacyVersion}
            required
            maxLength={100}
            onChange={(event) => setPrivacyVersion(event.target.value)}
          />
        </label>
      </div>
      <p className="privacy-warning" role="note">
        O grupo assistido usa apenas faixa etária e residência generalizada;
        não há campo livre para dados pessoais.
      </p>
      <VisitorEditor
        ref={visitorEditorRef}
        disabled={operation.busy}
        issues={issues}
        visitors={visitors}
        onChange={(nextVisitors) => {
          setVisitors(nextVisitors);
          setIssues([]);
        }}
      />
      <div className="button-row">
        <button
          type="button"
          disabled={isUnavailable(operation.busy, stayId)}
          onClick={() =>
            void operation.run(
              "Consultar grupo",
              () => client.getStayGroup(stayId),
              (result) => {
                captureEtag(result.etag, setEtag);
                setProjection(
                  `Grupo da estadia ${result.data.stay_id}: ${result.data.visitors.length} visitante(s).`,
                );
              },
            )
          }
        >
          Consultar grupo
        </button>
        <button
          type="button"
          disabled={
            isUnavailable(operation.busy, stayId, privacyVersion) ||
            !validStrongEtag(etag)
          }
          onClick={submitAssistedGroup}
        >
          Enviar grupo assistido
        </button>
        <button
          type="button"
          disabled={
            isUnavailable(operation.busy, stayId, privacyVersion) ||
            !validStrongEtag(etag)
          }
          onClick={() =>
            void operation.run("Criar convite", createInvite)
          }
        >
          Criar QR de convite
        </button>
      </div>
      <div className="button-row">
        <button
          type="button"
          disabled={
            isUnavailable(operation.busy, stayId) || !validStrongEtag(etag)
          }
          onClick={() =>
            void operation.run(
              "Registrar check-in",
              () =>
                client.checkInStay(stayId, {}, etag, keys.current.checkIn),
              (result) => projectLifecycle(result, "checkIn"),
            )
          }
        >
          Check-in
        </button>
        <button
          type="button"
          disabled={
            isUnavailable(operation.busy, stayId) || !validStrongEtag(etag)
          }
          onClick={() =>
            void operation.run(
              "Registrar check-out",
              () =>
                client.checkOutStay(stayId, {}, etag, keys.current.checkOut),
              (result) => projectLifecycle(result, "checkOut"),
            )
          }
        >
          Check-out
        </button>
        <button
          type="button"
          disabled={
            isUnavailable(operation.busy, stayId) || !validStrongEtag(etag)
          }
          onClick={() =>
            void operation.run(
              "Cancelar estadia",
              () =>
                client.cancelStay(
                  stayId,
                  { reason_code: "guest_request", correction: false },
                  etag,
                  keys.current.cancel,
                ),
              (result) => projectLifecycle(result, "cancel"),
            )
          }
        >
          Cancelar
        </button>
        <button
          type="button"
          disabled={
            isUnavailable(operation.busy, stayId) || !validStrongEtag(etag)
          }
          onClick={() =>
            void operation.run(
              "Registrar não comparecimento",
              () =>
                client.markStayNoShow(
                  stayId,
                  { reason_code: "guest_absent" },
                  etag,
                  keys.current.noShow,
                ),
              (result) => projectLifecycle(result, "noShow"),
            )
          }
        >
          Não compareceu
        </button>
      </div>
      {showInvite && inviteUrlRef.current !== null ? (
        <InviteQr url={inviteUrlRef.current} onDiscard={discardInvite} />
      ) : null}
      <output className="result-projection" aria-live="polite">
        {projection}
      </output>
      <p role="status" aria-live="polite">
        {operation.status}
      </p>
    </section>
  );
}

export function OperatorWorkspace() {
  const { endSession } = useAuthSession();

  return (
    <section aria-labelledby="operator-title">
      <div className="workspace-heading">
        <div>
          <h2 id="operator-title">Operação de estadias</h2>
          <p>
            Respostas autenticadas permanecem somente na memória da tela e não
            são persistidas em cache ou armazenamento local.
          </p>
        </div>
        <button type="button" onClick={() => void endSession()}>
          Encerrar sessão e apagar rascunhos
        </button>
      </div>
      <div className="operation-grid">
        <AccommodationOperations />
        <StayOperations />
        <GroupAndLifecycleOperations />
      </div>
    </section>
  );
}
