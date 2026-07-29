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

function evaluateCondition(
  condition: Condition,
  answers: Record<string, AnswerValue | undefined>,
) {
  const answer = answers[condition.question];
  switch (condition.operator) {
    case "answered":
      return answer !== undefined;
    case "equals":
      return sameValue(answer, condition.value);
    case "not_equals":
      return !sameValue(answer, condition.value);
    case "in":
      return Array.isArray(condition.value) && condition.value.some((value) => sameValue(answer, value));
    case "contains":
      return includesValue(answer, condition.value);
  }
}

function visible(
  question: Question,
  answers: Record<string, AnswerValue | undefined>,
) {
  const rule = question.visibility_rule;
  if (rule === null || rule === undefined) {
    return true;
  }
  if (rule.all !== undefined) {
    return rule.all.every((condition) => evaluateCondition(condition, answers));
  }
  return rule.any?.some((condition) => evaluateCondition(condition, answers)) ?? false;
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
