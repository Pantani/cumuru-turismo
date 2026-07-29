import {
  forwardRef,
  useImperativeHandle,
  useRef,
} from "react";

import type { components } from "../../generated/schema";
import { createUuidV7 } from "../../shared/identity/uuid-v7";
import type { ValidationIssue } from "../../shared/validation/phase2-validation";

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
  visitors: VisitorInput[];
}

const ageBands: ReadonlyArray<
  readonly [VisitorInput["age_band"], string]
> = [
  ["0_5", "0 a 5"],
  ["6_11", "6 a 11"],
  ["12_17", "12 a 17"],
  ["18_24", "18 a 24"],
  ["25_34", "25 a 34"],
  ["35_44", "35 a 44"],
  ["45_59", "45 a 59"],
  ["60_plus", "60 ou mais"],
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
  visitor: VisitorInput;
}

function CoreFields(props: CoreFieldsProps) {
  const roleErrorId = `visitor-${props.index}-role-error`;
  const roleInvalid = props.groupIssue !== undefined;
  return (
    <>
      <label>
        Papel do visitante {props.number}
        <select
          ref={(target) => props.register(props.index, "role", target)}
          value={props.visitor.role}
          aria-invalid={roleInvalid || undefined}
          aria-describedby={roleInvalid ? roleErrorId : undefined}
          onChange={(event) =>
            props.change(props.index, "role", event.target.value)
          }
        >
          <option value="responsible">Responsável</option>
          <option value="companion">Acompanhante</option>
          <option value="minor">Menor</option>
        </select>
        {props.groupIssue === undefined ? null : (
          <span id={roleErrorId} className="field-error">
            {props.groupIssue.message}
          </span>
        )}
      </label>
      <label>
        Faixa etária do visitante {props.number}
        <select
          ref={(target) => props.register(props.index, "age_band", target)}
          value={props.visitor.age_band}
          onChange={(event) =>
            props.change(props.index, "age_band", event.target.value)
          }
        >
          {ageBands.map(([value, label]) => (
            <option key={value} value={value}>
              {label}
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
  visitor: VisitorInput;
}

function BrazilResidenceFields(props: ResidenceFieldsProps) {
  return (
    <>
      <VisitorTextField
        id={`visitor-${props.index}-state`}
        index={props.index}
        field="residence_state"
        label={`UF de residência do visitante ${props.number}`}
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
        label={`Município IBGE do visitante ${props.number}`}
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
        label={`País de residência do visitante ${props.number}`}
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
  visitorCount: number;
}

function VisitorFields(props: VisitorFieldsProps) {
  const groupIssue =
    props.index === 0
      ? props.issues.find((issue) => issue.field === "visitors")
      : undefined;
  return (
    <fieldset disabled={props.disabled}>
      <legend>Visitante {props.number}</legend>
      <div className="field-grid">
        <CoreFields {...props} groupIssue={groupIssue} />
        <ResidenceFields {...props} />
      </div>
      <button
        type="button"
        disabled={props.disabled || props.visitorCount === 1}
        onClick={() => props.onRemove(props.index)}
      >
        Remover visitante {props.number}
      </button>
    </fieldset>
  );
}

export const VisitorEditor = forwardRef<
  VisitorEditorHandle,
  VisitorEditorProps
>(function VisitorEditor(
  { disabled = false, issues = [], onChange, visitors },
  forwardedRef,
) {
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
          change={change}
          register={register}
          onRemove={remove}
        />
      ))}
      <button
        type="button"
        disabled={disabled || visitors.length >= 100}
        onClick={add}
      >
        Adicionar visitante
      </button>
    </div>
  );
});
