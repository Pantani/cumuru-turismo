import {
  describedBy,
  FieldError,
  invalidFlag,
} from "../../shared/forms/FieldFeedback";
import type { MessageKey, Translate } from "../../shared/i18n/translate";
import { accessRequestCategories } from "../../shared/api/invite-request-client";
import {
  ACCOMMODATION_NAME_MAX,
  CITY_MAX,
  CONTACT_EMAIL_MAX,
  CONTACT_NAME_MAX,
  CONTACT_PHONE_MAX,
  fieldElementId,
  type InviteRequestField,
  type InviteRequestFormState,
} from "./invite-request-form";

export type FieldChange = (field: InviteRequestField, value: string) => void;

interface FieldProps {
  disabled: boolean;
  form: InviteRequestFormState;
  issueOf: (field: InviteRequestField) => string | undefined;
  onChange: FieldChange;
  t: Translate;
}

interface TextFieldProps {
  autoComplete?: string;
  disabled: boolean;
  field: InviteRequestField;
  inputMode?: "email" | "numeric" | "tel" | "text";
  issue: string | undefined;
  label: string;
  maxLength: number;
  onChange: FieldChange;
  pattern?: string;
  required?: boolean;
  type?: "email" | "tel" | "text";
  value: string;
}

/**
 * O erro fica fora do `label` de propósito: dentro dele, o leitor de tela
 * juntaria rótulo e recusa num único nome de campo. Fora, ligado por
 * `aria-describedby`, ele é anunciado como descrição — e `aria-invalid` marca a
 * falha sem depender da cor da borda.
 */
function TextField({
  autoComplete,
  disabled,
  field,
  inputMode,
  issue,
  label,
  maxLength,
  onChange,
  pattern,
  required,
  type = "text",
  value,
}: TextFieldProps) {
  const id = fieldElementId(field);
  return (
    <div className="field-control">
      <label htmlFor={id}>{label}</label>
      <input
        id={id}
        type={type}
        value={value}
        disabled={disabled}
        required={required}
        maxLength={maxLength}
        inputMode={inputMode}
        pattern={pattern}
        autoComplete={autoComplete}
        aria-invalid={invalidFlag(issue)}
        aria-describedby={describedBy(`${id}-error`, issue)}
        onChange={(event) => onChange(field, event.target.value)}
      />
      <FieldError id={`${id}-error`} message={issue} />
    </div>
  );
}

const categoryLabels: Readonly<Record<string, MessageKey>> = {
  camping: "inviteRequest.category.camping",
  family_hosting: "inviteRequest.category.familyHosting",
  formal_lodging: "inviteRequest.category.formalLodging",
  other: "inviteRequest.category.other",
  regularizing: "inviteRequest.category.regularizing",
  seasonal_rental: "inviteRequest.category.seasonalRental",
};

function CategorySelect({ disabled, form, issueOf, onChange, t }: FieldProps) {
  const field: InviteRequestField = "category";
  const id = fieldElementId(field);
  const issue = issueOf(field);
  return (
    <div className="field-control">
      <label htmlFor={id}>{t("inviteRequest.field.category")}</label>
      <select
        id={id}
        value={form.category}
        disabled={disabled}
        required
        aria-invalid={invalidFlag(issue)}
        aria-describedby={describedBy(`${id}-error`, issue)}
        onChange={(event) => onChange(field, event.target.value)}
      >
        <option value="">{t("inviteRequest.category.placeholder")}</option>
        {accessRequestCategories.map((category) => (
          <option key={category} value={category}>
            {t(categoryLabels[category]!)}
          </option>
        ))}
      </select>
      <FieldError id={`${id}-error`} message={issue} />
    </div>
  );
}

export function LodgingFields(props: FieldProps) {
  const { disabled, form, issueOf, onChange, t } = props;
  return (
    <fieldset>
      <legend>{t("inviteRequest.group.lodging")}</legend>
      <TextField
        disabled={disabled}
        field="accommodation_name"
        issue={issueOf("accommodation_name")}
        label={t("inviteRequest.field.name")}
        maxLength={ACCOMMODATION_NAME_MAX}
        onChange={onChange}
        required
        value={form.accommodation_name}
      />
      <CategorySelect {...props} />
      <TextField
        disabled={disabled}
        field="capacity"
        inputMode="numeric"
        issue={issueOf("capacity")}
        label={t("inviteRequest.field.capacity")}
        maxLength={5}
        onChange={onChange}
        pattern="\d+"
        required
        value={form.capacity}
      />
      <TextField
        autoComplete="address-level2"
        disabled={disabled}
        field="city_label"
        issue={issueOf("city_label")}
        label={t("inviteRequest.field.city")}
        maxLength={CITY_MAX}
        onChange={onChange}
        required
        value={form.city_label}
      />
      <TextField
        autoComplete="address-level1"
        disabled={disabled}
        field="state_code"
        issue={issueOf("state_code")}
        label={t("inviteRequest.field.state")}
        maxLength={2}
        onChange={onChange}
        pattern="[A-Za-z]{2}"
        required
        value={form.state_code}
      />
    </fieldset>
  );
}

export function ContactFields({
  disabled,
  form,
  issueOf,
  onChange,
  t,
}: FieldProps) {
  return (
    <fieldset>
      <legend>{t("inviteRequest.group.contact")}</legend>
      <TextField
        autoComplete="name"
        disabled={disabled}
        field="contact_name"
        issue={issueOf("contact_name")}
        label={t("inviteRequest.field.contactName")}
        maxLength={CONTACT_NAME_MAX}
        onChange={onChange}
        required
        value={form.contact_name}
      />
      <TextField
        autoComplete="email"
        disabled={disabled}
        field="contact_email"
        inputMode="email"
        issue={issueOf("contact_email")}
        label={t("inviteRequest.field.contactEmail")}
        maxLength={CONTACT_EMAIL_MAX}
        onChange={onChange}
        required
        type="email"
        value={form.contact_email}
      />
      <TextField
        autoComplete="tel"
        disabled={disabled}
        field="contact_phone"
        inputMode="tel"
        issue={issueOf("contact_phone")}
        label={t("inviteRequest.field.contactPhone")}
        maxLength={CONTACT_PHONE_MAX}
        onChange={onChange}
        type="tel"
        value={form.contact_phone}
      />
    </fieldset>
  );
}
