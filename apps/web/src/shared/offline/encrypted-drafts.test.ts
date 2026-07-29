import { afterEach, describe, expect, it } from "vitest";

import {
  clearAllDrafts,
  deleteDraft,
  inspectDraftRecord,
  inspectDraftPresence,
  loadDraft,
  purgeExpiredDrafts,
  saveDraft,
} from "./encrypted-drafts";

const payload = {
  client_submission_id: "018f4e59-7a2a-7b12-8fd7-5d2e8dc99b80",
  privacy_notice_version: "2026-07",
  visitors: [],
};

function openDraftDatabase() {
  return new Promise<IDBDatabase>((resolve, reject) => {
    const request = indexedDB.open("cumuru-encrypted-drafts", 1);
    request.addEventListener("success", () => resolve(request.result));
    request.addEventListener("error", () => reject(request.error));
  });
}

function requestDone(request: IDBRequest) {
  return new Promise<void>((resolve, reject) => {
    request.addEventListener("success", () => resolve());
    request.addEventListener("error", () => reject(request.error));
  });
}

async function replaceDraftRecord(
  id: string,
  update: (record: Record<string, unknown>) => Record<string, unknown>,
) {
  const database = await openDraftDatabase();
  const transaction = database.transaction("drafts", "readwrite");
  const store = transaction.objectStore("drafts");
  const read = store.get(id);
  await requestDone(read);
  await requestDone(store.put(update(read.result as Record<string, unknown>)));
  database.close();
}

describe("rascunhos cifrados", () => {
  afterEach(() => clearAllDrafts());

  it("persiste somente ciphertext, IV e metadados mínimos", async () => {
    const first = await saveDraft(payload);
    const record = await inspectDraftRecord(first.id);

    expect(record).toMatchObject({ id: first.id, schemaVersion: 2 });
    expect(record?.ciphertext.byteLength).toBeGreaterThan(0);
    expect(record?.iv.byteLength).toBe(12);
    expect(JSON.stringify(record)).not.toContain("privacy_notice_version");
    await expect(loadDraft(first.id)).resolves.toEqual(payload);
  });

  it("usa IV de 96 bits novo em cada gravação", async () => {
    const first = await saveDraft(payload);
    const before = await inspectDraftRecord(first.id);
    await saveDraft({ ...payload, visitors: [] }, { id: first.id });
    const after = await inspectDraftRecord(first.id);

    expect(before?.iv).toHaveLength(12);
    expect(after?.iv).toHaveLength(12);
    expect(after?.iv).not.toEqual(before?.iv);
  });

  it("expira em 24 horas e elimina chave e conteúdo", async () => {
    const first = await saveDraft(payload, { now: 1_000 });

    await purgeExpiredDrafts(1_000 + 86_400_001);

    await expect(loadDraft(first.id)).resolves.toBeNull();
    await deleteDraft(first.id);
  });

  it("expurga payload e chave quando o schema ficou obsoleto", async () => {
    const saved = await saveDraft(payload);
    await replaceDraftRecord(saved.id, (record) => ({
      ...record,
      schemaVersion: 1,
    }));

    await expect(loadDraft(saved.id)).resolves.toBeNull();
    await expect(inspectDraftPresence(saved.id)).resolves.toEqual({
      draft: false,
      key: false,
    });
  });

  it("expurga payload e chave quando a autenticação da cifra falha", async () => {
    const saved = await saveDraft(payload);
    await replaceDraftRecord(saved.id, (record) => ({
      ...record,
      ciphertext: new Uint8Array([1, 2, 3]).buffer,
    }));

    await expect(loadDraft(saved.id)).resolves.toBeNull();
    await expect(inspectDraftPresence(saved.id)).resolves.toEqual({
      draft: false,
      key: false,
    });
  });
});
