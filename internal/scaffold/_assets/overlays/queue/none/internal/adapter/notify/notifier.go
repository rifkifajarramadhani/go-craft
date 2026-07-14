package notify

import (
	"context"
	"log/slog"

	appmail "__GO_CRAFT_MODULE__/internal/mail"
	"__GO_CRAFT_MODULE__/internal/user"
)

type VerificationNotifier struct {
	mailer *appmail.Mailer
	logger *slog.Logger
}

func NewVerificationNotifier(mailer *appmail.Mailer, logger *slog.Logger) *VerificationNotifier {
	return &VerificationNotifier{mailer: mailer, logger: logger}
}

func (n *VerificationNotifier) NotifyVerification(ctx context.Context, account user.User, token string) error {
	err := n.mailer.Send(ctx, appmail.EmailVerification{
		Username: account.Username,
		Email:    account.Email,
		Token:    token,
	})
	if err != nil {
		n.logger.WarnContext(ctx, "send email verification failed", "user_id", account.ID, "error", err)
	}

	return err
}

type WelcomeNotifier struct {
	mailer *appmail.Mailer
	logger *slog.Logger
}

func NewWelcomeNotifier(mailer *appmail.Mailer, logger *slog.Logger) *WelcomeNotifier {
	return &WelcomeNotifier{mailer: mailer, logger: logger}
}

func (n *WelcomeNotifier) NotifyWelcome(ctx context.Context, account user.User) error {
	err := n.mailer.Send(ctx, appmail.Welcome{Username: account.Username, Email: account.Email})
	if err != nil {
		n.logger.WarnContext(ctx, "send welcome email failed", "user_id", account.ID, "error", err)
	}

	return err
}
