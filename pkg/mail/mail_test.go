package mail

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateRenderer(t *testing.T) {
	fs := fstest.MapFS{
		"welcome.html": &fstest.MapFile{
			Data: []byte("<h1>Welcome, {{.Name}}!</h1>"),
		},
	}

	renderer := NewTemplateRenderer(fs)

	t.Run("Valid Template", func(t *testing.T) {
		res, err := renderer.Render("welcome.html", map[string]string{"Name": "Astra"})
		require.NoError(t, err)
		assert.Equal(t, "<h1>Welcome, Astra!</h1>", res)
	})

	t.Run("Missing Template", func(t *testing.T) {
		_, err := renderer.Render("missing.html", nil)
		assert.Error(t, err)
	})
}

// MockMailer captures sent messages for test_util.
type MockMailer struct {
	SentMessages []*Message
}

func (m *MockMailer) Send(ctx context.Context, msg *Message) error {
	m.SentMessages = append(m.SentMessages, msg)
	return nil
}

func TestMockMailer(t *testing.T) {
	ctx := context.Background()
	mailer := &MockMailer{}

	msg := &Message{
		To:      []string{"test@example.com"},
		Subject: "Hello",
		Body:    "World",
	}

	err := mailer.Send(ctx, msg)
	require.NoError(t, err)
	require.Len(t, mailer.SentMessages, 1)
	assert.Equal(t, "Hello", mailer.SentMessages[0].Subject)
}

func TestMessageValidation(t *testing.T) {
	// Although SMTPMailer.Send has validation, Message itself might need it
	// But Message is just a struct. Let's test the logic in SMTPMailer indirectly if we can.
}
