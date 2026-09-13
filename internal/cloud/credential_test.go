package cloud

import (
	"os"
	"strings"
	"testing"
)

func TestCredentialsRoundTripPerServiceAndPerEstate(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	c, err := LoadCredentials()
	if err != nil {
		t.Fatalf("an absent credential file is somebody who has not signed in yet, not a fault: %v", err)
	}
	if _, ok := c.Account("https://a.example"); ok {
		t.Error("an empty store answered with an account")
	}

	c.SetAccount("https://a.example", AccountCredential{Token: "acct", AccountID: "acc_1", Label: "laptop"})
	c.SetEstate("payments", EstateCredential{
		Service: "https://a.example", Estate: "grp_1", Token: "estate-token",
	})
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}

	again, err := LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	a, ok := again.Account("https://a.example")
	if !ok || a.Token != "acct" || a.AccountID != "acc_1" {
		t.Errorf("account %+v %v", a, ok)
	}
	e, ok := again.Estate("payments")
	if !ok || e.Token != "estate-token" || e.Estate != "grp_1" {
		t.Errorf("estate %+v %v", e, ok)
	}
}

// THE ACCOUNT CREDENTIAL AND THE ESTATE CREDENTIAL ARE NOT THE SAME THING, and
// the store keeps them apart rather than trusting a caller to. The estate one
// is scoped to its estate and cannot provision another; handing it to the
// provisioning endpoint is refused there, and it would be refused for the
// wrong-looking reason if this side conflated them.
func TestAnEstateCredentialIsNotOfferedAsAnAccountCredential(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c, _ := LoadCredentials()
	c.SetEstate("payments", EstateCredential{Service: "https://a.example", Estate: "grp_1", Token: "estate-token"})
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	if a, ok := c.Account("https://a.example"); ok {
		t.Errorf("storing an estate credential produced an account credential: %+v", a)
	}
}

// Mode 600. This file holds every credential this control node has, and a
// config directory is not a secret store by default.
func TestTheCredentialFileIsNotReadableByAnybodyElse(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c, _ := LoadCredentials()
	c.SetAccount("https://a.example", AccountCredential{Token: "acct"})
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	p, err := CredentialsPath()
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("the credential file is mode %04o, want 0600", st.Mode().Perm())
	}
}

// A file that will not parse is not an empty store. Treating it as one would
// silently discard every credential on the machine at the moment somebody most
// needs them, and then write an empty file over the top.
func TestAnUnreadableCredentialFileIsReportedRatherThanTreatedAsEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	c, _ := LoadCredentials()
	c.SetAccount("https://a.example", AccountCredential{Token: "acct"})
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	p, _ := CredentialsPath()
	if err := os.WriteFile(p, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadCredentials()
	if err == nil {
		t.Fatal("an unreadable credential file was treated as no credentials at all")
	}
	if !strings.Contains(err.Error(), p) {
		t.Errorf("the error does not name the file to look at: %v", err)
	}
}

func TestForgettingAnAccountLeavesTheOthersAlone(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	c, _ := LoadCredentials()
	c.SetAccount("https://a.example", AccountCredential{Token: "a"})
	c.SetAccount("https://b.example", AccountCredential{Token: "b"})
	if !c.ForgetAccount("https://a.example") {
		t.Error("forgetting an account that was there reported nothing to forget")
	}
	if c.ForgetAccount("https://a.example") {
		t.Error("forgetting an account twice reported it was there the second time")
	}
	if _, ok := c.Account("https://b.example"); !ok {
		t.Error("forgetting one account forgot another")
	}
}
