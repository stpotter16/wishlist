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

// familyFormProps configures the shared new and edit family form.
type familyFormProps struct {
	viewProps
	Heading      string
	BackURL      string
	SubmitMethod string
	SubmitURL    string
	Family       types.Family
	// EmptyMember renders the blank section that "Add member" clones.
	EmptyMember types.FamilyMember
}

func parseFamilyFormTemplate() *template.Template {
	return template.Must(
		template.New("base.html").
			Funcs(template.FuncMap{
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
				"templates/pages/family-form.html",
			))
}

func adminNewFamilyGet() http.HandlerFunc {
	t := parseFamilyFormTemplate()
	return func(w http.ResponseWriter, r *http.Request) {
		nonce, err := extractCspNonceOnly(r)
		if err != nil {
			log.Printf("Could not extract csp nonce from ctx: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		props := familyFormProps{
			viewProps:    viewProps{CspNonce: nonce, ActivePage: "dashboard"},
			Heading:      "New family",
			BackURL:      "/admin",
			SubmitMethod: http.MethodPost,
			SubmitURL:    "/admin/families",
		}

		if err := t.Execute(w, props); err != nil {
			log.Printf("Could not create new family page: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
		}
	}
}

func adminEditFamilyGet(s store.Store) http.HandlerFunc {
	t := parseFamilyFormTemplate()
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

		family, err := s.GetFamily(r.Context(), id)
		if errors.Is(err, store.ErrFamilyNotFound) {
			renderAppError(w, r, http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("adminEditFamilyGet: failed to get family %d: %v", id, err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		familyURL := "/admin/families/" + strconv.Itoa(id)
		props := familyFormProps{
			viewProps:    viewProps{CspNonce: nonce, ActivePage: "dashboard"},
			Heading:      "Edit Family " + strconv.Itoa(id),
			BackURL:      familyURL,
			SubmitMethod: http.MethodPut,
			SubmitURL:    familyURL,
			Family:       family,
		}

		if err := t.Execute(w, props); err != nil {
			log.Printf("Could not create edit family page: %v", err)
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

func adminFamilyGet(s store.Store) http.HandlerFunc {
	t := template.Must(
		template.New("base.html").
			ParseFS(
				templateFS,
				"templates/layouts/base.html",
				"templates/layouts/app.html",
				"templates/partials/family-members.html",
				"templates/pages/admin-family.html",
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

		family, err := s.GetFamily(r.Context(), id)
		if errors.Is(err, store.ErrFamilyNotFound) {
			renderAppError(w, r, http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("adminFamilyGet: failed to get family %d: %v", id, err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		}

		var claim *types.Claim
		c, err := s.GetClaim(r.Context(), id)
		switch {
		case errors.Is(err, store.ErrClaimNotFound):
		case err != nil:
			log.Printf("adminFamilyGet: failed to get claim for family %d: %v", id, err)
			renderAppError(w, r, http.StatusInternalServerError)
			return
		default:
			claim = &c
		}

		props := struct {
			viewProps
			Family types.Family
			Claim  *types.Claim
		}{
			viewProps: viewProps{CspNonce: nonce, ActivePage: "dashboard"},
			Family:    family,
			Claim:     claim,
		}

		if err := t.Execute(w, props); err != nil {
			log.Printf("Could not create admin family page: %v", err)
			renderAppError(w, r, http.StatusInternalServerError)
		}
	}
}

func adminClaimDelete(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "This family has no claim.", http.StatusNotFound)
			return
		}

		err = s.DeleteClaim(r.Context(), id)
		if errors.Is(err, store.ErrClaimNotFound) {
			http.Error(w, "This family has no claim.", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("adminClaimDelete: failed to release claim on family %d: %v", id, err)
			http.Error(w, "Server issue - try again later", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func adminClaimTokenPost(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "This family has no claim.", http.StatusNotFound)
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
			http.Error(w, "This family has no claim.", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("adminClaimTokenPost: failed to replace token on family %d: %v", id, err)
			http.Error(w, "Server issue - try again later", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(struct {
			URL string `json:"url"`
		}{URL: "/claims/" + token}); err != nil {
			log.Printf("adminClaimTokenPost: failed to write response: %v", err)
		}
	}
}

func adminFamilyPut(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "This family doesn't exist.", http.StatusNotFound)
			return
		}

		family, err := parse.ParseFamilyPost(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err = s.UpdateFamily(r.Context(), id, family)
		if errors.Is(err, store.ErrFamilyNotFound) {
			http.Error(w, "This family doesn't exist.", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("adminFamilyPut: failed to update family %d: %v", id, err)
			http.Error(w, "Server issue - try again later", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(struct {
			ID int `json:"id"`
		}{ID: id}); err != nil {
			log.Printf("adminFamilyPut: failed to write response: %v", err)
		}
	}
}

func adminFamilyDelete(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, "This family doesn't exist.", http.StatusNotFound)
			return
		}

		err = s.DeleteFamily(r.Context(), id)
		switch {
		case errors.Is(err, store.ErrFamilyNotFound):
			http.Error(w, "This family doesn't exist.", http.StatusNotFound)
			return
		case errors.Is(err, store.ErrFamilyAlreadyClaimed):
			http.Error(w, "Release this family's claim before deleting it.", http.StatusConflict)
			return
		case err != nil:
			log.Printf("adminFamilyDelete: failed to delete family %d: %v", id, err)
			http.Error(w, "Server issue - try again later", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
