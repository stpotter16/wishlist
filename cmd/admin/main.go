package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/stpotter16/wishlist/internal/auth"
	"github.com/stpotter16/wishlist/internal/store"
	"github.com/stpotter16/wishlist/internal/store/db"
	"github.com/stpotter16/wishlist/internal/store/sqlite"
)

const usage = `usage: admin create <username>

Reads the new admin's password from the first line of stdin.`

func run(
	ctx context.Context,
	args []string,
	getenv func(string) string,
	stdin io.Reader,
	stdout, stderr io.Writer,
) error {
	if len(args) != 3 || args[1] != "create" {
		return errors.New(usage)
	}
	username := strings.TrimSpace(args[2])
	if username == "" {
		return errors.New("username must not be empty")
	}

	dbPath := getenv("WISHLIST_DB_PATH")
	if dbPath == "" {
		return errors.New("WISHLIST_DB_PATH environment variable not set")
	}

	fmt.Fprint(stderr, "Password: ")
	password, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("could not read password: %w", err)
	}
	password = strings.TrimRight(password, "\r\n")
	if password == "" {
		return errors.New("password must not be empty")
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	database, err := db.New(dbPath)
	if err != nil {
		return err
	}

	s, err := sqlite.New(database)
	if err != nil {
		return err
	}

	if err := s.CreateAdmin(ctx, username, passwordHash); err != nil {
		if errors.Is(err, store.ErrAdminExists) {
			return fmt.Errorf("admin %q already exists", username)
		}
		return err
	}

	fmt.Fprintf(stdout, "Created admin %q\n", username)
	return nil
}

func main() {
	ctx := context.Background()
	if err := run(
		ctx,
		os.Args,
		os.Getenv,
		os.Stdin,
		os.Stdout,
		os.Stderr,
	); err != nil {
		log.Fatalf("%s", err)
	}
}
