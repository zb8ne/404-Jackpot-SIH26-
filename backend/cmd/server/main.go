// Command server is the REST backend for the credential registry demo.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"credreg/backend/internal/api"
	"credreg/backend/internal/auth"
	"credreg/backend/internal/chain"
	"credreg/backend/internal/store"
)

func main() {
	var (
		addr         = flag.String("addr", listenAddr(), "listen address")
		rpcURL       = flag.String("rpc", envOr("RPC_URL", "http://127.0.0.1:8545"), "Anvil JSON-RPC endpoint")
		dbPath       = flag.String("db", envOr("DB_PATH", "credentials.db"), "SQLite file")
		contractFlag = flag.String("contract", os.Getenv("CONTRACT_ADDRESS"), "registry address (defaults to contracts/deployment.txt)")
		publicWebURL = flag.String("public-web-url", envOr("PUBLIC_WEB_URL", "http://127.0.0.1:5173"), "public frontend URL embedded in credential QR codes")
		publicAPIURL = flag.String("public-api-url", envOr("PUBLIC_API_URL", "http://127.0.0.1:8088"), "public backend URL embedded in QR download links")
		deployFile   = flag.String("deployment", envOr("DEPLOYMENT_FILE", "../contracts/deployment.txt"), "file holding the deployed address")
	)
	flag.Parse()

	addrHex := *contractFlag
	if addrHex == "" {
		b, err := os.ReadFile(*deployFile)
		if err != nil {
			log.Fatalf("no -contract given and could not read %s: %v (run `make deploy` first)", *deployFile, err)
		}
		addrHex = strings.TrimSpace(string(b))
	}
	if !common.IsHexAddress(addrHex) {
		log.Fatalf("%q is not a contract address", addrHex)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := chain.Dial(ctx, *rpcURL, common.HexToAddress(addrHex))
	if err != nil {
		log.Fatalf("connecting to the chain: %v", err)
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("opening %s: %v", *dbPath, err)
	}
	defer st.Close()
	if err := validateDepartments(st); err != nil {
		log.Fatalf("department configuration: %v", err)
	}

	log.Printf("registry  %s", addrHex)
	log.Printf("rpc       %s", *rpcURL)
	log.Printf("db        %s", *dbPath)
	log.Printf("listening on %s", *addr)

	supabaseURL := os.Getenv("SUPABASE_URL")
	if supabaseURL == "" {
		log.Fatal("SUPABASE_URL is required")
	}

	verifier := auth.NewVerifier(supabaseURL)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.New(c, st, verifier, *publicWebURL, *publicAPIURL),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

func validateDepartments(st *store.Store) error {
	departments, err := st.Departments()
	if err != nil {
		return err
	}
	for _, d := range departments {
		if !d.Active {
			continue
		}
		chainDepartment, ok := chain.DepartmentBySlug(d.ID)
		if !ok {
			return fmt.Errorf("active department %q has no chain signer", d.ID)
		}
		if chainDepartment.DocType != d.DocType {
			return fmt.Errorf("department %q has document type %d in SQLite and %d in chain configuration", d.ID, d.DocType, chainDepartment.DocType)
		}
	}
	return nil
}

// listenAddr resolves where to listen. Platforms like Railway assign a port at
// run time and hand it over as PORT, so that wins when it is set; ADDR stays the
// explicit override for local runs.
func listenAddr() string {
	if port := os.Getenv("PORT"); port != "" {
		return ":" + port
	}
	return envOr("ADDR", ":8088")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
