package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"credreg/backend/internal/store"
)

func main() {
	db := flag.String("db", "credentials.db", "SQLite database path")
	id := flag.String("id", "", "stable citizen application ID")
	name := flag.String("name", "", "citizen display name")
	email := flag.String("email", "", "citizen email (required)")
	phone := flag.String("phone", "", "optional citizen phone")
	supabaseID := flag.String("supabase-user-id", "", "optional future Supabase identity")
	active := flag.Bool("active", true, "whether the citizen account is active")
	linkDocID := flag.String("link-doc-id", "", "optional existing document ID to link to this citizen")
	flag.Parse()
	if strings.TrimSpace(*id) == "" || strings.TrimSpace(*name) == "" || strings.TrimSpace(*email) == "" {
		log.Fatal("-id, -name, and -email are required")
	}
	s, err := store.Open(*db)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	account := store.CitizenAccount{ID: strings.TrimSpace(*id), DisplayName: strings.TrimSpace(*name), Email: strings.TrimSpace(*email), Active: *active}
	if value := strings.TrimSpace(*phone); value != "" {
		account.Phone = &value
	}
	if value := strings.TrimSpace(*supabaseID); value != "" {
		account.SupabaseUserID = &value
	}
	if err := s.UpsertCitizenAccount(account); err != nil {
		log.Fatal(err)
	}
	if value := strings.TrimSpace(*linkDocID); value != "" {
		count, err := s.LinkDocumentToCitizen(value, account.ID)
		if err != nil {
			log.Fatal(err)
		}
		if count == 0 {
			log.Fatalf("no document found with id %s", value)
		}
	}
	fmt.Printf("provisioned citizen %s (%s)\n", account.ID, account.DisplayName)
}
