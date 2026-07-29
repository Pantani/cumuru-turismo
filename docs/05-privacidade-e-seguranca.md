# Privacidade e segurança

## Modelo de ameaça

Riscos prioritários:

- comerciante ou analista reidentificar visitante por data, origem e preferência;
- operador consultar estadia de outra hospedagem;
- invasor reutilizar convite ou registrar pessoas falsas;
- administrador criar pergunta excessiva ou sensível;
- credencial FNRH vazar em log, banco ou suporte;
- filtros sucessivos revelarem grupos pequenos por diferença;
- dispositivo compartilhado manter rascunho pessoal;
- duplicação causada por rede intermitente;
- planilha exportada circular sem controle;
- conta administrativa comprometida.

## Separação de dados

| Classe | Exemplos | Tratamento |
|---|---|---|
| Pública | métrica arredondada e metodologia | CDN e leitura pública |
| Operacional | status, capacidade e cobertura | acesso por função |
| Pessoal | nome, contato, origem detalhada | cifrado e minimizado |
| Sensível | saúde, religião, biometria | proibido no MVP |
| Segredo | token OIDC, chave FNRH | cofre/KMS, nunca logar |

Texto livre é tratado como pessoal por padrão, pois o respondente pode inserir
informação identificável ou sensível.

## Criptografia

- TLS 1.3 externamente; TLS interno quando a plataforma cruzar redes.
- Disco e backups cifrados pelo provedor.
- Criptografia em envelope para colunas de identidade.
- Chave mestra no KMS; chave de dados por ambiente e versão.
- Credenciais FNRH com chave distinta dos dados pessoais.
- HMAC de documento com chave de busca separada da chave de cifra.
- Rotação sem reescrever toda a aplicação.

## Autenticação e autorização

- OIDC com PKCE para usuários.
- MFA obrigatório para gestores, privacidade e responsáveis de hospedagem.
- Sessões curtas e refresh token rotativo.
- Convites com entropia mínima de 128 bits, uso limitado e expiração.
- RBAC mais verificação de organização/acomodação em toda query.
- Elevação temporária e justificada para busca de identidade.
- Reautenticação para exportar, trocar credencial ou concluir direito do titular.

## Dashboard seguro

O papel público só possui `SELECT` nas tabelas liberadas do esquema
`public_data`.
Mesmo que a API pública seja comprometida, ela não consegue consultar identidade,
respostas brutas ou estadias.

Proteções contra reidentificação:

- dimensões e combinações em lista fechada;
- supressão de célula pequena;
- supressão complementar para impedir cálculo por subtração;
- generalização de origem e idade;
- arredondamento estocástico estável ou publicação por faixas;
- previsão futura em intervalos;
- mínimo de estabelecimentos participantes;
- atraso para dimensões raras;
- sem estabelecimento, endereço ou texto livre;
- revisão antes de liberar uma nova dimensão.

## Logs e telemetria

Permitir:

- request ID;
- rota normalizada;
- status e duração;
- ator pseudônimo;
- organização;
- tipo de erro;
- quantidade, nunca conteúdo.

Proibir:

- corpo de requisição/resposta;
- query string com token;
- nome, documento, e-mail ou telefone;
- texto livre;
- credencial FNRH;
- cabeçalhos `Authorization` e `Cookie`.

## Segurança de aplicação

- CSP restrita, sem scripts inline não autorizados.
- Cookies `Secure`, `HttpOnly` e `SameSite`.
- Proteção CSRF quando cookies autenticarem.
- CORS em lista explícita.
- Limite de tamanho por campo e corpo.
- Queries parametrizadas.
- Sanitização contextual ao exibir texto administrativo.
- Dependências travadas e SBOM por release.
- SAST, análise de dependências, `govulncheck` e scanner de contêiner.
- Imagens sem usuário root e filesystem somente leitura quando possível.

## Backups

- backup contínuo com point-in-time recovery;
- cópia separada e cifrada;
- teste de restauração trimestral no piloto e mensal após criticidade alta;
- restauração respeita retenção e pedidos de eliminação por meio de lista de
  reaplicação pós-restore;
- acesso a backup é mais restrito que acesso à produção.

## Incidente

Runbook mínimo:

1. conter acesso e preservar evidências;
2. identificar classes e titulares afetados;
3. trocar chaves e revogar sessões quando necessário;
4. avaliar risco ou dano relevante;
5. acionar controlador, encarregado e jurídico;
6. cumprir comunicação à ANPD e aos titulares quando aplicável;
7. corrigir, validar e registrar lições;
8. não divulgar detalhes pessoais no relatório público.

## Gate de segurança

Antes do piloto:

- threat model revisado;
- teste de isolamento entre hospedagens;
- teste de repetição e concorrência;
- teste de supressão e differencing;
- restauração comprovada;
- segredo FNRH ausente em logs e dumps;
- dependências sem vulnerabilidade crítica conhecida;
- teste de autorização em todos os endpoints;
- política e canal de incidente definidos.
