package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/godfreygan/ai-script/backend/internal/conf"
	"github.com/godfreygan/ai-script/backend/internal/repo"
	"github.com/godfreygan/ai-script/backend/internal/server"
	"github.com/godfreygan/ai-script/backend/internal/service"
	"github.com/godfreygan/ai-script/backend/pkg/logger"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	loginAttemptsPrefix = "login_attempts"
	loginLockedPrefix   = "login_locked"
)

func main() {
	username := flag.String("username", "admin", "username to reset")
	password := flag.String("password", "", "new password")
	flag.Parse()

	if *username == "" {
		fail("username is required")
	}
	if *password == "" {
		fail("password is required")
	}
	if err := service.ValidatePassword(*password, *username); err != nil {
		fail("invalid password: %v", err)
	}

	cfg, err := conf.Load()
	if err != nil {
		fail("load config failed: %v", err)
	}

	log, err := logger.New(cfg.App.LogLevel, cfg.App.Env)
	if err != nil {
		fail("init logger failed: %v", err)
	}
	defer log.Sync()

	db, err := server.NewDB(cfg, log)
	if err != nil {
		fail("connect mysql failed: %v", err)
	}

	rdb := server.NewRedis(cfg)
	repos := repo.NewRepositories(db, rdb)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	user, err := repos.User.GetByUsername(ctx, *username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fail("user %q not found", *username)
		}
		fail("load user failed: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		fail("hash password failed: %v", err)
	}
	if err := repos.User.UpdatePassword(ctx, user.ID, string(hash)); err != nil {
		fail("update password failed: %v", err)
	}

	clearLoginState(ctx, rdb, *username)

	fmt.Printf("password reset succeeded for user %q\n", *username)
}

func clearLoginState(ctx context.Context, rdb *redis.Client, username string) {
	if rdb == nil {
		return
	}
	_ = rdb.Del(
		ctx,
		fmt.Sprintf("%s:%s", loginAttemptsPrefix, username),
		fmt.Sprintf("%s:%s", loginLockedPrefix, username),
	).Err()
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
