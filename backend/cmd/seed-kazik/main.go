package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"cashx/internal/offers"
	"cashx/internal/platform"
)

func main() {
	cfg, err := platform.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	pool, err := pgxpool.New(context.Background(), cfg.AdminDatabaseURL)
	if err != nil {
		log.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()
	var projID string
	err = pool.QueryRow(ctx, `SELECT id FROM projects WHERE slug='kazik'`).Scan(&projID)
	if err != nil {
		fmt.Println("project not found, creating...")
		err = pool.QueryRow(ctx, `INSERT INTO projects (id, slug, name, destination_url) VALUES (gen_random_uuid(), 'kazik', 'kazik main', $1) RETURNING id`, cfg.FrontendOrigin).Scan(&projID)
		if err != nil {
			log.Fatalf("insert project: %v", err)
		}
		fmt.Printf("created project %s\n", projID)
	} else {
		fmt.Printf("existing project %s\n", projID)
	}

	var offerID string
	err = pool.QueryRow(ctx, `SELECT id FROM offers WHERE project_id=$1 AND status='active' LIMIT 1`, projID).Scan(&offerID)
	if err != nil {
		fmt.Println("creating offer...")
		offersSvc := &offers.Service{Pool: pool, IntegrationEncryptionKey: cfg.IntegrationKeyEncryptionKey, WebOrigin: cfg.WebOrigin}
		card, err := offersSvc.Create(ctx, nil, projID, "kazik-deposits", nil, nil, &cfg.FrontendOrigin, "active", 4000)
		if err != nil {
			log.Fatalf("create offer: %v", err)
		}
		offerID = card.ID
		fmt.Printf("created offer %s\n", offerID)
	} else {
		fmt.Printf("existing offer %s\n", offerID)
	}

	offersSvc := &offers.Service{Pool: pool, IntegrationEncryptionKey: cfg.IntegrationKeyEncryptionKey, WebOrigin: cfg.WebOrigin}
	keys, err := offersSvc.ListKeys(ctx, projID)
	if err != nil {
		log.Fatalf("list keys: %v", err)
	}
	if len(keys) > 0 {
		fmt.Printf("existing key %s hint %s\n", keys[0].KeyID, keys[0].SecretHint)
		fmt.Println("To use for kazik, set KAZIK_CASHX_KEY_ID and KAZIK_CASHX_SECRET from previous creation — secret shown only once, so create new one")
	}
	pair, err := offersSvc.CreateKey(ctx, nil, projID)
	if err != nil {
		log.Fatalf("create key: %v", err)
	}
	fmt.Printf("NEW KEY_ID=%s\n", pair.KeyID)
	fmt.Printf("NEW SECRET=%s\n", pair.Secret)
	fmt.Printf("Set in kazik/back/.env:\nKAZIK_CASHX_KEY_ID=%s\nKAZIK_CASHX_SECRET=%s\nKAZIK_CASHX_CLICK_TOKEN_SECRET=%s\n", pair.KeyID, pair.Secret, cfg.ClickTokenSecret)
	os.WriteFile("/tmp/kazik_cashx_key.env", []byte(fmt.Sprintf("KAZIK_CASHX_KEY_ID=%s\nKAZIK_CASHX_SECRET=%s\nKAZIK_CASHX_CLICK_TOKEN_SECRET=%s\nKAZIK_CASHX_BASE_URL=%s\nKAZIK_CASHX_PROJECT_SLUG=kazik\nKAZIK_CASHX_SYNC=true\n", pair.KeyID, pair.Secret, cfg.ClickTokenSecret, cfg.APIOrigin)), 0644)
	fmt.Println("written to /tmp/kazik_cashx_key.env")
}
