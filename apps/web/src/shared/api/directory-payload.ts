/**
 * Allowlist da lista pública de hospedagens.
 *
 * O documento é nominal — nome e telefone de quem quer ser encontrado — e chega
 * por rota aberta. Por isso a forma é conferida antes de virar link discável:
 * um telefone fora de E.164 vira `tel:` que não disca, e uma propriedade que o
 * contrato não declara não deveria alcançar a tela de ninguém.
 */

import {
  arrayValidator,
  integerValidator,
  isDateTime,
  literalValidator,
  nullableValidator,
  objectValidator,
  stringValidator,
  type Validator,
} from "./payload-validators";

const phonePattern = /^\+[1-9][0-9]{9,14}$/u;
const uuidPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/u;

/** Mesmo vocabulário de AccommodationCategory, unclassified incluído. */
const isCategory = literalValidator(
  "formal_lodging",
  "seasonal_rental",
  "family_hosting",
  "camping",
  "regularizing",
  "other",
  "unclassified",
);

const isHttpsUrl: Validator = (value) => {
  if (typeof value !== "string" || value.length > 180) {
    return false;
  }
  return URL.canParse(value) && new URL(value).protocol === "https:";
};

const isEntry = objectValidator({
  id: stringValidator(uuidPattern, 36),
  name: stringValidator(/^.{1,200}$/su, 200),
  category: isCategory,
  capacity: nullableValidator(integerValidator(1, 10000)),
  area_code: nullableValidator(stringValidator(/^.{1,100}$/su, 100)),
  phone: stringValidator(phonePattern, 16),
  whatsapp: (value) => typeof value === "boolean",
  website: nullableValidator(isHttpsUrl),
});

export const isAccommodationDirectory = objectValidator({
  updated_at: isDateTime,
  count: integerValidator(0),
  entries: arrayValidator(isEntry),
});
