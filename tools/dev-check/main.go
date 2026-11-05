// dev-check only probes the existing fixed dev readiness and authentication gate.
// It creates no registration, identity, CA, client state or APRS connection.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/BI8SYN/rf-link-server-beacon/internal/signer"
	"net/http"
	"os"
	"time"
)

func main() {
	origin := "http://localhost:18105"
	client := http.Client{Timeout: 5 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	response, e := client.Get(origin + "/readyz")
	if e != nil {
		fmt.Fprintln(os.Stderr, "fixed dev is unreachable")
		os.Exit(1)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		fmt.Fprintln(os.Stderr, "fixed dev is not ready")
		os.Exit(1)
	}
	const invalid = "fmba1.95b976da-d670-44ec-bb72-b876ad05f134.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	c, e := signer.NewApplication(origin, invalid, true)
	if e != nil {
		fmt.Fprintln(os.Stderr, "cannot initialize smoke client")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, e = c.Poll(ctx, "95b976da-d670-44ec-bb72-b876ad05f134")
	var api *signer.APIError
	if !errors.As(e, &api) || api.Status != 401 || api.Code != "application_access_invalid" {
		fmt.Fprintln(os.Stderr, "fixed dev authentication gate did not match the client contract")
		os.Exit(1)
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"fixed_dev_ready": true, "client_auth_contract_verified": true, "valid_credential_used": false, "identity_created": false, "aprs_sent": false})
}
