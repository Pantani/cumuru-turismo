const DATABASE_NAME = "cumuru-encrypted-drafts";
const DATABASE_VERSION = 1;
const DRAFT_STORE = "drafts";
const KEY_STORE = "keys";
const SCHEMA_VERSION = 2;
const TTL_MILLISECONDS = 24 * 60 * 60 * 1_000;

interface DraftRecord {
  ciphertext: ArrayBuffer;
  createdAt: number;
  expiresAt: number;
  id: string;
  iv: Uint8Array<ArrayBuffer>;
  schemaVersion: number;
  updatedAt: number;
}

interface KeyRecord {
  id: string;
  key: CryptoKey;
}

export interface SaveDraftOptions {
  id?: string;
  now?: number;
}

function requestResult<T>(request: IDBRequest<T>) {
  return new Promise<T>((resolve, reject) => {
    request.addEventListener("success", () => resolve(request.result));
    request.addEventListener("error", () => reject(request.error));
  });
}

function transactionDone(transaction: IDBTransaction) {
  return new Promise<void>((resolve, reject) => {
    transaction.addEventListener("complete", () => resolve());
    transaction.addEventListener("abort", () => reject(transaction.error));
    transaction.addEventListener("error", () => reject(transaction.error));
  });
}

function openDatabase() {
  return new Promise<IDBDatabase>((resolve, reject) => {
    const request = indexedDB.open(DATABASE_NAME, DATABASE_VERSION);
    request.addEventListener("upgradeneeded", () => {
      request.result.createObjectStore(DRAFT_STORE, { keyPath: "id" });
      request.result.createObjectStore(KEY_STORE, { keyPath: "id" });
    });
    request.addEventListener("success", () => resolve(request.result));
    request.addEventListener("error", () => reject(request.error));
  });
}

async function generateKey() {
  return crypto.subtle.generateKey(
    { name: "AES-GCM", length: 256 },
    false,
    ["encrypt", "decrypt"],
  );
}

function randomIv(previous?: Uint8Array) {
  const iv = crypto.getRandomValues(new Uint8Array(12));
  if (previous !== undefined && sameBytes(iv, previous)) {
    return crypto.getRandomValues(new Uint8Array(12));
  }
  return iv;
}

function sameBytes(left: Uint8Array, right: Uint8Array) {
  return (
    left.length === right.length &&
    left.every((value, index) => value === right[index])
  );
}

async function getStored<T>(
  database: IDBDatabase,
  storeName: string,
  id: string,
) {
  const transaction = database.transaction(storeName, "readonly");
  return requestResult(
    transaction.objectStore(storeName).get(id) as IDBRequest<T | undefined>,
  );
}

async function storeEncrypted(
  database: IDBDatabase,
  record: DraftRecord,
  key: CryptoKey,
) {
  const transaction = database.transaction(
    [DRAFT_STORE, KEY_STORE],
    "readwrite",
  );
  transaction.objectStore(DRAFT_STORE).put(record);
  transaction.objectStore(KEY_STORE).put({ id: record.id, key } satisfies KeyRecord);
  await transactionDone(transaction);
}

function resolveDraftId(id: string | undefined) {
  return id ?? crypto.randomUUID();
}

async function resolveDraftKey(stored: KeyRecord | undefined) {
  return stored?.key ?? (await generateKey());
}

function draftRecord(
  id: string,
  previous: DraftRecord | undefined,
  now: number,
  iv: Uint8Array<ArrayBuffer>,
  ciphertext: ArrayBuffer,
): DraftRecord {
  return {
    id,
    schemaVersion: SCHEMA_VERSION,
    createdAt: previous?.createdAt ?? now,
    updatedAt: now,
    expiresAt: previous?.expiresAt ?? now + TTL_MILLISECONDS,
    iv,
    ciphertext,
  };
}

export async function saveDraft<T>(
  value: T,
  options: SaveDraftOptions = {},
) {
  const database = await openDatabase();
  const id = resolveDraftId(options.id);
  const previous = await getStored<DraftRecord>(database, DRAFT_STORE, id);
  const storedKey = await getStored<KeyRecord>(database, KEY_STORE, id);
  const key = await resolveDraftKey(storedKey);
  const iv = randomIv(previous?.iv);
  const plaintext = new TextEncoder().encode(JSON.stringify(value));
  const ciphertext = await crypto.subtle.encrypt({ name: "AES-GCM", iv }, key, plaintext);
  const now = options.now ?? Date.now();
  const record = draftRecord(id, previous, now, iv, ciphertext);
  await storeEncrypted(database, record, key);
  database.close();
  return { id, expiresAt: record.expiresAt };
}

export async function deleteDraft(id: string) {
  const database = await openDatabase();
  const transaction = database.transaction(
    [DRAFT_STORE, KEY_STORE],
    "readwrite",
  );
  transaction.objectStore(DRAFT_STORE).delete(id);
  transaction.objectStore(KEY_STORE).delete(id);
  await transactionDone(transaction);
  database.close();
}

async function decryptDraft<T>(
  record: DraftRecord,
  keyRecord: KeyRecord | undefined,
) {
  if (keyRecord === undefined) {
    return null;
  }
  const plaintext = await crypto.subtle.decrypt(
    { name: "AES-GCM", iv: Uint8Array.from(record.iv) },
    keyRecord.key,
    record.ciphertext,
  );
  return JSON.parse(new TextDecoder().decode(plaintext)) as T;
}

export async function loadDraft<T>(id: string): Promise<T | null> {
  const database = await openDatabase();
  const record = await getStored<DraftRecord>(database, DRAFT_STORE, id);
  const key = await getStored<KeyRecord>(database, KEY_STORE, id);
  database.close();
  if (
    record === undefined ||
    record.schemaVersion !== SCHEMA_VERSION ||
    record.expiresAt <= Date.now()
  ) {
    await deleteDraft(id);
    return null;
  }
  try {
    return await decryptDraft<T>(record, key);
  } catch {
    await deleteDraft(id);
    return null;
  }
}

export async function inspectDraftRecord(id: string) {
  const database = await openDatabase();
  const record = await getStored<DraftRecord>(database, DRAFT_STORE, id);
  database.close();
  return record;
}

export async function inspectDraftPresence(id: string) {
  const database = await openDatabase();
  const draft = await getStored<DraftRecord>(database, DRAFT_STORE, id);
  const key = await getStored<KeyRecord>(database, KEY_STORE, id);
  database.close();
  return { draft: draft !== undefined, key: key !== undefined };
}

export async function purgeExpiredDrafts(now = Date.now()) {
  const database = await openDatabase();
  const transaction = database.transaction(
    [DRAFT_STORE, KEY_STORE],
    "readwrite",
  );
  const drafts = transaction.objectStore(DRAFT_STORE);
  const records = await requestResult(
    drafts.getAll() as IDBRequest<DraftRecord[]>,
  );
  for (const record of records) {
    if (record.expiresAt <= now || record.schemaVersion !== SCHEMA_VERSION) {
      drafts.delete(record.id);
      transaction.objectStore(KEY_STORE).delete(record.id);
    }
  }
  await transactionDone(transaction);
  database.close();
}

export async function clearAllDrafts() {
  const database = await openDatabase();
  const transaction = database.transaction(
    [DRAFT_STORE, KEY_STORE],
    "readwrite",
  );
  transaction.objectStore(DRAFT_STORE).clear();
  transaction.objectStore(KEY_STORE).clear();
  await transactionDone(transaction);
  database.close();
}
