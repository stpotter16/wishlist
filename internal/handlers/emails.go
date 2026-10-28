package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/stpotter16/wishlist/internal/types"
)

// Mailer sends a plain-text email. Its errors must never include the body,
// which carries the giver's private link.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// sendClaimEmail emails a giver their private link and a copy of the
// household's wishlist. It reports whether the email went out. A failure
// never undoes the claim, so an admin can re-send the link later.
func sendClaimEmail(ctx context.Context, mailer Mailer, to, link string, household types.Household, isNewLink bool) bool {
	// Finish sending even if the browser disconnects, so the giver isn't left
	// without the email.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()

	subject := fmt.Sprintf("Your link for Blackbird Place Resident/Family %d", household.ID)
	if isNewLink {
		subject = fmt.Sprintf("Your new link for Blackbird Place Resident/Family %d", household.ID)
	}
	if err := mailer.Send(ctx, to, subject, claimEmailBody(link, household, isNewLink)); err != nil {
		log.Printf("sendClaimEmail: failed to email the link for household %d: %v", household.ID, err)
		return false
	}
	return true
}

// claimEmailBody shows members by label, never by name, since an email can be
// forwarded or kept for years.
func claimEmailBody(link string, household types.Household, isNewLink bool) string {
	var b strings.Builder

	if isNewLink {
		fmt.Fprintf(&b, "Here is your new private link to Blackbird Place Resident/Family %d. Your old link no longer works.\n\n", household.ID)
	} else {
		fmt.Fprintf(&b, "Thank you for adopting Blackbird Place Resident/Family %d!\n\nHere is your private link to the Blackbird Place Resident/Family's wishlists:\n\n", household.ID)
	}
	fmt.Fprintf(&b, "%s\n\n", link)
	b.WriteString("Keep this link to yourself. Anyone who has it can see the Blackbird Place Resident/Family.\n\n")
	b.WriteString("Below is a copy of the wishlists as of this email. The drive's organizers may update them, so check your link for the latest version.\n\n")
	b.WriteString("Questions? Reply to this email to reach the organizers.\n")

	for _, m := range household.Members {
		b.WriteString("\n")
		heading := m.Label
		if m.Gender != "" {
			heading += ", " + m.Gender
		}
		fmt.Fprintf(&b, "== %s ==\n", heading)

		writeItems(&b, "Needs", m.Needs)
		writeItems(&b, "Wishes", m.Wishes)

		writeField(&b, "Shirt size", m.ShirtSize)
		writeField(&b, "Pant size", m.PantSize)
		writeField(&b, "Shoe size", m.ShoeSize)
		writeField(&b, "Brands or colors", m.PreferredBrandsColors)
		writeField(&b, "Toiletry scents", m.PreferredScents)
		if m.PrefersUnscented {
			writeField(&b, "Sensitive skin", "Prefers unscented products")
		}
		writeField(&b, "Gift cards", m.PreferredGiftCards)
		writeField(&b, "Additional info", m.AdditionalInfo)
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

func writeField(b *strings.Builder, label, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(b, "%s: %s\n", label, value)
}
