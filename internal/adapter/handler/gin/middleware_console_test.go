package gin

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"ai-office-backend/internal/adapter/auth"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

const testBreakGlass = "break-glass-token"
const testSecret = "console-secret"

func newConsoleTestRouter(issuer port.TokenIssuer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ConsoleAuth(issuer, testBreakGlass))
	r.GET("/protected", RequireRole(domain.RoleOperator), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"role": ConsoleRoleFrom(c)})
	})
	return r
}

func doConsoleReq(r *gin.Engine, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestConsoleAuth_NoHeader_Unauthorized(t *testing.T) {
	issuer := auth.NewJWTIssuer(testSecret, time.Hour)
	r := newConsoleTestRouter(issuer)

	w := doConsoleReq(r, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestConsoleAuth_BreakGlass_Admin_Passes(t *testing.T) {
	issuer := auth.NewJWTIssuer(testSecret, time.Hour)
	r := newConsoleTestRouter(issuer)

	w := doConsoleReq(r, testBreakGlass)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestConsoleAuth_ValidJWT_Viewer_Forbidden(t *testing.T) {
	issuer := auth.NewJWTIssuer(testSecret, time.Hour)
	r := newConsoleTestRouter(issuer)

	tok, err := issuer.Issue(port.Claims{UserID: "u1", Username: "viewer1", Role: domain.RoleViewer})
	if err != nil {
		t.Fatal(err)
	}

	w := doConsoleReq(r, tok)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestConsoleAuth_ValidJWT_Operator_OK(t *testing.T) {
	issuer := auth.NewJWTIssuer(testSecret, time.Hour)
	r := newConsoleTestRouter(issuer)

	tok, err := issuer.Issue(port.Claims{UserID: "u2", Username: "operator1", Role: domain.RoleOperator})
	if err != nil {
		t.Fatal(err)
	}

	w := doConsoleReq(r, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestConsoleAuth_GarbageJWT_Unauthorized(t *testing.T) {
	issuer := auth.NewJWTIssuer(testSecret, time.Hour)
	r := newConsoleTestRouter(issuer)

	w := doConsoleReq(r, "garbage.not.a.jwt")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}
