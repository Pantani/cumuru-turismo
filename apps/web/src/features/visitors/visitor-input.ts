import type { components } from "../../generated/schema";

type VisitorInput = components["schemas"]["VisitorInput"];

/**
 * Canonicalises what the visitor editor produces before it reaches the wire:
 * ISO codes upper-cased, and the Brazilian residence pair dropped entirely for
 * a foreign residence, so no empty string is ever sent where the contract
 * expects the field to be absent.
 */
export function normalizedVisitor(visitor: VisitorInput): VisitorInput {
  const country = visitor.residence_country.toUpperCase();
  const common = {
    client_id: visitor.client_id,
    role: visitor.role,
    age_band: visitor.age_band,
    residence_country: country,
  };
  if (country !== "BR") {
    return common;
  }
  return {
    ...common,
    residence_state: (visitor.residence_state ?? "").toUpperCase(),
    residence_city_code: visitor.residence_city_code ?? "",
  };
}
