import { notifyCapabilityChange } from "./capability-events";

let surveyCapability: string | null = null;

const capabilityPattern = /^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/;

export function setSurveyCapability(value: string | null) {
  surveyCapability =
    value !== null && capabilityPattern.test(value) ? value : null;
  notifyCapabilityChange();
}

export function peekSurveyCapability() {
  return surveyCapability;
}

export function clearSurveyCapability() {
  surveyCapability = null;
  notifyCapabilityChange();
}
