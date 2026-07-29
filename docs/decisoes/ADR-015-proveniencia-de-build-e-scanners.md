# ADR-015 — Proveniência de build, scanners e SBOM de imagem

**Status:** aceito.

## Contexto

A primeira versão da fundação passava uma data fixa diretamente pelo Compose,
aceitava `unknown` como metadado no Dockerfile e executava o Trivy por uma tag
mutável com o socket do Docker montado no container. O SBOM cobria o módulo Go
e o workspace npm, mas não as imagens OCI efetivamente construídas.

Isso prejudicava a associação entre metadados e artefatos e entregava ao
scanner controle desnecessário do daemon Docker.

## Decisão

`deploy/build-metadata.env` registra a versão da baseline e seu
`SOURCE_DATE_EPOCH`. O wrapper `deploy/scripts/with-build-metadata.sh` deriva:

- `CUMURU_BUILD_VERSION` da baseline versionada;
- `CUMURU_BUILD_REVISION` do commit exato quando há Git, ou de um hash
  determinístico do código, instruções e entradas de build quando
  `SCM=ABSENT`;
- `CUMURU_BUILD_TIME` do timestamp do commit ou do `SOURCE_DATE_EPOCH`
  versionado, sempre em RFC 3339 UTC.

O Compose exige os três valores; chamadas de build passam pelo wrapper. A CI
informa o SHA e o wrapper usa o timestamp desse commit. O Dockerfile rejeita
valor ausente, `unknown` ou timestamp fora do formato UTC esperado e grava os
três valores em labels OCI. Os gates conferem essas labels antes do scan ou
SBOM. A data representa o instante reprodutível da fonte, não o relógio da
máquina que recompila a imagem.

API e worker repetem essa validação no startup antes de abrir conexão ou
listener. Versão e revisão vazias, `unknown` ou fora do alfabeto permitido são
rejeitadas; `built_at` precisa usar exatamente `YYYY-MM-DDTHH:MM:SSZ`, sem
frações, offsets ou substituição por Unix epoch. Erros identificam somente o
campo técnico inválido, sem refletir o valor recebido.

Ferramentas executadas em containers na CI são fixadas pelo digest do
manifesto oficial. A atualização de versão exige:

1. baixar a tag oficial explicitamente;
2. obter `RepoDigests` com `docker image inspect`;
3. atualizar tag e digest juntos no Makefile;
4. executar os scans e revisar o resultado antes de aceitar a alteração.

Cada fluxo materializa explicitamente a referência `tag@sha256` com
`docker image pull` antes de inspecionar ou executar a ferramenta. O pull
verifica o conteúdo no registry; em seguida, o gate confirma que
`RepoDigests` contém o digest esperado. Cache ausente, registry indisponível ou
manifesto divergente falha fechado.

O scan de imagem não recebe o socket Docker. O host exporta cada imagem com
`docker image save`, e o Trivy lê o tar por `--input` em volume somente leitura.
A mesma imagem fixada produz SBOM CycloneDX para API/worker e web. Um manifesto
JSON registra versão, revisão, data reprodutível, referência, ID local e
`RepoDigest` de cada imagem; ausência de qualquer vínculo falha o gate.

## Consequências

- Recompilar a mesma fonte usa metadados estáveis e auditáveis.
- Alterações sem Git ainda mudam a revisão pelo hash do conteúdo.
- Um binário sem metadata válida falha antes de publicar estado operacional.
- O scanner não controla o daemon e sua identidade é imutável na execução.
- SBOMs de dependência e de imagem ficam vinculados aos artefatos verificados.
- `docker compose` para builds deve ser invocado pelos targets documentados do
  Makefile, que aplicam o wrapper fail-closed.
