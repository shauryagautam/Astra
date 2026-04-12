// Package notification provides a unified abstraction for sending
// notifications across multiple channels (email, SMS, Slack, push, etc.).
//
// Usage:
//
//	type WelcomeNotification struct {
//	    User schema.User
//	}
//
//	func (n *WelcomeNotification) Via() []string { return []string{"mail"} }
//
//	func (n *WelcomeNotification) ToMail() *mail.Message {
//	    return mail.NewMessage().
//	        To(n.User.Email).
//	        Subject("Welcome to Astra!").
//	        Text("Hi " + n.User.Name + ", welcome!")
//	}
//
//	// Send:
//	notifier.Send(ctx, &WelcomeNotification{User: user})
package notification
