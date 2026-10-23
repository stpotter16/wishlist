package handlers

import (
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/stpotter16/wishlist/internal/store"
	"github.com/stpotter16/wishlist/internal/types"
)

func familiesGet(s store.Store) http.HandlerFunc {
	t := template.Must(
		template.New("base.html").
			ParseFS(
				templateFS,
				"templates/layouts/base.html",
				"templates/pages/families.html",
			))
	return func(w http.ResponseWriter, r *http.Request) {
		nonce, err := extractCspNonceOnly(r)
		if err != nil {
			log.Printf("Could not extract csp nonce from ctx: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		families, err := s.ListUnclaimedFamilies(r.Context())
		if err != nil {
			log.Printf("familiesGet: failed to list unclaimed families: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		props := struct {
			viewProps
			Families []types.FamilySummary
		}{
			viewProps: viewProps{CspNonce: nonce},
			Families:  families,
		}

		if err := t.Execute(w, props); err != nil {
			log.Printf("Could not create families page: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
		}
	}
}

func familyGet(s store.Store) http.HandlerFunc {
	t := template.Must(
		template.New("base.html").
			ParseFS(
				templateFS,
				"templates/layouts/base.html",
				"templates/partials/family-members.html",
				"templates/pages/family.html",
			))
	return func(w http.ResponseWriter, r *http.Request) {
		nonce, err := extractCspNonceOnly(r)
		if err != nil {
			log.Printf("Could not extract csp nonce from ctx: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			renderAppError(w, r, http.StatusNotFound)
			return
		}

		family, err := s.GetUnclaimedFamily(r.Context(), id)
		if errors.Is(err, store.ErrFamilyNotFound) {
			renderAppError(w, r, http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("familyGet: failed to get family %d: %v", id, err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		props := struct {
			viewProps
			Family types.Family
		}{
			viewProps: viewProps{CspNonce: nonce},
			Family:    family,
		}

		if err := t.Execute(w, props); err != nil {
			log.Printf("Could not create family page: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
		}
	}
}
