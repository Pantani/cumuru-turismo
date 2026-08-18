import {
  forwardRef,
  useImperativeHandle,
  useRef,
} from "react";

import type { components } from "../../generated/schema";
import { useLocale } from "../../shared/i18n/LocaleProvider";
import type { MessageKey, Translate } from "../../shared/i18n/translate";
import { createUuidV7 } from "../../shared/identity/uuid-v7";
import type { ValidationIssue } from "../../shared/validation/core-validation";

type VisitorInput = components["schemas"]["VisitorInput"];
type VisitorField =
  | "age_band"
  | "residence_city_code"
  | "residence_country"
  | "residence_state"
  | "role";
type FocusTarget = HTMLInputElement | HTMLSelectElement;

export interface VisitorEditorHandle {
  focus: (field: string) => void;
}

interface VisitorEditorProps {
  disabled?: boolean;
  issues?: ValidationIssue[];
  onChange: (visitors: VisitorInput[]) => void;
  /**
   * The open self-service channel narrows this to responsible and companion:
   * ADR-040 refuses `minor` there, and a role that cannot be chosen is better
   * than one that is chosen and then rejected.
   */
  roles?: readonly VisitorInput["role"][];
  visitors: VisitorInput[];
}

const visitorRoleKeys: Record<VisitorInput["role"], MessageKey> = {
  responsible: "visitor.role.responsible",
  companion: "visitor.role.companion",
  minor: "visitor.role.minor",
};

export function visitorRoleLabel(t: Translate, role: VisitorInput["role"]) {
  return t(visitorRoleKeys[role]);
}

const allVisitorRoles = [
  "responsible",
  "companion",
  "minor",
] as const satisfies readonly VisitorInput["role"][];

const ageBandKeys: ReadonlyArray<
  readonly [VisitorInput["age_band"], MessageKey]
> = [
  ["0_5", "visitor.ageBand.0_5"],
  ["6_11", "visitor.ageBand.6_11"],
  ["12_17", "visitor.ageBand.12_17"],
  ["18_24", "visitor.ageBand.18_24"],
  ["25_34", "visitor.ageBand.25_34"],
  ["35_44", "visitor.ageBand.35_44"],
  ["45_59", "visitor.ageBand.45_59"],
  ["60_plus", "visitor.ageBand.60_plus"],
];

export function createVisitor(
  role: VisitorInput["role"] = "companion",
): VisitorInput {
  return {
    client_id: createUuidV7(),
    role,
    age_band: role === "minor" ? "0_5" : "25_34",
    residence_country: "BR",
    residence_state: "",
    residence_city_code: "",
  };
}

function fieldIssue(
  issues: ValidationIssue[],
  index: number,
  field: VisitorField,
) {
  return issues.find((issue) => issue.field === `visitors.${index}.${field}`);
}

function updateResidenceCountry(
  visitor: VisitorInput,
  value: string,
): VisitorInput {
  const residenceCountry = value.toUpperCase();
  if (residenceCountry === "BR") {
    return { ...visitor, residence_country: residenceCountry };
  }
  const next = { ...visitor, residence_country: residenceCountry };
  delete next.residence_state;
  delete next.residence_city_code;
  return next;
}

function updateVisitor(
  visitor: VisitorInput,
  field: VisitorField,
  value: string,
): VisitorInput {
  if (field === "residence_country") {
    return updateResidenceCountry(visitor, value);
  }
  return { ...visitor, [field]: value };
}

type ChangeVisitor = (
  index: number,
  field: VisitorField,
  value: string,
) => void;
type RegisterTarget = (
  index: number,
  field: VisitorField,
  target: FocusTarget | null,
) => void;

interface VisitorTextFieldProps {
  field: VisitorField;
  id: string;
  index: number;
  inputMode?: "numeric";
  issue: ValidationIssue | undefined;
  label: string;
  maxLength: number;
  onChange: ChangeVisitor;
  pattern: string;
  register: RegisterTarget;
  value: string;
}

function VisitorTextField(props: VisitorTextFieldProps) {
  const errorId = `${props.id}-error`;
  const invalid = props.issue !== undefined;
  return (
    <div className="field-control">
      <label htmlFor={props.id}>{props.label}</label>
      <input
        id={props.id}
        ref={(target) => props.register(props.index, props.field, target)}
        value={props.value}
        inputMode={props.inputMode}
        pattern={props.pattern}
        maxLength={props.maxLength}
        required
        aria-invalid={invalid || undefined}
        aria-describedby={invalid ? errorId : undefined}
        onChange={(event) =>
          props.onChange(props.index, props.field, event.target.value)
        }
      />
      {props.issue === undefined ? null : (
        <span id={errorId} className="field-error">
          {props.issue.message}
        </span>
      )}
    </div>
  );
}

interface CoreFieldsProps {
  change: ChangeVisitor;
  groupIssue: ValidationIssue | undefined;
  index: number;
  number: number;
  register: RegisterTarget;
  roles: readonly VisitorInput["role"][];
  t: Translate;
  visitor: VisitorInput;
}

