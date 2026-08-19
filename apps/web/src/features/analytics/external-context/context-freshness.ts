/**
 * Frescor do card publicado.
 *
 * ADR-045 §7: servir o último valor conhecido é `published` com `retrieved_at`
 * antigo, e o leitor vê a defasagem no corpo — nunca zero, nunca valor
 * inventado, nunca último valor conhecido servido em silêncio. Este módulo
 * decide apenas se aquele "antigo" já é visível ao leitor; não altera, não
 * corrige e não interpola nenhum número.
 *
 * O limiar sai da própria fonte: `declared_lag_seconds` é a defasagem que ela
 * declara, e enquanto a coleta couber dentro dela não há atraso nosso a
 * relatar. A folga fixa acomoda um ciclo do worker, cuja agenda é ancorada no
 * calendário civil e não no relógio da requisição.
 */

/** Uma hora: um ciclo de ingestão, não um número negociável por fonte. */
export const STALE_GRACE_SECONDS = 3600;

export type Freshness = "fresh" | "stale";

const MILLISECONDS = 1000;

/**
 * Idade da coleta em segundos. Instante ilegível devolve `null` em vez de
 * `NaN`: um card cuja idade não se pode afirmar não é um card fresco.
 */
export function collectionAgeSeconds(
  retrievedAt: string,
  now: number,
): number | null {
  const retrieved = Date.parse(retrievedAt);
  if (!Number.isFinite(retrieved)) {
    return null;
  }
  return (now - retrieved) / MILLISECONDS;
}

export function freshnessOf(
  retrievedAt: string,
  declaredLagSeconds: number,
  now: number,
): Freshness {
  const age = collectionAgeSeconds(retrievedAt, now);
  if (age === null) {
    return "stale";
  }
  return age > declaredLagSeconds + STALE_GRACE_SECONDS ? "stale" : "fresh";
}
