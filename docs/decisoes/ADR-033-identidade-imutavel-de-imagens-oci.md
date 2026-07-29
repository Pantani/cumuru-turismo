# ADR-033 — Identidade imutável de imagens OCI

**Status:** aceito.

## Contexto

Os scanners da fundação já eram fixados por `tag@sha256`, mas os Dockerfiles,
o Compose local e os serviços de infraestrutura/observabilidade ainda
referenciavam imagens de terceiros apenas por tag. O deploy Ansible também
formava as imagens da aplicação como `repositório:tag`, embora o publicador
consultasse e exibisse o digest retornado pelo ECR.

Além disso, o ADR-015 exigia `RepoDigest` para qualquer SBOM de imagem. Uma
imagem recém-construída e ainda não publicada possui ID de conteúdo imutável,
mas não possui `RepoDigests`; tratar os dois conceitos como equivalentes fazia
o manifesto local aceitar `null` sem declarar o escopo ou, alternativamente,
tornava a geração pré-publicação impossível.

## Decisão

Toda imagem de terceiro usada em build, teste, validação, observabilidade ou
runtime fica fixada como `tag@sha256:<digest-do-manifesto>`. A tag continua
visível para manutenção humana, enquanto o digest determina o conteúdo
executado. Atualizações exigem consultar o manifesto no registry, alterar tag e
digest juntos e repetir build, smoke e scans.

Imagens próprias em produção são referenciadas como
`repositório:tag@sha256:<digest-do-ECR>`. O tag identifica a release, mas o
Ansible falha fechado se os digests da API/worker e do web estiverem ausentes
ou fora do formato SHA-256. O mesmo digest da API é reutilizado pelo worker.

O manifesto de SBOM distingue dois escopos:

- `local-build`: antes da publicação, exige o ID local `sha256` e permite
  `repo_digest: null`;
- `registry`: quando a referência fornecida já contém `@sha256`, exige que
  Docker exponha um `RepoDigest` correspondente.

Esta decisão substitui apenas a frase do ADR-015 que exigia `RepoDigest` para
uma imagem estritamente local. Identidade de conteúdo, labels de build, hash do
SBOM e referência continuam obrigatórios nos dois escopos. Deploy e evidência
de release continuam exigindo o digest do registry.

## Consequências

- mover uma tag no registry não altera silenciosamente builds ou serviços;
- o deploy usa exatamente o conteúdo publicado e verificado no ECR;
- SBOM pré-publicação permanece possível sem fingir que existe identidade de
  distribuição;
- atualizar imagens exige patch explícito e nova validação;
- indisponibilidade ou divergência do manifesto impede a atualização e não é
  convertida em PASS.
