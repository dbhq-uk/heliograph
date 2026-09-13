package cloud

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dbhq-uk/heliograph/internal/estate"
)

// The credential store, and why it is a file rather than an environment
// variable.
//
// `HELIOGRAPH_RELAY_TOKEN` works and stays working: it is the form somebody
// uses in CI, and an environment variable is the right place for a credential a
// pipeline injects. It is the wrong place for one a PERSON acquired, because
// the acquisition happens once in a browser and the variable has to be set in
// every shell afterwards - which in practice means it is pasted into a dotfile,
// which is the copy-paste this whole flow exists to remove.
//
// So: acquired by `login`, written here at mode 600, read at the moment of use,
// and never printed. `heliograph-io/heliograph-cloud#11` states the property
// this keeps - "the control token never touches the clipboard".
type Credentials struct {
	Version int `json:"version"`

	// Keyed by the service base URL, because one machine may talk to a hosted
	// service and a self-hosted one and they are not the same account.
	Accounts map[string]AccountCredential `json:"accounts,omitempty"`

	// Keyed by the LOCAL estate name, the same name `heliograph estates`
	// prints, because that is what somebody types.
	Estates map[string]EstateCredential `json:"estates,omitempty"`

	path string
}

// AccountCredential is what `login` collects: the credential for an ACCOUNT,
// which may provision estates.
type AccountCredential struct {
	Token     string `json:"token"`
	AccountID string `json:"account_id,omitempty"`
	Label     string `json:"label,omitempty"`
	Obtained  string `json:"obtained,omitempty"`
}

// EstateCredential is what provisioning hands back: scoped to one estate, and
// unable to provision another.
//
// IT IS A SEPARATE TYPE FROM AccountCredential ON PURPOSE. They are different
// credentials with different scopes, and the one thing this flow may not do is
// flatten them into one. A single map of strings would make that an
// autocomplete away.
type EstateCredential struct {
	Service  string `json:"service"`
	Estate   string `json:"estate"`
	Token    string `json:"token"`
	Obtained string `json:"obtained,omitempty"`
}

// CredentialsPath is where the file lives.
func CredentialsPath() (string, error) {
	dir, err := estate.ConfigHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

// LoadCredentials reads the store. An absent file is somebody who has not
// signed in yet, which is not a fault.
//
// A FILE THAT WILL NOT PARSE IS NOT AN EMPTY STORE. Treating it as one would
// discard every credential on the machine and then write an empty file over the
// top, which is unrecoverable: none of these can be read back from the service.
// The same argument the relay makes about its own state file.
func LoadCredentials() (*Credentials, error) {
	path, err := CredentialsPath()
	if err != nil {
		return nil, err
	}
	c := &Credentials{Version: 1, path: path,
		Accounts: map[string]AccountCredential{}, Estates: map[string]EstateCredential{}}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	var on Credentials
	if err := json.Unmarshal(b, &on); err != nil {
		return nil, fmt.Errorf("%s is not readable: %w.\n"+
			"  It holds every credential this control node has, and none of them can be\n"+
			"  read back from the service, so it is left alone rather than replaced.\n"+
			"  Move it aside deliberately and sign in again if it is genuinely lost", path, err)
	}
	if on.Accounts != nil {
		c.Accounts = on.Accounts
	}
	if on.Estates != nil {
		c.Estates = on.Estates
	}
	return c, nil
}

func (c *Credentials) Account(service string) (AccountCredential, bool) {
	a, ok := c.Accounts[normaliseService(service)]
	return a, ok
}

func (c *Credentials) SetAccount(service string, a AccountCredential) {
	if c.Accounts == nil {
		c.Accounts = map[string]AccountCredential{}
	}
	c.Accounts[normaliseService(service)] = a
}

func (c *Credentials) ForgetAccount(service string) bool {
	key := normaliseService(service)
	if _, ok := c.Accounts[key]; !ok {
		return false
	}
	delete(c.Accounts, key)
	return true
}

func (c *Credentials) Estate(name string) (EstateCredential, bool) {
	e, ok := c.Estates[name]
	return e, ok
}

func (c *Credentials) SetEstate(name string, e EstateCredential) {
	if c.Estates == nil {
		c.Estates = map[string]EstateCredential{}
	}
	c.Estates[name] = e
}

func (c *Credentials) ForgetEstate(name string) bool {
	if _, ok := c.Estates[name]; !ok {
		return false
	}
	delete(c.Estates, name)
	return true
}

// Save writes the store at mode 600.
//
// Write-then-rename, because a half-written credential file is a machine that
// cannot sign in and cannot say why. Named for this process, because two
// invocations sharing one temporary path is how a truncated file gets renamed
// into place - the same trap the relay's state file documents.
func (c *Credentials) Save() error {
	if c.path == "" {
		p, err := CredentialsPath()
		if err != nil {
			return err
		}
		c.path = p
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return err
	}
	c.Version = 1
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.%d.tmp", c.path, os.Getpid())
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, c.path); err != nil {
		return err
	}
	// Rename keeps the temporary file's mode, but a file that already existed
	// keeps ITS mode, so this is stated rather than assumed.
	return os.Chmod(c.path, 0o600)
}

// normaliseService makes `https://x/` and `https://x` the same account.
func normaliseService(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
