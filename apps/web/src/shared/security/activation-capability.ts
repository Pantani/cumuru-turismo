import { createFragmentCapability } from "./fragment-capability";

/**
 * Single-use account activation capability (ADR-041), read from
 * `/ativacao#<token>`. Distinct purpose from the invite, therefore a distinct
 * header and a distinct in-memory slot: separate purposes are separate
 * cryptographic domains and must not be interchangeable in the client either.
 */
const capability = createFragmentCapability("/ativacao");

export const captureActivationCapability = capability.capture;
export const clearActivationCapability = capability.clear;
export const peekActivationCapability = capability.peek;
