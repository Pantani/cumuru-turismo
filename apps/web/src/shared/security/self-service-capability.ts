import { createFragmentCapability } from "./fragment-capability";

/** Poster token of the reusable accommodation invite (ADR-039), read from `/i#<token>`. */
const capability = createFragmentCapability("/i");

export const captureSelfServiceCapability = capability.capture;
export const clearSelfServiceCapability = capability.clear;
export const peekSelfServiceCapability = capability.peek;
