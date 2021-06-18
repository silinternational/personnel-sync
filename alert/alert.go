package alert

import (
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
)

type Config struct {
	AWSRegion          string
	CharSet            string
	ReturnToAddr       string
	SubjectText        string
	RecipientEmails    []string
	AWSAccessKeyID     string
	AWSSecretAccessKey string
}

func SendEmail(config Config, body string) {
	charSet := config.CharSet

	subject := config.SubjectText
	subjContent := ses.Content{
		Charset: &charSet,
		Data:    &subject,
	}

	msgContent := ses.Content{
		Charset: &charSet,
		Data:    &body,
	}

	msgBody := ses.Body{
		Text: &msgContent,
	}

	emailMsg := ses.Message{}
	emailMsg.SetSubject(&subjContent)
	emailMsg.SetBody(&msgBody)

	// Only report the last email error
	lastError := ""
	badRecipients := []string{}

	// Send emails to one recipient at a time to avoid one bad email sabotaging it all
	for _, address := range config.RecipientEmails {
		err := sendAnEmail(emailMsg, address, config)
		if err != nil {
			lastError = err.Error()
			badRecipients = append(badRecipients, address)
		}
	}

	if lastError != "" {
		addresses := strings.Join(badRecipients, ", ")
		log.Printf("Error sending email from '%s' to '%s': %s",
			config.ReturnToAddr, addresses, lastError)
	}
}

func sendAnEmail(emailMsg ses.Message, recipient string, config Config) error {
	recipients := []*string{&recipient}

	input := &ses.SendEmailInput{
		Destination: &ses.Destination{
			ToAddresses: recipients,
		},
		Message: &emailMsg,
		Source:  aws.String(config.ReturnToAddr),
	}

	cfg := &aws.Config{Region: aws.String(config.AWSRegion)}
	if config.AWSAccessKeyID != "" && config.AWSSecretAccessKey != "" {
		cfg.Credentials = credentials.NewStaticCredentials(config.AWSAccessKeyID, config.AWSSecretAccessKey, "")
	}
	sess, err := session.NewSession(cfg)
	if err != nil {
		return fmt.Errorf("error creating AWS session: %s", err)
	}

	svc := ses.New(sess)
	result, err := svc.SendEmail(input)
	if err != nil {
		return fmt.Errorf("error sending email, result: %s, error: %s", result, err)
	}
	log.Printf("alert message sent to %s, message ID: %s", recipient, *result.MessageId)
	return nil
}
