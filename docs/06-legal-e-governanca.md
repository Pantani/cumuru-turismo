# Legal e governança

> Este documento é um roteiro de engenharia e governança, não um parecer
> jurídico. A Procuradoria do Município de Prado e o encarregado de dados devem
> validar a base legal, o instrumento normativo, os campos e os prazos.

## Competência territorial

Cumuruxatiba é distrito do Município de Prado. A Prefeitura de Prado informa que
o município é formado pelos distritos de Prado e Cumuruxatiba. A política e o
eventual dever municipal devem, portanto, ser instituídos pelo Município de
Prado, não por uma entidade informal denominada “cidade de Cumuruxatiba”.

Fonte: [Prefeitura de Prado — História](https://prado.ba.gov.br/historia/).

## Regra federal existente

A Lei Geral do Turismo determina que meios de hospedagem forneçam ao Ministério
do Turismo informações sobre perfil e quantitativo de hóspedes, ocupação e
permanência, observando privacidade e intimidade. A FNRH Digital é o instrumento
federal atual.

Fontes:

- [Lei nº 11.771/2008, especialmente arts. 23 e 26](https://www.planalto.gov.br/ccivil_03/_ato2007-2010/2008/lei/l11771.htm)
- [Portaria MTur nº 41/2025](https://www.gov.br/turismo/pt-br/publicacoes/atos-normativos-2/2025/portaria-mtur-no-41-de-14-de-novembro-de-2025)
- [Portaria MTur nº 4/2026](https://www.gov.br/turismo/pt-br/centrais-de-conteudo-/publicacoes/atos-normativos-2/2026/portaria-mtur-no-4-de-18-de-fevereiro-de-2026)

A Portaria nº 41/2025 também veda, no âmbito da plataforma federal, divulgação
pública de dados individualizados de pessoa ou estabelecimento, inclusive
ocupação e número de hóspedes.

## Consequência para o projeto

O Observatório não deve duplicar cegamente a FNRH. Existem três caminhos:

1. **PMS integrador:** o Observatório recebe o registro da hospedagem e o envia à
   FNRH usando a credencial daquele estabelecimento.
2. **Pesquisa complementar:** a FNRH continua sendo preenchida diretamente e o
   Observatório coleta somente dados locais adicionais, preferencialmente
   opcionais e sem identidade.
3. **Acordo institucional:** o Município busca cooperação e acesso legítimo a
   estatísticas agregadas, sem receber dados pessoais desnecessários.

O caminho pode variar por tipo de hospedagem. A API da FNRH existe para
integração de PMS e cada meio de hospedagem gera sua própria chave.

Fonte: [Ministério do Turismo — integração PMS/FNRH](https://www.gov.br/turismo/pt-br/acesso-a-informacao/acoes-e-programas/programas-projetos-acoes-obras-e-atividades/ficha-nacional-de-registro-de-hospedes/modulo-meio-de-hospedagem/meios-de-hospedagem-com-pms).

## Gate para obrigatoriedade municipal

Não colocar em produção um campo obrigatório municipal até haver:

- identificação da competência legal municipal;
- finalidade pública específica e documentada;
- parecer jurídico sobre necessidade e proporcionalidade;
- análise de conflito ou duplicação com a obrigação federal;
- definição de quem é controlador, operador e suboperadores;
- instrumento normativo aprovado e publicado;
- aviso de privacidade e canal do titular;
- Relatório de Impacto à Proteção de Dados;
- política de retenção;
- matriz de acesso;
- plano de segurança e incidentes;
- tratamento específico de crianças e adolescentes;
- desenho de fiscalização, recurso e contingência sem exclusão digital.

Sem esse conjunto, o sistema deve operar como piloto voluntário e pesquisa
opcional.

## Base legal

Não usar “consentimento” como justificativa genérica para tudo.

- Tratamentos necessários à execução de política pública devem indicar a
  competência, norma, finalidade e necessidade.
- Preferências sem necessidade pública demonstrada devem ser opcionais.
- Marketing ou contato promocional exige escolha separada e não pré-marcada.
- Recusa a pergunta opcional não impede hospedagem, check-in ou serviço público.
- Mudança de finalidade exige nova análise e transparência.

Referências:

- [LGPD — Lei nº 13.709/2018](https://www.planalto.gov.br/ccivil_03/_ato2015-2018/2018/lei/l13709.htm)
- [ANPD — Guia para tratamento pelo Poder Público](https://www.gov.br/anpd/pt-br/centrais-de-conteudo/materiais-educativos-e-publicacoes/guia_tratamento_de_dados_pessoais_pelo_poder_publico___defeso_eleitoral.pdf)
- [ANPD — estudo técnico sobre anonimização](https://www.gov.br/anpd/pt-br/centrais-de-conteudo/documentos-tecnicos-orientativos/estudo_tecnico_sobre_anonimizacao_de_dados_na_lgpd___analise_juridica.pdf)

## Crianças e adolescentes

- responsável cadastra integrantes menores;
- evitar contato próprio e texto livre;
- não coletar preferências individualizadas de criança no MVP;
- usar apenas faixa etária necessária à estatística;
- aviso em linguagem clara;
- jurídico e encarregado definem hipótese e salvaguardas;
- nunca usar dados de menores para marketing ou perfil comercial.

## Governança proposta

| Papel | Responsável sugerido |
|---|---|
| Controlador | Município de Prado, formalmente designado |
| Dono do produto | Secretaria Municipal competente pelo turismo |
| Encarregado | encarregado de dados do Município |
| Operador técnico | fornecedor contratado ou equipe municipal |
| Comitê de dados | turismo, jurídico, TI, encarregado e sociedade |
| Publicação estatística | gestor designado + revisão de privacidade |

A Prefeitura informa que sua Secretaria de Turismo possui competência para
elaborar projetos e atividades ligadas ao turismo e articular o setor.
Fonte: [Prefeitura de Prado — Turismo](https://prado.ba.gov.br/turismo/).

## Cadastro de casas e aluguel por temporada

Não presumir que toda locação residencial é automaticamente “meio de hospedagem”
para a FNRH. Natureza da operação, serviços, cobrança, habitualidade e regras
locais precisam ser avaliadas. O produto deve suportar categorias distintas:

- meio de hospedagem com Cadastur;
- imóvel de temporada;
- hospedagem familiar;
- camping ou categoria em regularização.

Isso evita atribuir obrigação federal indevida. A eventual obrigação municipal
para anfitriões precisa de base própria e análise de proporcionalidade.

## Documentos antes do piloto

- termo de abertura do projeto;
- inventário de dados e operações;
- RIPD;
- parecer jurídico;
- minuta do instrumento normativo, se aplicável;
- contrato e aditivo de proteção de dados com operador;
- aviso de privacidade curto e completo;
- política de retenção;
- matriz de responsabilidades;
- plano de resposta a incidentes;
- metodologia pública dos indicadores.

## Registro de decisões

Toda nova pergunta deve ter:

```text
finalidade
classificação
base legal ou caráter opcional
público-alvo
prazo de retenção
quem acessa
se gera agregado público
limiar de publicação
aprovação do encarregado quando necessária
```

## Fontes verificadas

Consulta realizada em 27 de julho de 2026. Como leis, portarias e APIs podem
mudar, a equipe deve repetir a verificação antes do piloto e de cada integração.
