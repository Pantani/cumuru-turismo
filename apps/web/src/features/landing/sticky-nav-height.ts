import { useLayoutEffect, useRef, type RefObject } from "react";

/** Nome da variável lida pelo `scroll-margin-top` das âncoras da capa. */
const NAV_HEIGHT_VAR = "--lp-nav-h";

/**
 * Publica no CSS a altura real da barra de seções.
 *
 * A âncora precisa parar a altura das barras coladas no topo antes do começo
 * da seção, e a barra de seções não tem altura fixa: ela quebra em duas linhas
 * quando o idioma alonga os rótulos (espanhol já quebra a 810px) e quebra de
 * novo abaixo de 50rem, quando o botão de cadastro desce para a própria linha.
 * Uma constante no CSS acertaria um desses casos e erraria os outros — ou
 * cobrindo a primeira linha da seção, ou abrindo um vão antes dela —, então a
 * altura é medida no navegador e devolvida ao CSS como variável.
 */
export function useStickyNavHeight(): RefObject<HTMLElement | null> {
  const ref = useRef<HTMLElement | null>(null);

  useLayoutEffect(() => {
    const nav = ref.current;
    if (!nav) {
      throw new Error("useStickyNavHeight exige a barra de seções montada.");
    }
    const root = document.documentElement;
    const observer = new ResizeObserver(() => {
      root.style.setProperty(
        NAV_HEIGHT_VAR,
        `${nav.getBoundingClientRect().height}px`,
      );
    });
    observer.observe(nav);
    return () => {
      observer.disconnect();
      root.style.removeProperty(NAV_HEIGHT_VAR);
    };
  }, []);

  return ref;
}
