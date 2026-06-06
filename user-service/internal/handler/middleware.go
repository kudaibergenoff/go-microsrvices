package handler

import (
	"context"
	"log"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gofiber/fiber/v2"
)

func KeycloakAuthMiddleware(issuerURL string) fiber.Handler {
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
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid authorization format"})
		}

		idToken, err := verifier.Verify(c.Context(), token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
		}

		var claims struct {
			Sub   string `json:"sub"`
			Email string `json:"email"`
		}
		if err := idToken.Claims(&claims); err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "failed to parse token claims"})
		}

		c.Locals("keycloak_sub", claims.Sub)
		c.Locals("email", claims.Email)

		return c.Next()
	}
}
