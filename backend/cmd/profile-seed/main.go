// Command profile-seed provisions one backend authorization profile for an
// existing Supabase user. It never authenticates as or creates that user.
package main

import (
	"flag"
	"log"
	"strings"

	"credreg/backend/internal/store"
)

func main() {
	dbPath := flag.String("db", "credentials.db", "SQLite database path")
	id := flag.String("id", "", "Supabase user ID (sub)")
	email := flag.String("email", "", "profile email")
	name := flag.String("name", "", "profile display name")
	role := flag.String("role", "", "CONTROLLER, ADMIN, or OFFICIAL")
	department := flag.String("department", "", "stable department ID; empty for CONTROLLER")
	inactive := flag.Bool("inactive", false, "create or update the profile as inactive")
	flag.Parse()

	profile := store.UserProfile{
		SupabaseUserID: strings.TrimSpace(*id),
		Email:          strings.TrimSpace(*email),
		DisplayName:    strings.TrimSpace(*name),
		Role:           strings.ToUpper(strings.TrimSpace(*role)),
		Active:         !*inactive,
	}
	if profile.SupabaseUserID == "" || profile.Email == "" || profile.Role == "" {
		log.Fatal("-id, -email, and -role are required")
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()
	if err := st.UpsertUserProfile(profile, strings.TrimSpace(*department)); err != nil {
		log.Fatalf("save profile: %v", err)
	}
	log.Printf("saved %s profile for %s", profile.Role, profile.SupabaseUserID)
}
