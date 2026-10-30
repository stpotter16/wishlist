package handlers

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"strings"
	"time"

	"github.com/stpotter16/wishlist/internal/types"
)

// Mailer sends an email with plain-text and HTML bodies. Its errors must never
// include either body, which carry the giver's private link.
type Mailer interface {
	Send(ctx context.Context, to, subject, text, html string) error
}

var claimEmailTmpl = template.Must(template.ParseFS(templateFS, "templates/emails/claim.html"))

// claimEmail is what both bodies of the claim email show, so the plain-text
// and HTML versions can't drift apart.
type claimEmail struct {
	Subject   string
	Link      string
	HeadName  string
	IsNewLink bool
	Members   []claimEmailMember
}

type claimEmailMember struct {
	Name string
	// Label adds the member's gender, if any, such as "Child, 7, Girl".
	Label   string
	Needs   []types.WishlistItem
	Wishes  []types.WishlistItem
	Details []claimEmailField
}

type claimEmailField struct {
	Label string
	Value string
}

// sendClaimEmail emails a giver their private link and a copy of the
// household's wishlist. It reports whether the email went out. A failure
// never undoes the claim, so an admin can re-send the link later.
func sendClaimEmail(ctx context.Context, mailer Mailer, to, link string, household types.Household, isNewLink bool) bool {
	// Finish sending even if the browser disconnects, so the giver isn't left
	// without the email.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()

	email := newClaimEmail(link, household, isNewLink)
	html, err := claimEmailHTML(email)
	if err != nil {
		log.Printf("sendClaimEmail: failed to render the email for household %d: %v", household.ID, err)
		return false
	}
	if err := mailer.Send(ctx, to, email.Subject, claimEmailText(email), html); err != nil {
		log.Printf("sendClaimEmail: failed to email the link for household %d: %v", household.ID, err)
		return false
	}
	return true
}

func newClaimEmail(link string, household types.Household, isNewLink bool) claimEmail {
	subject := fmt.Sprintf("Your link for %s", household.HeadName())
	if isNewLink {
		subject = fmt.Sprintf("Your new link for %s", household.HeadName())
	}

	email := claimEmail{
		Subject:   subject,
		Link:      link,
		HeadName:  household.HeadName(),
		IsNewLink: isNewLink,
	}
	for _, m := range household.Members {
		label := m.Label
		if m.Gender != "" {
			label += ", " + m.Gender
		}

		var details []claimEmailField
		addField := func(label, value string) {
			if value != "" {
				details = append(details, claimEmailField{Label: label, Value: value})
			}
		}
		addField("Shirt size", m.ShirtSize)
		addField("Pants size", m.PantSize)
		addField("Shoe size", m.ShoeSize)
		addField("Brands or colors", m.PreferredBrandsColors)
		addField("Toiletry scents", m.PreferredScents)
		if m.PrefersUnscented {
			addField("Sensitive skin", "Prefers unscented products")
		}
		addField("Gift cards", m.PreferredGiftCards)
		addField("Additional info", m.AdditionalInfo)

		email.Members = append(email.Members, claimEmailMember{
			Name:    m.Name(),
			Label:   label,
			Needs:   m.Needs,
			Wishes:  m.Wishes,
			Details: details,
		})
	}
	return email
}

func claimEmailHTML(email claimEmail) (string, error) {
	var b bytes.Buffer
	if err := claimEmailTmpl.Execute(&b, email); err != nil {
		return "", err
	}
	return b.String(), nil
}

func claimEmailText(email claimEmail) string {
	var b strings.Builder

	if email.IsNewLink {
		fmt.Fprintf(&b, "Here is your new private link to %s's wishlists. Your old link no longer works.\n\n", email.HeadName)
	} else {
		fmt.Fprintf(&b, "Thank you for adopting %s!\n\nHere is your private link to the Blackbird Place Resident/Family's wishlists:\n\n", email.HeadName)
	}
	fmt.Fprintf(&b, "%s\n\n", email.Link)
	b.WriteString("Keep this link to yourself. Anyone who has it can see the Blackbird Place Resident/Family.\n\n")
	b.WriteString("Below is a copy of the wishlists as of this email. The drive's organizers may update them, so check your link for the latest version.\n\n")
	b.WriteString("Questions? Reply to this email to reach the organizers.\n")

	for _, m := range email.Members {
		b.WriteString("\n")
		fmt.Fprintf(&b, "== %s (%s) ==\n", m.Name, m.Label)

		writeItems(&b, "Needs", m.Needs)
		writeItems(&b, "Wishes", m.Wishes)

		for _, f := range m.Details {
			fmt.Fprintf(&b, "%s: %s\n", f.Label, f.Value)
		}
	}

	return b.String()
}

func writeItems(b *strings.Builder, heading string, items []types.WishlistItem) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n", heading)
	for _, item := range items {
		if item.Notes != "" {
			fmt.Fprintf(b, "  - %s (%s)\n", item.Name, item.Notes)
		} else {
			fmt.Fprintf(b, "  - %s\n", item.Name)
		}
	}
}
