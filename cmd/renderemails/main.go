// Command renderemails renderiza las plantillas de mail localmente, con datos
// completos y con datos vacios, para poder revisar las dos ramas de cada
// {{ if }} antes de que un usuario reciba un mail roto. GoTrue las renderiza
// con html/template de Go, asi que esto reproduce la salida real.
package main

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
)

type templateData struct {
	ConfirmationURL string
	Email           string
	SiteURL         string
	Token           string
	Data            interface{}
}

func main() {
	const outDir = "tmp/emails"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic(err)
	}

	const confirmURL = "https://cdjehkbidqqsldaajbui.supabase.co/auth/v1/verify?token=abc123&type=invite&redirect_to=https://embolsadora.site/s/11b36b85-033d-4bb3-9e31-4c92161887c0/auth/callback"

	cases := map[string]templateData{
		"completo": {
			ConfirmationURL: confirmURL,
			Email:           "usuario@ejemplo.com",
			SiteURL:         "https://embolsadora.site",
			Token:           "123456",
			Data: map[string]string{
				"tenant_name":  "MRG SRL",
				"inviter_name": "Federico De Giovanni",
				"role_name":    "Operador",
			},
		},
		"vacio": {
			ConfirmationURL: confirmURL,
			Email:           "usuario@ejemplo.com",
			SiteURL:         "https://embolsadora.site",
			Token:           "123456",
			Data:            map[string]string{},
		},
		"nil": {
			ConfirmationURL: confirmURL,
			Email:           "usuario@ejemplo.com",
			SiteURL:         "https://embolsadora.site",
			Token:           "123456",
			Data:            nil,
		},
	}

	files, err := filepath.Glob("emails/*.html")
	if err != nil {
		panic(err)
	}
	if len(files) == 0 {
		panic("no se encontraron plantillas en emails/")
	}

	for _, f := range files {
		name := filepath.Base(f)
		tpl, err := template.ParseFiles(f)
		if err != nil {
			panic(fmt.Sprintf("%s: %v", name, err))
		}
		for caseName, data := range cases {
			out := filepath.Join(outDir, caseName+"-"+name)
			fh, err := os.Create(out)
			if err != nil {
				panic(err)
			}
			if err := tpl.Execute(fh, data); err != nil {
				fh.Close()
				panic(fmt.Sprintf("%s (caso %s): %v", name, caseName, err))
			}
			fh.Close()
			fmt.Println("escrito", out)
		}
	}
}
