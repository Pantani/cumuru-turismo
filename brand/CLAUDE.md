# Observatório Turístico de Cumuruxatiba

O brandkit do projeto é `Brandkit Observatorio.dc.html`. Ele é obrigatório: toda peça nova
(página, painel, PDF, apresentação, post) segue essas regras. Leia o arquivo antes de desenhar.

## Cores (não introduzir uma sexta)
- Oceano `#04171a` — fundo padrão, texto sobre coral/areia
- Oceano elevado `#0a2529` — cartões, campos, linhas de tabela (cabeçalho de tabela `#0e2c31`)
- Coral `#ff5a36` — ação, ênfase, dado previsto, numeração de seção
- Turquesa `#2fc2af` — dado observado, rótulos, confirmação
- Areia `#f2e9dc` — texto sobre escuro e fundo de seção clara (alternativa escura `#e8dbc8`)
- Texto sobre escuro: `#f2e9dc` / `#b7c9c6` / `#7f9995`. Sobre claro: `#04171a` / `#3d5a5c`
- Proibido: gradiente decorativo de fundo, azul de link padrão, cinza neutro puro,
  verde/vermelho de status genéricos

## Tipografia
- **Bricolage Grotesque 800** — só títulos e números-herói; line-height .86–.95;
  letter-spacing −.04 a −.05em; nunca em corpo nem abaixo de 24px
- **Geist 300–700** — corpo e interface; 15–19px; line-height 1.55–1.65; medida máx. 56ch;
  parágrafos com `text-wrap: pretty`
- **Geist Mono 600** — eyebrows, rótulos, navegação, chips e números tabulares;
  caixa alta 10.5–12.5px, letter-spacing .1–.18em
- Escala: hero `clamp(52px,8.4vw,132px)` · seção `clamp(40px,6.6vw,100px)` ·
  subtítulo `clamp(28px,3.4vw,48px)` · número-herói `clamp(56px,12vw,168px)`

## Botões e campos
- Botão sempre cápsula (`border-radius:999px`), peso 650; um primário por tela
- Dois tamanhos: padrão `16px 26px` / 15px (altura mín. 48px) e compacto `11px 18px` / 13px
  (altura mín. 38px), só na barra fixa do topo
- Única exceção à cápsula: seletor de idioma — aba mono 11px, `padding:4px 2px`,
  régua inferior de 2px coral no ativo. Não usar fora do cabeçalho
- Link de navegação: mono 11.5px caixa alta peso 500, sem fundo, hover coral
- Escuro: primário coral/texto oceano (hover areia) · secundário contorno areia 34% (hover turquesa)
- Coral: primário oceano/texto areia · secundário contorno oceano 45%
- Areia: primário oceano/texto areia (hover coral) · secundário contorno oceano 35%
- Campo: raio 12px, altura mín. 50px, contorno areia 24% sobre escuro

## Layout
- Contêiner 1320px, padding lateral 28px, respiro de seção 96px
- Alternância de fundo: oceano → areia → areia escura → coral → oceano;
  no máximo uma seção coral chapada por página
- Toda seção abre com eyebrow: número coral + régua 24px + rótulo mono caixa alta
- Métrica secundária em linha de régua (borda 1px a 18%), não em cartão
- Grades de régua: padding uniforme + `gap: 0 32px` — nunca padding assimétrico por coluna
- Raios: cartão 20 · painel 22 · campo 12 · imagem 18 · botão/chip 999. Sem sombra colorida
- Faixa de dados (assinatura): coral sangrado, réguas 2px oceano, mono 12.5px caixa alta,
  separador `◆` a 50%, lista duplicada com `aria-hidden` em `width:max-content`,
  `animation: cxmarquee 34s linear infinite` + `@keyframes cxmarquee` no helmet.
  Uma por página, logo abaixo do hero
- Imagem: foto real ou placeholder arrastável. Sem ilustração vetorial, ícone decorativo ou emoji

## Voz e dado
- Frase curta e concreta; sem jargão de gestão; sem tom de fiscalização
- Sempre nomear a unidade (pessoas-dia); previsão nunca sem faixa provável
- Dizer o limite do dado (cobertura parcial, não censo)
- Rótulo "dados fictícios de protótipo" visível em toda peça de demonstração
- Nunca expor número por estabelecimento em peça pública
- Idiomas do site: PT (padrão), EN, ES
