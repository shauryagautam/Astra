package providers

import (
	mail "github.com/shaurya/adonis/app/Mail"
	"github.com/shaurya/adonis/contracts"
)

// MailProvider registers the Mail manager and job handlers into the container.
// Mirrors AdonisJS's @adonisjs/mail provider.
type MailProvider struct {
	BaseProvider
}

// NewMailProvider creates a new MailProvider.
func NewMailProvider(app contracts.ApplicationContract) *MailProvider {
	return &MailProvider{
		BaseProvider: NewBaseProvider(app),
	}
}

// Register binds the Mail manager as a singleton.
func (p *MailProvider) Register() error {
	p.App.Singleton("Adonis/Core/Mail", func(c contracts.ContainerContract) (any, error) {
		env := c.Use("Env").(*EnvManager)
		queue, _ := c.Make("Queue")

		var q contracts.QueueContract
		if queue != nil {
			q = queue.(contracts.QueueContract)
		}

		defaultMailer := env.Get("MAIL_MAILER", "smtp")
		manager := mail.NewMailManager(defaultMailer, q)

		// Register SMTP driver
		smtpConfig := mail.SMTPConfig{
			Host:     env.Get("MAIL_HOST", "127.0.0.1"),
			Port:     env.GetInt("MAIL_PORT", 587),
			Username: env.Get("MAIL_USERNAME", ""),
			Password: env.Get("MAIL_PASSWORD", ""),
			From:     env.Get("MAIL_FROM_ADDRESS", "hello@example.com"),
		}
		manager.Register("smtp", mail.NewSMTPDriver(smtpConfig))

		return manager, nil
	})

	p.App.Alias("Mail", "Adonis/Core/Mail")

	return nil
}

// Boot registers the mail job handler in the registry.
func (p *MailProvider) Boot() error {
	registry := p.App.Use("JobRegistry").(contracts.JobRegistry)

	registry.Register("adonis:mail", func(data []byte) error {
		manager := p.App.Use("Mail").(contracts.MailerContract)
		return mail.HandleMailJob(data, manager)
	})

	return nil
}
