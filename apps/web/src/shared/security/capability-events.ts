export const CAPABILITY_CHANGE_EVENT = "cumuru:capability-change";

export function notifyCapabilityChange() {
  if (typeof window !== "undefined") {
    window.dispatchEvent(new Event(CAPABILITY_CHANGE_EVENT));
  }
}
