const capabilityPattern = /^[A-Za-z0-9_-]{64,128}$/;
const invitePathPattern = /^\/convites\/([^/]+)\/?$/i;
const sensitiveInvitePathPattern = /^\/convites(?:\/|$)/i;
const queryKeys = new Set(["convite", "invite", "invite_token", "token"]);

let inviteCapability: string | null = null;

function capabilityFromPath(pathname: string) {
  const candidate = invitePathPattern.exec(pathname)?.[1];
  if (candidate === undefined) {
    return null;
  }
  try {
    const decoded = decodeURIComponent(candidate);
    return capabilityPattern.test(decoded) ? decoded : null;
  } catch {
    return null;
  }
}

function capabilityFromQuery(searchParams: URLSearchParams) {
  for (const [key, candidate] of searchParams.entries()) {
    if (queryKeys.has(key.toLowerCase()) && capabilityPattern.test(candidate)) {
      return candidate;
    }
  }
  return null;
}

function sensitiveQueryKeys(searchParams: URLSearchParams) {
  return [
    ...new Set(
      [...searchParams.keys()].filter((key) =>
        queryKeys.has(key.toLowerCase()),
      ),
    ),
  ];
}

function scrubbedPath(
  url: URL,
  cameFromInvitePath: boolean,
  sensitiveKeys: string[],
) {
  for (const key of sensitiveKeys) {
    url.searchParams.delete(key);
  }
  const pathname = cameFromInvitePath ? "/registro" : url.pathname;
  return `${pathname}${url.search}${url.hash}`;
}

export function captureInviteCapability(
  url: URL,
  replace: (path: string) => void,
) {
  const sensitiveKeys = sensitiveQueryKeys(url.searchParams);
  const cameFromInvitePath = sensitiveInvitePathPattern.test(url.pathname);
  if (!cameFromInvitePath && sensitiveKeys.length === 0) {
    return false;
  }
  const fromPath = capabilityFromPath(url.pathname);
  const capability = fromPath ?? capabilityFromQuery(url.searchParams);
  inviteCapability = capability;
  replace(scrubbedPath(url, cameFromInvitePath, sensitiveKeys));
  return true;
}

export function peekInviteCapability() {
  return inviteCapability;
}

export function clearInviteCapability() {
  inviteCapability = null;
}
