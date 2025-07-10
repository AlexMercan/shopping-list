package api

import (
	"log"
	"net/http"
)

const apiKeyHeader = "X-Api-Key"

type AuthService struct {
	apiKey string
}

func NewAuthService(apiKey string) AuthService {
	return AuthService{apiKey: apiKey}
}

func (authService AuthService) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get(apiKeyHeader)

		if apiKey != authService.apiKey {
			w.WriteHeader(http.StatusUnauthorized)
			_, err := w.Write([]byte("UNAUTHORIZED"))
			if err != nil {
				log.Printf("%v", err)
			}
			
			return
		}

		next.ServeHTTP(w, r)
	})
}
