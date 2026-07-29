import type { components } from "../../generated/schema";

import type { AnswerValue } from "./survey-logic";

type Question = components["schemas"]["PublicQuestion"];

interface Props {
  disabled: boolean;
  question: Question;
  value: AnswerValue | undefined;
  onChange: (value: AnswerValue | undefined) => void;
}

function ChoiceInput({ disabled, question, value, onChange }: Props) {
  if (question.answer_type === "single_choice") {
    return (
      <select
        id={`survey-${question.id}`}
        aria-labelledby={`survey-label-${question.id}`}
        disabled={disabled}
        value={typeof value === "string" ? value : ""}
        onChange={(event) => onChange(event.target.value || undefined)}
      >
        <option value="">Selecione</option>
        {question.options.map((option) => (
          <option key={option.id} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    );
  }
  const selected = Array.isArray(value) ? value : [];
  return (
    <div className="choice-list">
      {question.options.map((option) => (
        <label key={option.id}>
          <input
            type="checkbox"
            disabled={disabled}
            checked={selected.includes(option.value)}
            onChange={(event) => {
              const next = event.target.checked
                ? [...selected, option.value]
                : selected.filter((item) => item !== option.value);
              onChange(next.length === 0 ? undefined : next);
            }}
          />
          {option.label}
        </label>
      ))}
    </div>
  );
}

function StateCityInput({ disabled, value, onChange }: Props) {
  const structured =
    typeof value === "object" && !Array.isArray(value) ? value : undefined;
  const state = structured?.state ?? "";
  const cityCode = structured?.city_code ?? "";
  const update = (nextState: string, nextCityCode: string) => {
    onChange(
      nextState.length === 0 && nextCityCode.length === 0
        ? undefined
        : { state: nextState.toUpperCase(), city_code: nextCityCode },
    );
  };
  return (
    <div className="field-grid">
      <label>
        UF
        <input
          disabled={disabled}
          value={state}
          maxLength={2}
          onChange={(event) => update(event.target.value, cityCode)}
        />
      </label>
      <label>
        Município IBGE
        <input
          disabled={disabled}
          value={cityCode}
          inputMode="numeric"
          maxLength={7}
          onChange={(event) => update(state, event.target.value)}
        />
      </label>
    </div>
  );
}

function BooleanInput({ disabled, question, value, onChange }: Props) {
  return (
    <select
      id={`survey-${question.id}`}
      aria-labelledby={`survey-label-${question.id}`}
      disabled={disabled}
      value={typeof value === "boolean" ? String(value) : ""}
      onChange={(event) =>
        onChange(
          event.target.value === "" ? undefined : event.target.value === "true",
        )
      }
    >
      <option value="">Selecione</option>
      <option value="true">Sim</option>
      <option value="false">Não</option>
    </select>
  );
}

function numericQuestion(question: Question) {
  return (
    question.answer_type === "integer_range" ||
    question.answer_type === "rating"
  );
}

function inputType(question: Question) {
  if (question.answer_type === "date") {
    return "date";
  }
  return numericQuestion(question) ? "number" : "text";
}

function BasicInput({ disabled, question, value, onChange }: Props) {
  const numeric = numericQuestion(question);
  const displayed =
    typeof value === "string" || typeof value === "number" ? value : "";
  return (
    <input
      id={`survey-${question.id}`}
      aria-labelledby={`survey-label-${question.id}`}
      disabled={disabled}
      type={inputType(question)}
      value={displayed}
      min={question.validation?.minimum}
      max={question.validation?.maximum}
      minLength={question.validation?.min_length}
      maxLength={question.validation?.max_length}
      onChange={(event) => {
        if (event.target.value === "") {
          onChange(undefined);
          return;
        }
        onChange(numeric ? Number(event.target.value) : event.target.value);
      }}
    />
  );
}

function QuestionControl(props: Props) {
  const { question } = props;
  const choice =
    question.answer_type === "single_choice" ||
    question.answer_type === "multiple_choice";
  if (choice) {
    return <ChoiceInput {...props} />;
  }
  if (question.answer_type === "state_city") {
    return <StateCityInput {...props} />;
  }
  if (question.answer_type === "long_text") {
    return (
      <textarea
        id={`survey-${question.id}`}
        aria-labelledby={`survey-label-${question.id}`}
        disabled={props.disabled}
        value={typeof props.value === "string" ? props.value : ""}
        rows={5}
        maxLength={question.validation?.max_length}
        onChange={(event) => props.onChange(event.target.value || undefined)}
      />
    );
  }
  if (question.answer_type === "boolean") {
    return <BooleanInput {...props} />;
  }
  return <BasicInput {...props} />;
}

function isFreeText(question: Question) {
  return (
    question.answer_type === "short_text" ||
    question.answer_type === "long_text"
  );
}

export function SurveyQuestion(props: Props) {
  const { question } = props;
  return (
    <fieldset className="survey-question">
      <legend id={`survey-label-${question.id}`}>
        {question.prompt}
        {question.required ? " (obrigatória)" : ""}
      </legend>
      {question.help_text ? <p>{question.help_text}</p> : null}
      {isFreeText(question) ? (
        <p className="sensitive-data-hint">
          Não informe nome, documento, contato, credencial ou dado sensível.
        </p>
      ) : null}
      <QuestionControl {...props} />
    </fieldset>
  );
}
