package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// grpcMessage extracts clean error message from gRPC error.
func grpcMessage(err error) string {
	if st, ok := status.FromError(err); ok {
		return st.Message()
	}
	msg := err.Error()
	if idx := strings.LastIndex(msg, "desc = "); idx != -1 {
		return msg[idx+7:]
	}
	return msg
}

// grpcHTTPStatus maps gRPC status codes to HTTP status codes.
func grpcHTTPStatus(err error) int {
	st, ok := status.FromError(err)
	if !ok {
		return fiber.StatusInternalServerError
	}

	switch st.Code() {
	case codes.NotFound:
		return fiber.StatusNotFound
	case codes.AlreadyExists:
		return fiber.StatusConflict
	case codes.InvalidArgument:
		return fiber.StatusBadRequest
	case codes.Unauthenticated:
		return fiber.StatusUnauthorized
	case codes.PermissionDenied:
		return fiber.StatusForbidden
	case codes.Unavailable:
		return fiber.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		return fiber.StatusGatewayTimeout
	default:
		return fiber.StatusInternalServerError
	}
}

// grpcError returns a Fiber JSON error response with proper HTTP status from gRPC error.
func grpcError(c *fiber.Ctx, err error) error {
	return c.Status(grpcHTTPStatus(err)).JSON(fiber.Map{"error": grpcMessage(err)})
}
