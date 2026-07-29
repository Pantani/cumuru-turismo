const SHELL_CACHE_PREFIX = "cumuru-shell-";
const SHELL_CACHE = `${SHELL_CACHE_PREFIX}v1`;
const SHELL_ENTRY = "/";
const CAPABILITY_QUERY_KEYS = new Set([
  "token",
  "invite",
  "invite_token",
  "convite",
]);

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(SHELL_CACHE)
      .then((cache) => cache.add(new Request(SHELL_ENTRY, { cache: "reload" }))),
  );
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((names) =>
        Promise.all(
          names
            .filter((name) => name.startsWith(SHELL_CACHE_PREFIX) && name !== SHELL_CACHE)
            .map((name) => caches.delete(name)),
        ),
      )
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (event) => {
  const url = new URL(event.request.url);
  if (!isShellRequest(event.request, url)) {
    return;
  }
  if (event.request.mode === "navigate") {
    event.respondWith(navigateWithShellFallback(event.request));
    return;
  }
  event.respondWith(cacheStaticAsset(event.request));
});

function isShellRequest(request, url) {
  if (request.method !== "GET" || url.origin !== self.location.origin) {
    return false;
  }
  if (request.headers.has("authorization") || containsCapability(url)) {
    return false;
  }
  return (
    request.mode === "navigate" ||
    (url.pathname.startsWith("/assets/") && url.search === "")
  );
}

function containsCapability(url) {
  const path = url.pathname.toLowerCase();
  if (path.startsWith("/api/") || path.startsWith("/convites/") || path.startsWith("/invites/")) {
    return true;
  }
  for (const key of url.searchParams.keys()) {
    if (CAPABILITY_QUERY_KEYS.has(key.toLowerCase())) {
      return true;
    }
  }
  return false;
}

async function navigateWithShellFallback(request) {
  try {
    return await fetch(request);
  } catch {
    const cached = await caches.match(SHELL_ENTRY);
    return cached ?? Response.error();
  }
}

async function cacheStaticAsset(request) {
  const cached = await caches.match(request);
  if (cached) {
    return cached;
  }
  const response = await fetch(request);
  if (response.ok && response.type === "basic") {
    const cache = await caches.open(SHELL_CACHE);
    await cache.put(request, response.clone());
  }
  return response;
}
