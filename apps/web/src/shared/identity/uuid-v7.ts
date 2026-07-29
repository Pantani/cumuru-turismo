export const uuidV7Pattern =
  /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

function writeTimestamp(bytes: Uint8Array, timestamp: number) {
  let remaining = Math.trunc(timestamp);
  for (let index = 5; index >= 0; index -= 1) {
    bytes[index] = remaining % 256;
    remaining = Math.floor(remaining / 256);
  }
}

function hexadecimal(bytes: Uint8Array) {
  return Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join(
    "",
  );
}

export function createUuidV7(
  now = Date.now(),
  randomness = crypto.getRandomValues(new Uint8Array(10)),
) {
  const bytes = new Uint8Array(16);
  writeTimestamp(bytes, now);
  bytes.set(randomness.slice(0, 10), 6);
  bytes[6] = (bytes[6]! & 0x0f) | 0x70;
  bytes[8] = (bytes[8]! & 0x3f) | 0x80;
  const value = hexadecimal(bytes);
  return `${value.slice(0, 8)}-${value.slice(8, 12)}-${value.slice(12, 16)}-${value.slice(16, 20)}-${value.slice(20)}`;
}
