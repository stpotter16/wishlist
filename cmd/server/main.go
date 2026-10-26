package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/stpotter16/wishlist/internal/handlers"
	"github.com/stpotter16/wishlist/internal/handlers/authentication"
	"github.com/stpotter16/wishlist/internal/handlers/sessions"
	"github.com/stpotter16/wishlist/internal/store/db"
	"github.com/stpotter16/wishlist/internal/store/sqlite"
	"github.com/stpotter16/wishlist/internal/turnstile"
)

func run(
	ctx context.Context,
	args []string,
	getenv func(string) string,
	stdin io.Reader,
	stdout, stderr io.Writer,
) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	dbPath := getenv("WISHLIST_DB_PATH")
	if dbPath == "" {
		return errors.New("WISHLIST_DB_PATH environment variable not set")
	}

	log.Printf("Opening database in %v", dbPath)
	database, err := db.New(dbPath)
	if err != nil {
		return err
	}

	store, err := sqlite.New(database)
	if err != nil {
		return err
	}

	sessionManager, err := sessions.New(database, getenv)
	if err != nil {
		return err
	}

	authenticator := authentication.New(store)

	turnstileSiteKey := getenv("WISHLIST_TURNSTILE_SITE_KEY")
	if turnstileSiteKey == "" {
		return errors.New("WISHLIST_TURNSTILE_SITE_KEY environment variable not set")
	}
	turnstileSecretKey := getenv("WISHLIST_TURNSTILE_SECRET_KEY")
	if turnstileSecretKey == "" {
		return errors.New("WISHLIST_TURNSTILE_SECRET_KEY environment variable not set")
	}
	humanVerifier := turnstile.New(turnstileSecretKey, turnstile.SiteverifyURL)

	handler := handlers.NewServer(store, sessionManager, authenticator, turnstileSiteKey, humanVerifier, time.Now)
	port := getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v\n", err)
		}
	}()

	<-ctx.Done()
	log.Println("Received termination signal. Shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}
	return nil
}

func main() {
	ctx := context.Background()
	if err := run(
		ctx,
		os.Args,
		os.Getenv,
		nil,
		os.Stdout,
		os.Stderr,
	); err != nil {
		log.Fatalf("%s", err)
	}
}
