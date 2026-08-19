package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"regexp"
)

// Peças comuns às rotas abertas que servem documento inteiro e cacheável: os
// painéis públicos de analytics e a lista pública de hospedagens. Ficam juntas
// porque as três regras — seletor exausto, ETag forte e forma da falha — são a
// mesma promessa ao cache intermediário, e separá-las por feature deixaria as
// cópias livres para divergir.
var strongDocumentETag = regexp.MustCompile(`^"sha256-[0-9a-f]{64}"$`)

// O ETag cobre operação e seletor além do corpo, para que dois documentos de
// mesmo conteúdo e origens diferentes nunca colidam no cache.
func publicDocumentETag(operation, selector string, payload []byte) string {
	source := make([]byte, 0, len(operation)+len(selector)+len(payload)+2)
	source = append(source, operation...)
	source = append(source, '\n')
	source = append(source, selector...)
	source = append(source, '\n')
	source = append(source, payload...)
	sum := sha256.Sum256(source)
	return `"sha256-` + hex.EncodeToString(sum[:]) + `"`
}

func validNoQuery(request *http.Request) bool {
	return len(request.URL.Query()) == 0
}

func validIfNoneMatch(value string) bool {
	return value == "" || strongDocumentETag.MatchString(value)
}

func writePublicBadRequest(writer http.ResponseWriter, request *http.Request) {
	writeProblem(writer, request, http.StatusBadRequest, "invalid-request", "Requisição inválida")
}

func writePublicUnavailable(writer http.ResponseWriter, request *http.Request) {
	writeProblem(
		writer, request, http.StatusServiceUnavailable,
		"dependency-unavailable", "Serviço temporariamente indisponível",
	)
}
