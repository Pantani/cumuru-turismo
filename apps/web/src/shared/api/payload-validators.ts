/**
 * Combinadores de allowlist em tempo de execução.
 *
 * Todo documento público é lido por chamador anônimo e passa por cache
 * compartilhado, então o cliente recusa corpo com forma ou propriedade que o
 * contrato não declara em vez de renderizar o que chegou. As peças estão aqui,
 * e não dentro de um allowlist específico, porque a lista pública de
 * hospedagens precisa exatamente da mesma disciplina dos painéis de analytics —
 * e duas cópias das mesmas regras divergiriam uma da outra. Nada neste módulo
 * fala com a rede.
 */

export type Validator = (value: unknown) => boolean;
export type ValidatorMap = Readonly<Record<string, Validator>>;

const datePattern = /^\d{4}-\d{2}-\d{2}$/u;
const dateTimePattern =
  /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/u;

export function record(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function hasOwn(object: Record<string, unknown>, key: string) {
  return Object.prototype.hasOwnProperty.call(object, key);
}

function allowedKeys(object: Record<string, unknown>, allowed: Set<string>) {
  return Object.keys(object).every((key) => allowed.has(key));
}

function requiredValid(object: Record<string, unknown>, required: ValidatorMap) {
  return Object.entries(required).every(
    ([key, validate]) => hasOwn(object, key) && validate(object[key]),
  );
}

function optionalValid(object: Record<string, unknown>, optional: ValidatorMap) {
  return Object.entries(optional).every(
    ([key, validate]) => !hasOwn(object, key) || validate(object[key]),
  );
}

export function objectValidator(
  required: ValidatorMap,
  optional: ValidatorMap = {},
): Validator {
  const allowed = new Set([...Object.keys(required), ...Object.keys(optional)]);
  return (value) => {
    const object = record(value);
    return (
      object !== null &&
      allowedKeys(object, allowed) &&
      requiredValid(object, required) &&
      optionalValid(object, optional)
    );
  };
}

export function arrayValidator(
  validateItem: Validator,
  minimum = 0,
  maximum = Number.POSITIVE_INFINITY,
): Validator {
  return (value) =>
    Array.isArray(value) &&
    value.length >= minimum &&
    value.length <= maximum &&
    value.every(validateItem);
}

export function unionValidator(...validators: Validator[]): Validator {
  return (value) => validators.some((validate) => validate(value));
}

export function literalValidator(...allowed: readonly unknown[]): Validator {
  const values = new Set(allowed);
  return (value) => values.has(value);
}

export function exactArrayValidator(...expected: readonly unknown[]): Validator {
  return (value) =>
    Array.isArray(value) &&
    value.length === expected.length &&
    expected.every((item, index) => Object.is(value[index], item));
}

export function integerValidator(
  minimum: number,
  maximum = Number.POSITIVE_INFINITY,
  multiple = 1,
): Validator {
  return (value) =>
    typeof value === "number" &&
    Number.isInteger(value) &&
    value >= minimum &&
    value <= maximum &&
    value % multiple === 0;
}

export function stringValidator(pattern: RegExp, maximum: number): Validator {
  return (value) =>
    typeof value === "string" && value.length <= maximum && pattern.test(value);
}

/** Nulo é valor declarado no contrato, não ausência do campo. */
export function nullableValidator(validate: Validator): Validator {
  return (value) => value === null || validate(value);
}

export const isDate: Validator = (value) =>
  typeof value === "string" && datePattern.test(value);

export const isDateTime: Validator = (value) =>
  typeof value === "string" &&
  dateTimePattern.test(value) &&
  Number.isFinite(Date.parse(value));
