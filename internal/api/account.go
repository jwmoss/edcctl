package api

import (
	"context"
	"fmt"
	"strings"
)

// Account is the primary contact record on the portal.
type Account struct {
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Phone2    string `json:"phone_2,omitempty"`
	Email     string `json:"email,omitempty"`
	Address   string `json:"address,omitempty"`
}

// Account parses the account detail form on my_account.php. The page
// renders the saved values as form input values.
func (c *Client) Account(ctx context.Context) (*Account, error) {
	page, err := c.Get(ctx, "/my_account.php", nil)
	if err != nil {
		return nil, fmt.Errorf("load account page: %w", err)
	}
	doc, err := parseHTML(page)
	if err != nil {
		return nil, err
	}

	account := &Account{}
	form := doc.Find(`input[name="first_name"][value]`).First().ParentsFiltered("form").First()
	input := func(name string) string {
		return strings.TrimSpace(form.Find(`input[name="`+name+`"]`).First().AttrOr("value", ""))
	}
	account.FirstName = input("first_name")
	account.LastName = input("last_name")
	account.Phone = input("phone_1")
	account.Phone2 = input("phone_2")
	account.Email = input("email")
	if address := form.Find(`textarea[name="address"]`).First(); address.Length() > 0 {
		account.Address = strings.TrimSpace(address.Text())
	}
	return account, nil
}
