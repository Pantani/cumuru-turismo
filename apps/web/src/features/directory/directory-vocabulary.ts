import type { AccommodationDirectoryEntry } from "../../shared/api/directory-client";
import type { MessageKey } from "../../shared/i18n/translate";

export type DirectoryCategory = AccommodationDirectoryEntry["category"];

/**
 * Mesmo vocabulário do formulário público e da fila da administração, mais
 * `unclassified`, que só a lista encontra: é estado de linha herdada, e nenhum
 * formulário permite escolhê-lo.
 */
export const directoryCategoryKeys: Record<DirectoryCategory, MessageKey> = {
  formal_lodging: "inviteRequest.category.formalLodging",
  seasonal_rental: "inviteRequest.category.seasonalRental",
  family_hosting: "inviteRequest.category.familyHosting",
  camping: "inviteRequest.category.camping",
  regularizing: "inviteRequest.category.regularizing",
  other: "inviteRequest.category.other",
  unclassified: "directory.category.unclassified",
};

/**
 * O número é publicado em E.164 justamente para virar link sem tratamento; a
 * exibição volta a ter espaços, porque é lida em voz alta por quem vai ligar.
 */
export function formatPhone(phone: string) {
  const brazilian = /^\+55(\d{2})(\d{4,5})(\d{4})$/u.exec(phone);
  if (brazilian === null) {
    return phone;
  }
  return `(${brazilian[1]}) ${brazilian[2]}-${brazilian[3]}`;
}

/** wa.me quer o número sem o `+`; qualquer outra forma abre conversa vazia. */
export function whatsappHref(phone: string) {
  return `https://wa.me/${phone.slice(1)}`;
}

export function matchesSearch(
  entry: AccommodationDirectoryEntry,
  term: string,
) {
  const needle = term.trim().toLocaleLowerCase("pt-BR");
  if (needle.length === 0) {
    return true;
  }
  const haystack = [entry.name, entry.area_code ?? ""]
    .join(" ")
    .toLocaleLowerCase("pt-BR");
  return haystack.includes(needle);
}

export function filterEntries(
  entries: readonly AccommodationDirectoryEntry[],
  category: DirectoryCategory | "all",
  term: string,
) {
  return entries.filter(
    (entry) =>
      (category === "all" || entry.category === category) &&
      matchesSearch(entry, term),
  );
}

/**
 * Só as categorias que a lista realmente tem viram filtro: oferecer um filtro
 * que devolve nada é prometer hospedagem que ninguém publicou.
 */
export function availableCategories(
  entries: readonly AccommodationDirectoryEntry[],
): DirectoryCategory[] {
  const present = new Set(entries.map((entry) => entry.category));
  return (Object.keys(directoryCategoryKeys) as DirectoryCategory[]).filter(
    (category) => present.has(category),
  );
}