function CoreFields(props: CoreFieldsProps) {
  const roleErrorId = `visitor-${props.index}-role-error`;
  const roleInvalid = props.groupIssue !== undefined;
  return (
    <>
      <label>
        {props.t("visitor.roleLabel", { number: props.number })}
        <select
          ref={(target) => props.register(props.index, "role", target)}
          value={props.visitor.role}
          aria-invalid={roleInvalid || undefined}
          aria-describedby={roleInvalid ? roleErrorId : undefined}
          onChange={(event) =>
            props.change(props.index, "role", event.target.value)
          }
        >
          {props.roles.map((role) => (
            <option key={role} value={role}>
              {visitorRoleLabel(props.t, role)}
            </option>
          ))}
        </select>
        {props.groupIssue === undefined ? null : (
          <span id={roleErrorId} className="field-error">
            {props.groupIssue.message}
          </span>
        )}
      </label>
      <label>
        {props.t("visitor.ageBandLabel", { number: props.number })}
        <select
          ref={(target) => props.register(props.index, "age_band", target)}
          value={props.visitor.age_band}
          onChange={(event) =>
            props.change(props.index, "age_band", event.target.value)
          }
        >
          {ageBandKeys.map(([value, key]) => (
            <option key={value} value={value}>
              {props.t(key)}
            </option>
          ))}
        </select>
      </label>
    </>
  );
}

interface ResidenceFieldsProps {
  change: ChangeVisitor;
  index: number;
  issues: ValidationIssue[];
  number: number;
  register: RegisterTarget;
  t: Translate;
  visitor: VisitorInput;
}

function BrazilResidenceFields(props: ResidenceFieldsProps) {
  return (
    <>
      <VisitorTextField
        id={`visitor-${props.index}-state`}
        index={props.index}
        field="residence_state"
        label={props.t("visitor.residenceState", { number: props.number })}
        value={props.visitor.residence_state ?? ""}
        pattern="[A-Za-z]{2}"
        maxLength={2}
        issue={fieldIssue(props.issues, props.index, "residence_state")}
        onChange={props.change}
        register={props.register}
      />
      <VisitorTextField
        id={`visitor-${props.index}-city`}
        index={props.index}
        field="residence_city_code"
        label={props.t("visitor.residenceCity", { number: props.number })}
        value={props.visitor.residence_city_code ?? ""}
        inputMode="numeric"
        pattern="\d{7}"
        maxLength={7}
        issue={fieldIssue(props.issues, props.index, "residence_city_code")}
        onChange={props.change}
        register={props.register}
      />
    </>
  );
}

function ResidenceFields(props: ResidenceFieldsProps) {
  return (
    <>
      <VisitorTextField
        id={`visitor-${props.index}-country`}
        index={props.index}
        field="residence_country"
        label={props.t("visitor.residenceCountry", { number: props.number })}
        value={props.visitor.residence_country}
        pattern="[A-Za-z]{2}"
        maxLength={2}
        issue={fieldIssue(props.issues, props.index, "residence_country")}
        onChange={props.change}
        register={props.register}
      />
      {props.visitor.residence_country === "BR" ? (
        <BrazilResidenceFields {...props} />
      ) : null}
    </>
  );
}

interface VisitorFieldsProps extends ResidenceFieldsProps {
  disabled: boolean;
  onRemove: (index: number) => void;
  roles: readonly VisitorInput["role"][];
  visitorCount: number;
}

function VisitorFields(props: VisitorFieldsProps) {
  const groupIssue =
    props.index === 0
      ? props.issues.find((issue) => issue.field === "visitors")
      : undefined;
  return (
    <fieldset disabled={props.disabled}>
      <legend>{props.t("visitor.legend", { number: props.number })}</legend>
      <div className="field-grid">
        <CoreFields {...props} groupIssue={groupIssue} />
        <ResidenceFields {...props} />
      </div>
      <button
        type="button"
        disabled={props.disabled || props.visitorCount === 1}
        onClick={() => props.onRemove(props.index)}
      >
        {props.t("visitor.remove", { number: props.number })}
      </button>
    </fieldset>
  );
}

export const VisitorEditor = forwardRef<
  VisitorEditorHandle,
  VisitorEditorProps
>(function VisitorEditor(
  { disabled = false, issues = [], onChange, roles = allVisitorRoles, visitors },
  forwardedRef,
) {
  const { t } = useLocale();
  const targets = useRef<Record<string, FocusTarget | null>>({});

  useImperativeHandle(
    forwardedRef,
    () => ({
      focus: (field) => targets.current[field]?.focus(),
    }),
    [],
  );

  function change(index: number, field: VisitorField, value: string) {
    onChange(
      visitors.map((visitor, visitorIndex) =>
        visitorIndex === index
          ? updateVisitor(visitor, field, value)
          : visitor,
      ),
    );
  }

  function remove(index: number) {
    onChange(visitors.filter((_, visitorIndex) => visitorIndex !== index));
  }

  function add() {
    onChange([...visitors, createVisitor()]);
  }

  function register(
    index: number,
    field: VisitorField,
    target: FocusTarget | null,
  ) {
    targets.current[`visitors.${index}.${field}`] = target;
  }

  return (
    <div className="visitor-editor">
      {visitors.map((visitor, index) => (
        <VisitorFields
          key={visitor.client_id}
          visitor={visitor}
          visitorCount={visitors.length}
          index={index}
          number={index + 1}
          issues={issues}
          disabled={disabled}
          roles={roles}
          change={change}
          register={register}
          onRemove={remove}
          t={t}
        />
      ))}
      <button
        type="button"
        disabled={disabled || visitors.length >= 100}
        onClick={add}
      >
        {t("visitor.add")}
      </button>
    </div>
  );
});
