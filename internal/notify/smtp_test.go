package notify

import (
	"bufio"
	"encoding/base64"
	"io"
	"mime"
	"net"
	"net/mail"
	"strings"
	"sync"
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

// fakeSMTP 是按脚本应答的最小 SMTP 服务器，记录收到的命令与邮件内容。
type fakeSMTP struct {
	ln   net.Listener
	auth string // EHLO 声明的认证机制
	tls  bool   // 是否声明 STARTTLS
	mu   sync.Mutex
	cmds []string
	data string
	done chan struct{}
}

func startFakeSMTP(t *testing.T, auth string, starttls bool) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSMTP{ln: ln, auth: auth, tls: starttls, done: make(chan struct{})}
	t.Cleanup(func() { ln.Close() })
	go f.serve()
	return f
}

func (f *fakeSMTP) port() string { return strings.Split(f.ln.Addr().String(), ":")[1] }

func (f *fakeSMTP) serve() {
	defer close(f.done)
	conn, err := f.ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := func(s string) { io.WriteString(conn, s+"\r\n") }
	read := func() string {
		l, _ := r.ReadString('\n')
		l = strings.TrimRight(l, "\r\n")
		f.mu.Lock()
		f.cmds = append(f.cmds, l)
		f.mu.Unlock()
		return l
	}
	w("220 fake ESMTP")
	for {
		line := read()
		up := strings.ToUpper(line)
		switch {
		case line == "":
			return
		case strings.HasPrefix(up, "EHLO"):
			w("250-fake")
			if f.tls {
				w("250-STARTTLS")
			}
			w("250 AUTH " + f.auth)
		case strings.HasPrefix(up, "AUTH PLAIN"):
			w("235 ok")
		case strings.HasPrefix(up, "AUTH LOGIN"):
			w("334 " + base64.StdEncoding.EncodeToString([]byte("Username:")))
			read()
			w("334 " + base64.StdEncoding.EncodeToString([]byte("Password:")))
			read()
			w("235 ok")
		case strings.HasPrefix(up, "DATA"):
			w("354 go")
			var sb strings.Builder
			for {
				l, _ := r.ReadString('\n')
				if l == ".\r\n" || l == "" {
					break
				}
				sb.WriteString(l)
			}
			f.mu.Lock()
			f.data = sb.String()
			f.mu.Unlock()
			w("250 queued")
		case strings.HasPrefix(up, "QUIT"):
			w("221 bye")
			return
		default:
			w("250 ok")
		}
	}
}

func TestSMTPPlain(t *testing.T) {
	fixNow(t, fixedTime)
	f := startFakeSMTP(t, "PLAIN LOGIN", false)
	n := mustNew(t, "smtp", schema.Config{"host": "127.0.0.1", "port": f.port(), "security": "none",
		"username": "me@example.com", "password": "pw", "from": "测试 <me@example.com>", "to": "a@example.com, b@example.com"})
	if err := send(n); err != nil {
		t.Fatal(err)
	}
	<-f.done
	cmds := strings.Join(f.cmds, "\n")
	wantAuth := "AUTH PLAIN " + base64.StdEncoding.EncodeToString([]byte("\x00me@example.com\x00pw"))
	for _, want := range []string{wantAuth, "MAIL FROM:<me@example.com>", "RCPT TO:<a@example.com>", "RCPT TO:<b@example.com>", "QUIT"} {
		if !strings.Contains(cmds, want) {
			t.Errorf("缺少命令 %q\n%s", want, cmds)
		}
	}

	m, err := mail.ReadMessage(strings.NewReader(f.data))
	if err != nil {
		t.Fatal(err)
	}
	subj, err := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	if err != nil || subj != testMsg.Title {
		t.Errorf("Subject = %q (%v), 原始 %q", subj, err, m.Header.Get("Subject"))
	}
	if !strings.HasPrefix(m.Header.Get("Subject"), "=?UTF-8?b?") {
		t.Errorf("Subject 未按 RFC 2047 编码: %s", m.Header.Get("Subject"))
	}
	from, _ := m.Header.AddressList("From")
	to, _ := m.Header.AddressList("To")
	if len(from) != 1 || from[0].Name != "测试" || len(to) != 2 {
		t.Errorf("From/To 不符: %v %v", from, to)
	}
	if m.Header.Get("Content-Type") != "text/plain; charset=UTF-8" || m.Header.Get("Content-Transfer-Encoding") != "base64" {
		t.Errorf("头部不符: %v", m.Header)
	}
	raw, _ := io.ReadAll(m.Body)
	body, err := base64.StdEncoding.DecodeString(strings.NewReplacer("\r", "", "\n", "").Replace(string(raw)))
	if err != nil || string(body) != testMsg.Content {
		t.Errorf("正文 = %q (%v)", body, err)
	}
}

func TestSMTPLoginAuth(t *testing.T) {
	f := startFakeSMTP(t, "LOGIN", false)
	n := mustNew(t, "smtp", schema.Config{"host": "127.0.0.1", "port": f.port(), "security": "none",
		"username": "user", "password": "secret", "to": "a@example.com", "from": "me@example.com"})
	if err := send(n); err != nil {
		t.Fatal(err)
	}
	<-f.done
	cmds := strings.Join(f.cmds, "\n")
	for _, want := range []string{"AUTH LOGIN", base64.StdEncoding.EncodeToString([]byte("user")), base64.StdEncoding.EncodeToString([]byte("secret"))} {
		if !strings.Contains(cmds, want) {
			t.Errorf("缺少 %q\n%s", want, cmds)
		}
	}
}

func TestSMTPErrors(t *testing.T) {
	f := startFakeSMTP(t, "PLAIN", false)
	n := mustNew(t, "smtp", schema.Config{"host": "127.0.0.1", "port": f.port(), "security": "starttls",
		"username": "u", "from": "me@example.com", "to": "a@example.com"})
	if err := send(n); err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Errorf("err = %v", err)
	}
	n = mustNew(t, "smtp", schema.Config{"host": "127.0.0.1", "to": "a@example.com"})
	if err := send(n); err == nil || !strings.Contains(err.Error(), "发件人") {
		t.Errorf("err = %v", err)
	}
	n = mustNew(t, "smtp", schema.Config{"host": "127.0.0.1", "from": "a@example.com", "to": "not an address"})
	if err := send(n); err == nil || !strings.Contains(err.Error(), "收件人") {
		t.Errorf("err = %v", err)
	}
}
