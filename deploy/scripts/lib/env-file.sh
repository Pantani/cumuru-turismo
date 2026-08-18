# shellcheck shell=bash
#
# Leitura de credencial do mesmo arquivo que alimenta o Compose.
#
# O Compose semeia as contas a partir de `--env-file`. Um script que repita um
# padrão embutido passa a tentar entrar com credencial diferente da semeada
# assim que alguém edita o arquivo, e o sintoma é uma conta que parece não
# existir — falha real relatada como se fosse outra coisa.
#
# O recorte de aspas não é cosmético. O parser dotenv do Compose remove aspas
# que envolvem o valor inteiro, então `SENHA="x"` semeia `x`. Preservá-las aqui
# reintroduziria, por outra porta, exatamente a divergência que esta função
# existe para eliminar.

cumuru_env_file_value() {
  sed -n "s/^$2=//p" "$1" | tail -n 1 |
    sed -e 's/^"\(.*\)"$/\1/' -e "s/^'\(.*\)'$/\1/"
}

# Verificação de regressão do recorte de aspas. Roda em memória, sem depender do
# `.env` de quem executa: um arquivo temporário com as três formas prova que o
# valor entregue é o mesmo que o Compose semearia. Sem isto, a divergência de
# aspas só reapareceria como "senha inválida" numa conta que existe.
cumuru_assert_env_file_parsing() {
  local probe
  probe="$(mktemp)"
  printf '%s\n' \
    'CUMURU_PROBE_BARE=segredo-ficticio' \
    'CUMURU_PROBE_DOUBLE="segredo-ficticio"' \
    "CUMURU_PROBE_SINGLE='segredo-ficticio'" >"${probe}"
  local key value
  for key in CUMURU_PROBE_BARE CUMURU_PROBE_DOUBLE CUMURU_PROBE_SINGLE; do
    value="$(cumuru_env_file_value "${probe}" "${key}")"
    if test "${value}" != "segredo-ficticio"; then
      rm -f "${probe}"
      echo "env file parsing regressed for ${key}: ${value}" >&2
      return 1
    fi
  done
  rm -f "${probe}"
}
