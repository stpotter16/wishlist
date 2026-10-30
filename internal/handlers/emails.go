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

// giverTips are the drive organizers' instructions for givers, which both
// bodies of the claim email show above the wishlists. The wording and bold
// phrases follow the organizers' tip sheet. Update the drop-off date each
// season.
var giverTips = []giverTip{
	{
		Parts: []tipPart{{Text: "Plan to spend "}, {Text: "at least $50/family member on your list", Bold: true}},
		Note:  "*Feel free to team up with others at Good Shepherd to fulfill a wishlist and share the financial commitment",
	},
	{Parts: []tipPart{{Text: "Please WRAP gifts intended for the Blackbird Place resident", Bold: true}, {Text: "/mother of the family"}}},
	{Parts: []tipPart{{Text: "Please DO NOT WRAP gifts intended for children", Bold: true}, {Text: " (to provide mothers the opportunity to wrap their children's gifts)"}}},
	{Parts: []tipPart{{Text: "Please clearly label each gift with the name of its recipient", Bold: true}, {Text: " using the provided gift tag stickers in your kit."}}},
	{Parts: []tipPart{{Text: "Please sign the Christmas card", Bold: true}, {Text: " provided in your kit and drop it off along with your gifts."}}},
	{Parts: []tipPart{{Text: "If you are providing a gift card in addition to wishlist requests or in response to specific gift card requests, "}, {Text: "place gift cards in envelopes labeled with the amount", Bold: true}}},
	{Parts: []tipPart{{Text: "All gifts must be dropped off at Good Shepherd (890 Boulevard) by "}, {Text: "Sunday, 12/13", Bold: true}, {Text: " so we can ensure every family receives their gifts on time"}}},
}

// organizerEmail is where the tips send givers' questions.
const organizerEmail = "outreach@goodshepherdathens.com"

type giverTip struct {
	// Parts make up the tip's sentence, so the HTML can bold some phrases.
	Parts []tipPart
	// Note is an optional aside, shown under the tip.
	Note string
}

type tipPart struct {
	Text string
	Bold bool
}

// claimEmail is what both bodies of the claim email show, so the plain-text
// and HTML versions can't drift apart.
type claimEmail struct {
	Subject        string
	Link           string
	HeadName       string
	IsNewLink      bool
	Tips           []giverTip
	OrganizerEmail string
	Members        []claimEmailMember
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
		Subject:        subject,
		Link:           link,
		HeadName:       household.HeadName(),
		IsNewLink:      isNewLink,
		Tips:           giverTips,
		OrganizerEmail: organizerEmail,
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

	b.WriteString("\n== Blackbird Place Christmas Gift Drive Helpful Tips ==\n")
	for _, tip := range email.Tips {
		b.WriteString("- ")
		for _, part := range tip.Parts {
			b.WriteString(part.Text)
		}
		b.WriteString("\n")
		if tip.Note != "" {
			fmt.Fprintf(&b, "  %s\n", tip.Note)
		}
	}
	fmt.Fprintf(&b, "- Please reach out to us at %s if you have any questions.\n\n", email.OrganizerEmail)
	b.WriteString("Thank you for adopting a Blackbird Place resident and her family for Christmas!\n")

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
