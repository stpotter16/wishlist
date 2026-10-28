package handlers

import (
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/stpotter16/wishlist/internal/auth"
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

		households, err := s.ListHouseholds(r.Context())
		if err != nil {
			log.Printf("adminGet: failed to list households: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		props := struct {
			viewProps
			Households []types.HouseholdSummary
		}{
			viewProps:  viewProps{CspNonce: nonce, ActivePage: "dashboard"},
			Households: households,
		}

		if err := t.Execute(w, props); err != nil {
			log.Printf("Could not create admin page: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
		}
	}
}

// householdFormProps configures the shared new and edit household form.
type householdFormProps struct {
	viewProps
	Heading      string
	BackURL      string
	SubmitMethod string
	SubmitURL    string
	Household    types.Household
	// EmptyMember renders the head's section on a new household and the blank
	// section that "Add child" clones.
	EmptyMember types.HouseholdMember
}

// memberSection is what the member-fieldset partial renders. The head of the
// household's section, always first, has a last initial but no age, gender,
// or remove button.
type memberSection struct {
	Member types.HouseholdMember
	IsHead bool
}

func parseHouseholdFormTemplate() *template.Template {
	return template.Must(
		template.New("base.html").
			Funcs(template.FuncMap{
				"memberSection": func(member types.HouseholdMember, isHead bool) memberSection {
					return memberSection{Member: member, IsHead: isHead}
				},
				// itemNames puts one item per line, the format the form's
				// needs and wishes textareas use.
				"itemNames": func(items []types.WishlistItem) string {
					names := make([]string, 0, len(items))
					for _, item := range items {
						names = append(names, item.Name)
					}
					return strings.Join(names, "\n")
				},
			}).
			ParseFS(
				templateFS,
				"templates/layouts/base.html",
				"templates/layouts/app.html",
				"templates/partials/member-fieldset.html",
				"templates/pages/household-form.html",
			))
}

func adminNewHouseholdGet() http.HandlerFunc {
	t := parseHouseholdFormTemplate()
	return func(w http.ResponseWriter, r *http.Request) {
		nonce, err := extractCspNonceOnly(r)
		if err != nil {
			log.Printf("Could not extract csp nonce from ctx: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		props := householdFormProps{
			viewProps:    viewProps{CspNonce: nonce, ActivePage: "dashboard"},
			Heading:      "New household",
			BackURL:      "/admin",
			SubmitMethod: http.MethodPost,
			SubmitURL:    "/admin/households",
			Household:    types.Household{Kind: types.HouseholdKindFamily},
		}

		if err := t.Execute(w, props); err != nil {
			log.Printf("Could not create new household page: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
		}
	}
}

func adminEditHouseholdGet(s store.Store) http.HandlerFunc {
	t := parseHouseholdFormTemplate()
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

		household, err := s.GetHousehold(r.Context(), id)
		if errors.Is(err, store.ErrHouseholdNotFound) {
			renderAppError(w, r, http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("adminEditHouseholdGet: failed to get household %d: %v", id, err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		householdURL := "/admin/households/" + strconv.Itoa(id)
		props := householdFormProps{
			viewProps:    viewProps{CspNonce: nonce, ActivePage: "dashboard"},
			Heading:      "Edit Household " + strconv.Itoa(id),
			BackURL:      householdURL,
			SubmitMethod: http.MethodPut,
			SubmitURL:    householdURL,
			Household:    household,
		}

		if err := t.Execute(w, props); err != nil {
			log.Printf("Could not create edit household page: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
		}
	}
}

func adminHouseholdsPost(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		household, err := parse.ParseHouseholdPost(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		id, err := s.CreateHousehold(r.Context(), household)
		if err != nil {
			log.Printf("adminHouseholdsPost: failed to create household: %v", err)
			http.Error(w, "Server issue - try again later", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(struct {
			ID int `json:"id"`
		}{ID: id}); err != nil {
			log.Printf("adminHouseholdsPost: failed to write response: %v", err)
		}
	}
}

func adminHouseholdGet(s store.Store) http.HandlerFunc {
	t := template.Must(
		template.New("base.html").
			ParseFS(
				templateFS,
				"templates/layouts/base.html",
				"templates/layouts/app.html",
				"templates/partials/household-members.html",
				"templates/pages/admin-household.html",
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

		household, err := s.GetHousehold(r.Context(), id)
		if errors.Is(err, store.ErrHouseholdNotFound) {
			renderAppError(w, r, http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("adminHouseholdGet: failed to get household %d: %v", id, err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		var claim *types.Claim
		c, err := s.GetClaim(r.Context(), id)
		switch {
		case errors.Is(err, store.ErrClaimNotFound):
		case err != nil:
			log.Printf("adminHouseholdGet: failed to get claim for household %d: %v", id, err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		default:
			claim = &c
		}

		props := struct {
			viewProps
			Household types.Household
			Claim     *types.Claim
		}{
			viewProps: viewProps{CspNonce: nonce, ActivePage: "dashboard"},
			Household: household,
			Claim:     claim,
		}

		if err := t.Execute(w, props); err != nil {
			log.Printf("Could not create admin household page: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
		}
	}
}

func adminClaimDelete(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "This household has no claim.", http.StatusNotFound)
			return
		}

		err = s.DeleteClaim(r.Context(), id)
		if errors.Is(err, store.ErrClaimNotFound) {
			http.Error(w, "This household has no claim.", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("adminClaimDelete: failed to release claim on household %d: %v", id, err)
			http.Error(w, "Server issue - try again later", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func adminClaimTokenPost(s store.Store, mailer Mailer, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "This household has no claim.", http.StatusNotFound)
			return
		}

		token, tokenHash, err := auth.NewAccessToken()
		if err != nil {
			log.Printf("adminClaimTokenPost: failed to generate access token: %v", err)
			http.Error(w, "Server issue - try again later", http.StatusInternalServerError)
			return
		}

		err = s.ReplaceClaimToken(r.Context(), id, tokenHash)
		if errors.Is(err, store.ErrClaimNotFound) {
			http.Error(w, "This household has no claim.", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("adminClaimTokenPost: failed to replace token on household %d: %v", id, err)
			http.Error(w, "Server issue - try again later", http.StatusInternalServerError)
			return
		}

		// The new token is already saved, so the admin gets the link even if
		// the email fails and can send it themselves.
		emailSent := false
		claim, err := s.GetClaim(r.Context(), id)
		if err != nil {
			log.Printf("adminClaimTokenPost: failed to load claim on household %d for email: %v", id, err)
		}
		household, householdErr := s.GetHousehold(r.Context(), id)
		if householdErr != nil {
			log.Printf("adminClaimTokenPost: failed to load household %d for email: %v", id, householdErr)
		}
		if err == nil && householdErr == nil {
			emailSent = sendClaimEmail(r.Context(), mailer, claim.Giver.Email, baseURL+"/claims/"+token, household, true)
		}

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(struct {
			URL       string `json:"url"`
			EmailSent bool   `json:"emailSent"`
		}{URL: "/claims/" + token, EmailSent: emailSent}); err != nil {
			log.Printf("adminClaimTokenPost: failed to write response: %v", err)
		}
	}
}

func adminHouseholdPut(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "This household doesn't exist.", http.StatusNotFound)
			return
		}

		household, err := parse.ParseHouseholdPost(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err = s.UpdateHousehold(r.Context(), id, household)
		if errors.Is(err, store.ErrHouseholdNotFound) {
			http.Error(w, "This household doesn't exist.", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("adminHouseholdPut: failed to update household %d: %v", id, err)
			http.Error(w, "Server issue - try again later", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(struct {
			ID int `json:"id"`
		}{ID: id}); err != nil {
			log.Printf("adminHouseholdPut: failed to write response: %v", err)
		}
	}
}

func adminHouseholdDelete(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "This household doesn't exist.", http.StatusNotFound)
			return
		}

		err = s.DeleteHousehold(r.Context(), id)
		switch {
		case errors.Is(err, store.ErrHouseholdNotFound):
			http.Error(w, "This household doesn't exist.", http.StatusNotFound)
			return
		case errors.Is(err, store.ErrHouseholdAlreadyClaimed):
			http.Error(w, "Release this household's claim before deleting it.", http.StatusConflict)
			return
		case err != nil:
			log.Printf("adminHouseholdDelete: failed to delete household %d: %v", id, err)
			http.Error(w, "Server issue - try again later", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
