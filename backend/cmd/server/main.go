// Command server is the REST backend for the credential registry demo.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"credreg/backend/internal/api"
	"credreg/backend/internal/chain"
	"credreg/backend/internal/store"
)

func main() {
	var (
		addr         = flag.String("addr", envOr("ADDR", ":8088"), "listen address")
		rpcURL       = flag.String("rpc", envOr("RPC_URL", "http://127.0.0.1:8545"), "Anvil JSON-RPC endpoint")
		dbPath       = flag.String("db", envOr("DB_PATH", "credentials.db"), "SQLite file")
		contractFlag = flag.String("contract", os.Getenv("CONTRACT_ADDRESS"), "registry address (defaults to contracts/deployment.txt)")
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

	log.Printf("registry  %s", addrHex)
	log.Printf("rpc       %s", *rpcURL)
	log.Printf("db        %s", *dbPath)
	log.Printf("listening on %s", *addr)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           api.New(c, st),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
