export function registerServiceWorker() {
  if (!("serviceWorker" in navigator)) {
    return;
  }
  window.addEventListener(
    "load",
    () => {
      void navigator.serviceWorker.register("/service-worker.js", {
        scope: "/",
      });
    },
    { once: true },
  );
}
