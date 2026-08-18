export const appPaths = [
  "/",
  "/registro",
  // Short by design: the poster token rides in the fragment of `/i#<token>`
  // and the printed code is read by phone cameras (ADR-039).
  "/i",
  // Pública e sem token: é por aqui que uma hospedagem sem conta pede acesso.
  "/convite",
  "/ativacao",
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
