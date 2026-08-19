import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";

import {
  availableCategories,
  directoryCategoryKeys,
  filterEntries,
  formatPhone,
  whatsappHref,
  type DirectoryCategory,
} from "./directory-vocabulary";
import type {
  AccommodationDirectory as Listing,
  AccommodationDirectoryEntry,
  DirectoryClient,
} from "../../shared/api/directory-client";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import type { Translate } from "../../shared/i18n/translate";

const DIRECTORY_QUERY_KEY = "public-accommodations";

interface DirectoryProps {
  client: DirectoryClient;
}

interface FiltersProps {
  categories: readonly DirectoryCategory[];
  category: DirectoryCategory | "all";
  onCategory: (next: DirectoryCategory | "all") => void;
  onTerm: (next: string) => void;
  term: string;
}

function DirectoryFilters({
  categories,
  category,
  onCategory,
  onTerm,
  term,
}: FiltersProps) {
  const { t } = useLocale();
  return (
    <div className="directory-filters">
      <p className="field-control">
        <label htmlFor="directory-search">{t("directory.search.label")}</label>
        <input
          id="directory-search"
          type="search"
          value={term}
          placeholder={t("directory.search.placeholder")}
          onChange={(event) => onTerm(event.target.value)}
        />
      </p>
      <p className="field-control">
        <label htmlFor="directory-category">{t("directory.filter.label")}</label>
        <select
          id="directory-category"
          value={category}
          onChange={(event) =>
            onCategory(event.target.value as DirectoryCategory | "all")
          }
        >
          <option value="all">{t("directory.filter.all")}</option>
          {categories.map((option) => (
            <option key={option} value={option}>
              {t(directoryCategoryKeys[option])}
            </option>
          ))}
        </select>
      </p>
    </div>
  );
}

/**
 * O telefone é o motivo da lista existir, então é link e não texto: no celular
 * o toque disca, e quem lê pela tela ouve o nome da hospedagem junto da ação.
 */
function EntryContact({ entry, t }: { entry: AccommodationDirectoryEntry; t: Translate }) {
  return (
    <p className="directory-contact">
      <a
        className="directory-call"
        href={`tel:${entry.phone}`}
        aria-label={t("directory.card.call", { name: entry.name })}
      >
        {formatPhone(entry.phone)}
      </a>
      {entry.whatsapp ? (
        <a
          className="directory-whatsapp"
          href={whatsappHref(entry.phone)}
          rel="noreferrer noopener"
          target="_blank"
          aria-label={t("directory.card.whatsapp", { name: entry.name })}
        >
          WhatsApp
        </a>
      ) : null}
      {entry.website === null ? null : (
        <a
          className="directory-website"
          href={entry.website}
          rel="noreferrer noopener"
          target="_blank"
          aria-label={t("directory.card.website", { name: entry.name })}
        >
          {t("directory.card.websiteLabel")}
        </a>
      )}
    </p>
  );
}

function DirectoryCard({ entry }: { entry: AccommodationDirectoryEntry }) {
  const { t } = useLocale();
  return (
    <li className="directory-card">
      <h3>{entry.name}</h3>
      <p className="directory-meta">
        <span className="directory-category">
          {t(directoryCategoryKeys[entry.category])}
        </span>
        {entry.area_code === null ? null : (
          <span className="directory-area">{entry.area_code}</span>
        )}
        {entry.capacity === null ? null : (
          <span className="directory-capacity">
            {t("directory.card.capacity", { count: entry.capacity })}
          </span>
        )}
      </p>
      <EntryContact entry={entry} t={t} />
    </li>
  );
}

function DirectoryList({
  entries,
}: {
  entries: readonly AccommodationDirectoryEntry[];
}) {
  const { t } = useLocale();
  if (entries.length === 0) {
    return <p className="directory-empty">{t("directory.noMatches")}</p>;
  }
  return (
    <ul className="directory-list">
      {entries.map((entry) => (
        <DirectoryCard key={entry.id} entry={entry} />
      ))}
    </ul>
  );
}

function DirectorySummary({ listing }: { listing: Listing }) {
  const { t, locale } = useLocale();
  const published =
    listing.count === 1
      ? t("directory.countOne")
      : t("directory.count", { count: listing.count });
  const updated = t("directory.updatedAt", {
    date: new Date(listing.updated_at).toLocaleDateString(locale),
  });
  return <p className="directory-summary">{`${published} · ${updated}`}</p>;
}

function DirectoryBody({ listing }: { listing: Listing }) {
  const { t } = useLocale();
  const [term, setTerm] = useState("");
  const [category, setCategory] = useState<DirectoryCategory | "all">("all");
  const entries = listing.entries;
  const visible = useMemo(
    () => filterEntries(entries, category, term),
    [entries, category, term],
  );

  if (entries.length === 0) {
    return <p className="directory-empty">{t("directory.empty")}</p>;
  }
  return (
    <div className="directory">
      <DirectorySummary listing={listing} />
      <DirectoryFilters
        categories={availableCategories(entries)}
        category={category}
        onCategory={setCategory}
        onTerm={setTerm}
        term={term}
      />
      <DirectoryList entries={visible} />
    </div>
  );
}

export function AccommodationDirectory({ client }: DirectoryProps) {
  const { t } = useLocale();
  const query = useQuery({
    queryKey: [DIRECTORY_QUERY_KEY],
    queryFn: () => client.listAccommodations(),
    select: (result) => result.data,
  });

  if (query.isPending) {
    return (
      <p className="route-status" role="status">
        {t("directory.loading")}
      </p>
    );
  }
  // Erro e ausência de corpo são a mesma coisa para quem lê: sem documento, não
  // há lista, e a página diz isso em vez de mostrar uma seção vazia.
  if (query.data === undefined) {
    return (
      <p className="form-error" role="alert">
        {t("directory.failed")}
      </p>
    );
  }
  return <DirectoryBody listing={query.data} />;
}
