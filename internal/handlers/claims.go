package handlers

import (
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/stpotter16/wishlist/internal/auth"
	"github.com/stpotter16/wishlist/internal/parse"
	"github.com/stpotter16/wishlist/internal/store"
	"github.com/stpotter16/wishlist/internal/types"
)

func familyClaimPost(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		familyID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "This family is no longer available.", http.StatusNotFound)
			return
		}

		giver, err := parse.ParseClaimPost(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		token, tokenHash, err := auth.NewAccessToken()
		if err != nil {
			log.Printf("familyClaimPost: failed to generate access token: %v", err)
			http.Error(w, "Server issue - try again later", http.StatusInternalServerError)
			return
		}

		err = s.CreateClaim(r.Context(), types.NewClaim{
			FamilyID:        familyID,
			Giver:           giver,
			AccessTokenHash: tokenHash,
		})
		switch {
		case errors.Is(err, store.ErrFamilyNotFound):
			http.Error(w, "This family is no longer available.", http.StatusNotFound)
			return
		case errors.Is(err, store.ErrFamilyAlreadyClaimed):
			http.Error(w, "Another giver has already adopted this family.", http.StatusConflict)
			return
		case errors.Is(err, store.ErrEmailAlreadyClaimed):
			http.Error(w, "This email address has already adopted a family.", http.StatusConflict)
			return
		case err != nil:
			log.Printf("familyClaimPost: failed to claim family %d: %v", familyID, err)
			http.Error(w, "Server issue - try again later", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(struct {
			URL string `json:"url"`
		}{URL: "/claims/" + token}); err != nil {
			log.Printf("familyClaimPost: failed to write response: %v", err)
		}
	}
}

func claimGet(s store.Store) http.HandlerFunc {
	t := template.Must(
		template.New("base.html").
			ParseFS(
				templateFS,
				"templates/layouts/base.html",
				"templates/partials/family-members.html",
				"templates/pages/claim.html",
			))
	return func(w http.ResponseWriter, r *http.Request) {
		// The URL carries the giver's access token, so keep it out of caches
		// and out of the Referer header on any link the giver follows.
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")

		nonce, err := extractCspNonceOnly(r)
		if err != nil {
			log.Printf("Could not extract csp nonce from ctx: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		family, err := s.GetClaimedFamily(r.Context(), auth.HashAccessToken(r.PathValue("token")))
		if errors.Is(err, store.ErrClaimNotFound) || errors.Is(err, store.ErrFamilyNotFound) {
			renderAppError(w, r, http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("claimGet: failed to get claimed family: %v", err)
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
			log.Printf("Could not create claim page: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
		}
	}
}
