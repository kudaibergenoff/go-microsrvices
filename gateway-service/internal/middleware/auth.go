package middleware

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gofiber/fiber/v2"
)

func KeycloakAuth(issuerURL string) fiber.Handler {
	provider, err := oidc.NewProvider(context.Background(), issuerURL)
	if err != nil {
		log.Fatalf("failed to create OIDC provider for %s: %v", issuerURL, err)
	}

	verifier := provider.Verifier(&oidc.Config{
		SkipClientIDCheck: true,
	})

	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authorization header required"})
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid authorization format, use: Bearer <token>"})
		}

		idToken, err := verifier.Verify(c.Context(), token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
		}

		// Parse all claims including roles
		var rawClaims json.RawMessage
		if err := idToken.Claims(&rawClaims); err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "failed to parse token claims"})
		}

		var claims struct {
			Sub               string `json:"sub"`
			Email             string `json:"email"`
			PreferredUsername string `json:"preferred_username"`
			RealmAccess       struct {
				Roles []string `json:"roles"`
			} `json:"realm_access"`
		}
		if err := json.Unmarshal(rawClaims, &claims); err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "failed to parse token claims"})
		}

		c.Locals("keycloak_sub", claims.Sub)
		c.Locals("email", claims.Email)
		c.Locals("roles", claims.RealmAccess.Roles)
		c.Request().Header.Set("X-User-Sub", claims.Sub)
		c.Request().Header.Set("X-User-Email", claims.Email)
		c.Request().Header.Set("X-User-Roles", strings.Join(claims.RealmAccess.Roles, ","))

		return c.Next()
	}
}

// RequireRole returns a middleware that checks if the user has the required role.
func RequireRole(role string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		roles, ok := c.Locals("roles").([]string)
		if !ok {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "access denied"})
		}

		for _, r := range roles {
			if r == role {
				return c.Next()
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "insufficient permissions, required role: " + role})
	}
}
