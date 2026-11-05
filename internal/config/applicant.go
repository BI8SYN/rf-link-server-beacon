package config

import (
	"errors"
	"net/mail"
	"strings"
	"unicode/utf8"
)

type Applicant struct {
	Name              string `json:"name"`
	Email             string `json:"email"`
	AuthorizationNote string `json:"authorizationNote"`
}

func (a Applicant) Validate() error {
	for _, f := range []struct {
		value string
		limit int
	}{{a.Name, 128}, {a.Email, 254}, {a.AuthorizationNote, 1024}} {
		if f.value == "" || len(f.value) > f.limit || strings.TrimSpace(f.value) != f.value || !utf8.ValidString(f.value) || strings.ContainsRune(f.value, 0) {
			return errors.New("applicant name, email and authorization note are required within byte limits")
		}
	}
	parsed, e := mail.ParseAddress(a.Email)
	if e != nil || parsed.Address != a.Email || strings.ContainsAny(a.Email, "\r\n<>") {
		return errors.New("valid applicant email required")
	}
	return nil
}

const DefaultAuthorizationNote = "申请服务器发现广播签名用途，请管理员人工核验负责人、呼号资格和身份归属。"
