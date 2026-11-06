// Synthetic HTTP authentication backend for isolated Broker routing tests only.
// This does not implement FMO certificate authentication and is never deployed.
package main

import (
	"encoding/json"
	"net/http"
	"os"
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "fmo" && os.Args[1] != "rflink") {
		os.Exit(2)
	}
	role := os.Args[1]
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		result := "deny"
		if r.Method == "POST" && json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body) == nil && body.Username == role && body.Password == "synthetic-test-only" {
			result = "allow"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"result": result, "is_superuser": false})
	})
	if http.ListenAndServe(":8080", mux) != nil {
		os.Exit(1)
	}
}
