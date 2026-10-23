package handlers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"

	"github.com/stpotter16/wishlist/internal/parse"
	"github.com/stpotter16/wishlist/internal/store"
	"github.com/stpotter16/wishlist/internal/types"
)

func adminGet(s store.Store) http.HandlerFunc {
	t := template.Must(
		template.New("base.html").
			ParseFS(
				templateFS,
				"templates/layouts/base.html",
				"templates/layouts/app.html",
				"templates/pages/admin.html",
			))
	return func(w http.ResponseWriter, r *http.Request) {
		nonce, err := extractCspNonceOnly(r)
		if err != nil {
			log.Printf("Could not extract csp nonce from ctx: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		families, err := s.ListFamilies(r.Context())
		if err != nil {
			log.Printf("adminGet: failed to list families: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		props := struct {
			viewProps
			Families []types.FamilySummary
		}{
			viewProps: viewProps{CspNonce: nonce, ActivePage: "dashboard"},
			Families:  families,
		}

		if err := t.Execute(w, props); err != nil {
			log.Printf("Could not create admin page: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
		}
	}
}

func adminNewFamilyGet() http.HandlerFunc {
	t := template.Must(
		template.New("base.html").
			ParseFS(
				templateFS,
				"templates/layouts/base.html",
				"templates/layouts/app.html",
				"templates/pages/new-family.html",
			))
	return func(w http.ResponseWriter, r *http.Request) {
		nonce, err := extractCspNonceOnly(r)
		if err != nil {
			log.Printf("Could not extract csp nonce from ctx: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		props := viewProps{CspNonce: nonce, ActivePage: "dashboard"}

		if err := t.Execute(w, props); err != nil {
			log.Printf("Could not create new family page: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
		}
	}
}

func adminFamiliesPost(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		family, err := parse.ParseFamilyPost(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		id, err := s.CreateFamily(r.Context(), family)
		if err != nil {
			log.Printf("adminFamiliesPost: failed to create family: %v", err)
			http.Error(w, "Server issue - try again later", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(struct {
			ID int `json:"id"`
		}{ID: id}); err != nil {
			log.Printf("adminFamiliesPost: failed to write response: %v", err)
		}
	}
}
