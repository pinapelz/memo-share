package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net/http"
)

func authToken(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

func isAuthorized(r *http.Request) bool {
	cookie, err := r.Cookie("lcs_auth")
	if err != nil {
		return false
	}
	expected := authToken(authConfig.Password)
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(expected)) == 1
}

func withAuth(next http.Handler) http.Handler {
	if !authConfig.Enabled {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			handleLogin(w, r)
			return
		}
		if isAuthorized(r) {
			next.ServeHTTP(w, r)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if isAuthorized(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, loginPageHTML)
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		password := r.FormValue("password")

		passOK := subtle.ConstantTimeCompare([]byte(password), []byte(authConfig.Password)) == 1
		if !passOK {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "lcs_auth",
			Value:    authToken(authConfig.Password),
			Path:     "/",
			MaxAge:   60 * 60 * 24 * 30,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

const loginPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Login</title>
  <style>
    :root {
      color-scheme: light dark;
    }
    html, body {
      margin: 0;
      padding: 0;
      font-family: Inter, system-ui, -apple-system, sans-serif;
      min-height: 100vh;
      background: #11111b;
      color: #cdd6f4;
    }
    .wrap {
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 16px;
    }
    .card {
      width: 100%;
      max-width: 360px;
      background: #1e1e2e;
      border-radius: 0;
      padding: 20px;
      box-sizing: border-box;
      border: 1px solid #313244;
    }
    h1 {
      font-size: 1.2rem;
      margin: 0 0 16px;
      color: #cba6f7;
    }
    input[type="password"] {
      width: 100%;
      box-sizing: border-box;
      margin: 8px 0;
      border-radius: 0;
      border: 1px solid #45475a;
      padding: 10px;
      background: #181825;
      color: #cdd6f4;
    }
    label {
      display: flex;
      gap: 8px;
      align-items: center;
      font-size: 0.9rem;
      color: #bac2de;
      margin: 8px 0 14px;
    }
    button {
      width: 100%;
      border: 0;
      border-radius: 0;
      padding: 10px;
      font-weight: 600;
      cursor: pointer;
      background: #89b4fa;
      color: #11111b;
    }
    #error {
      color: #f38ba8;
      min-height: 1.2em;
      margin-top: 8px;
      font-size: 0.9rem;
    }
  </style>
</head>
<body>
  <div class="wrap">
    <form class="card" id="login-form">
      <h1>memo share login</h1>
      <input type="password" id="password" name="password" placeholder="Password" required />
      <label>
        <input type="checkbox" id="remember" />
        Remember password on this browser
      </label>
      <button type="submit">Login</button>
      <div id="error"></div>
    </form>
  </div>

  <script>
    const form = document.getElementById('login-form');
    const passwordInput = document.getElementById('password');
    const rememberInput = document.getElementById('remember');
    const errorEl = document.getElementById('error');

    const savedPassword = localStorage.getItem('lcs_auth_password');

    if (savedPassword) {
      passwordInput.value = savedPassword;
      rememberInput.checked = true;
    }

    async function submitLogin() {
      errorEl.textContent = '';
      const body = new URLSearchParams();
      body.set('password', passwordInput.value);

      const res = await fetch('/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded;charset=UTF-8'
        },
        body: body.toString()
      });

      if (res.ok) {
        if (rememberInput.checked) {
          localStorage.setItem('lcs_auth_password', passwordInput.value);
        } else {
          localStorage.removeItem('lcs_auth_password');
        }
        window.location.replace('/');
        return;
      }

      localStorage.removeItem('lcs_auth_password');
      errorEl.textContent = 'Invalid password';
    }

    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      try {
        await submitLogin();
      } catch (_) {
        errorEl.textContent = 'Login failed. Try again.';
      }
    });
  </script>
</body>
</html>`
