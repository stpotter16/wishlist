package handlers

import (
	"context"
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

// HumanVerifier checks the token a bot-defense widget gave the giver's
// browser. It returns an error only when it can't reach a verdict, so callers
// can tell an outage from a rejected token.
type HumanVerifier interface {
	Verify(ctx context.Context, token string) (bool, error)
}

func householdClaimPost(s store.Store, verifier HumanVerifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		householdID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "This household is no longer available.", http.StatusNotFound)
			return
		}

		giver, turnstileToken, err := parse.ParseClaimPost(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Fail closed: if Cloudflare is unreachable, claiming stops rather
		// than leaving the endpoint open to bots.
		isHuman, err := verifier.Verify(r.Context(), turnstileToken)
		if err != nil {
			log.Printf("householdClaimPost: failed to verify turnstile token: %v", err)
			http.Error(w, "We couldn't run the bot check right now. Please try again in a few minutes.", http.StatusServiceUnavailable)
			return
		}
		if !isHuman {
			http.Error(w, "We couldn't confirm you're not a bot. Please try again.", http.StatusForbidden)
			return
		}

		token, tokenHash, err := auth.NewAccessToken()
		if err != nil {
			log.Printf("householdClaimPost: failed to generate access token: %v", err)
			http.Error(w, "Server issue - try again later", http.StatusInternalServerError)
			return
		}

		err = s.CreateClaim(r.Context(), types.NewClaim{
			HouseholdID:     householdID,
			Giver:           giver,
			AccessTokenHash: tokenHash,
		})
		switch {
		case errors.Is(err, store.ErrHouseholdNotFound):
			http.Error(w, "This household is no longer available.", http.StatusNotFound)
			return
		case errors.Is(err, store.ErrHouseholdAlreadyClaimed):
			http.Error(w, "Another giver has already adopted this household.", http.StatusConflict)
			return
		case errors.Is(err, store.ErrEmailAlreadyClaimed):
			http.Error(w, "This email address has already adopted a household.", http.StatusConflict)
			return
		case err != nil:
			log.Printf("householdClaimPost: failed to claim household %d: %v", householdID, err)
			http.Error(w, "Server issue - try again later", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(struct {
			URL string `json:"url"`
		}{URL: "/claims/" + token}); err != nil {
			log.Printf("householdClaimPost: failed to write response: %v", err)
		}
	}
}

func claimGet(s store.Store) http.HandlerFunc {
	t := template.Must(
		template.New("base.html").
			ParseFS(
				templateFS,
				"templates/layouts/base.html",
				"templates/partials/household-members.html",
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

		household, err := s.GetClaimedHousehold(r.Context(), auth.HashAccessToken(r.PathValue("token")))
		if errors.Is(err, store.ErrClaimNotFound) || errors.Is(err, store.ErrHouseholdNotFound) {
			renderAppError(w, r, http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("claimGet: failed to get claimed household: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		props := struct {
			viewProps
			Household types.Household
		}{
			viewProps: viewProps{CspNonce: nonce},
			Household: household,
		}

		if err := t.Execute(w, props); err != nil {
			log.Printf("Could not create claim page: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
		}
	}
}
