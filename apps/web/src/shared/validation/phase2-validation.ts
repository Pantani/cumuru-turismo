import type { components } from "../../generated/schema";
import { uuidV7Pattern } from "../identity/uuid-v7";

type CreateStayRequest = components["schemas"]["CreateStayRequest"];
type CreateAccommodationRequest =
  components["schemas"]["CreateAccommodationRequest"];
type CreateAccommodationInput = Omit<CreateAccommodationRequest, "category"> & {
  category: string;
};
type SubmitGroupRequest = components["schemas"]["SubmitGroupRequest"];
type VisitorInput = components["schemas"]["VisitorInput"];

export interface ValidationIssue {
  field: string;
  message: string;
}

const uuidPattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const countryPattern = /^[A-Z]{2}$/;
const statePattern = /^[A-Z]{2}$/;
const cityPattern = /^\d{7}$/;
const datePattern = /^\d{4}-\d{2}-\d{2}$/;
const accommodationInputCategories = new Set<string>([
  "formal_lodging",
  "seasonal_rental",
  "family_hosting",
  "camping",
  "regularizing",
  "other",
]);

function uuidIssue(field: string, value: string): ValidationIssue[] {
  return uuidPattern.test(value)
    ? []
    : [{ field, message: "Informe um identificador válido." }];
}

function uuidV7Issue(field: string, value: string): ValidationIssue[] {
  return uuidV7Pattern.test(value)
    ? []
    : [{ field, message: "Informe um identificador UUIDv7 válido." }];
}

function accommodationNameIssues(name: string) {
  if (name.trim().length > 0 && name.length <= 200) {
    return [];
  }
  return [{ field: "name", message: "Informe o nome do local." }];
}

function accommodationCategoryIssues(category: string) {
  if (accommodationInputCategories.has(category)) {
    return [];
  }
  return [
    { field: "category", message: "Escolha um tipo de hospedagem da lista." },
  ];
}

function capacityIssues(capacity: number) {
  const inRange =
    Number.isInteger(capacity) && capacity >= 1 && capacity <= 10_000;
  if (inRange) {
    return [];
  }
  return [
    {
      field: "capacity",
      message: "Informe uma capacidade entre 1 e 10.000 pessoas.",
    },
  ];
}

export function validateCreateAccommodation(value: CreateAccommodationInput) {
  return [
    ...uuidV7Issue("client_submission_id", value.client_submission_id),
    ...accommodationNameIssues(value.name),
    ...accommodationCategoryIssues(value.category),
    ...capacityIssues(value.capacity),
  ];
}

function civilDateIssues(field: string, value: string, label: string) {
  return datePattern.test(value)
    ? []
    : [{ field, message: `Informe a data de ${label}.` }];
}

function dateOrderIssues(value: CreateStayRequest) {
  const wellFormed =
    datePattern.test(value.planned_arrival_on) &&
    datePattern.test(value.planned_departure_on);
  if (!wellFormed || value.planned_departure_on > value.planned_arrival_on) {
    return [];
  }
  return [
    {
      field: "planned_departure_on",
      message: "A saída deve ser posterior à chegada.",
    },
  ];
}

function validateDateRange(value: CreateStayRequest): ValidationIssue[] {
  return [
    ...civilDateIssues("planned_arrival_on", value.planned_arrival_on, "chegada"),
    ...civilDateIssues("planned_departure_on", value.planned_departure_on, "saída"),
    ...dateOrderIssues(value),
  ];
}

export function validateCreateStay(value: CreateStayRequest) {
  const issues = [
    ...uuidIssue("accommodation_id", value.accommodation_id),
    ...uuidV7Issue("client_submission_id", value.client_submission_id),
    ...validateDateRange(value),
  ];
  if (
    !Number.isInteger(value.expected_guest_count) ||
    value.expected_guest_count < 1 ||
    value.expected_guest_count > 100
  ) {
    issues.push({
      field: "expected_guest_count",
      message: "Informe de 1 a 100 visitantes.",
    });
  }
  return issues;
}

function optionalValue(value: string | null | undefined) {
  return value ?? "";
}

function validateBrazilResidence(visitor: VisitorInput, prefix: string) {
  const issues: ValidationIssue[] = [];
  if (!statePattern.test(optionalValue(visitor.residence_state))) {
    issues.push({
      field: `${prefix}.residence_state`,
      message: "Informe a UF brasileira com duas letras maiúsculas.",
    });
  }
  if (!cityPattern.test(optionalValue(visitor.residence_city_code))) {
    issues.push({
      field: `${prefix}.residence_city_code`,
      message: "Informe o código IBGE de sete dígitos.",
    });
  }
  return issues;
}

function validateForeignResidence(visitor: VisitorInput, prefix: string) {
  const issues: ValidationIssue[] = [];
  if (optionalValue(visitor.residence_state).length > 0) {
    issues.push({
      field: `${prefix}.residence_state`,
      message: "Não informe UF para residência fora do Brasil.",
    });
  }
  if (optionalValue(visitor.residence_city_code).length > 0) {
    issues.push({
      field: `${prefix}.residence_city_code`,
      message: "Não informe município IBGE para residência fora do Brasil.",
    });
  }
  return issues;
}

function validateResidence(visitor: VisitorInput, index: number) {
  const prefix = `visitors.${index}`;
  if (!countryPattern.test(visitor.residence_country)) {
    return [{
      field: `${prefix}.residence_country`,
      message: "Use o código ISO alpha-2 com duas letras maiúsculas.",
    }];
  }
  return visitor.residence_country === "BR"
    ? validateBrazilResidence(visitor, prefix)
    : validateForeignResidence(visitor, prefix);
}

function validateVisitor(visitor: VisitorInput, index: number) {
  return [
    ...uuidV7Issue(`visitors.${index}.client_id`, visitor.client_id),
    ...validateResidence(visitor, index),
  ];
}

function duplicateVisitorIssues(visitors: VisitorInput[]) {
  const seen = new Set<string>();
  const issues: ValidationIssue[] = [];
  visitors.forEach((visitor, index) => {
    if (seen.has(visitor.client_id)) {
      issues.push({
        field: `visitors.${index}.client_id`,
        message: "Cada visitante deve ter um identificador diferente.",
      });
    }
    seen.add(visitor.client_id);
  });
  return issues;
}

export function validateSubmitGroup(value: SubmitGroupRequest) {
  const issues = uuidV7Issue(
    "client_submission_id",
    value.client_submission_id,
  );
  if (value.privacy_notice_version.trim().length === 0) {
    issues.push({
      field: "privacy_notice_version",
      message: "Confirme a versão do aviso de privacidade.",
    });
  }
  if (value.visitors.length < 1 || value.visitors.length > 100) {
    issues.push({
      field: "visitors",
      message: "Informe de 1 a 100 visitantes.",
    });
  }
  const responsibleCount = value.visitors.filter(
    (visitor) => visitor.role === "responsible",
  ).length;
  if (responsibleCount !== 1) {
    issues.push({
      field: "visitors",
      message: "O grupo deve ter exatamente uma pessoa responsável.",
    });
  }
  value.visitors.forEach((visitor, index) => {
    issues.push(...validateVisitor(visitor, index));
  });
  issues.push(...duplicateVisitorIssues(value.visitors));
  return issues;
}
