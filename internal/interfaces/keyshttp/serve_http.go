package keyshttp

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
)

func (service *KeyStoreServiceHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		res.Header().Set("Allow", "GET, HEAD")
		http.Error(res, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := req.Context()
	jwks, err := service.getKeySet(ctx)
	if err != nil {
		slog.Error("getting key set", "error", err)
		http.Error(res, "internal server error", http.StatusInternalServerError)
		return
	}

	respJSON, err := json.Marshal(jwks)
	if err != nil {
		slog.Error("marshaling key set", "error", err)
		http.Error(res, "internal server error", http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Type", "application/jwk-set+json")
	res.Header().Set("Cache-Control", "no-store")
	res.Header().Set("Content-Length", strconv.Itoa(len(respJSON)))
	res.WriteHeader(http.StatusOK)

	if req.Method == http.MethodHead {
		return
	}

	_, err = res.Write(respJSON)
	if err != nil {
		// Can't do much at this point, just log the error
		slog.Error("writing response", "error", err)
		return
	}
}
