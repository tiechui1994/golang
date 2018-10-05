package logging

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// 通过给定的SMTP服务器发送电子邮件
// SMT引擎
type SMTPWriter struct {
	Username           string   `json:"username"`
	Password           string   `json:"password"`
	Host               string   `json:"host"`
	Subject            string   `json:"subject"`
	FromAddress        string   `json:"fromAddress"` // 发送者邮箱(可以是匿名的)
	RecipientAddresses []string `json:"sendTos"`     // 接受者邮箱
	Level              int      `json:"level"`
}

func newSMTPWriter() Logger {
	return &SMTPWriter{Level: LevelTrace}
}

/*
配置信息:
{
	"username":"example@gmail.com",
	"password:"password",
	"host":"smtp.gmail.com:465",
	"subject":"email title",
	"fromAddress":"from@example.com",
	"sendTos":["email1","email2"],
	"level":LevelError
}
*/
func (s *SMTPWriter) Init(jsonConfig string) error {
	return json.Unmarshal([]byte(jsonConfig), s)
}

// SMTP 认证
func (s *SMTPWriter) getSMTPAuth(host string) smtp.Auth {
	if len(strings.Trim(s.Username, " ")) == 0 && len(strings.Trim(s.Password, " ")) == 0 {
		return nil
	}
	return smtp.PlainAuth(
		"",
		s.Username,
		s.Password,
		host,
	)
}

// 发送邮件
func (s *SMTPWriter) sendMail(hostAddressWithPort string, auth smtp.Auth, fromAddress string, recipients []string, msgContent []byte) error {
	client, err := smtp.Dial(hostAddressWithPort)
	if err != nil {
		return err
	}

	host, _, _ := net.SplitHostPort(hostAddressWithPort)
	tlsConn := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	}
	if err = client.StartTLS(tlsConn); err != nil {
		return err
	}

	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return err
		}
	}

	if err = client.Mail(fromAddress); err != nil {
		return err
	}

	for _, rec := range recipients {
		if err = client.Rcpt(rec); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msgContent)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return client.Quit()
}

// 核心: 发送邮件
func (s *SMTPWriter) WriteMsg(when time.Time, msg string, level int) error {
	if level > s.Level {
		return nil
	}

	hp := strings.Split(s.Host, ":")

	// 获取身份认证信息
	auth := s.getSMTPAuth(hp[0])

	// 连接到服务器, 进行身份验证, 设置发件人和收件人, 并一步发送电子邮件。
	contentType := "Content-Type: text/plain" + "; charset=UTF-8"
	mailMsg := []byte("To: " + strings.Join(s.RecipientAddresses, ";") + "\r\nFrom: " + s.FromAddress + "<" + s.FromAddress +
		">\r\nSubject: " + s.Subject + "\r\n" + contentType + "\r\n\r\n" + fmt.Sprintf(".%s", when.Format("2006-01-02 15:04:05")) + msg)

	return s.sendMail(s.Host, auth, s.FromAddress, s.RecipientAddresses, mailMsg)
}

func (s *SMTPWriter) Flush() {
}

func (s *SMTPWriter) Destroy() {
}

func init() {
	Register(AdapterMail, newSMTPWriter)
}
