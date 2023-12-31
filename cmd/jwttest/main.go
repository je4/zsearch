package main

import (
	"github.com/golang-jwt/jwt/v5"
)

func main() {
	tokenstring := "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzM4NCJ9.eyJpc3MiOiJhdXRoLmhnay5maG53LmNoL2xvY2FsaG9zdCIsIm5iZiI6MTY5NjQzNzk3NSwiaWF0IjoxNjk2NDM3OTc1LCJleHAiOjE3OTY0Mzg4ODAsInVzZXJJZCI6IjUyMCIsImVtYWlsIjoianVlcmdlbi5lbmdlQHVuaWJhcy5jaCIsImZpcnN0TmFtZSI6IkrDvHJnZW4iLCJsYXN0TmFtZSI6IkVuZ2UiLCJob21lT3JnIjoidW5pYmFzLmNoIiwiZ3JvdXBzIjoiZ2xvYmFsL2FkbWluO2dsb2JhbC9kaWdtYTtnbG9iYWwvZ3Vlc3Q7Z2xvYmFsL3VzZXI7bG9jYWxob3N0L2FkbWluIn0.f5j8Po8iU9O7PawtEFy_85Dar23gs1Y-Aey5swODUvnsAxfJvSYVDYWCf2akNw6a"
	token, err := jwt.Parse(tokenstring, func(token *jwt.Token) (interface{}, error) {
		return []byte(":Xf/#|IKYrDsNi4]LN*o(W7;:"), nil
	})
	_ = token
	_ = err
}
