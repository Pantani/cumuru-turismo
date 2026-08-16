import type { components } from "../../generated/schema";

type Schemas = components["schemas"];
export type AnswerValue = Schemas["AnswerValue"];
export type AnswerState = Record<string, AnswerValue | undefined>;
type Question = Schemas["PublicQuestion"];
type Condition = Schemas["Condition"];

function stableAnswers(
  questions: Question[],
  answers: AnswerState,
): Record<string, AnswerValue | undefined> {
  return Object.fromEntries(
    questions.map((question) => [
      question.stable_key,
      answers[question.id],
    ]),
  );
}

function sameValue(left: AnswerValue | undefined, right: unknown) {
  return JSON.stringify(left) === JSON.stringify(right);
}

function includesValue(answer: AnswerValue | undefined, value: unknown) {
  return Array.isArray(answer) && answer.some((item) => sameValue(item, value));
}

type Answers = Record<string, AnswerValue | undefined>;

/**
 * One entry per operator of the allowed DSL. A table keeps every operator
 * visible side by side and makes an unhandled operator a type error rather than
 * a silently falsy branch.
 */
const conditionOperators: Record<
  Condition["operator"],
  (answer: AnswerValue | undefined, value: unknown) => boolean
> = {
  answered: (answer) => answer !== undefined,
  equals: (answer, value) => sameValue(answer, value),
  not_equals: (answer, value) => !sameValue(answer, value),
  in: (answer, value) =>
    Array.isArray(value) && value.some((item) => sameValue(answer, item)),
  contains: (answer, value) => includesValue(answer, value),
};

function evaluateCondition(condition: Condition, answers: Answers) {
  return conditionOperators[condition.operator](
    answers[condition.question],
    condition.value,
  );
}

function matchesRule(
  rule: NonNullable<Question["visibility_rule"]>,
  answers: Answers,
) {
  if (rule.all !== undefined) {
    return rule.all.every((condition) => evaluateCondition(condition, answers));
  }
  return (
    rule.any?.some((condition) => evaluateCondition(condition, answers)) ?? false
  );
}

function visible(question: Question, answers: Answers) {
  const rule = question.visibility_rule;
  if (rule === null || rule === undefined) {
    return true;
  }
  return matchesRule(rule, answers);
}

export function visibleQuestions(questions: Question[], answers: AnswerState) {
  const byKey = stableAnswers(questions, answers);
  return questions.filter((question) => visible(question, byKey));
}

export function applicableQuestions(
  questions: Question[],
  consents: Record<string, boolean>,
) {
  return questions.filter(
    (question) => consents[question.purpose_code] === true,
  );
}

export function answerInputs(questions: Question[], answers: AnswerState) {
  return visibleQuestions(questions, answers).flatMap((question) => {
    const value = answers[question.id];
    return value === undefined ? [] : [{ question_id: question.id, value }];
  });
}

export function missingRequired(questions: Question[], answers: AnswerState) {
  return visibleQuestions(questions, answers).filter(
    (question) => question.required && answers[question.id] === undefined,
  );
}

export function consentInputs(
  requirements: Schemas["ConsentRequirementDefinition"][],
  consents: Record<string, boolean>,
) {
  return requirements.map((requirement) => ({
    purpose_code: requirement.purpose_code,
    notice_version: requirement.notice_version,
    granted: consents[requirement.purpose_code] === true,
  }));
}

export function missingRequiredConsent(
  requirements: Schemas["ConsentRequirementDefinition"][],
  consents: Record<string, boolean>,
) {
  return requirements.some(
    (requirement) =>
      requirement.required_for_answers &&
      consents[requirement.purpose_code] !== true,
  );
}
