import type { components, operations } from "../../generated/schema";
import { isAccommodationDirectory } from "./directory-payload";
import type { HttpClientOptions } from "./http-client";
import { createDocumentReader } from "./public-document-client";

type Schemas = components["schemas"];

/**
 * Lista pública de hospedagens: um GET anônimo, um documento só, cacheável por
 * inteiro. Fica fora do cliente de analytics de propósito — o painel publica
 * estatística agregada e a lista publica contato consentido, e juntá-los faria
 * a política de supressão parecer aplicável a um dado que não é amostra de
 * nada. Os dois compartilham só o transporte.
 */
export type AccommodationDirectory = Schemas["PublicAccommodationDirectory"];
export type AccommodationDirectoryEntry =
  Schemas["PublicAccommodationEntry"];

export const directoryOperationNames = [
  "listPublicAccommodations",
] as const satisfies readonly (keyof operations)[];

export function createDirectoryClient(options: HttpClientOptions) {
  const reader = createDocumentReader(options);
  return {
    listAccommodations: () =>
      reader.published<AccommodationDirectory>(
        "/api/v1/public/accommodations",
        isAccommodationDirectory,
      ),
  };
}

export type DirectoryClient = ReturnType<typeof createDirectoryClient>;

export const publicDirectoryClient = createDirectoryClient({
  getAccessToken: () => null,
});
