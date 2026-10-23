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
	"golang.org/x/term"
)

const usage = `usage: admin create <username>

Prompts for the new admin's password without echoing it. When stdin is not a
terminal, reads the password from the first line of stdin instead.`

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

	password, err := readPassword(stdin, stderr)
	if err != nil {
		return err
	}
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

func readPassword(stdin io.Reader, stderr io.Writer) (string, error) {
	f, ok := stdin.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		line, err := bufio.NewReader(stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("could not read password: %w", err)
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	fmt.Fprint(stderr, "Password: ")
	password, err := term.ReadPassword(int(f.Fd()))
	fmt.Fprintln(stderr)
	if err != nil {
		return "", fmt.Errorf("could not read password: %w", err)
	}

	fmt.Fprint(stderr, "Confirm password: ")
	confirmation, err := term.ReadPassword(int(f.Fd()))
	fmt.Fprintln(stderr)
	if err != nil {
		return "", fmt.Errorf("could not read password: %w", err)
	}

	if string(password) != string(confirmation) {
		return "", errors.New("passwords do not match")
	}

	return string(password), nil
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
