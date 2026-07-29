export const appPaths = [
  "/",
  "/registro",
  "/pesquisa",
  "/acesso",
  "/questionarios",
  "/qualidade",
] as const;

export type AppPath = (typeof appPaths)[number];

export function normalizePathname(pathname: string): string {
  if (pathname === "/") {
    return pathname;
  }

  return pathname.replace(/\/+$/, "") || "/";
}
