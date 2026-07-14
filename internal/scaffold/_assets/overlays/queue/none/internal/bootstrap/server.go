package bootstrap

import (
	"log/slog"
	"time"

	"__GO_CRAFT_MODULE__/internal/adapter/jwt"
	"__GO_CRAFT_MODULE__/internal/adapter/notify"
	"__GO_CRAFT_MODULE__/internal/adapter/password"
	"__GO_CRAFT_MODULE__/internal/adapter/smtp"
	"__GO_CRAFT_MODULE__/internal/auth"
	"__GO_CRAFT_MODULE__/internal/config"
	appmail "__GO_CRAFT_MODULE__/internal/mail"
	"__GO_CRAFT_MODULE__/internal/queue"
	"__GO_CRAFT_MODULE__/internal/user"
	"gorm.io/gorm"
)

type HTTPServices struct {
	Users  *user.Service
	Auth   *auth.Service
	Tokens *jwt.Service
}

func WireHTTPServices(cfg *config.Config, db *gorm.DB, logger *slog.Logger, _ queue.Dispatcher) HTTPServices {
	repository := identityRepository(db)
	hasher := password.Bcrypt{}
	users := user.NewService(repository, hasher)
	tokens := jwt.NewService(
		cfg.Auth.JWTAccessSecret,
		cfg.Auth.JWTRefreshSecret,
		cfg.Auth.AccessTTLMinutes,
		cfg.Auth.RefreshTTLHours,
		cfg.Auth.Issuer,
		cfg.Auth.Audience,
	)

	var transport appmail.Transport

	smtpTransport, err := smtp.NewTransport(cfg.Mail)
	if err != nil {
		logger.Error("create SMTP transport failed", "error", err)
	} else {
		transport = smtpTransport
	}

	mailer := appmail.NewMailer(
		appmail.Address{Name: cfg.Mail.FromName, Address: cfg.Mail.FromAddress},
		transport,
		nil,
	)
	authService := auth.NewService(
		repository,
		repository,
		repository,
		tokens,
		hasher,
		notify.NewVerificationNotifier(mailer, logger),
		notify.NewWelcomeNotifier(mailer, logger),
		time.Duration(cfg.Auth.VerificationTTLHours)*time.Hour,
		cfg.Auth.BootstrapAdminEmail,
	)

	return HTTPServices{Users: users, Auth: authService, Tokens: tokens}
}
