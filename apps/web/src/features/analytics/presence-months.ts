/**
 * Aritmética de mês civil para a consulta histórica. Tudo aqui trabalha sobre
 * a string `YYYY-MM` do contrato e ancora as datas em UTC: usar o fuso do
 * navegador faria o mesmo mês render um limite diferente para cada leitor.
 */

export interface MonthRange {
  max: string;
  min: string;
}

const MONTH_PATTERN = /^\d{4}-(?:0[1-9]|1[0-2])$/u;

export function isCivilMonth(value: string): boolean {
  return MONTH_PATTERN.test(value);
}

function monthIndex(month: string): number {
  const [year, index] = month.split("-");
  return Number(year) * 12 + Number(index) - 1;
}

function monthFrom(index: number): string {
  const year = Math.floor(index / 12);
  const month = (index % 12) + 1;
  return `${String(year).padStart(4, "0")}-${String(month).padStart(2, "0")}`;
}

function monthOf(date: string): string {
  return date.slice(0, 7);
}

/**
 * Os meses que a release cobre. O primeiro dia publicado quase nunca cai no dia
 * primeiro, então o mês mais antigo é oferecido parcial — e o painel diz
 * quantos dias dele chegaram em vez de esconder o recorte.
 */
export function monthRange(asOf: string, historyDays: number): MonthRange {
  const last = new Date(`${asOf}T00:00:00Z`);
  const first = new Date(last);
  first.setUTCDate(first.getUTCDate() - (historyDays - 1));
  return {
    max: monthOf(last.toISOString().slice(0, 10)),
    min: monthOf(first.toISOString().slice(0, 10)),
  };
}

export function shiftMonth(month: string, step: number): string {
  return monthFrom(monthIndex(month) + step);
}

export function clampMonth(month: string, range: MonthRange): string {
  if (monthIndex(month) < monthIndex(range.min)) {
    return range.min;
  }
  return monthIndex(month) > monthIndex(range.max) ? range.max : month;
}

export function monthWithin(month: string, range: MonthRange): boolean {
  return isCivilMonth(month) && clampMonth(month, range) === month;
}
