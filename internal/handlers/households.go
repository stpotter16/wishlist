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

func householdsGet(s store.Store) http.HandlerFunc {
	t := template.Must(
		template.New("base.html").
			ParseFS(
				templateFS,
				"templates/layouts/base.html",
				"templates/pages/households.html",
			))
	return func(w http.ResponseWriter, r *http.Request) {
		nonce, err := extractCspNonceOnly(r)
		if err != nil {
			log.Printf("Could not extract csp nonce from ctx: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		households, err := s.ListUnclaimedHouseholds(r.Context())
		if err != nil {
			log.Printf("householdsGet: failed to list unclaimed households: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		props := struct {
			viewProps
			Households []types.HouseholdSummary
		}{
			viewProps:  viewProps{CspNonce: nonce},
			Households: households,
		}

		if err := t.Execute(w, props); err != nil {
			log.Printf("Could not create households page: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
		}
	}
}

func householdGet(s store.Store, turnstileSiteKey string) http.HandlerFunc {
	t := template.Must(
		template.New("base.html").
			ParseFS(
				templateFS,
				"templates/layouts/base.html",
				"templates/partials/household-members.html",
				"templates/pages/household.html",
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

		household, err := s.GetUnclaimedHousehold(r.Context(), id)
		if errors.Is(err, store.ErrHouseholdNotFound) {
			renderAppError(w, r, http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("householdGet: failed to get household %d: %v", id, err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		props := struct {
			viewProps
			Household        types.Household
			TurnstileSiteKey string
		}{
			viewProps:        viewProps{CspNonce: nonce},
			Household:        household,
			TurnstileSiteKey: turnstileSiteKey,
		}

		if err := t.Execute(w, props); err != nil {
			log.Printf("Could not create household page: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
		}
	}
}
